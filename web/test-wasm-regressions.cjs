"use strict";

const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { createVirtualFs } = require("./fs-shim.js");
const { createUtauTTSAssetPaths } = require("../qt/wasm/asset-paths.js");
const UtteranceCore = require("../qt/qml/UtteranceCore.js");
const UnitOverrides = require("../qt/qml/UnitOverrides.js");
const QmlCore = require("../qt/qml/QmlCore.js");
const EditorValues = require("../qt/qml/EditorValues.js");
const root = path.join(__dirname, "..");
const source = name => fs.readFileSync(path.join(root, name), "utf8");

function callbackCall(vfs, method, ...args) {
  let value;
  vfs[method](...args, (error, result) => {
    if (error) throw error;
    value = result;
  });
  return value;
}

test("FS detects same-size writes and invalidates stat after both truncate forms", () => {
  const vfs = createVirtualFs();
  vfs.mountText("/tmp/out.txt", "old");
  const before = vfs.fileVersions().get("/tmp/out.txt");
  assert.equal(vfs._stat("/tmp/out.txt").size, 3);
  const fd = vfs._open("/tmp/out.txt", vfs.constants.O_RDWR);
  callbackCall(vfs, "write", fd, new TextEncoder().encode("new"), 0, 3, 0);
  assert.notEqual(vfs.fileVersions().get("/tmp/out.txt"), before);
  vfs._truncate("/tmp/out.txt", 1);
  assert.equal(vfs._stat("/tmp/out.txt").size, 1);
  callbackCall(vfs, "ftruncate", fd, 5);
  assert.equal(vfs._fstat(fd).size, 5);
});

test("FS append/exclusive creation and failed rename preserve existing data", () => {
  const vfs = createVirtualFs();
  vfs.mountText("/tmp/file", "a");
  const fd = vfs._open("/tmp/file", vfs.constants.O_WRONLY | vfs.constants.O_APPEND);
  callbackCall(vfs, "write", fd, new Uint8Array([98]), 0, 1, null);
  assert.equal(new TextDecoder().decode(vfs.readFile("/tmp/file")), "ab");
  assert.throws(() => vfs._open("/tmp/file", vfs.constants.O_CREAT | vfs.constants.O_EXCL), { code: "EEXIST" });
  assert.throws(() => vfs._rename("/tmp/file", "/missing/file"), { code: "ENOENT" });
  assert.equal(vfs._stat("/tmp/file").size, 2);
  vfs._rename("/tmp/file", "/tmp/file");
  assert.equal(vfs._stat("/tmp/file").size, 2);
});

test("FS chunked writes reuse capacity without exposing unused bytes", () => {
  const vfs = createVirtualFs();
  const fd = vfs._open("/out.wav", vfs.constants.O_WRONLY | vfs.constants.O_CREAT);
  const chunk = new Uint8Array(8192).fill(37);
  const buffers = new Set();
  for (let i = 0; i < 128; i++) {
    callbackCall(vfs, "write", fd, chunk, 0, chunk.length, null);
    const data = vfs.readFile("/out.wav");
    buffers.add(data.buffer);
    assert.equal(data.length, (i + 1) * chunk.length);
    assert.equal(vfs._fstat(fd).size, data.length);
  }
  assert.ok(buffers.size <= 8);
  assert.ok(vfs.readFile("/out.wav").every(value => value === 37));
});

