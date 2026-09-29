// Node上でGo wasm + OpenJTalk wasm + WORLD wasm を統合し、テキストからWAVまで合成する。
// 事前に web/build.ps1 と web/build-openjtalk.ps1 と web/build-world.ps1 を実行しておく。
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
const ROOT = path.join(__dirname, "..");
require(path.join(DIST, "wasm_exec.js"));
const { installVirtualFs } = require(path.join(__dirname, "fs-shim.js"));
const { createOpenJTalkBridge } = require(path.join(__dirname, "openjtalk-bridge.js"));
const { createWorldBridge } = require(path.join(__dirname, "world-bridge.js"));

const VOICE_NAME = "足立レイver3.5.0";
const VOICE_DIR = path.join(ROOT, "voice", VOICE_NAME);
const VOICE_PATH = "/voice/" + VOICE_NAME;
const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
const DICT_PATH = "/dict";
const OUTPUT_PATH = "/out/utautts.wav";

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

function mountTree(virtualFs, hostDir, guestDir) {
  let count = 0;
  for (const entry of fs.readdirSync(hostDir, { withFileTypes: true })) {
    const host = path.join(hostDir, entry.name);
    const guest = guestDir + "/" + entry.name;
    if (entry.isDirectory()) count += mountTree(virtualFs, host, guest);
    else {
      virtualFs.mountFile(guest, new Uint8Array(fs.readFileSync(host)));
      count++;
    }
  }
  return count;
}

async function loadOpenJTalk() {
  const Module = await require(path.join(DIST, "openjtalk", "utautts-openjtalk.js"))();
  Module.FS.mkdir(DICT_PATH);
  const manifest = JSON.parse(
    fs.readFileSync(path.join(DIST, "openjtalk", "dict-manifest.json"), "utf8").replace(/^\uFEFF/, "")
  );
  for (const name of manifest.files) {
    Module.FS.writeFile(DICT_PATH + "/" + name, new Uint8Array(fs.readFileSync(path.join(DIST, "openjtalk", "dict", name))));
  }
  const bridge = createOpenJTalkBridge(Module);
  bridge.init(DICT_PATH);
  globalThis.utauttsOpenJTalk = bridge;
}

async function loadWorld() {
  const Module = await require(path.join(DIST, "world", "utautts-world.js"))();
  globalThis.utauttsWorld = createWorldBridge(Module);
}

(async () => {
  const virtualFs = installVirtualFs({ cwd: "/" });
  virtualFs.mountFile(MODEL_PATH, new Uint8Array(fs.readFileSync(path.join(ROOT, "models", "frame-intonation-tcn-v9.1-t.json"))));
  console.log("voicebank files:", mountTree(virtualFs, VOICE_DIR, VOICE_PATH));

  await loadWorld();
  console.log("world ready");
  await loadOpenJTalk();
  console.log("openjtalk ready");

  const bytes = fs.readFileSync(path.join(DIST, "utautts.wasm"));
  const go = new Go();
  go.env = { TMPDIR: "/tmp", UTAUTTS_WORLD_PROFILE: "/out/profile.jsonl", UTAUTTS_TTS_PROFILE: "1" };
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.synthesize, 10000);

  const text = process.argv[2] || "こんにちは、今日はいい天気です。";
  for (const label of ["predict", "predict2"]) {
    const started = Date.now();
    globalThis.utauttsWasm.predictProsody({ text, modelPath: MODEL_PATH, strength: 1 });
    console.log(label + " elapsed=" + (Date.now() - started) + "ms");
  }
  let data = null;
  let raw = "";
  for (let run = 1; run <= 2; run++) {
    const started = Date.now();
    raw = globalThis.utauttsWasm.synthesize({ text, modelPath: MODEL_PATH, voicebankPath: VOICE_PATH, outputPath: OUTPUT_PATH, strength: 1 });
    console.log("run " + run + " elapsed=" + (Date.now() - started) + "ms");
    const profile = virtualFs.readFile("/out/profile.jsonl");
    if (profile) console.log("profile " + run + ":", Buffer.from(profile).toString("utf8").trim());
  }
  data = JSON.parse(raw);
  if (data.error) {
    console.error("ERROR:", data.error);
    process.exit(1);
  }
  console.log("synthesize:", JSON.stringify(data));

  const wav = virtualFs.readFile(OUTPUT_PATH);
  if (!wav) throw new Error("output WAV not found in virtual fs");
  const outDir = path.join(DIST, "out");
  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(path.join(outDir, "utautts.wav"), Buffer.from(wav));
  console.log("wav bytes:", wav.length, "->", path.join(outDir, "utautts.wav"));
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
