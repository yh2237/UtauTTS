// Node上でwasm版Phase 1を検証する。web/build.ps1 か web/build.sh を先に実行する。
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

require(path.join(__dirname, "dist", "wasm_exec.js"));

function waitFor(predicate, timeoutMS) {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + timeoutMS;
    const tick = () => {
      if (predicate()) return resolve();
      if (Date.now() > deadline) {
        return reject(new Error("timeout waiting for the wasm API"));
      }
      setTimeout(tick, 20);
    };
    tick();
  });
}

(async () => {
  const wasmPath = path.join(__dirname, "dist", "utautts.wasm");
  if (!fs.existsSync(wasmPath)) {
    throw new Error("web/dist/utautts.wasm not found; run web/build.ps1 first");
  }
  const wasmBytes = fs.readFileSync(wasmPath);
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
  go.run(instance);

  await waitFor(
    () => globalThis.utauttsWasm && globalThis.utauttsWasm.predictProsody,
    10000
  );
  const api = globalThis.utauttsWasm;
  console.log("wasm API version:", api.version);

  const modelJSON = fs.readFileSync(
    path.join(__dirname, "..", "models", "frame-intonation-tcn-v9.1-t.json"),
    "utf8"
  );
  const kana = "こんにちは、きょうはいいてんきです";
  const raw = api.predictProsody({ kana, modelJSON, strength: 1 });
  const data = JSON.parse(raw);
  if (data.error) {
    console.error("ERROR:", data.error);
    process.exit(1);
  }
  console.log("reading: ", data.reading);
  console.log("morae:   ", data.morae.map((mora) => mora.text || "|").join(" "));
  console.log(
    "durations:",
    data.moraDurationsMS.map((value) => value.toFixed(1)).join(",")
  );
  console.log(
    "frame:   ",
    "frameMS=" + data.frameMS,
    "frames=" + (data.framePitchCents || []).length,
    "total=" + data.totalDurationMS.toFixed(1) + "ms"
  );
  console.log(
    "pitchPoints:",
    data.pitchPoints.map((value) => value.toFixed(1)).join(",")
  );
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
