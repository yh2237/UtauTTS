// Node上で「Go wasm + Open JTalk wasm」を統合し、テキストから抑揚曲線まで通す。
// 事前に web/build.ps1 と web/build-openjtalk.ps1 を実行しておく。
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
const { createOpenJTalkBridge } = require(path.join(__dirname, "openjtalk-bridge.js"));

const DICT_DIR =
  process.env.OPENJTALK_DICT ||
  path.join(__dirname, "..", ".tmp-openjtalk", "pyopenjtalk", "open_jtalk_dic_utf_8-1.11");

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

async function loadGoWasm() {
  const bytes = fs.readFileSync(path.join(__dirname, "dist", "utautts.wasm"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.predictProsody, 10000);
}

async function loadOpenJTalk() {
  const createOpenJTalk = require(path.join(__dirname, "dist", "openjtalk", "utautts-openjtalk.js"));
  const Module = await createOpenJTalk();
  Module.FS.mkdir("/dict");
  for (const name of fs.readdirSync(DICT_DIR)) {
    const full = path.join(DICT_DIR, name);
    if (!fs.statSync(full).isFile()) continue;
    Module.FS.writeFile("/dict/" + name, new Uint8Array(fs.readFileSync(full)));
  }
  const bridge = createOpenJTalkBridge(Module);
  bridge.init("/dict");
  globalThis.utauttsOpenJTalk = bridge;
}

(async () => {
  await loadOpenJTalk();
  console.log("openjtalk ready");
  await loadGoWasm();
  console.log("go wasm ready");

  const modelJSON = fs.readFileSync(
    path.join(__dirname, "..", "models", "frame-intonation-tcn-v9.1-t.json"),
    "utf8"
  );
  const text = process.argv[2] || "こんにちは、今日はいい天気です。";
  const raw = globalThis.utauttsWasm.predictProsody({ text, modelJSON, strength: 1 });
  const data = JSON.parse(raw);
  if (data.error) {
    console.error("ERROR:", data.error);
    process.exit(1);
  }

  const reportPath = path.join(__dirname, "dist", "openjtalk", "predict.json");
  fs.writeFileSync(reportPath, JSON.stringify(data, null, 2), "utf8");
  const morae = data.morae.map((mora) => mora.text || "|").join(" ");
  console.log("reading:", data.reading);
  console.log("morae:", morae);
  console.log(
    "frames:",
    (data.framePitchCents || []).length,
    "frameMS:",
    data.frameMS,
    "total:",
    data.totalDurationMS.toFixed(1)
  );
  console.log("wrote:", reportPath);
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
