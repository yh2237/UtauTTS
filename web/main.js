"use strict";

const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
const DICT_PATH = "/dict";
const VOICE_ROOT = "/voice";
const OUTPUT_PATH = "/out/utautts.wav";

const $ = (id) => document.getElementById(id);
const status = (text) => {
  $("status").textContent = text;
};

let virtualFs = null;
let voicebankPath = "";

function waitFor(predicate, timeoutMS) {
  return new Promise((resolve, reject) => {
    const deadline = performance.now() + timeoutMS;
    const tick = () => {
      if (predicate()) return resolve();
      if (performance.now() > deadline) return reject(new Error("timeout"));
      setTimeout(tick, 20);
    };
    tick();
  });
}

async function loadGoWasm() {
  const go = new Go();
  const response = await fetch("./utautts.wasm");
  if (!response.ok) throw new Error("failed to load utautts.wasm: " + response.status);
  const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.synthesize, 30000);
}

async function mountModel() {
  const response = await fetch("./models/frame-intonation-tcn-v9.1-t.json");
  if (!response.ok) throw new Error("failed to load model: " + response.status);
  virtualFs.mountFile(MODEL_PATH, new Uint8Array(await response.arrayBuffer()));
}

async function loadOpenJTalk() {
  const Module = await createUtauTTSOpenJTalk();
  Module.FS.mkdir(DICT_PATH);
  const manifest = await (await fetch("./openjtalk/dict-manifest.json")).json();
  let loaded = 0;
  for (const name of manifest.files) {
    const response = await fetch("./openjtalk/dict/" + encodeURIComponent(name));
    if (!response.ok) throw new Error("failed to load dictionary file: " + name);
    Module.FS.writeFile(DICT_PATH + "/" + name, new Uint8Array(await response.arrayBuffer()));
    loaded++;
    status("loading Open JTalk dictionary… (" + loaded + "/" + manifest.files.length + ")");
  }
  const bridge = createOpenJTalkBridge(Module);
  bridge.init(DICT_PATH);
  globalThis.utauttsOpenJTalk = bridge;
}

async function loadWorld() {
  const Module = await createUtauTTSWorld();
  globalThis.utauttsWorld = createWorldBridge(Module);
}

async function mountVoicebank(fileList) {
  const files = Array.from(fileList);
  if (files.length === 0) return;
  let root = "";
  let mounted = 0;
  for (const file of files) {
    const relative = file.webkitRelativePath || file.name;
    if (root === "") root = relative.split("/")[0];
    virtualFs.mountFile(VOICE_ROOT + "/" + relative, new Uint8Array(await file.arrayBuffer()));
    mounted++;
    if (mounted % 100 === 0) {
      $("voice-status").textContent = "読み込み中… (" + mounted + "/" + files.length + ")";
    }
  }
  voicebankPath = VOICE_ROOT + "/" + root;
  $("voice-status").textContent = mounted + " ファイル (" + root + ")";
  $("run").disabled = false;
}

function playWav(bytes) {
  const blob = new Blob([bytes], { type: "audio/wav" });
  const player = $("player");
  if (player.dataset.url) URL.revokeObjectURL(player.dataset.url);
  const url = URL.createObjectURL(blob);
  player.dataset.url = url;
  player.src = url;
  player.play().catch(() => {});
}

function drawCurve(data) {
  const canvas = $("curve");
  const ctx = canvas.getContext("2d");
  const width = canvas.width;
  const height = canvas.height;
  ctx.clearRect(0, 0, width, height);
  const cents = data.framePitchCents || [];
  if (cents.length < 2) return;
  let min = Infinity;
  let max = -Infinity;
  for (const value of cents) {
    if (value < min) min = value;
    if (value > max) max = value;
  }
  const span = Math.max(1, max - min);
  ctx.strokeStyle = "#4c8dff";
  ctx.lineWidth = 2;
  ctx.beginPath();
  cents.forEach((value, index) => {
    const x = (index / (cents.length - 1)) * (width - 20) + 10;
    const y = height - 10 - ((value - min) / span) * (height - 20);
    if (index === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.stroke();
}

async function run() {
  const text = $("text").value.trim();
  if (!text || !voicebankPath) return;
  const strength = Number($("strength").value) || 1;

  status("抑揚を予測中…");
  const preview = JSON.parse(globalThis.utauttsWasm.predictProsody({ text, modelPath: MODEL_PATH, strength }));
  if (!preview.error) drawCurve(preview);

  status("合成中…");
  const started = performance.now();
  const raw = globalThis.utauttsWasm.synthesize({
    text,
    modelPath: MODEL_PATH,
    voicebankPath,
    outputPath: OUTPUT_PATH,
    strength,
  });
  const elapsed = performance.now() - started;
  const result = JSON.parse(raw);
  if (result.error) {
    status("error: " + result.error);
    return;
  }
  const wav = virtualFs.readFile(OUTPUT_PATH);
  if (wav) playWav(wav);
  $("out").textContent = JSON.stringify(result, null, 2);
  status("完了 (" + elapsed.toFixed(0) + " ms)");
}

async function boot() {
  try {
    status("installing virtual fs…");
    virtualFs = installVirtualFs({ cwd: "/" });
    status("loading Go wasm…");
    await loadGoWasm();
    status("loading model…");
    await mountModel();
    await loadOpenJTalk();
    status("loading WORLD…");
    await loadWorld();
    status("ready");
    $("voice").addEventListener("change", (event) => mountVoicebank(event.target.files));
    $("run").addEventListener("click", run);
  } catch (error) {
    status("error: " + error.message);
    console.error(error);
  }
}

boot();