test("FS growth and truncation zero holes and preserve mounted slice bounds", () => {
  const vfs = createVirtualFs();
  const original = new Uint8Array([99, 1, 2, 99]);
  vfs.mountFile("/file", original.subarray(1, 3));
  const fd = vfs._open("/file", vfs.constants.O_RDWR);
  callbackCall(vfs, "write", fd, new Uint8Array([3]), 0, 1, 4);
  assert.deepEqual(Array.from(vfs.readFile("/file")), [1, 2, 0, 0, 3]);
  assert.deepEqual(Array.from(original), [99, 1, 2, 99]);
  vfs._truncate("/file", 1);
  callbackCall(vfs, "ftruncate", fd, 5);
  assert.deepEqual(Array.from(vfs.readFile("/file")), [1, 0, 0, 0, 0]);
  const append = vfs._open("/file", vfs.constants.O_WRONLY | vfs.constants.O_APPEND);
  callbackCall(vfs, "write", append, new Uint8Array([7]), 0, 1, 0);
  assert.equal(vfs._fstat(fd).size, 6);
  vfs._open("/file", vfs.constants.O_WRONLY | vfs.constants.O_TRUNC);
  assert.equal(vfs._fstat(fd).size, 0);
  callbackCall(vfs, "write", fd, new Uint8Array([8]), 0, 1, 2);
  assert.deepEqual(Array.from(vfs.readFile("/file")), [0, 0, 8]);
});

test("remote FS directory discovery and exclusive checks do not download WAVs", () => {
  let downloads = 0;
  const previous = global.XMLHttpRequest;
  global.XMLHttpRequest = class {
    open() {}
    send() { downloads++; this.status = 200; this.response = new Uint8Array([1, 2]).buffer; }
  };
  try {
    const vfs = createVirtualFs({ remote: {
      prefix: "/voice", baseURL: "/assets/", files: { "bank/oto.ini": 2, "bank/a.wav": 2 },
    } });
    assert.deepEqual(vfs._readdir("/voice/bank"), ["oto.ini", "a.wav"]);
    assert.equal(vfs._stat("/voice/bank/a.wav").size, 2);
    vfs._open("/voice/bank", vfs.constants.O_DIRECTORY);
    assert.throws(() => vfs._open("/voice/bank/a.wav", vfs.constants.O_CREAT | vfs.constants.O_EXCL), { code: "EEXIST" });
    assert.equal(downloads, 0);
    vfs._open("/voice/bank/a.wav", 0);
    vfs._open("/voice/bank/a.wav", 0);
    assert.equal(downloads, 1);
  } finally { global.XMLHttpRequest = previous; }
});

test("remote FS prefetch is bounded, deduplicated and leaves failures retryable", async () => {
  const originalFetch = global.fetch;
  const originalXHR = global.XMLHttpRequest;
  let active = 0, maximum = 0, requests = 0, synchronous = 0;
  const files = Object.fromEntries(Array.from({length: 20}, (_, i) => ["bank/" + i + ".wav", 2]));
  global.fetch = async url => {
    requests++;
    active++;
    maximum = Math.max(maximum, active);
    await new Promise(resolve => setImmediate(resolve));
    active--;
    return {ok: !url.endsWith("19.wav"), arrayBuffer: async () => new Uint8Array([1, 2]).buffer};
  };
  global.XMLHttpRequest = class {
    open() {}
    send() { synchronous++; this.status = 200; this.response = new Uint8Array([3, 4]).buffer; }
  };
  try {
    const vfs = createVirtualFs({remote: {prefix: "/voice", baseURL: "/assets/", files}});
    vfs.mountFile("/voice/bank/0.wav", new Uint8Array([7, 8]));
    const paths = Object.keys(files).map(path => "/voice/" + path);
    await vfs.prefetchFiles([...paths, paths[1], "/voice/bank/unknown.wav"]);
    assert.equal(requests, 19);
    assert.ok(maximum > 1 && maximum <= 8);
    assert.deepEqual(Array.from(vfs.readFile(paths[0])), [7, 8]);
    assert.deepEqual(Array.from(vfs.readFile(paths[1])), [1, 2]);
    assert.equal(synchronous, 0);
    assert.deepEqual(Array.from(vfs.readFile(paths[19])), [3, 4]);
    assert.equal(synchronous, 1);
    await vfs.prefetchFiles(paths);
    assert.equal(requests, 19);
  } finally { global.fetch = originalFetch; global.XMLHttpRequest = originalXHR; }
});

