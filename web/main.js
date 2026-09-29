"use strict";

const $ = (id) => document.getElementById(id);
const status = (text) => {
  $("status").textContent = text;
};

let client = null;

function playWav(buffer) {
  const blob = new Blob([buffer], { type: "audio/wav" });
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
  if (!text) return;
  const strength = Number($("strength").value) || 1;
  $("run").disabled = true;
  status("合成中…");
  try {
    const { result, wav, elapsedMS } = await client.synthesize(text, strength);
    if (wav) playWav(wav);
    $("out").textContent = JSON.stringify(result, null, 2);
    status("完了 (" + elapsedMS.toFixed(0) + " ms)");
  } catch (error) {
    status("error: " + error.message);
  } finally {
    $("run").disabled = false;
  }
}

async function onVoice(event) {
  const files = event.target.files;
  if (!files || files.length === 0) return;
  $("run").disabled = true;
  try {
    const mounted = await client.mountVoicebank(files);
    $("voice-status").textContent = mounted.count + " ファイル (" + mounted.root + ")";
    $("run").disabled = false;
  } catch (error) {
    $("voice-status").textContent = "error: " + error.message;
  }
}

function boot() {
  client = new UtauTTSClient("./worker.js");
  client.onStatus = status;
  client.onPreview = drawCurve;
  client.onError = (message) => status("error: " + message);
  client.ready.then(() => status("ready")).catch(() => {});
  $("voice").addEventListener("change", onVoice);
  $("run").addEventListener("click", run);
}

boot();
