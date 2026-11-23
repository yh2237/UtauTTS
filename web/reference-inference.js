"use strict";

// 参照照合のKokoroと音声エンコーダを、ONNX Runtime Webで推論する（Web Worker内）。
// Goは推論が要ると {ok:false, need:[...]} を返す。ここで求めた結果をprovideInferenceで渡し、同じ呼び出しをやり直す。
// モデルは要る分だけ、初めて使うときに読み込む。ダウンロードしたものはCache Storageに残す。
// ONNX Runtime Webは、ページが作った専用のWorker（serveOrt）で動かし、requestPortで受け取ったMessagePortで頼む。
// エンジンのWorkerの中や、そこから作ったWorkerでは、推論の途中でWorkerが応答しなくなった。
function inferenceThreads() {
  return globalThis.crossOriginIsolated ? Math.min(8, navigator.hardwareConcurrency || 1) : 1;
}

// serveOrtは推論用のWorkerで動き、最初のメッセージで受け取ったMessagePortからの頼みを処理する。
function serveOrt(ortURL) {
  importScripts(ortURL + "ort.wasm.min.js");
  ort.env.wasm.wasmPaths = ortURL;
  ort.env.wasm.numThreads = inferenceThreads();
  const sessions = new Map();
  self.onmessage = ({ ports }) => {
    const port = ports[0];
    port.onmessage = async ({ data }) => {
      try {
        if (data.op === "create") {
          sessions.set(data.session, await ort.InferenceSession.create(new Uint8Array(data.bytes), { executionProviders: ["wasm"] }));
          port.postMessage({ id: data.id });
          return;
        }
        const feeds = {};
        for (const [name, value] of Object.entries(data.feeds)) feeds[name] = new ort.Tensor(value.type, value.data, value.dims);
        const outputs = await sessions.get(data.session).run(feeds);
        const result = {}, transfer = [];
        for (const [name, value] of Object.entries(outputs)) {
          result[name] = { data: value.data, dims: value.dims };
          transfer.push(value.data.buffer);
        }
        port.postMessage({ id: data.id, outputs: result }, transfer);
      } catch (error) {
        port.postMessage({ id: data.id, error: String((error && error.message) || error) });
      }
    };
  };
}