test("Worker waits for candidate prefetch before synthesis and still mirrors output", async () => {
  const vfs = createVirtualFs();
  const calls = [], messages = [];
  let completePrefetch;
  vfs.prefetchFiles = paths => {
    calls.push("fetch:" + paths[0]);
    return new Promise(resolve => { completePrefetch = resolve; });
  };
  const context = vm.createContext({
    importScripts() {}, console, Uint8Array, URL, testFs: vfs,
    location: {href: "https://example.test/app/engine-worker.js"}, createUtauTTSAssetPaths,
    self: {postMessage(message) { messages.push(message); }},
    utauttsWasm: {call(method) {
      calls.push(method);
      if (method === "prefetchSources") return '{"ok":true,"result":["/voice/bank/a.wav"]}';
      vfs.mountFile("/tmp/output.wav", new Uint8Array([1, 2]));
      return '{"ok":true,"result":{}}';
    }},
  });
  const worker = source("qt/wasm/engine-worker.js");
  vm.runInContext(worker.slice(0, worker.indexOf("\ninit().catch(")), context);
  vm.runInContext("virtualFs = testFs; runtimePromise = Promise.resolve()", context);
  const pending = context.handleCall(1, "synthesize", "{}");
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls, ["prefetchSources", "fetch:/voice/bank/a.wav"]);
  completePrefetch();
  await pending;
  assert.deepEqual(calls, ["prefetchSources", "fetch:/voice/bank/a.wav", "synthesize"]);
  const result = messages.find(message => message.type === "callResult");
  assert.equal(result.files[0].path, "/tmp/output.wav");
});

test("Worker applies file messages in order around an asynchronous call", async () => {
  const vfs = createVirtualFs();
  const messages = [];
  const context = vm.createContext({
    importScripts() {}, console, Uint8Array, URL, testFs: vfs,
    location: {href: "https://example.test/app/engine-worker.js"}, createUtauTTSAssetPaths,
    self: {postMessage(message) { messages.push(message); }},
    utauttsWasm: {call() { return '{"ok":true,"result":{}}'; }},
  });
  const worker = source("qt/wasm/engine-worker.js");
  vm.runInContext(worker.slice(0, worker.indexOf("\ninit().catch(")), context);
  vm.runInContext("virtualFs = testFs", context);
  let finish;
  context.handleCall = () => new Promise(resolve => { finish = resolve; });
  context.self.onmessage({data: {type: "call", id: 1, method: "settings", request: "{}"}});
  await new Promise(resolve => setImmediate(resolve));
  context.self.onmessage({data: {type: "mountFile", path: "/tmp/late", bytes: new Uint8Array([7]).buffer}});
  assert.equal(vfs.readFile("/tmp/late"), null);
  finish();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(Array.from(vfs.readFile("/tmp/late")), [7]);
  assert.equal(messages.length, 0);
});

test("bulk overrides preserve update semantics", () => {
  const initial = [{unit_index: 3, energy_factor: 1.5}, {unit_index: 1, pitch_factor: 2}];
  const indices = [1, 3, 5, 1, -1, 2.5];
  let expected = initial;
  for (const index of indices) expected = UnitOverrides.update(expected, index, "resampler_volume", 80);
  assert.deepEqual(UnitOverrides.updateMany(initial, indices, "resampler_volume", 80), expected);
  assert.equal(EditorValues.commonUnitValue([{}, {}, {}, {}, {}, {}], expected, "resampler_volume"), 100);
});

