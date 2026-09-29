"use strict";

// Qt wasm アプリのページで Go エンジン一式を用意する。
// 既定は Worker（engine-worker.js）でエンジンを動かし、メインスレッドは
// 非同期呼び出しとFSミラーだけを持つ（UIが固まらない）。
// URL に ?async=0 を付けると従来のメインスレッド同期方式へフォールバックする。
(() => {
  const paths = window.utauttsAssetPaths;
  const ENGINE_BASE = paths.engineBaseURL;
  const DICT_PATH = "/dict";
  const OPEN_PROJECT_PATH = "/tmp/utautts-open-project.utautts";
  const ASYNC = new URLSearchParams(location.search).get("async") !== "0";
  window.utauttsAsyncMode = ASYNC;

  function waitFor(predicate, timeoutMS) {
    return new Promise((resolve, reject) => {
      const deadline = Date.now() + timeoutMS;
      const tick = () => {
        if (predicate()) return resolve();
        if (Date.now() > deadline) return reject(new Error("timeout"));
        setTimeout(tick, 20);
      };
      tick();
    });
  }

  function qtModule() {
    return window.utauttsQtModule || window.Module;
  }

  function setMirror(path, bytes) {
    const data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
    window.utauttsFs.mountFile(path, data);
  }

  // --- 同期モード（互換用フォールバック） ---

  async function initSync() {
    const virtualFs = installVirtualFs({ cwd: "/" });
    window.utauttsFs = virtualFs;
    const go = new Go();
    const bytes = await (await fetch(ENGINE_BASE + "utautts.wasm")).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance);
    await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 30000);

    const modelManifest = await (await fetch(ENGINE_BASE + "models/manifest.json")).json();
    for (const modelName of modelManifest.models || []) {
      virtualFs.mountFile(
        "/models/" + modelName,
        new Uint8Array(await (await fetch(ENGINE_BASE + "models/" + encodeURIComponent(modelName))).arrayBuffer())
      );
    }
    virtualFs.mountFile(
      "/renderer/utautts-world-phrase/renderer.json",
      new Uint8Array(await (await fetch(paths.renderer("utautts-world-phrase/renderer.json"))).arrayBuffer())
    );

    const openjtalk = await createUtauTTSOpenJTalk({
      locateFile: (file) => ENGINE_BASE + "openjtalk/" + file,
    });
    openjtalk.FS.mkdir(DICT_PATH);
    const dictBase = paths.dictBaseURL;
    const dictManifestURL = paths.dictManifestURL;
    const manifest = await (await fetch(dictManifestURL)).json();
    for (const name of manifest.files) {
      openjtalk.FS.writeFile(
        DICT_PATH + "/" + name,
        new Uint8Array(await (await fetch(dictBase + encodeURIComponent(name))).arrayBuffer())
      );
    }
    const jtalkBridge = createOpenJTalkBridge(openjtalk);
    jtalkBridge.init(DICT_PATH);
    globalThis.utauttsOpenJTalk = jtalkBridge;

    const world = await createUtauTTSWorld({
      locateFile: (file) => ENGINE_BASE + "world/" + file,
    });
    globalThis.utauttsWorld = createWorldBridge(world);
  }

  // --- 非同期モード（Worker） ---

  function initAsync() {
    window.utauttsFs = createVirtualFs();
    const workerURL = new URL(paths.app("engine-worker.js"));
    // Capture this page's exact deployment config instead of fetching a mutable
    // config again in the Worker after another deployment has gone live.
    workerURL.searchParams.set("config", JSON.stringify(globalThis.UtauTTSConfig || {}));
    const worker = new Worker(workerURL);
    let resolveReady;
    let rejectReady;
    const ready = new Promise((resolve, reject) => {
      resolveReady = resolve;
      rejectReady = reject;
    });
    const pendingCalls = new Set();
    let workerError = null;

    function complete(id, response) {
      pendingCalls.delete(id);
      const json = JSON.stringify(response);
      const module = qtModule();
      if (module && typeof module._utauttsCallCompleted === "function") {
        window.utauttsPendingResultId = id;
        window.utauttsPendingResultJson = json;
        module._utauttsCallCompleted();
      } else {
        window.utauttsPendingResults = window.utauttsPendingResults || [];
        window.utauttsPendingResults.push({ id, json });
      }
    }

    function failWorker(message) {
      workerError = String(message);
      rejectReady(new Error(workerError));
      for (const id of Array.from(pendingCalls))
        complete(id, { ok: false, error: workerError });
    }

    worker.onmessage = (event) => {
      const data = event.data || {};
      if (data.type === "ready") {
        resolveReady();
        return;
      }
      if (data.type === "error") {
        console.error("engine worker:", data.message);
        failWorker(data.message);
        return;
      }
      if (data.type === "status") {
        window.utauttsEngineStatus = data.text;
        return;
      }
      if (data.type === "callResult") {
        for (const path of data.removed || []) {
          if (window.utauttsFs.readFile(path)) window.utauttsFs._unlink(path);
          if (window.utauttsFileUrlCache) delete window.utauttsFileUrlCache[path];
        }
        for (const file of data.files || []) {
          setMirror(file.path, new Uint8Array(file.bytes));
          if (window.utauttsFileUrlCache) {
            delete window.utauttsFileUrlCache[file.path];
          }
        }
        complete(data.id, data.response);
      }
    };
    worker.onerror = (event) => {
      const message = (event && (event.message || event.filename)) || "engine worker error";
      console.error("engine worker error:", event);
      failWorker(message);
    };
    worker.onmessageerror = () => failWorker("engine worker message could not be decoded");

    window.utauttsCallAsync = function (method, requestJSON, id) {
      if (workerError) {
        complete(id, { ok: false, error: workerError });
        return;
      }
      pendingCalls.add(id);
      try {
        worker.postMessage({ type: "call", id, method, request: requestJSON });
      } catch (error) {
        complete(id, { ok: false, error: String(error) });
      }
    };
    window.utauttsMountFile = function (path, bytes) {
      const data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
      window.utauttsFs.mountFile(path, data);
      const copy = data.slice();
      worker.postMessage({ type: "mountFile", path, bytes: copy.buffer }, [copy.buffer]);
    };
    window.utauttsRemoveFile = function (path) {
      if (window.utauttsFs.readFile(path)) window.utauttsFs._unlink(path);
      if (window.utauttsFileUrlCache) delete window.utauttsFileUrlCache[path];
      worker.postMessage({ type: "removeFile", path });
    };
    window.utauttsFlushPendingResults = function () {
      const module = qtModule();
      if (!module || typeof module._utauttsCallCompleted !== "function") return;
      const pending = window.utauttsPendingResults || [];
      window.utauttsPendingResults = [];
      for (const item of pending) {
        if (module && typeof module._utauttsCallCompleted === "function") {
          window.utauttsPendingResultId = item.id;
          window.utauttsPendingResultJson = item.json;
          module._utauttsCallCompleted();
        }
      }
    };

    return ready;
  }

  // --- ファイル選択・メディア（両モード共通） ---

  window.utauttsPickVoiceDirectory = function () {
    return new Promise((resolve, reject) => {
      const input = document.createElement("input");
      input.type = "file";
      input.webkitdirectory = true;
      input.multiple = true;
      input.style.display = "none";
      document.body.appendChild(input);
      input.addEventListener("change", async () => {
        try {
          const files = Array.from(input.files || []);
          if (files.length === 0) {
            resolve(null);
            return;
          }
          let root = "";
          for (const file of files) {
            const relative = file.webkitRelativePath || file.name;
            if (root === "") {
              root = relative.split("/")[0];
            }
            window.utauttsMountFile("/voice/" + relative, new Uint8Array(await file.arrayBuffer()));
          }
          if (ASYNC) {
            if (typeof globalThis.utauttsRefreshMetadata === "function") {
              globalThis.utauttsRefreshMetadata();
            }
          } else {
            globalThis.utauttsWasm.call("reloadVoicebanks", "{}");
            if (typeof globalThis.utauttsRefreshMetadata === "function") {
              globalThis.utauttsRefreshMetadata();
            }
          }
          resolve(root);
        } catch (error) {
          reject(error);
        } finally {
          input.remove();
        }
      });
      input.click();
    });
  };

  // プロジェクトファイルを選ばせ、FSへ載せてから C++ へ通知する。
  window.utauttsPickProjectFile = function () {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".utautts,application/json";
    input.style.display = "none";
    document.body.appendChild(input);
    input.addEventListener("change", async () => {
      try {
        const file = (input.files || [])[0];
        if (!file) {
          return;
        }
        window.utauttsMountFile(OPEN_PROJECT_PATH, new Uint8Array(await file.arrayBuffer()));
        const module = qtModule();
        if (module && typeof module._utauttsProjectFilePicked === "function") {
          module._utauttsProjectFilePicked();
        }
      } catch (error) {
        console.error("utauttsPickProjectFile", error);
      } finally {
        input.remove();
      }
    });
    input.click();
  };

  // 音源ZIPを複数選ばせ、FSへ載せてから C++ へ通知する。
  window.utauttsPickVoicebankArchives = function () {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".zip";
    input.multiple = true;
    input.style.display = "none";
    document.body.appendChild(input);
    input.addEventListener("change", async () => {
      try {
        const files = Array.from(input.files || []);
        const paths = [];
        for (let index = 0; index < files.length; ++index) {
          const path = "/tmp/voicebank-" + index + ".zip";
          window.utauttsMountFile(path, new Uint8Array(await files[index].arrayBuffer()));
          paths.push(path);
        }
        window.utauttsVoicebankZipPaths = paths;
        const module = qtModule();
        if (module && typeof module._utauttsVoicebankArchivesPicked === "function") {
          module._utauttsVoicebankArchivesPicked();
        }
      } catch (error) {
        console.error("utauttsPickVoicebankArchives", error);
      } finally {
        input.remove();
      }
    });
    input.click();
  };

  window.utauttsMediaPlay = function (url, volume, muted) {
    try {
      const path = decodeURIComponent(new URL(url, location.href).pathname);
      const bytes = window.utauttsFs.readFile(path);
      if (!bytes) {
        console.warn("utauttsMediaPlay: audio not found in virtual fs:", path);
        return;
      }
      const blob = new Blob([bytes], { type: "audio/wav" });
      if (!window.utauttsAudio) {
        window.utauttsAudio = new Audio();
      }
      const audio = window.utauttsAudio;
      if (window.utauttsAudioUrl) {
        URL.revokeObjectURL(window.utauttsAudioUrl);
      }
      window.utauttsAudioUrl = URL.createObjectURL(blob);
      audio.src = window.utauttsAudioUrl;
      audio.volume = muted ? 0 : Math.max(0, Math.min(1, volume));
      audio.currentTime = 0;
      audio.play().catch(() => {});
    } catch (error) {
      console.error("utauttsMediaPlay", error);
    }
  };

  window.utauttsMediaPause = function () {
    if (window.utauttsAudio) {
      window.utauttsAudio.pause();
    }
  };

  window.utauttsMediaStop = function () {
    if (window.utauttsAudio) {
      window.utauttsAudio.pause();
      window.utauttsAudio.currentTime = 0;
    }
  };

  window.utauttsMediaSeek = function (milliseconds) {
    if (window.utauttsAudio) {
      window.utauttsAudio.currentTime = Math.max(0, milliseconds / 1000);
    }
  };

  window.utauttsMediaPosition = function () {
    return window.utauttsAudio ? Math.round(window.utauttsAudio.currentTime * 1000) : 0;
  };

  window.utauttsMediaDuration = function () {
    const audio = window.utauttsAudio;
    if (audio && isFinite(audio.duration)) {
      return Math.round(audio.duration * 1000);
    }
    return 0;
  };

  window.utauttsMediaEnded = function () {
    return !!(window.utauttsAudio && window.utauttsAudio.ended);
  };

  window.utauttsFileUrl = function (path) {
    try {
      if (!window.utauttsFileUrlCache) {
        window.utauttsFileUrlCache = {};
      }
      if (window.utauttsFileUrlCache[path]) {
        return window.utauttsFileUrlCache[path];
      }
      const bytes = window.utauttsFs.readFile(path);
      if (!bytes) {
        return "";
      }
      const extension = String(path).split(".").pop().toLowerCase();
      const mime = extension === "bmp" ? "image/bmp"
        : (extension === "jpg" || extension === "jpeg") ? "image/jpeg"
          : extension === "gif" ? "image/gif" : "image/png";
      let binary = "";
      const chunk = 0x8000;
      for (let index = 0; index < bytes.length; index += chunk) {
        binary += String.fromCharCode.apply(null, bytes.subarray(index, index + chunk));
      }
      const url = "data:" + mime + ";base64," + btoa(binary);
      window.utauttsFileUrlCache[path] = url;
      return url;
    } catch (error) {
      console.error("utauttsFileUrl", error);
      return "";
    }
  };

  if (ASYNC) {
    window.utauttsMountFile = function (path, bytes) {
      setMirror(path, bytes);
    };
    window.utauttsEngineReady = initAsync();
  } else {
    window.utauttsRemoveFile = function (path) {
      if (window.utauttsFs.readFile(path)) window.utauttsFs._unlink(path);
      if (window.utauttsFileUrlCache) delete window.utauttsFileUrlCache[path];
    };
    window.utauttsMountFile = function (path, bytes) {
      setMirror(path, bytes);
    };
    window.utauttsEngineReady = initSync();
  }
})();
