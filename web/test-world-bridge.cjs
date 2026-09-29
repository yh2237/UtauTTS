// Node上でWORLD wasmブリッジ（flat C ABI）を検証する。web/build-world.ps1 実行後に使う。
"use strict";

const path = require("path");

const FACTORY = path.join(__dirname, "dist", "world", "utautts-world.js");
const { createWorldBridge } = require(path.join(__dirname, "world-bridge.js"));

(async () => {
  const createWorld = require(FACTORY);
  const Module = await createWorld();
  const bridge = createWorldBridge(Module);

  const sampleRate = 16000;
  const length = 32000;
  const samples = new Float64Array(length);
  for (let i = 0; i < length; i++) {
    const time = i / sampleRate;
    let value = 0;
    for (let harmonic = 1; harmonic <= 7; harmonic++) {
      value += Math.sin(2 * Math.PI * 200 * harmonic * time) / harmonic;
    }
    samples[i] = value;
  }

  const analysis = bridge.analyze(samples, sampleRate, null);
  if (!analysis.ok) throw new Error("analyze: " + analysis.error);
  console.log(
    "analyze: frames=" + analysis.frames,
    "fft=" + analysis.fftSize,
    "spectrum=" + analysis.spectrum.length,
    "aperiodicity=" + analysis.aperiodicity.length
  );

  const synthesis = bridge.synthesize(
    analysis.f0,
    analysis.spectrum,
    analysis.aperiodicity,
    analysis.frames,
    analysis.fftSize,
    sampleRate
  );
  if (!synthesis.ok) throw new Error("synthesize: " + synthesis.error);
  let energy = 0;
  let peak = 0;
  for (const value of synthesis.samples) {
    energy += value * value;
    peak = Math.max(peak, Math.abs(value));
  }
  console.log(
    "synthesize: samples=" + synthesis.samples.length,
    "rms=" + Math.sqrt(energy / synthesis.samples.length).toFixed(4),
    "peak=" + peak.toFixed(4)
  );
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
