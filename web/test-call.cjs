// native.Engine.call を wasm から叩けるか確認する最小テスト。
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
installVirtualFs({ cwd: "/" });

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
  const bytes = fs.readFileSync(path.join(DIST, "utautts.wasm"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 10000);
  for (const method of ["health", "models", "renderers", "voicebanks"]) {
    const raw = globalThis.utauttsWasm.call(method, "{}");
    console.log(method + ":", String(raw).slice(0, 200));
  }
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
