"use strict";

(() => {
  const scriptURL = document.currentScript.src;
  const config = globalThis.UtauTTSConfig || {};
  const paths = createUtauTTSAssetPaths(scriptURL, config);
  window.utauttsAssetPaths = paths;

  // 自動保存はブラウザの保存領域に置く。容量や設定で使えないこともある。
  const AUTOSAVE_KEY = "utautts.autosave.v1";
  // engine-loader.jsのOPEN_PROJECT_PATHと同じ場所に置く。
  const OPEN_PROJECT_PATH = "/tmp/utautts-open-project.utautts";
  let unsaved = false;
  let autosaveFailed = false;

  function loadScript(url) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = url;
      script.onload = resolve;
      script.onerror = () => reject(new Error("script load failed: " + url));
      document.head.appendChild(script);
    });
  }

  function showNotice(text, actions) {
    const notice = document.getElementById("notice");
    if (!notice) return;
    notice.textContent = "";
    for (const line of Array.isArray(text) ? text : [text]) {
      const message = document.createElement("div");
      message.textContent = line;
      notice.appendChild(message);
    }
    if (actions && actions.length) {
      const row = document.createElement("div");
      row.className = "actions";
      for (const item of actions) {
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = item.label;
        if (item.primary) button.className = "primary";
        button.addEventListener("click", item.action);
        row.appendChild(button);
      }
      notice.appendChild(row);
    }
    notice.classList.remove("hidden");
  }

  function hideNotice() {
    const notice = document.getElementById("notice");
    if (notice) notice.classList.add("hidden");
  }

  // 辞書とWORLDの読み込みはQtの起動後に始まる。画面が出たあとも、
  // 状態が変わる間だけ小さく出す。2秒変わらなければ終わったとみなす。
  let uiLoaded = false;
  let engineStatus = "";
  let engineStatusStrip = null;
  let engineStatusTimer = null;

  function showEngineStatus(text) {
    if (!engineStatusStrip) {
      engineStatusStrip = document.createElement("div");
      engineStatusStrip.style.cssText = "position:absolute;left:50%;"
        + "bottom:calc(12px + env(safe-area-inset-bottom));transform:translateX(-50%);"
        + "max-width:calc(100% - 32px);padding:6px 12px;border-radius:6px;"
        + "background:rgba(32,33,36,.86);color:#fff;font-size:12px;z-index:15;"
        + "pointer-events:none;";
      document.body.appendChild(engineStatusStrip);
    }
    engineStatusStrip.textContent = text;
    engineStatusStrip.style.display = "block";
    clearTimeout(engineStatusTimer);
    engineStatusTimer = setTimeout(() => {
      engineStatusStrip.style.display = "none";
    }, 2000);
  }

  // エンジンの状態は、読み込み画面では#statusに、画面が出たあとは下部に出す。
  Object.defineProperty(window, "utauttsEngineStatus", {
    configurable: true,
    get: () => engineStatus,
    set: value => {
      engineStatus = String(value || "");
      if (!engineStatus) return;
      if (uiLoaded) {
        showEngineStatus(engineStatus);
        return;
      }
      const status = document.getElementById("status");
      if (status) status.textContent = engineStatus;
    },
  });

  function readAutosave() {
    try {
      return window.localStorage.getItem(AUTOSAVE_KEY) || "";
    } catch (error) {
      return "";
    }
  }

  function clearAutosave() {
    try {
      window.localStorage.removeItem(AUTOSAVE_KEY);
    } catch (error) {
      // 保存領域が使えなければ消すものもない。
    }
  }

  // 容量超過や保存の禁止で書けなくなることがある。黙って諦めず、書き出しを促す。
  function writeAutosave(json) {
    if (autosaveFailed) return;
    try {
      window.localStorage.setItem(AUTOSAVE_KEY, json);
    } catch (error) {
      autosaveFailed = true;
      showNotice("作業内容を自動保存できませんでした。保存容量が不足しているか、ブラウザが保存を禁止しています。プロジェクトを書き出してください。",
        [{ label: "閉じる", action: hideNotice }]);
    }
  }

  function restoreAutosave(json) {
    const module = window.utauttsQtModule || window.Module;
    if (!module || typeof module._utauttsProjectFilePicked !== "function"
        || typeof window.utauttsMountFile !== "function") {
      return;
    }
    hideNotice();
    window.utauttsMountFile(OPEN_PROJECT_PATH, new TextEncoder().encode(json));
    module._utauttsProjectFilePicked();
  }

  function offerRestore() {
    const saved = readAutosave();
    if (!saved) return;
    showNotice("前回の作業内容が残っています。復元しますか？", [
      { label: "復元", primary: true, action: () => restoreAutosave(saved) },
      { label: "破棄", action: () => { clearAutosave(); hideNotice(); } },
    ]);
  }

  // C++側から未保存の状態と内容を受け取る。
  window.utauttsSetUnsaved = value => {
    unsaved = !!value;
  };
  window.utauttsAutosave = json => {
    if (typeof json === "string" && json.length) writeAutosave(json);
  };
  window.utauttsClearAutosave = clearAutosave;

  window.addEventListener("beforeunload", event => {
    if (!unsaved) return;
    event.preventDefault();
    event.returnValue = "";
  });

  window.addEventListener("wheel", event => {
    if (event.ctrlKey) event.preventDefault();
  }, { passive: false, capture: true });
  window.addEventListener("touchmove", event => event.preventDefault(),
    { passive: false, capture: true });

  // 動かないブラウザでは白い画面のまま待たせず、理由を出す。
  function unsupportedReason() {
    if (typeof WebAssembly !== "object" || typeof WebAssembly.instantiate !== "function"
        || typeof WebAssembly.compile !== "function")
      return "このブラウザはWebAssemblyに対応していません。";
    if (typeof Worker !== "function")
      return "このブラウザはWeb Workerに対応していません。";
    if (typeof fetch !== "function" || typeof Promise !== "function")
      return "このブラウザはFetch APIに対応していません。";
    if (typeof TextDecoder !== "function" || typeof TextEncoder !== "function"
        || typeof Blob !== "function" || typeof URL.createObjectURL !== "function")
      return "このブラウザは必要なAPIに対応していません。";
    return "";
  }

  // 読み込みの進み具合を出す。長さが分からなければ今までどおり待たせる。
  async function loadUiModule(onProgress) {
    const response = await fetch(paths.asset("utautts.wasm"));
    if (!response.ok) throw new Error("UI WASM HTTP " + response.status);
    const total = Number(response.headers.get("Content-Length")) || 0;
    if (!response.body || total <= 0) {
      if (typeof WebAssembly.compileStreaming === "function"
          && response.headers.get("Content-Type")?.split(";")[0].trim() === "application/wasm")
        return WebAssembly.compileStreaming(response);
      return WebAssembly.compile(await response.arrayBuffer());
    }
    const reader = response.body.getReader();
    const chunks = [];
    let received = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      received += value.length;
      onProgress(Math.min(1, received / total));
    }
    return WebAssembly.compile(await new Blob(chunks).arrayBuffer());
  }

  async function init() {
    const loading = document.getElementById("loading");
    const screen = document.getElementById("screen");
    const status = document.getElementById("status");
    const reason = unsupportedReason();
    if (reason) {
      status.textContent = reason + "最新のChrome・Edge・Firefox・Safariで開いてください。";
      const spinner = document.querySelector("#loading .spinner");
      if (spinner) spinner.style.display = "none";
      return;
    }
    const setStatus = text => {
      status.textContent = text;
    };
    let timeout;
    try {
      const uiModule = loadUiModule(ratio => {
        setStatus("読み込み中… " + Math.floor(ratio * 100) + "%");
      });
      const engineAndScripts = (async () => {
        // WorkerモードではUI側にFSミラーだけを置く。
        await loadScript(paths.engine("fs-shim.js"));
        if (new URLSearchParams(location.search).get("async") === "0") {
          for (const file of ["wasm_exec.js", "openjtalk-bridge.js", "world-bridge.js",
            "openjtalk/utautts-openjtalk.js", "world/utautts-world.js"])
            await loadScript(paths.engine(file));
        }
        await loadScript(paths.app("engine-loader.js"));
        await Promise.all([
          window.utauttsEngineReady,
          loadScript(paths.app("utautts.js")), loadScript(paths.app("qtloader.js")),
        ]);
      })();
      await Promise.race([
        Promise.all([engineAndScripts, uiModule]),
        new Promise((_, reject) => {
          timeout = setTimeout(() => reject(new Error("engine load timeout")), 180000);
        }),
      ]);
      clearTimeout(timeout);
      setStatus("UIを起動中…");
      const qtInstance = await qtLoad({
        locateFile: file => paths.asset(file),
        qt: {
          module: uiModule,
          fontDpi: 96,
          onLoaded: () => {
            uiLoaded = true;
            loading.classList.add("hidden");
            // QMLが要求を受け取れるようになるまで少し待つ。
            setTimeout(offerRestore, 800);
          },
          onExit: data => {
            setStatus("終了" + (data && data.code !== undefined ? " (code " + data.code + ")" : ""));
            loading.classList.remove("hidden");
          },
          entryFunction: window.utautts_entry,
          containerElements: [screen],
        },
      });
      window.utauttsQtModule = qtInstance || window.Module;
      window.utauttsRefreshMetadata = () => {
        const module = window.utauttsQtModule;
        if (module && typeof module._utauttsRefreshMetadata === "function")
          module._utauttsRefreshMetadata();
      };
      if (typeof window.utauttsFlushPendingResults === "function")
        window.utauttsFlushPendingResults();
    } catch (error) {
      clearTimeout(timeout);
      setStatus("エラー: " + error);
      console.error(error);
    }
  }
  init();
})();
