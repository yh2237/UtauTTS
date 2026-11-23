// Web版の参照照合を node で通す。Goが頼む推論をONNX Runtime Webで求め、渡してやり直す流れを確かめる。
// 実行前に web/build（UTAUTTS_WEB_REFERENCE 付き）、build-openjtalk、build-world で資産を用意する。
// ONNX Runtime Web は ORT_WEB_MODULE（node_modules/onnxruntime-web）から読む。
"use strict";

const fs = require("fs");
const path = require("path");
const assert = require("node:assert/strict");

const DIST = path.join(__dirname, "dist");
const ROOT = path.join(__dirname, "..");
const REFERENCE_DIR = process.env.UTAUTTS_TEST_REFERENCE_DIR || path.join(DIST, "reference");
require(path.join(DIST, "wasm_exec.js"));
const { installVirtualFs } = require(path.join(__dirname, "fs-shim.js"));
const { createWorldBridge } = require(path.join(__dirname, "world-bridge.js"));
const { createOpenJTalkBridge } = require(path.join(__dirname, "openjtalk-bridge.js"));
const { createReferenceInference } = require(path.join(__dirname, "reference-inference.js"));

const VOICE_DIR = process.env.UTAUTTS_TEST_VOICE_DIR || path.join(ROOT, "voice",
  process.env.UTAUTTS_TEST_VOICE || fs.readdirSync(path.join(ROOT, "voice"), { withFileTypes: true }).find(e => e.isDirectory()).name);
const VOICE_NAME = path.basename(VOICE_DIR);
const OUT_DIR = process.env.UTAUTTS_TEST_OUT_DIR || path.join(DIST, "out");

function mountTree(virtualFs, hostDir, guestDir) {
  for (const entry of fs.readdirSync(hostDir, { withFileTypes: true })) {
    const host = path.join(hostDir, entry.name);
    if (entry.isDirectory()) mountTree(virtualFs, host, guestDir + "/" + entry.name);
    else virtualFs.mountFile(guestDir + "/" + entry.name, new Uint8Array(fs.readFileSync(host)));
  }
}

