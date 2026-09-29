"use strict";

// Go エンジン一式（Go wasm・Open JTalk・WORLD・モデル・音源）を Worker 内で動かし、
// メインスレッドからは postMessage で非同期に呼び出す。
// 生成されたファイルはメインのFSミラーへ転送し、同期読み出し（再生・保存・画像）を成立させる。
importScripts(
  "/web/dist/wasm_exec.js",
  "/web/dist/fs-shim.js",
  "/web/dist/openjtalk-bridge.js",
  "/web/dist/world-bridge.js",
  "/web/dist/openjtalk/utautts-openjtalk.js",
  "/web/dist/world/utautts-world.js"
);
try {
  // 辞書URL等の任意設定。無くても動く。
  importScripts("/web/dist/config.js");
} catch (error) {
  console.warn("engine-worker: config.js not loaded");
}

const ENGINE_BASE = "/web/dist/";
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
  const config = globalThis.UtauTTSConfig || {};
  const engineBase = new URL(ENGINE_BASE, location.href).toString();
  const dictBase = new URL(config.dictBaseURL || "openjtalk/dict/", engineBase).toString();
  const dictManifestURL = new URL(config.dictManifestURL || "openjtalk/dict-manifest.json", engineBase).toString();
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
    new Uint8Array(await (await fetchAsset("/renderer/utautts-world-phrase/renderer.json")).arrayBuffer())
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
  // 音源一覧を返すときは、画像を先読みしてメインのミラーへ渡せるようにする。
  if (method === "voicebanks" && response && response.ok && response.result) {
    const banks = response.result.voicebanks || [];
    for (const bank of banks) {
      if (bank && bank.image_path) {
        try { virtualFs.readFile(bank.image_path); } catch (error) { /* optional */ }
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
  // 初期メタデータ取得が終わってから、辞書/WORLDをバックグラウンドで読み込む。
  // （起動直後に読み込むと単一スレッドを占有し、音源一覧の取得が待たされる）
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
