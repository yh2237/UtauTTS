// Node上でglobalThis.fsシムを検証する。モデルを仮想FSへ置き、Goのos.ReadFileで読ませる。
// 事前に web/build.ps1 を実行しておく。
"use strict";

const fs = require("fs");
const path = require("path");
const { TextEncoder, TextDecoder } = require("util");

if (!globalThis.TextEncoder) globalThis.TextEncoder = TextEncoder;
if (!globalThis.TextDecoder) globalThis.TextDecoder = TextDecoder;
if (!globalThis.performance) {
  Object.defineProperty(globalThis, "performance", {
    value: require("perf_hooks").performance,
    configurable: true,
  });
}
if (!globalThis.crypto) {
  Object.defineProperty(globalThis, "crypto", {
    value: require("crypto").webcrypto,
    configurable: true,
  });
}

// wasm_exec.js は既定のfs/process/pathを入れる。その後で仮想FSへ差し替える。
require(path.join(__dirname, "dist", "wasm_exec.js"));
const { installVirtualFs } = require(path.join(__dirname, "fs-shim.js"));
const virtualFs = installVirtualFs({ cwd: "/" });

const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
virtualFs.mountText(
  MODEL_PATH,
  fs.readFileSync(path.join(__dirname, "..", "models", "frame-intonation-tcn-v9.1-t.json"), "utf8")
);
console.log("mounted:", MODEL_PATH, "cwd:", globalThis.process.cwd());

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

(async () => {
  const bytes = fs.readFileSync(path.join(__dirname, "dist", "utautts.wasm"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.predictProsody, 10000);

  const raw = globalThis.utauttsWasm.predictProsody({
    kana: "こんにちは、きょうはいいてんきです",
    modelPath: MODEL_PATH,
    strength: 1,
  });
  const data = JSON.parse(raw);
  if (data.error) {
    console.error("ERROR:", data.error);
    process.exit(1);
  }
  console.log(
    "ok: morae=" + data.morae.length,
    "frames=" + (data.framePitchCents || []).length,
    "total=" + data.totalDurationMS.toFixed(1)
  );
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