test("unit edits preserve sparse identity through project state and editor preview", () => {
  const item = { utteranceId: "a", revision: 0, phonemeOverridesJson: "[]" };
  const context = sharedState([item]);
  UtteranceCore.updateUnitOverride(context, 3, "pitch_factor", 2);
  UtteranceCore.updateUnitOverride(context, 1, "resampler_volume", 80);
  UtteranceCore.updateUnitOverride(context, 3, "energy_factor", 1.5);
  let values = JSON.parse(item.phonemeOverridesJson);
  assert.deepEqual(values, [
    { unit_index: 3, pitch_factor: 2, energy_factor: 1.5 },
    { unit_index: 1, resampler_volume: 80 },
  ]);
  const units = [{}, {}, {}, { pitch_factor: 1 }];
  assert.equal(EditorValues.unitValue(units, values, null, 3, "pitch_factor"), 2);
  assert.equal(EditorValues.unitValue(units, values, null, 1, "resampler_volume"), 80);
  const preview = { index: 3, key: "pitch_factor", value: 2.2 };
  assert.equal(EditorValues.unitValue(units, values, preview, 3, "pitch_factor"), 2.2);
  UtteranceCore.clearUnitOverride(context, 3);
  values = JSON.parse(item.phonemeOverridesJson);
  assert.deepEqual(values, [{ unit_index: 1, resampler_volume: 80 }]);
  assert.equal(EditorValues.unitValue(units, values, null, 3, "pitch_factor"), 1);
  assert.equal(EditorValues.unitValue(units, values, null, 0, "resampler_volume"), 100);
  assert.equal(UnitOverrides.update(values, 1, "pitch_factor", NaN)[0].pitch_factor, undefined);
  assert.equal(UnitOverrides.normalize([null, { pitch_factor: 2 }])[0].unit_index, 1);
});

test("volume edits every voiced unit and resets only VOL", () => {
  const item = { utteranceId: "a", revision: 0, phonemeOverridesJson: '[{"unit_index":0,"pitch_factor":2}]' };
  const context = sharedState([item]);
  const units = [{}, { silent: true }, {}, {}];
  const indices = EditorValues.voicedUnitIndices(units);
  assert.deepEqual(indices, [0, 2, 3]);
  UtteranceCore.updateAllUnitOverrides(context, indices, "resampler_volume", 70);
  UtteranceCore.updateUnitOverride(context, 3, "resampler_volume", 120);
  let values = JSON.parse(item.phonemeOverridesJson);
  assert.equal(EditorValues.commonUnitValue(units, values, "resampler_volume"), 70);
  assert.equal(EditorValues.unitValue(units, values, null, 1, "resampler_volume"), 100);
  UtteranceCore.clearAllUnitOverrides(context, "resampler_volume");
  values = JSON.parse(item.phonemeOverridesJson);
  assert.deepEqual(values, [{ unit_index: 0, pitch_factor: 2 }]);
  assert.equal(EditorValues.commonUnitValue(units, values, "resampler_volume"), 100);
});

test("Worker mirrors same-size output rewrites, excludes source WAVs and reports deletion", async () => {
  const vfs = createVirtualFs();
  vfs.mountText("/tmp/out.txt", "old");
  const messages = [];
  const context = vm.createContext({
    importScripts() {}, console, Uint8Array, URL, testFs: vfs,
    location: { href: "https://example.test/app/engine-worker.js" }, createUtauTTSAssetPaths,
    self: { postMessage(message) { messages.push(message); } },
    utauttsWasm: { call() {
      vfs.mountText("/tmp/out.txt", "new");
      vfs.mountFile("/voice/bank/a.wav", new Uint8Array([1, 2]));
      vfs._unlink("/tmp/deleted.txt");
      return '{"ok":true,"result":{}}';
    } },
  });
  const worker = source("qt/wasm/engine-worker.js");
  vm.runInContext(worker.slice(0, worker.indexOf("\ninit().catch(")), context);
  vfs.mountText("/tmp/deleted.txt", "bye");
  vm.runInContext("virtualFs = testFs", context);
  await context.handleCall(1, "writeSidecars", "{}");
  const result = messages.find(message => message.type === "callResult");
  assert.deepEqual(Array.from(result.files, file => file.path), ["/tmp/out.txt"]);
  assert.deepEqual(Array.from(result.removed), ["/tmp/deleted.txt"]);
  assert.equal(new TextDecoder().decode(result.files[0].bytes), "new");
});

