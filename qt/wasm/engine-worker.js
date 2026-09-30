"use strict";

// エンジンと資産をWorkerで保持し、生成ファイルをUI側のFSミラーへ渡す。
importScripts("./asset-paths.js");
const config = JSON.parse(new URL(location.href).searchParams.get("config") || "{}");
const paths = createUtauTTSAssetPaths(location.href, config);
const ENGINE_BASE = paths.engineBaseURL;
importScripts(...[
  "wasm_exec.js", "fs-shim.js", "openjtalk-bridge.js", "world-bridge.js",
  "openjtalk/utautts-openjtalk.js", "world/utautts-world.js",
].map(file => paths.engine(file)));
const DICT_PATH = "/dict";
const MIRROR_EXTENSIONS = new Set([
  "wav", "txt", "lab", "ustx", "exo", "png", "jpg", "jpeg", "bmp", "gif",
]);

let virtualFs = null;

async function fetchAsset(url) {
  const response = await fetch(url);
  if (!response.ok) throw new Error(url + " HTTP " + response.status);
  return response;
}

function status(text) {
  self.postMessage({ type: "status", text });
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

function snapshot() {
  return virtualFs.fileVersions();
}

function changedFiles(before) {
  const files = [];
  for (const [path, version] of virtualFs.fileVersions()) {
    if (before.get(path) === version) {
      continue;
    }
    const ext = path.slice(path.lastIndexOf(".") + 1).toLowerCase();
    if (!MIRROR_EXTENSIONS.has(ext)) {
      continue;
    }
    if (path.startsWith("/voice/") && ext === "wav") continue;
    const data = virtualFs.readFile(path);
    if (!data) continue;
    const copy = data.slice();
    files.push({ path, bytes: copy.buffer });
  }
  return files;
}

async function loadOpenJTalk() {
  status("辞書を読み込み中…");
  const openjtalk = await createUtauTTSOpenJTalk({
    locateFile: (file) => ENGINE_BASE + "openjtalk/" + file,
  });
  openjtalk.FS.mkdir(DICT_PATH);
  const dictBase = paths.dictBaseURL;
  const dictManifestURL = paths.dictManifestURL;
  const manifest = await (await fetchAsset(dictManifestURL)).json();
  let loaded = 0;
  for (const name of manifest.files) {
    openjtalk.FS.writeFile(
      DICT_PATH + "/" + name,
      new Uint8Array(await (await fetchAsset(dictBase + encodeURIComponent(name))).arrayBuffer())
    );
    loaded++;
    status("辞書を読み込み中… (" + loaded + "/" + manifest.files.length + ")");
  }
  const bridge = createOpenJTalkBridge(openjtalk);
  bridge.init(DICT_PATH);
  globalThis.utauttsOpenJTalk = bridge;
}

async function loadWorld() {
  status("WORLDを読み込み中…");
  const world = await createUtauTTSWorld({
    locateFile: (file) => ENGINE_BASE + "world/" + file,
  });
  globalThis.utauttsWorld = createWorldBridge(world);
}

async function loadModels() {
  status("モデルを読み込み中…");
  const manifest = await (await fetchAsset(ENGINE_BASE + "models/manifest.json")).json();
  for (const name of manifest.models || []) {
    virtualFs.mountFile(
      "/models/" + name,
      new Uint8Array(await (await fetchAsset(ENGINE_BASE + "models/" + encodeURIComponent(name))).arrayBuffer())
    );
  }
  virtualFs.mountFile(
    "/renderer/utautts-world-phrase/renderer.json",
    new Uint8Array(await (await fetchAsset(paths.renderer("utautts-world-phrase/renderer.json"))).arrayBuffer())
  );
}

async function init() {
  status("エンジンを初期化中…");
  let voiceManifest = null;
  try {
    voiceManifest = await (await fetchAsset(ENGINE_BASE + "voice/manifest.json")).json();
  } catch (error) {
    console.warn("bundled voice manifest unavailable", error);
    voiceManifest = null;
  }
  virtualFs = installVirtualFs({
    cwd: "/",
    remote: voiceManifest && voiceManifest.files ? {
      prefix: "/voice",
      baseURL: ENGINE_BASE + "voice/",
      files: voiceManifest.files,
    } : null,
  });
  const go = new Go();
  status("エンジンWASMを取得中…");
  const response = await fetch(ENGINE_BASE + "utautts.wasm");
  if (!response.ok) {
    throw new Error("utautts.wasm HTTP " + response.status);
  }
  const bytes = await response.arrayBuffer();
  status("WASMを初期化中…");
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  await waitFor(() => globalThis.utauttsWasm && globalThis.utauttsWasm.call, 30000);

  await loadModels();

  self.postMessage({ type: "ready" });
}

let runtimePromise = null;
let runtimeKicked = false;
function ensureRuntime() {
  if (!runtimePromise) {
    runtimePromise = (async () => {
      if (!globalThis.utauttsOpenJTalk) await loadOpenJTalk();
      if (!globalThis.utauttsWorld) await loadWorld();
    })().catch(error => {
      runtimePromise = null;
      throw error;
    });
  }
  return runtimePromise;
}

function handleMountFile(path, buffer) {
  virtualFs.mountFile(path, new Uint8Array(buffer));
}

async function handleCall(id, method, requestJSON) {
  if (method === "synthesize" || method === "analyze" || method === "predictProsody") {
    try {
      await ensureRuntime();
    } catch (error) {
      self.postMessage({
        type: "callResult", id,
        response: { ok: false, error: String((error && error.message) || error) },
        files: [],
      });
      return;
    }
  }
  const before = snapshot();
  let response;
  try {
    const raw = globalThis.utauttsWasm.call(method, requestJSON || "{}");
    response = JSON.parse(raw);
  } catch (error) {
    response = { ok: false, error: String((error && error.message) || error) };
  }
  // 一覧画像は先読みしてUI側のFSミラーへ渡す。
  if (method === "voicebanks" && response && response.ok && response.result) {
    const banks = response.result.voicebanks || [];
    for (const bank of banks) {
      if (bank && bank.image_path) {
        try { virtualFs.readFile(bank.image_path); } catch (error) { /* 画像なしでも一覧を返す。 */ }
      }
    }
  }
  const files = changedFiles(before);
  const after = virtualFs.fileVersions();
  const removed = Array.from(before.keys()).filter(path => !after.has(path));
  self.postMessage(
    { type: "callResult", id, response, files, removed },
    files.map((file) => file.bytes)
  );
  // 音源一覧を待たせないよう、辞書とWORLDはメタデータ取得後に読み込む。
  if (!runtimeKicked && method === "renderers") {
    runtimeKicked = true;
    ensureRuntime().catch((error) => console.error("runtime preload failed", error));
  }
}

let callChain = Promise.resolve();

self.onmessage = (event) => {
  const data = event.data || {};
  try {
    if (data.type === "mountFile") {
      handleMountFile(data.path, data.bytes);
    } else if (data.type === "removeFile") {
      if (virtualFs && virtualFs.readFile(data.path)) virtualFs._unlink(data.path);
    } else if (data.type === "call") {
      // 非同期のランタイム読み込みを含むため直列化する。
      callChain = callChain
        .then(() => handleCall(data.id, data.method, data.request))
        .catch((error) => {
          self.postMessage({
            type: "callResult", id: data.id,
            response: { ok: false, error: String((error && error.message) || error) },
            files: [],
          });
        });
    }
  } catch (error) {
    const message = String((error && error.message) || error);
    if (data.id !== undefined) {
      self.postMessage({ type: "callResult", id: data.id, response: { ok: false, error: message }, files: [] });
    } else {
      self.postMessage({ type: "error", message });
    }
  }
};

init().catch((error) => {
  self.postMessage({ type: "error", message: String((error && error.message) || error) });
});
