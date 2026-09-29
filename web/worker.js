"use strict";

// 音声合成エンジン一式（fsシム・Go wasm・Open JTalk・WORLD・音源）をWorker内に閉じ込める。
importScripts(
  "./wasm_exec.js",
  "./fs-shim.js",
  "./openjtalk-bridge.js",
  "./world-bridge.js",
  "./openjtalk/utautts-openjtalk.js",
  "./world/utautts-world.js"
);

const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
const DICT_PATH = "/dict";
const VOICE_ROOT = "/voice";
const OUTPUT_PATH = "/out/utautts.wav";

let virtualFs = null;
let voicebankPath = "";

function post(type, payload, transfer) {
  self.postMessage(Object.assign({ type }, payload), transfer || []);
}

function status(text) {
  post("status", { text });
}

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

async function loadOpenJTalk() {
  // Worker内ではEmscriptenのスクリプト位置検出が当てにならないため、wasmの場所を明示する。
  const Module = await createUtauTTSOpenJTalk({ locateFile: (file) => "./openjtalk/" + file });
  Module.FS.mkdir(DICT_PATH);
  const manifest = await (await fetch("./openjtalk/dict-manifest.json")).json();
  let loaded = 0;
  for (const name of manifest.files) {
    const response = await fetch("./openjtalk/dict/" + encodeURIComponent(name));
    if (!response.ok) throw new Error("failed to load dictionary file: " + name);
    Module.FS.writeFile(DICT_PATH + "/" + name, new Uint8Array(await response.arrayBuffer()));
    loaded++;
    status("辞書を読み込み中… (" + loaded + "/" + manifest.files.length + ")");
  }
  const bridge = createOpenJTalkBridge(Module);
  bridge.init(DICT_PATH);
  self.utauttsOpenJTalk = bridge;
  globalThis.utauttsOpenJTalk = bridge;
}

async function loadWorld() {
  const Module = await createUtauTTSWorld({ locateFile: (file) => "./world/" + file });
  const bridge = createWorldBridge(Module);
  globalThis.utauttsWorld = bridge;
}

async function init() {
  status("仮想FSを準備中…");
  virtualFs = installVirtualFs({ cwd: "/" });
  status("Go wasmを読み込み中…");
  const go = new Go();
  const wasmBytes = await (await fetch("./utautts.wasm")).arrayBuffer();
  const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.synthesize, 30000);
  status("モデルを読み込み中…");
  virtualFs.mountFile(
    MODEL_PATH,
    new Uint8Array(await (await fetch("./models/frame-intonation-tcn-v9.1-t.json")).arrayBuffer())
  );
  status("Open JTalkを読み込み中…");
  await loadOpenJTalk();
  status("WORLDを読み込み中…");
  await loadWorld();
  post("ready");
}

async function mountVoicebank(id, entries) {
  let root = "";
  let mounted = 0;
  for (const entry of entries) {
    const relative = entry.path;
    if (root === "") root = relative.split("/")[0];
    const buffer = await entry.file.arrayBuffer();
    virtualFs.mountFile(VOICE_ROOT + "/" + relative, new Uint8Array(buffer));
    mounted++;
    if (mounted % 100 === 0) {
      status("音源を読み込み中… (" + mounted + "/" + entries.length + ")");
    }
  }
  voicebankPath = VOICE_ROOT + "/" + root;
  post("mounted", { id, count: mounted, root });
}

function synthesize(id, text, strength) {
  const preview = JSON.parse(globalThis.utauttsWasm.predictProsody({ text, modelPath: MODEL_PATH, strength }));
  post("preview", { preview });

  const started = Date.now();
  const raw = globalThis.utauttsWasm.synthesize({
    text,
    modelPath: MODEL_PATH,
    voicebankPath,
    outputPath: OUTPUT_PATH,
    strength,
  });
  const result = JSON.parse(raw);
  if (result.error) {
    post("error", { id, message: result.error });
    return;
  }
  const wav = virtualFs.readFile(OUTPUT_PATH);
  const buffer = wav ? wav.slice().buffer : null;
  post("result", { id, result, elapsedMS: Date.now() - started, wav: buffer }, buffer ? [buffer] : []);
}

self.onmessage = async (event) => {
  const data = event.data || {};
  try {
    if (data.type === "mountVoicebank") {
      await mountVoicebank(data.id, data.entries);
    } else if (data.type === "synthesize") {
      synthesize(data.id, data.text, data.strength);
    }
  } catch (error) {
    post("error", { id: data.id, message: String((error && error.message) || error) });
  }
};

init().catch((error) => {
  post("error", { message: String((error && error.message) || error) });
});
