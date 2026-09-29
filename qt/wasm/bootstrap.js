"use strict";

(() => {
  const scriptURL = document.currentScript.src;
  const config = globalThis.UtauTTSConfig || {};
  const paths = createUtauTTSAssetPaths(scriptURL, config);
  window.utauttsAssetPaths = paths;

  function loadScript(url) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = url;
      script.onload = resolve;
      script.onerror = () => reject(new Error("script load failed: " + url));
      document.head.appendChild(script);
    });
  }

  window.addEventListener("wheel", event => {
    if (event.ctrlKey) event.preventDefault();
  }, { passive: false, capture: true });
  window.addEventListener("touchmove", event => event.preventDefault(),
    { passive: false, capture: true });

  async function init() {
    const loading = document.getElementById("loading");
    const screen = document.getElementById("screen");
    const status = document.getElementById("status");
    const statusTimer = setInterval(() => {
      if (window.utauttsEngineStatus) status.textContent = window.utauttsEngineStatus;
    }, 200);
    let timeout;
    try {
      // The main thread only needs the FS mirror in Worker mode. Keep Go and
      // both native bridges out of the UI thread unless fallback is requested.
      await loadScript(paths.engine("fs-shim.js"));
      if (new URLSearchParams(location.search).get("async") === "0") {
        for (const file of ["wasm_exec.js", "openjtalk-bridge.js", "world-bridge.js",
          "openjtalk/utautts-openjtalk.js", "world/utautts-world.js"])
          await loadScript(paths.engine(file));
      }
      await loadScript(paths.app("engine-loader.js"));
      const uiScripts = Promise.all([
        loadScript(paths.app("utautts.js")), loadScript(paths.app("qtloader.js")),
      ]);
      await Promise.race([
        Promise.all([window.utauttsEngineReady, uiScripts]),
        new Promise((_, reject) => {
          timeout = setTimeout(() => reject(new Error("engine load timeout")), 180000);
        }),
      ]);
      clearTimeout(timeout);
      clearInterval(statusTimer);
      status.textContent = "UIを起動中…";
      const qtInstance = await qtLoad({
        locateFile: file => paths.asset(file),
        qt: {
          fontDpi: 96,
          onLoaded: () => loading.classList.add("hidden"),
          onExit: data => {
            status.textContent = "終了" + (data && data.code !== undefined ? " (code " + data.code + ")" : "");
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
      clearInterval(statusTimer);
      status.textContent = "エラー: " + error;
      console.error(error);
    }
  }
  init();
})();
