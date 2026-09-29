"use strict";

const MODEL_PATH = "/models/frame-intonation-tcn-v9.1-t.json";
const DICT_PATH = "/dict";

const $ = (id) => document.getElementById(id);
const status = (text) => {
  $("status").textContent = text;
};

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
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.predictProsody, 30000);
}

async function mountModel(virtualFs) {
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

async function predict() {
  const text = $("text").value.trim();
  if (!text) return;
  const strength = Number($("strength").value) || 1;
  const started = performance.now();
  const raw = globalThis.utauttsWasm.predictProsody({ text, modelPath: MODEL_PATH, strength });
  const elapsed = performance.now() - started;
  const data = JSON.parse(raw);
  if (data.error) {
    $("out").textContent = "error: " + data.error;
    return;
  }
  $("out").textContent = JSON.stringify(data, null, 2);
  status("ready (" + elapsed.toFixed(1) + " ms)");
  drawCurve(data);
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

async function boot() {
  try {
    status("installing virtual fs…");
    const virtualFs = installVirtualFs({ cwd: "/" });
    status("loading Go wasm…");
    await loadGoWasm();
    status("loading model…");
    await mountModel(virtualFs);
    await loadOpenJTalk();
    $("run").addEventListener("click", predict);
    status("ready");
    await predict();
  } catch (error) {
    status("error: " + error.message);
    console.error(error);
  }
}

boot();
