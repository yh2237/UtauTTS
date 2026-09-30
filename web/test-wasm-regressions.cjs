"use strict";

const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { createVirtualFs } = require("./fs-shim.js");
const { createUtauTTSAssetPaths } = require("../qt/wasm/asset-paths.js");
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

test("shared overrides preserve unit identity, merge parameters and remove without holes", () => {
  const context = vm.createContext({});
  vm.runInContext(source("qt/qml/UnitOverrides.js"), context);
  let values = context.update([], 3, "pitch_factor", 2);
  values = context.update(values, 1, "resampler_volume", 80);
  values = context.update(values, 3, "energy_factor", 1.5);
  assert.deepEqual(JSON.parse(JSON.stringify(values)), [
    { unit_index: 3, pitch_factor: 2, energy_factor: 1.5 },
    { unit_index: 1, resampler_volume: 80 },
  ]);
  values = context.remove(values, 3);
  assert.deepEqual(JSON.parse(JSON.stringify(values)), [{ unit_index: 1, resampler_volume: 80 }]);
  assert.equal(context.update(values, 1, "pitch_factor", NaN)[0].pitch_factor, undefined);
  assert.equal(context.normalize([null, { pitch_factor: 2 }])[0].unit_index, 1);
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
    Worker: class { constructor() { worker = this; } postMessage() {} },
  });
  context.window = context;
  context.utauttsAssetPaths = createUtauTTSAssetPaths(context.location.href);
  vm.runInContext(source("qt/wasm/engine-loader.js"), context);
  worker.onmessage({ data: { type: "ready" } });
  await context.utauttsEngineReady;
  context.utauttsCallAsync("synthesize", "{}", 4);
  worker.onerror({ message: "worker crashed" });
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

function sharedState(items) {
  const context = vm.createContext({
    console, UnitOverrides: {},
    utterances: { get count() { return items.length; }, get(i) { return items[i]; },
      setProperty(i, key, value) { items[i][key] = value; } },
    prosodyPreviewTimer: { restart() {} },
    player: { stop() {} },
  });
  const overrides = vm.createContext({});
  vm.runInContext(source("qt/qml/UnitOverrides.js"), overrides);
  context.UnitOverrides = overrides;
  context.window = context;
  context.selectedIndex = 0;
  context.appBackend = { busy: false, error: "", predictProsody() {} };
  context.pendingProsodyInFlight = false;
  context.synthesisUnits = [];
  const qml = source("qt/qml/Main.qml");
  for (const match of qml.matchAll(/^    function [\s\S]*?^    }/gm))
    vm.runInContext(match[0], context);
  context.beginHistoryChange = () => {};
  context.markUtteranceDirty = () => {};
  context.scheduleAutoPreview = () => {};
  return context;
}

test("both layouts update and reset the same canonical unit overrides", () => {
  const item = { utteranceId: "a", revision: 0, phonemeOverridesJson: "[]" };
  const context = sharedState([item]);
  context.updateUnitOverride(2, "pitch_factor", 1.7);
  assert.deepEqual(JSON.parse(item.phonemeOverridesJson), [{ unit_index: 2, pitch_factor: 1.7 }]);
  context.clearUnitOverride(2);
  assert.deepEqual(JSON.parse(item.phonemeOverridesJson), []);
});

test("shared PhonemeEditor reads canonical overrides and live preview before synthesis values", () => {
  const context = vm.createContext({});
  context.root = context;
  const qml = source("qt/qml/editors/PhonemeEditor.qml");
  for (const name of ["unitAt", "overrideAt", "unitValue", "paramRange"]) {
    const match = qml.match(new RegExp("^    function " + name + "\\([\\s\\S]*?^    }", "m"));
    vm.runInContext(match[0], context);
  }
  context.units = [{ pitch_factor: 1 }, {}, {}];
  context.overrides = [{ unit_index: 2, pitch_factor: 1.7 }];
  assert.equal(context.unitValue(2, "pitch_factor"), 1.7);
  assert.equal(context.unitValue(1, "pitch_factor"), 1);
  assert.equal(context.unitValue(1, "resampler_volume"), 100);
  context.previewUnit = { index: 2, key: "pitch_factor", value: 2.2 };
  assert.equal(context.unitValue(2, "pitch_factor"), 2.2);
  context.previewUnit = null;
  assert.equal(context.unitValue(2, "pitch_factor"), 1.7);
});

test("shared controller rejects outdated prosody and resolves moved utterances by identity", () => {
  const items = [ { utteranceId: "a", revision: 2 }, { utteranceId: "b", revision: 0 } ];
  const context = sharedState(items);
  vm.runInContext(source("qt/qml/Main.qml").match(/^        function onProsodyChanged\([\s\S]*?^        }/m)[0], context);
  context.pendingProsodyRequestId = "request";
  context.pendingProsodyUtteranceId = "a";
  context.pendingProsodyRevision = 1;
  context.appBackend.prosodyRequestId = "request";
  context.appBackend.prosodyJson = '{"reading":"old","morae":[]}';
  context.applyPronunciation = (index, reading) => { items[index].reading = reading; };
  context.applyAutomaticProsody = () => {};
  context.applyAutomaticFramePitch = () => {};
  context.scheduleExtendedEditorWaveform = () => {};
  context.onProsodyChanged();
  assert.equal(items[0].reading, undefined);
  context.pendingProsodyRevision = 2;
  items.reverse();
  context.onProsodyChanged();
  assert.equal(items[1].reading, "old");
  assert.equal(items[0].reading, undefined);
});

test("shared controller coalesces identical prosody requests and retries busy Backend", () => {
  const context = sharedState([{ utteranceId: "a", revision: 1, content: "hello", moraeJson: "[]" }]);
  let requests = 0;
  context.buildProsodyRequest = () => ({});
  context.appBackend.predictProsody = () => requests++;
  context.requestProsodyPreview(0);
  context.requestProsodyPreview(0);
  assert.equal(requests, 1);
  context.utterances.get(0).revision++;
  context.appBackend.busy = true;
  context.requestProsodyPreview(0);
  assert.equal(requests, 1);
  context.appBackend.busy = false;
  context.requestProsodyPreview(0);
  assert.equal(requests, 2);
});

test("shared controller does not display or play synthesis from an older revision or another utterance", () => {
  const item = { utteranceId: "a", revision: 2 };
  const context = sharedState([item]);
  const handler = source("qt/qml/Main.qml")
    .match(/^        function onPreviewReady\([\s\S]*?^        }/m)[0];
  vm.runInContext(handler, context);
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
  context.onPreviewReady();
  assert.equal(context.player.source, "previous");
  assert.equal(plays, 0);
  context.pendingRevision = 2;
  context.pendingUtteranceId = "b";
  context.onPreviewReady();
  assert.equal(context.synthesisUnits.length, 0);
  context.pendingUtteranceId = "a";
  context.pendingRevision = 2;
  context.onPreviewReady();
  assert.equal(context.player.source, "generated");
  assert.equal(context.synthesisUnits[0].pitch_factor, 1.4);
  assert.equal(plays, 1);
});
