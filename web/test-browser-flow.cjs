// ブラウザと同じ資産配置で、共通Engineによる解析と抑揚予測を検証する。
"use strict";

const fs = require("fs");
const assert = require("node:assert/strict");
const path = require("path");
const { TextEncoder, TextDecoder } = require("util");

if (!globalThis.TextEncoder) globalThis.TextEncoder = TextEncoder;
if (!globalThis.TextDecoder) globalThis.TextDecoder = TextDecoder;
if (!globalThis.performance) globalThis.performance = require("perf_hooks").performance;
if (!globalThis.crypto) {
  Object.defineProperty(globalThis, "crypto", { value: require("crypto").webcrypto, configurable: true });
}

const DIST = path.join(__dirname, "dist");
const ROOT = path.join(__dirname, "..");
require(path.join(DIST, "wasm_exec.js"));
const { installVirtualFs } = require(path.join(__dirname, "fs-shim.js"));
const { createOpenJTalkBridge } = require(path.join(__dirname, "openjtalk-bridge.js"));

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

const MODEL_PATH = "/models/frame-intonation-tcn-v10.json";
const DICT_PATH = "/dict";

function call(method, request) {
  const raw = globalThis.utauttsWasm.call(method, JSON.stringify(request || {}));
  const parsed = JSON.parse(String(raw));
  if (!parsed || parsed.ok !== true)
    throw new Error(method + ": " + ((parsed && parsed.error) || raw));
  return parsed.result;
}

(async () => {
  const virtualFs = installVirtualFs({ cwd: "/" });
  virtualFs.mountFile(
    MODEL_PATH,
    new Uint8Array(fs.readFileSync(path.join(DIST, "models", "frame-intonation-tcn-v10.json")))
  );
  virtualFs.mountFile(
    "/renderer/utautts-world-phrase/renderer.json",
    new Uint8Array(fs.readFileSync(path.join(ROOT, "renderer", "utautts-world-phrase", "renderer.json")))
  );

  const Module = await require(path.join(DIST, "openjtalk", "utautts-openjtalk.js"))();
  Module.FS.mkdir(DICT_PATH);
  const manifest = JSON.parse(
    fs.readFileSync(path.join(DIST, "openjtalk", "dict-manifest.json"), "utf8").replace(/^\uFEFF/, "")
  );
  for (const name of manifest.files) {
    Module.FS.writeFile(
      DICT_PATH + "/" + name,
      new Uint8Array(fs.readFileSync(path.join(DIST, "openjtalk", "dict", name)))
    );
  }
  const bridge = createOpenJTalkBridge(Module);
  bridge.init(DICT_PATH);
  globalThis.utauttsOpenJTalk = bridge;
  console.log("openjtalk dict files:", manifest.files.length);

  const bytes = fs.readFileSync(path.join(DIST, "utautts.wasm"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 10000);

  const modelID = call("models").models[0].id;
  const data = call("predictProsody", {
    text: "こんにちは、今日はいい天気です。",
    language: "ja",
    phonemizer: "ja-kana",
    model_id: modelID,
    renderer: "utautts-world-phrase",
    intonation_strength: 1,
    apply_pitch: true,
  });
  assert.equal(data.reading, "コンニチワ、キョーワイイテンキデス。");
  assert.equal(data.morae.length, 17);
  assert.equal(data.prosody_model_applied, true);
  assert.ok(data.frame_pitch_cents.length > 0);
  console.log(
    "ok: reading=" + data.reading,
    "morae=" + data.morae.length,
    "frames=" + (data.frame_pitch_cents || []).length
  );
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
