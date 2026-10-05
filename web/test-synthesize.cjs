// 実行前にweb/build、build-openjtalk、build-worldで資産を用意する。
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
const { createWorldBridge } = require(path.join(__dirname, "world-bridge.js"));

const VOICE_NAME = "足立レイver3.5.0";
const VOICE_DIR = path.join(ROOT, "voice", VOICE_NAME);
const VOICE_PATH = "/voice/" + VOICE_NAME;
const MODEL_PATH = "/models/frame-intonation-tcn-v10.json";
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

function call(method, request) {
  const raw = globalThis.utauttsWasm.call(method, JSON.stringify(request || {}));
  const parsed = JSON.parse(String(raw));
  if (!parsed || parsed.ok !== true)
    throw new Error(method + ": " + ((parsed && parsed.error) || raw));
  return parsed.result;
}

(async () => {
  const virtualFs = installVirtualFs({ cwd: "/" });
  virtualFs.mountFile(MODEL_PATH, new Uint8Array(fs.readFileSync(path.join(ROOT, "models", "frame-intonation-tcn-v10.json"))));
  virtualFs.mountFile(
    "/renderer/utautts-world-phrase/renderer.json",
    new Uint8Array(fs.readFileSync(path.join(ROOT, "renderer", "utautts-world-phrase", "renderer.json")))
  );
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
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 10000);

  const text = process.argv[2] || "こんにちは、今日はいい天気です。";
  const modelID = call("models").models[0].id;
  const voicebankID = call("voicebanks").voicebanks[0].id;
  const base = {
    text,
    language: "ja",
    phonemizer: "ja-kana",
    model_id: modelID,
    voicebank_id: voicebankID,
    renderer: "utautts-world-phrase",
    tone: "C4",
    intonation_strength: 1,
    apply_pitch: true,
  };

  for (const label of ["predict", "predict2"]) {
    const started = Date.now();
    call("predictProsody", base);
    console.log(label + " elapsed=" + (Date.now() - started) + "ms");
  }
  let data = null;
  for (let run = 1; run <= 2; run++) {
    const started = Date.now();
    data = call("synthesize", { ...base, output_path: OUTPUT_PATH });
    console.log("run " + run + " elapsed=" + (Date.now() - started) + "ms");
    const profile = virtualFs.readFile("/out/profile.jsonl");
    if (profile) console.log("profile " + run + ":", Buffer.from(profile).toString("utf8").trim());
  }
  assert.equal(data.engine, "utautts-world-phrase");
  assert.ok(data.reading.length > 0 && data.duration_ms > 0 && data.units.length > 0);
  console.log("synthesize:", data.engine, data.reading, data.duration_ms.toFixed(1) + "ms");

  const wav = virtualFs.readFile(OUTPUT_PATH);
  if (!wav) throw new Error("output WAV not found in virtual fs");
  assert.ok(wav.length > 44);
  assert.equal(Buffer.from(wav.subarray(0, 4)).toString("ascii"), "RIFF");
  assert.equal(Buffer.from(wav.subarray(8, 12)).toString("ascii"), "WAVE");
  const outDir = path.join(DIST, "out");
  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(path.join(outDir, "utautts.wav"), Buffer.from(wav));
  console.log("wav bytes:", wav.length, "->", path.join(outDir, "utautts.wav"));
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
