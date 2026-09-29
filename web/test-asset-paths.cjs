"use strict";

const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { createUtauTTSAssetPaths } = require("../qt/wasm/asset-paths.js");

test("local Qt build resolves engine assets in the repo, not below the build directory", () => {
  const paths = createUtauTTSAssetPaths("http://localhost:8010/build/qt-wasm/bootstrap.js", {
    engineBaseURL: "../../web/dist/", rendererBaseURL: "../../renderer/",
  });
  assert.equal(paths.engine("utautts.wasm"), "http://localhost:8010/web/dist/utautts.wasm");
  assert.equal(paths.asset("utautts.wasm"), "http://localhost:8010/build/qt-wasm/utautts.wasm");
  assert.equal(paths.renderer("utautts-world-phrase/renderer.json"),
    "http://localhost:8010/renderer/utautts-world-phrase/renderer.json");
});

test("assembled sites work at a subpath and relative dictionary overrides remain engine-relative", () => {
  const paths = createUtauTTSAssetPaths("https://example.test/UtauTTS/bootstrap.js", {
    dictBaseURL: "./custom-dict/", dictManifestURL: "./custom-dict/manifest.json",
  });
  assert.equal(paths.engine("models/manifest.json"), "https://example.test/UtauTTS/web/dist/models/manifest.json");
  assert.equal(paths.dictBaseURL, "https://example.test/UtauTTS/web/dist/custom-dict/");
  assert.equal(paths.app("engine-worker.js"), "https://example.test/UtauTTS/engine-worker.js");
});

test("Pages and Worker retain the same R2 release while the Worker stays same-origin", () => {
  const config = { assetBaseURL: "https://assets.example.test/utautts/releases/v1-sha" };
  const page = createUtauTTSAssetPaths("https://tts.pages.dev/app/v1-sha/bootstrap.js", config);
  const workerURL = new URL(page.app("engine-worker.js"));
  workerURL.searchParams.set("config", JSON.stringify(config));
  const worker = createUtauTTSAssetPaths(workerURL.href, JSON.parse(workerURL.searchParams.get("config")));
  assert.equal(workerURL.origin, "https://tts.pages.dev");
  assert.equal(page.engineBaseURL, worker.engineBaseURL);
  assert.equal(worker.engine("voice/足立レイ/a.wav"),
    "https://assets.example.test/utautts/releases/v1-sha/web/dist/voice/%E8%B6%B3%E7%AB%8B%E3%83%AC%E3%82%A4/a.wav");
  assert.equal(page.asset("utautts.wasm"), "https://assets.example.test/utautts/releases/v1-sha/utautts.wasm");
  assert.equal(page.dictManifestURL, worker.dictManifestURL);
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
