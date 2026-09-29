// ブラウザと同じ web/dist レイアウト（fsシム + manifest経由の辞書 + modelPath）で通し検証する。
"use strict";

const fs = require("fs");
const path = require("path");
const { TextEncoder, TextDecoder } = require("util");

if (!globalThis.TextEncoder) globalThis.TextEncoder = TextEncoder;
if (!globalThis.TextDecoder) globalThis.TextDecoder = TextDecoder;
if (!globalThis.performance) globalThis.performance = require("perf_hooks").performance;
if (!globalThis.crypto) {
  Object.defineProperty(globalThis, "crypto", { value: require("crypto").webcrypto, configurable: true });
}

const DIST = path.join(__dirname, "dist");
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

const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
const DICT_PATH = "/dict";

(async () => {
  const virtualFs = installVirtualFs({ cwd: "/" });
  virtualFs.mountFile(
    MODEL_PATH,
    new Uint8Array(fs.readFileSync(path.join(DIST, "models", "frame-intonation-tcn-v9.1-t.json")))
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
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.predictProsody, 10000);

  const raw = globalThis.utauttsWasm.predictProsody({
    text: "こんにちは、今日はいい天気です。",
    modelPath: MODEL_PATH,
    strength: 1,
  });
  const data = JSON.parse(raw);
  if (data.error) {
    console.error("ERROR:", data.error);
    process.exit(1);
  }
  console.log(
    "ok: reading=" + data.reading,
    "morae=" + data.morae.length,
    "frames=" + (data.framePitchCents || []).length,
    "total=" + data.totalDurationMS.toFixed(1)
  );
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
