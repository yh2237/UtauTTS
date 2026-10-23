"use strict";

const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { createUtauTTSAssetPaths } = require("../qt/wasm/asset-paths.js");

test("local Qt build resolves engine assets in the repo, not below the build directory", () => {
  const paths = createUtauTTSAssetPaths("http://localhost:8010/build/qt-wasm/bootstrap.js", {
    engineBaseURL: "../../web/dist/",
  });
  assert.equal(paths.engine("utautts.wasm"), "http://localhost:8010/web/dist/utautts.wasm");
  assert.equal(paths.asset("utautts.wasm"), "http://localhost:8010/build/qt-wasm/utautts.wasm");
});

test("assembled sites work at a subpath and relative dictionary overrides remain engine-relative", () => {
  const paths = createUtauTTSAssetPaths("https://example.test/UtauTTS/bootstrap.js", {
    dictBaseURL: "./custom-dict/", dictManifestURL: "./custom-dict/manifest.json",
  });
  assert.equal(paths.engine("models/manifest.json"), "https://example.test/UtauTTS/web/dist/models/manifest.json");
  assert.equal(paths.dictBaseURL, "https://example.test/UtauTTS/web/dist/custom-dict/");
  assert.equal(paths.app("engine-worker.js"), "https://example.test/UtauTTS/engine-worker.js");
});

test("R2 assets use the pinned release while the Worker stays on Pages", () => {
  const paths = createUtauTTSAssetPaths("https://tts.pages.dev/app/v1-sha/bootstrap.js", {
    assetBaseURL: "https://assets.example.test/utautts/releases/v1-sha",
  });
  assert.equal(paths.app("engine-worker.js"), "https://tts.pages.dev/app/v1-sha/engine-worker.js");
  assert.equal(paths.engine("voice/bank/a.wav"),
    "https://assets.example.test/utautts/releases/v1-sha/web/dist/voice/bank/a.wav");
  assert.equal(paths.asset("utautts.wasm"), "https://assets.example.test/utautts/releases/v1-sha/utautts.wasm");
});

test("bootstrap loads only the mirror on main in Worker mode and directs Qt wasm to R2", async () => {
  const loaded = [];
  let qtConfig;
  const context = vm.createContext({
    URL, URLSearchParams, console, createUtauTTSAssetPaths,
    UtauTTSConfig: { assetBaseURL: "https://assets.example.test/release/" },
    location: { href: "https://tts.pages.dev/?mobile=1", search: "?mobile=1" },
    setInterval() { return 1; }, clearInterval() {},
    setTimeout() { return 2; }, clearTimeout() {},
    document: {
      currentScript: { src: "https://tts.pages.dev/app/release/bootstrap.js" },
      createElement() { return {}; },
      getElementById() { return { classList: { add() {} } }; },
      head: { appendChild(script) {
        loaded.push(script.src);
        if (script.src.endsWith("engine-loader.js")) context.utauttsEngineReady = Promise.resolve();
        queueMicrotask(() => script.onload());
      } },
    },
    async qtLoad(config) { qtConfig = config; return {}; },
    addEventListener() {},
    async fetch() { return { ok: true, headers: new Map([["Content-Type", "application/wasm"]]) }; },
    WebAssembly: { async compileStreaming() { return {}; } },
  });
  context.window = context;
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../qt/wasm/bootstrap.js"), "utf8"), context);
  for (let i = 0; i < 20 && !qtConfig; i++) await new Promise(resolve => setImmediate(resolve));
  assert.ok(qtConfig);
  assert.equal(qtConfig.locateFile("utautts.wasm"), "https://assets.example.test/release/utautts.wasm");
  assert.deepEqual(loaded, [
    "https://assets.example.test/release/web/dist/fs-shim.js",
    "https://tts.pages.dev/app/release/engine-loader.js",
    "https://tts.pages.dev/app/release/utautts.js",
    "https://tts.pages.dev/app/release/qtloader.js",
  ]);
});