test("Worker failure after ready completes pending calls and preserves queued results", async () => {
  let worker;
  const context = vm.createContext({
    URL, URLSearchParams, Uint8Array, console: { error() {} }, createVirtualFs,
    location: { href: "https://example.test/app/index.html", search: "" },
    Worker: class { constructor() { worker = this; } postMessage() {} terminate() { this.terminated = true; } },
  });
  context.window = context;
  context.utauttsAssetPaths = createUtauTTSAssetPaths(context.location.href);
  vm.runInContext(source("qt/wasm/engine-loader.js"), context);
  worker.onmessage({ data: { type: "ready" } });
  await context.utauttsEngineReady;
  context.utauttsCallAsync("synthesize", "{}", 4);
  worker.onerror({ message: "worker crashed" });
  assert.equal(worker.terminated, true);
  assert.equal(context.utauttsPendingResults[0].id, 4);
  context.utauttsFlushPendingResults(); // Qt module not attached yet.
  assert.equal(context.utauttsPendingResults.length, 1);
  const completed = [];
  context.utauttsQtModule = { _utauttsCallCompleted() {
    completed.push({ id: context.utauttsPendingResultId, result: JSON.parse(context.utauttsPendingResultJson) });
  } };
  context.utauttsFlushPendingResults();
  assert.equal(completed[0].result.error, "worker crashed");
  context.utauttsCallAsync("synthesize", "{}", 5);
  assert.equal(completed[1].id, 5);
  assert.equal(completed[1].result.ok, false);
});

test("extended editor draws only the combined pitch curve", () => {
  const qml = source("qt/qml/editors/PhonemeEditor.qml");
  const canvas = qml.slice(qml.indexOf("id: waveformCanvas"));
  const paint = canvas.match(/onPaint: \{([\s\S]*?)\r?\n                \}\r?\n\r?\n                onWidthChanged/);
  assert.ok(paint);
  const strokes = [];
  let points = [];
  const ctx = {
    reset() {}, clearRect() {}, translate() {}, fill() {}, arc() {},
    beginPath() { points = []; },
    moveTo(x, y) { points.push([x, y]); },
    lineTo(x, y) { points.push([x, y]); },
    stroke() { strokes.push({ color: this.strokeStyle, points: points.slice() }); },
  };
  const autoFrames = [100, 200, 100];
  const manualFrames = [0, 30, 0];
  const editor = {
    autoFrames, manualFrames, frameDisplayCount: 3,
    timelineDuration: 20, leadingMargin: 0, sidePadding: 0, pixelsPerMs: 1,
    timingEditor: {}, pitchRange: 800, noteLaneH: 110,
    gridColor: "grid", axisColor: "axis", accentColor: "accent",
    exprLanes: [], morae: [], units: [], waveformMin: [], waveformMax: [],
    zoomedWidth: () => 20, frameStepMs: () => 10, frameTimeMs: i => i * 10,
    xToTime: x => x, timeToX: time => time, noteCenterY: () => 75,
    frameHasPronunciation: () => true, frameY: value => value,
    frameAutoAt: i => autoFrames[i], frameManualAt: i => manualFrames[i],
    frameTotalAt: i => autoFrames[i] + manualFrames[i],
  };
  vm.runInNewContext(paint[1], {
    root: editor, getContext: () => ctx, width: 560, height: 240, x: 0,
    timelineViewport: { contentX: 0, width: 560, contentWidth: 560 },
  });
  assert.equal(strokes.filter(stroke => stroke.color === "axis").length, 0);
  const curves = strokes.filter(stroke => stroke.color === "accent");
  assert.equal(curves.length, 1);
  assert.deepEqual(curves[0].points, [[0, 100], [10, 230], [20, 100]]);
  assert.deepEqual(autoFrames, [100, 200, 100]);
  assert.deepEqual(manualFrames, [0, 30, 0]);
});