function createReferenceInference({ baseURL, virtualFs, status, requestPort }) {
  const CACHE = "utautts-reference-v1";
  const OUT_DIR = "/tmp/utautts-inference";
  let runtime = null;
  const sessions = new Map();
  const voices = new Map();

  function tensor(type, data, dims) {
    return { type, data, dims };
  }

  // directRuntimeは、ortを読み込み済みの環境（nodeの試験）で、その場で推論する。
  function directRuntime(threads) {
    ort.env.wasm.numThreads = threads;
    return {
      async create(data) {
        const session = await ort.InferenceSession.create(data, { executionProviders: ["wasm"] });
        return async feeds => {
          const inputs = {};
          for (const [name, value] of Object.entries(feeds)) inputs[name] = new ort.Tensor(value.type, value.data, value.dims);
          const outputs = await session.run(inputs);
          const result = {};
          for (const [name, value] of Object.entries(outputs)) result[name] = { data: value.data, dims: value.dims };
          return result;
        };
      },
    };
  }

  // portRuntimeは、ページが作った推論用のWorker（serveOrt）にMessagePortで頼む。
  function portRuntime(port) {
    const pending = new Map();
    let next = 0, sessionCount = 0;
    port.onmessage = ({ data }) => {
      if (!data) return;
      const entry = pending.get(data.id);
      if (!entry) return;
      pending.delete(data.id);
      if (data.error) entry.reject(new Error(data.error));
      else entry.resolve(data.outputs);
    };
    port.onmessageerror = () => {
      for (const entry of pending.values()) entry.reject(new Error("inference port message could not be decoded"));
      pending.clear();
    };
    const request = (message, transfer) => new Promise((resolve, reject) => {
      const id = next++;
      pending.set(id, { resolve, reject });
      try {
        port.postMessage({ ...message, id }, transfer || []);
      } catch (error) {
        pending.delete(id);
        reject(error);
      }
    });
    return {
      async create(data) {
        const session = sessionCount++;
        const bytes = data.slice().buffer;
        await request({ op: "create", session, bytes }, [bytes]);
        return feeds => request({ op: "run", session, feeds });
      },
    };
  }

  function loadOrt(logs) {
    if (!runtime) {
      const threads = inferenceThreads();
      runtime = globalThis.ort ? Promise.resolve(directRuntime(threads)) : requestPort().then(portRuntime);
      runtime.catch(() => { runtime = null; });
      logs.push("推論: ONNX Runtime Web（" + threads + " スレッド）");
    }
    return runtime;
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
        const data = await fetchCached(file, logs);
        status("モデルを読み込み中: " + file);
        const started = performance.now();
        const result = await (await loadOrt(logs)).create(data);
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
    const started = performance.now();
    const table = await voice(job.voice, logs);
    const ids = tensor("int64", BigInt64Array.from(job.tokens.map(BigInt)), [1, job.tokens.length]);
    const ref = tensor("float32", table.slice(job.style * 256, (job.style + 1) * 256), [1, 256]);
    let durations = job.durations;
    if (!durations || durations.length === 0) {
      const predicted = await (await session(job.model + "/duration.onnx", logs))({
        input_ids: ids, ref_s: ref, speed: tensor("float32", Float32Array.from([job.speed || 1]), [1]),
      });
      durations = Array.from(predicted.duration.data, value => Math.max(1, Math.round(value)));
    }
    const lengths = tensor("int64", BigInt64Array.from(durations.map(BigInt)), [durations.length]);
    const result = { key: job.key, durations };
    if (job.audio) {
      status("参照の音声を作成中…");
      const out = await (await session(job.model + "/synth.onnx", logs))({ input_ids: ids, ref_s: ref, pred_dur: lengths });
      result.f0 = Array.from(out.f0.data);
      result.audio_path = writeFloat32(OUT_DIR + "/read-" + number + ".f32", out.audio.data);
    } else {
      const out = await (await session(job.model + "/prosody.onnx", logs))({ input_ids: ids, ref_s: ref, pred_dur: lengths });
      result.f0 = Array.from(out.f0.data);
    }
    logs.push("参照TTS: " + job.model + "（" + job.voice + "）で読みました（" +
      (job.audio ? "音声あり、" : "") + ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
    return result;
  }

  const WHISPER_WINDOW_SAMPLES = 32000;
  const WHISPER_CENTER_FRAMES = 25;
  const WHISPER_STEP_FRAMES = 50;
  const WHISPER_HOP_SAMPLES = 320;

  async function extractWhisperPosInv(extract, samples) {
    const totalFrames = Math.max(1, Math.round(samples.length / WHISPER_HOP_SAMPLES));
    const segment = new Float32Array(WHISPER_WINDOW_SAMPLES);
    let dims = 0;
    let combined = null;
    for (let f0 = 0; f0 < totalFrames; f0 += WHISPER_STEP_FRAMES) {
      segment.fill(0);
      const start = (f0 - WHISPER_CENTER_FRAMES) * WHISPER_HOP_SAMPLES;
      const end = Math.min(samples.length, start + WHISPER_WINDOW_SAMPLES);
      const offset = Math.max(0, start);
      if (offset < end) segment.set(samples.subarray(offset, end), offset - start);
      const out = await extract({ audio: tensor("float32", segment.slice(), [1, WHISPER_WINDOW_SAMPLES]) });
      const [winRows, winDims] = out.features.dims;
      if (!combined) {
        dims = winDims;
        combined = new Float32Array(totalFrames * dims);
      }
      const take = Math.min(WHISPER_STEP_FRAMES, totalFrames - f0, winRows);
      combined.set(out.features.data.subarray(0, take * dims), f0 * dims);
    }
    return { data: combined, frames: totalFrames, dims };
  }

  async function features(jobs, logs) {
    const results = [];
    if (jobs.length === 0) return results;
    const started = performance.now();
    const encoderIDs = new Set();
    let seconds = 0, shown = -1;
    for (let i = 0; i < jobs.length; i++) {
      const job = jobs[i];
      const bytes = virtualFs.readFile(job.samples_path);
      if (!bytes) throw new Error("missing " + job.samples_path);
      const samples = new Float32Array(bytes.slice().buffer);
      virtualFs._unlink(job.samples_path);
      seconds += samples.length / 16000;
      const encoderID = job.encoder_id || "wavlm-base-plus-l6";
      encoderIDs.add(encoderID);
      const file = encoderID === "wavlm-base-plus-l6" ? "wavlm6.onnx" : encoderID + ".onnx";
      const extract = await session(file, logs);
      let data, frames, dims;
      if (encoderID === "whisper-small-l12-posinv") {
        ({ data, frames, dims } = await extractWhisperPosInv(extract, samples));
      } else {
        const out = await extract({ audio: tensor("float32", samples, [1, samples.length]) });
        [frames, dims] = out.features.dims;
        data = out.features.data;
      }
      results.push({ encoder_id: encoderID, key: job.key, path: writeFloat32(OUT_DIR + "/features-" + i + ".f32", data), frames, dims });
      const step = Math.floor((i + 1) * 10 / jobs.length);
      if (jobs.length > 1 && step !== shown) {
        shown = step;
        status("特徴量を計算中…（" + step * 10 + "%）");
      }
    }
    logs.push("特徴量: " + [...encoderIDs].join(", ") + " で " + jobs.length + " 件（音声 " + seconds.toFixed(1) + " 秒、" +
      ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
    return results;
  }

  async function prefetch(jobs, logs) {
    const keys = [];
    for (const job of jobs) {
      const started = performance.now();
      status("音源の録音を取得中…（" + job.paths.length + " 件）");
      await virtualFs.prefetchFiles(job.paths);
      logs.push("音源の録音を取得しました（" + job.paths.length + " 件、" + ((performance.now() - started) / 1000).toFixed(1) + " 秒）");
      keys.push(job.key);
    }
    return keys;
  }

  // runは、Goが返した推論の頼みごとを実行し、結果をGoへ渡す。
  async function run(jobs, call, logs) {
    const prefetched = await prefetch(jobs.filter(job => job.kind === "files"), logs);
    const reads = [];
    for (const [number, job] of jobs.entries()) {
      if (job.kind === "kokoro") reads.push(await read(job, number, logs));
    }
    const provided = await features(jobs.filter(job => job.kind === "features"), logs);
    const response = JSON.parse(call("provideInference", JSON.stringify({ reads, features: provided, prefetched })));
    if (!response.ok) throw new Error(response.error || "provideInference failed");
  }

  return { run };
}

if (typeof module === "object" && module.exports)
  module.exports = { createReferenceInference };
