"use strict";

// 読み込み済みのWORLD Emscriptenモジュールを、Go wasmが使う契約へ包む。
// globalThis.utauttsWorld = {
//   ready,
//   analyze(samples: Float64Array, sampleRate, inputF0: Float64Array|null) -> { ok, frames, fftSize, f0, spectrum, aperiodicity, error },
//   synthesize(f0, spectrum, aperiodicity, frames, fftSize, sampleRate) -> { ok, samples, error }
// }
function createWorldBridge(Module) {
  const FRAME_MS = 10.0;
  const ERROR_SIZE = 512;
  const shapeFn = Module.cwrap("UtauTTSWorldAnalysisShape", "number", ["number", "number", "number"]);
  const analyzeFlat = Module.cwrap("UtauTTSWorldAnalyzeFlat", "number", [
    "number", "number", "number", "number", "number", "number",
    "number", "number", "number", "number", "number",
  ]);
  const synthesizeFlat = Module.cwrap("UtauTTSWorldSynthesizeFlat", "number", [
    "number", "number", "number", "number", "number", "number",
    "number", "number", "number", "number", "number",
  ]);

  function fail(errorPtr) {
    return { ok: false, error: Module.UTF8ToString(errorPtr) || "WORLD failed" };
  }

  return {
    ready: true,
    analyze(samples, sampleRate, inputF0) {
      const errorPtr = Module._malloc(ERROR_SIZE);
      const shapePtr = Module._malloc(32);
      try {
        Module.HEAP32[shapePtr >> 2] = samples.length;
        Module.HEAP32[(shapePtr >> 2) + 1] = sampleRate;
        Module.HEAPF64[(shapePtr >> 3) + 1] = FRAME_MS;
        if (!shapeFn(shapePtr, errorPtr, ERROR_SIZE)) return fail(errorPtr);
        let frames = Module.HEAP32[(shapePtr >> 2) + 4];
        const fftSize = Module.HEAP32[(shapePtr >> 2) + 5];
        const bins = (fftSize >> 1) + 1;
        const useInput = inputF0 && inputF0.length >= 2;
        if (useInput) frames = inputF0.length;

        const samplesPtr = Module._malloc(samples.length * 8);
        const inputPtr = useInput ? Module._malloc(inputF0.length * 8) : 0;
        const f0Ptr = Module._malloc(frames * 8);
        const spectrumPtr = Module._malloc(frames * bins * 8);
        const apPtr = Module._malloc(frames * bins * 8);
        try {
          Module.HEAPF64.set(samples, samplesPtr >> 3);
          if (useInput) Module.HEAPF64.set(inputF0, inputPtr >> 3);
          const ok = analyzeFlat(
            samplesPtr, samples.length, sampleRate, FRAME_MS,
            inputPtr, useInput ? inputF0.length : 0,
            f0Ptr, spectrumPtr, apPtr, errorPtr, ERROR_SIZE
          );
          if (!ok) return fail(errorPtr);
          return {
            ok: true,
            frames,
            fftSize,
            f0: Module.HEAPF64.slice(f0Ptr >> 3, (f0Ptr >> 3) + frames),
            spectrum: Module.HEAPF64.slice(spectrumPtr >> 3, (spectrumPtr >> 3) + frames * bins),
            aperiodicity: Module.HEAPF64.slice(apPtr >> 3, (apPtr >> 3) + frames * bins),
          };
        } finally {
          Module._free(samplesPtr);
          if (inputPtr) Module._free(inputPtr);
          Module._free(f0Ptr);
          Module._free(spectrumPtr);
          Module._free(apPtr);
        }
      } finally {
        Module._free(shapePtr);
        Module._free(errorPtr);
      }
    },
    synthesize(f0, spectrum, aperiodicity, frames, fftSize, sampleRate) {
      const required = frames < 2 ? 0 : Math.trunc(((frames - 1) * FRAME_MS) / 1000 * sampleRate) + 1;
      if (required <= 0) return { ok: false, error: "invalid synthesis length" };
      const errorPtr = Module._malloc(ERROR_SIZE);
      const f0Ptr = Module._malloc(frames * 8);
      const spectrumPtr = Module._malloc(spectrum.length * 8);
      const apPtr = Module._malloc(aperiodicity.length * 8);
      const outputPtr = Module._malloc(required * 8);
      try {
        Module.HEAPF64.set(f0, f0Ptr >> 3);
        Module.HEAPF64.set(spectrum, spectrumPtr >> 3);
        Module.HEAPF64.set(aperiodicity, apPtr >> 3);
        const ok = synthesizeFlat(
          f0Ptr, frames, spectrumPtr, apPtr, fftSize, FRAME_MS, sampleRate,
          outputPtr, required, errorPtr, ERROR_SIZE
        );
        if (!ok) return fail(errorPtr);
        return { ok: true, samples: Module.HEAPF64.slice(outputPtr >> 3, (outputPtr >> 3) + required) };
      } finally {
        Module._free(f0Ptr);
        Module._free(spectrumPtr);
        Module._free(apPtr);
        Module._free(outputPtr);
        Module._free(errorPtr);
      }
    },
  };
}

if (typeof module === "object" && module.exports) {
  module.exports = { createWorldBridge };
}