function sharedState(items) {
  const context = {
    console,
    utterances: { get count() { return items.length; }, get(i) { return items[i]; },
      setProperty(i, key, value) { items[i][key] = value; } },
    prosodyPreviewTimer: { restart() {} },
    player: { stop() {} },
    // Qt.callLaterは実行せず、呼ばれた関数だけを記録する。
    Qt: { callLater(callback) { context.deferredCallLater.push(callback); } },
  };
  context.deferredCallLater = [];
  context.selectedIndex = 0;
  context.appBackend = { busy: false, error: "", predictProsody() {} };
  context.pendingProsodyInFlight = false;
  context.synthesisUnits = [];
  context.decodeSequence = QmlCore.decodeSequence;
  context.copySequence = QmlCore.copySequence;
  context.current = () => QmlCore.currentUtterance(context.utterances, context.selectedIndex);
  context.utteranceIndex = id => QmlCore.utteranceIndex(context.utterances, id);
  context.beginHistoryChange = () => {};
  context.markUtteranceDirty = () => {};
  context.scheduleAutoPreview = () => {};
  context.window = context;
  context.overrides = UnitOverrides;
  context.qt = context.Qt;
  return context;
}

test("shared controller rejects outdated prosody and resolves moved utterances by identity", () => {
  const items = [ { utteranceId: "a", revision: 2 }, { utteranceId: "b", revision: 0 } ];
  const context = sharedState(items);
  context.pendingProsodyRequestId = "request";
  context.pendingProsodyUtteranceId = "a";
  context.pendingProsodyRevision = 1;
  context.appBackend.prosodyRequestId = "request";
  context.appBackend.prosodyJson = '{"reading":"old","morae":[]}';
  context.applyPronunciation = (index, reading) => { items[index].reading = reading; };
  context.applyAutomaticProsody = () => {};
  context.applyAutomaticFramePitch = () => {};
  context.scheduleExtendedEditorWaveform = () => {};
  UtteranceCore.handleProsodyChanged(context);
  assert.equal(items[0].reading, undefined);
  context.pendingProsodyRevision = 2;
  items.reverse();
  UtteranceCore.handleProsodyChanged(context);
  assert.equal(items[1].reading, "old");
  assert.equal(items[0].reading, undefined);
});

test("shared controller coalesces identical prosody requests and retries busy Backend", () => {
  const context = sharedState([{ utteranceId: "a", revision: 1, content: "hello", moraeJson: "[]" }]);
  let requests = 0;
  context.buildProsodyRequest = () => ({});
  context.appBackend.predictProsody = () => requests++;
  UtteranceCore.requestProsodyPreview(context, 0);
  UtteranceCore.requestProsodyPreview(context, 0);
  assert.equal(requests, 1);
  context.utterances.get(0).revision++;
  context.appBackend.busy = true;
  UtteranceCore.requestProsodyPreview(context, 0);
  assert.equal(requests, 1);
  context.appBackend.busy = false;
  UtteranceCore.requestProsodyPreview(context, 0);
  assert.equal(requests, 2);
});

test("shared controller does not display or play synthesis from an older revision or another utterance", () => {
  const item = { utteranceId: "a", revision: 2 };
  const context = sharedState([item]);
  let plays = 0;
  context.player.source = "previous";
  context.player.play = () => plays++;
  context.updateSynthesisViewFromBackend = () => {
    context.synthesisUnits = JSON.parse(context.appBackend.synthesisJson).units;
  };
  context.appBackend.previewUrl = "generated";
  context.appBackend.synthesisJson = '{"units":[{"pitch_factor":1.4}]}';
  context.pendingUtteranceId = "a";
  context.pendingRevision = 1;
  UtteranceCore.handlePreviewReady(context);
  assert.equal(context.player.source, "previous");
  assert.equal(plays, 0);
  // 古い結果の到着で、次に届いた音声の自動再生を1回だけ止める。
  assert.equal(context.autoplayPreview, false);
  context.pendingRevision = 2;
  context.pendingUtteranceId = "b";
  UtteranceCore.handlePreviewReady(context);
  assert.equal(context.synthesisUnits.length, 0);
  context.pendingUtteranceId = "a";
  context.pendingRevision = 2;
  UtteranceCore.handlePreviewReady(context);
  assert.equal(context.player.source, "generated");
  assert.equal(context.synthesisUnits[0].pitch_factor, 1.4);
  assert.equal(plays, 0);
  assert.equal(context.autoplayPreview, true);
});
