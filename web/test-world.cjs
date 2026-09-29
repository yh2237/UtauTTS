// Node上でWORLD wasmの解析を検証する。web/build-world 実行後に使う。
"use strict";

const path = require("path");

const FACTORY = path.join(__dirname, "dist", "world", "utautts-world.js");

function median(values) {
  if (!values.length) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

(async () => {
  const createUtauTTSWorld = require(FACTORY);
  const Module = await createUtauTTSWorld();

  const sampleRate = 16000;
  const frameMS = 10.0;
  const sampleCount = 32000;
  const samples = new Float64Array(sampleCount);
  for (let i = 0; i < sampleCount; i++) {
    const time = i / sampleRate;
    let value = 0;
    for (let harmonic = 1; harmonic <= 7; harmonic++) {
      value += Math.sin(2 * Math.PI * 200 * harmonic * time) / harmonic;
    }
    samples[i] = value;
  }

  const errorSize = 512;
  const samplePtr = Module._malloc(samples.length * 8);
  const shapePtr = Module._malloc(32);
  const errorPtr = Module._malloc(errorSize);
  Module.HEAPF64.set(samples, samplePtr / 8);
  Module.HEAP32 = new Int32Array(Module.HEAPU8.buffer);
  Module.HEAPF64 = new Float64Array(Module.HEAPU8.buffer);
  Module.HEAP32[shapePtr >> 2] = sampleCount;
  Module.HEAP32[(shapePtr >> 2) + 1] = sampleRate;
  Module.HEAPF64[(shapePtr >> 3) + 1] = frameMS;

  if (!Module._UtauTTSWorldAnalysisShape(shapePtr, errorPtr, errorSize)) {
    throw new Error("shape failed");
  }
  const frameCount = Module.HEAP32[(shapePtr >> 2) + 4];
  const fftSize = Module.HEAP32[(shapePtr >> 2) + 5];
  console.log("frameCount:", frameCount, "fftSize:", fftSize);

  const f0Ptr = Module._malloc(frameCount * 8);
  Module.HEAPF64 = new Float64Array(Module.HEAPU8.buffer);
  const produced = Module._UtauTTSWorldF0(
    samplePtr,
    sampleCount,
    sampleRate,
    frameMS,
    f0Ptr,
    frameCount,
    errorPtr,
    errorSize
  );
  Module.HEAPF64 = new Float64Array(Module.HEAPU8.buffer);
  const f0 = Array.from(Module.HEAPF64.subarray(f0Ptr / 8, f0Ptr / 8 + frameCount));
  const echoed = Array.from(Module.HEAPF64.subarray(samplePtr / 8, samplePtr / 8 + 4));
  const voiced = f0.filter((value) => value > 0);
  console.log("sample echo:", echoed.map((v) => v.toFixed(3)).join(", "));
  console.log("f0:", f0.map((v) => v.toFixed(0)).join(","));
  console.log("f0 frames:", produced, "voiced:", voiced.length);
  console.log("median voiced f0 (expect ~200):", median(voiced).toFixed(2));
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
