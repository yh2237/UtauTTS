"use strict";

// 参照照合のKokoroとWavLMを、ONNX Runtime Webで推論する（Web Worker内）。
// Goは推論が要ると {ok:false, need:[...]} を返す。ここで求めた結果をprovideInferenceで渡し、同じ呼び出しをやり直す。
// モデルは要る言語の分だけ、初めて使うときに読み込む。ダウンロードしたものはCache Storageに残す。
function createReferenceInference({ baseURL, ortURL, virtualFs, status }) {
  const CACHE = "utautts-reference-v1";
  const OUT_DIR = "/tmp/utautts-inference";
  let ortReady = null;
  const sessions = new Map();
  const voices = new Map();

  function loadOrt(logs) {
    if (!ortReady) {
      ortReady = (async () => {
        if (!globalThis.ort) {
          importScripts(ortURL + "ort.wasm.min.js");
          ort.env.wasm.wasmPaths = ortURL;
        }
        ort.env.wasm.numThreads = globalThis.crossOriginIsolated ? Math.min(8, navigator.hardwareConcurrency || 1) : 1;
        logs.push("推論: ONNX Runtime Web（" + ort.env.wasm.numThreads + " スレッド）");
      })().catch(error => {
        ortReady = null;
        throw error;
      });
    }
    return ortReady;
  }

  async function fetchCached(file, logs) {
    const url = baseURL + file;
    let cache = null;
    try {
      cache = await caches.open(CACHE);
      const hit = await cache.match(url);
      if (hit) return new Uint8Array(await hit.arrayBuffer());
    } catch (error) {
      cache = null;
    }
    const started = performance.now();
    const response = await fetch(url);
    if (!response.ok) throw new Error(url + " HTTP " + response.status);
    const total = Number(response.headers.get("Content-Length")) || 0;
    const reader = response.body.getReader();
    const chunks = [];
    let received = 0, shown = -1;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      received += value.length;
      const step = total ? Math.floor(received * 10 / total) : -1;
      if (step !== shown) {
        shown = step;
        status("モデルをダウンロード中: " + file + (total ? "（" + step * 10 + "%）" : ""));
      }
    }
    const data = new Uint8Array(received);
    let offset = 0;
    for (const chunk of chunks) {
      data.set(chunk, offset);
      offset += chunk.length;
    }
    logs.push("モデルをダウンロードしました: " + file + "（" + (received / 1048576).toFixed(0) + " MB、" +
      ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
    if (cache) {
      try { await cache.put(url, new Response(data)); } catch (error) { /* 容量が足りなければ毎回取得する。 */ }
    }
    return data;
  }

  async function session(file, logs) {
    if (!sessions.has(file)) {
      const pending = (async () => {
        await loadOrt(logs);
        const data = await fetchCached(file, logs);
        status("モデルを読み込み中: " + file);
        const started = performance.now();
        const result = await ort.InferenceSession.create(data, { executionProviders: ["wasm"] });
        logs.push("モデルを読み込みました: " + file + "（" + ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
        return result;
      })();
      sessions.set(file, pending);
      pending.catch(() => sessions.delete(file));
    }
    return sessions.get(file);
  }

  async function voice(name, logs) {
    if (!voices.has(name)) {
      const pending = fetchCached("voices/" + name + ".f32", logs).then(data => new Float32Array(data.buffer, data.byteOffset, data.byteLength / 4));
      voices.set(name, pending);
      pending.catch(() => voices.delete(name));
    }
    return voices.get(name);
  }

  function writeFloat32(path, values) {
    const bytes = new Uint8Array(values.buffer, values.byteOffset, values.byteLength);
    virtualFs.mountFile(path, bytes.slice());
    return path;
  }

  async function read(job, number, logs) {
    await loadOrt(logs);
    const started = performance.now();
    const table = await voice(job.voice, logs);
    const ids = new ort.Tensor("int64", BigInt64Array.from(job.tokens.map(BigInt)), [1, job.tokens.length]);
    const ref = new ort.Tensor("float32", table.slice(job.style * 256, (job.style + 1) * 256), [1, 256]);
    let durations = job.durations;
    if (!durations || durations.length === 0) {
      const predicted = await (await session(job.model + "/duration.onnx", logs)).run({
        input_ids: ids, ref_s: ref, speed: new ort.Tensor("float32", Float32Array.from([job.speed || 1]), [1]),
      });
      durations = Array.from(predicted.duration.data, value => Math.max(1, Math.round(value)));
    }
    const lengths = new ort.Tensor("int64", BigInt64Array.from(durations.map(BigInt)), [durations.length]);
    const result = { key: job.key, durations };
    if (job.audio) {
      status("参照の音声を作成中…");
      const out = await (await session(job.model + "/synth.onnx", logs)).run({ input_ids: ids, ref_s: ref, pred_dur: lengths });
      result.f0 = Array.from(out.f0.data);
      result.audio_path = writeFloat32(OUT_DIR + "/read-" + number + ".f32", out.audio.data);
    } else {
      const out = await (await session(job.model + "/prosody.onnx", logs)).run({ input_ids: ids, ref_s: ref, pred_dur: lengths });
      result.f0 = Array.from(out.f0.data);
    }
    logs.push("参照TTS: " + job.model + "（" + job.voice + "）で読みました（" +
      (job.audio ? "音声あり、" : "") + ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
    return result;
  }

  async function features(jobs, logs) {
    const results = [];
    if (jobs.length === 0) return results;
    const started = performance.now();
    const wavlm = await session("wavlm6.onnx", logs);
    let seconds = 0, shown = -1;
    for (let i = 0; i < jobs.length; i++) {
      const job = jobs[i];
      const bytes = virtualFs.readFile(job.samples_path);
      if (!bytes) throw new Error("missing " + job.samples_path);
      const samples = new Float32Array(bytes.slice().buffer);
      virtualFs._unlink(job.samples_path);
      seconds += samples.length / 16000;
      const out = await wavlm.run({ audio: new ort.Tensor("float32", samples, [1, samples.length]) });
      const [frames, dims] = out.features.dims;
      results.push({ key: job.key, path: writeFloat32(OUT_DIR + "/features-" + i + ".f32", out.features.data), frames, dims });
      const step = Math.floor((i + 1) * 10 / jobs.length);
      if (jobs.length > 1 && step !== shown) {
        shown = step;
        status("特徴量を計算中…（" + step * 10 + "%）");
      }
    }
    logs.push("特徴量: WavLM で " + jobs.length + " 件（音声 " + seconds.toFixed(1) + " 秒、" +
      ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
    return results;
  }

  // runは、Goが返した推論の頼みごとを実行し、結果をGoへ渡す。
  async function run(jobs, call, logs) {
    const reads = [];
    for (const [number, job] of jobs.entries()) {
      if (job.kind === "kokoro") reads.push(await read(job, number, logs));
    }
    const provided = await features(jobs.filter(job => job.kind === "features"), logs);
    const response = JSON.parse(call("provideInference", JSON.stringify({ reads, features: provided })));
    if (!response.ok) throw new Error(response.error || "provideInference failed");
  }

  return { run };
}

if (typeof module === "object" && module.exports)
  module.exports = { createReferenceInference };
