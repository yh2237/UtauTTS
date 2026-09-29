"use strict";

// Qt wasm アプリのページで Go エンジン一式（fsシム・Open JTalk・WORLD・モデル）を
// メインスレッドに読み込み、globalThis.utauttsWasm.call を C++ から同期的に使えるようにする。
(() => {
  const ENGINE_BASE = "/web/dist/";
  const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
  const DICT_PATH = "/dict";
  const OPEN_PROJECT_PATH = "/tmp/utautts-open-project.utautts";

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

  async function loadEngine() {
    const virtualFs = installVirtualFs({ cwd: "/" });
    window.utauttsFs = virtualFs;
    const go = new Go();
    const bytes = await (await fetch(ENGINE_BASE + "utautts.wasm")).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance);
    await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 30000);

    virtualFs.mountFile(
      MODEL_PATH,
      new Uint8Array(await (await fetch(ENGINE_BASE + "models/frame-intonation-tcn-v9.1-t.json")).arrayBuffer())
    );

    virtualFs.mountFile(
      "/renderer/utautts-world-phrase/renderer.json",
      new Uint8Array(await (await fetch("/renderer/utautts-world-phrase/renderer.json")).arrayBuffer())
    );

    const openjtalk = await createUtauTTSOpenJTalk({
      locateFile: (file) => ENGINE_BASE + "openjtalk/" + file,
    });
    openjtalk.FS.mkdir(DICT_PATH);
    const manifest = await (await fetch(ENGINE_BASE + "openjtalk/dict-manifest.json")).json();
    for (const name of manifest.files) {
      openjtalk.FS.writeFile(
        DICT_PATH + "/" + name,
        new Uint8Array(await (await fetch(ENGINE_BASE + "openjtalk/dict/" + encodeURIComponent(name))).arrayBuffer())
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

  function qtModule() {
    return window.utauttsQtModule || window.Module;
  }

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
            window.utauttsFs.mountFile("/voice/" + relative, new Uint8Array(await file.arrayBuffer()));
          }
          globalThis.utauttsWasm.call("reloadVoicebanks", "{}");
          if (typeof globalThis.utauttsRefreshMetadata === "function") {
            globalThis.utauttsRefreshMetadata();
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

  // プロジェクトファイルを選ばせ、Go エンジンの仮想FSへ載せてから C++ へ通知する。
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
        window.utauttsFs.mountFile(OPEN_PROJECT_PATH, new Uint8Array(await file.arrayBuffer()));
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

  window.utauttsEngineReady = loadEngine();
})();