(async () => {
  globalThis.ort = require(process.env.ORT_WEB_MODULE || "onnxruntime-web");
  // モデルはテストのフォルダから直接読む（ブラウザでは配信元から取る）。
  const baseURL = "test-reference:/";
  const realFetch = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    if (!String(url).startsWith(baseURL)) return realFetch(url, init);
    const file = path.join(REFERENCE_DIR, decodeURIComponent(String(url).slice(baseURL.length)));
    if (!fs.existsSync(file)) return new Response(null, { status: 404 });
    return new Response(fs.readFileSync(file), { headers: { "Content-Length": String(fs.statSync(file).size) } });
  };

  const virtualFs = installVirtualFs({ cwd: "/" });
  virtualFs.mountFile("/models/frame-intonation-tcn-v12.json", new Uint8Array(fs.readFileSync(path.join(ROOT, "models", "frame-intonation-tcn-v12.json"))));
  mountTree(virtualFs, path.join(REFERENCE_DIR, "fs"), "/models/reference");
  mountTree(virtualFs, VOICE_DIR, "/voice/" + VOICE_NAME);

  globalThis.utauttsWorld = createWorldBridge(await require(path.join(DIST, "world", "utautts-world.js"))());
  const jtalk = await require(path.join(DIST, "openjtalk", "utautts-openjtalk.js"))();
  jtalk.FS.mkdir("/dict");
  const manifest = JSON.parse(fs.readFileSync(path.join(DIST, "openjtalk", "dict-manifest.json"), "utf8").replace(/^﻿/, ""));
  for (const name of manifest.files) jtalk.FS.writeFile("/dict/" + name, new Uint8Array(fs.readFileSync(path.join(DIST, "openjtalk", "dict", name))));
  const bridge = createOpenJTalkBridge(jtalk);
  bridge.init("/dict");
  globalThis.utauttsOpenJTalk = bridge;

  const go = new Go();
  go.env = { TMPDIR: "/tmp", XDG_CACHE_HOME: "/cache" };
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(path.join(DIST, "utautts.wasm")), go.importObject);
  go.run(instance);
  while (!(globalThis.utauttsWasm && globalThis.utauttsWasm.call)) await new Promise(r => setTimeout(r, 20));
  const call = (method, json) => globalThis.utauttsWasm.call(method, json || "{}");
  const result = (method, request) => {
    const parsed = JSON.parse(call(method, JSON.stringify(request)));
    if (!parsed.ok) throw new Error(method + ": " + parsed.error);
    return parsed.result;
  };
  const inference = createReferenceInference({ baseURL, virtualFs, status: text => console.log("status:", text) });

  const providers = result("referenceProviders", {}).reference_providers.map(p => p.language);
  assert.ok(providers.includes("en"), "providers=" + providers);
  const voicebank = result("voicebanks", {}).voicebanks[0].id;
  const texts = process.argv.length > 2 ? process.argv.slice(2) : ["Could you help me with this box?"];
  fs.mkdirSync(OUT_DIR, { recursive: true });
  for (const [number, text] of texts.entries()) {
    const request = { text, language: "en", method: "reference-matching", voicebank_id: voicebank, tone: "C4", output_path: "/out/reference.wav" };
    const logs = [];
    let response = JSON.parse(call("synthesize", JSON.stringify(request)));
    let rounds = 0;
    const started = Date.now();
    while (Array.isArray(response.need) && rounds < 12) {
      console.log("round", ++rounds, response.need.length, "jobs:", [...new Set(response.need.map(job => job.kind))].join(","));
      await inference.run(response.need, call, logs);
      response = JSON.parse(call("synthesize", JSON.stringify(request)));
    }
    if (!response.ok) throw new Error(response.error);
    for (const line of logs.concat(response.result.logs || [])) console.log("log:", line);
    const wav = virtualFs.readFile("/out/reference.wav");
    assert.ok(wav && wav.length > 44, "no WAV");
    const name = texts.length > 1 ? "reference-" + (number + 1) + ".wav" : "reference.wav";
    fs.writeFileSync(path.join(OUT_DIR, name), Buffer.from(wav));
    console.log(name, "rounds", rounds, "seconds", ((Date.now() - started) / 1000).toFixed(1), "wav bytes", wav.length);
  }
  if (process.env.UTAUTTS_VERIFY_CACHE_HIT === "1") {
    const cachedReq = { text: "This box is very helpful.", language: "en", method: "reference-matching", voicebank_id: voicebank, tone: "C4", output_path: "/out/reference-cached.wav" };
    const cachedLogs = [];
    let resp2 = JSON.parse(call("synthesize", JSON.stringify(cachedReq)));
    let rounds2 = 0;
    const t2 = Date.now();
    while (Array.isArray(resp2.need) && rounds2 < 8) {
      const kinds = [...new Set(resp2.need.map(j => j.kind))];
      assert.ok(!kinds.includes("files") && resp2.need.length <= 4, "cache miss on 2nd synthesis: " + JSON.stringify(resp2.need.map(j => ({ kind: j.kind, enc: j.encoder_id }))));
      console.log("cache-hit round", ++rounds2, resp2.need.length, "jobs:", kinds.join(","));
      await inference.run(resp2.need, call, cachedLogs);
      resp2 = JSON.parse(call("synthesize", JSON.stringify(cachedReq)));
    }
    assert.ok(resp2.ok, resp2.error || "cache-hit synthesis failed");
    const wav2 = virtualFs.readFile("/out/reference-cached.wav");
    assert.ok(wav2 && wav2.length > 44, "no cached WAV");
    fs.writeFileSync(path.join(OUT_DIR, "reference-cached.wav"), Buffer.from(wav2));
    console.log("cache-hit ok: rounds", rounds2, "seconds", ((Date.now() - t2) / 1000).toFixed(1), "logs:", (resp2.result.logs || []).find(l => l.includes("キャッシュ")) || "");
  }
  process.exit(0);
})().catch(error => {
  console.error(error);
  process.exit(1);
});
