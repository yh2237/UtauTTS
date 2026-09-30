"use strict";

// Exercise the real packaged Qt + Worker + Go + bridges using separate local
// origins for Pages and R2. Requires build/web-release and Playwright Chromium.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const http = require("node:http");
const { execFileSync } = require("node:child_process");
const { chromium } = require("playwright");
const root = path.resolve(__dirname, "..");
const output = path.join(root, "build", "cloudflare-smoke");
let manifest;
const requests = [];
const types = { ".wasm": "application/wasm", ".js": "application/javascript",
  ".json": "application/json", ".html": "text/html", ".wav": "audio/wav" };

function staticServer(directory, assetServer) {
  return http.createServer((request, response) => {
    let relative = decodeURIComponent(new URL(request.url, "http://localhost").pathname).slice(1);
    if (assetServer) {
      const prefix = manifest.r2_prefix + "/";
      if (!relative.startsWith(prefix)) { response.writeHead(404); response.end(); return; }
      relative = relative.slice(prefix.length);
      requests.push(relative);
      response.setHeader("Access-Control-Allow-Origin", "*");
      response.setHeader("Cache-Control", "public, max-age=31536000, immutable");
    } else if (!relative) relative = "index.html";
    const file = path.resolve(directory, relative);
    if (!file.startsWith(directory + path.sep)) { response.writeHead(400); response.end(); return; }
    fs.stat(file, (error, stat) => {
      if (error || !stat.isFile()) { response.writeHead(404); response.end(); return; }
      response.setHeader("Content-Type", types[path.extname(file)] || "application/octet-stream");
      response.setHeader("Content-Length", stat.size);
      if (request.method === "HEAD") { response.end(); return; }
      fs.createReadStream(file).pipe(response);
    });
  });
}

async function listen(server) {
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  return "http://127.0.0.1:" + server.address().port;
}

async function installTestCalls(page) {
  await page.evaluate(() => {
    const pending = new Map();
    let serial = 1000000;
    // Subscribe after the loader so its file mirror is updated first. Observing
    // real Worker messages does not replace an Emscripten-exported function.
    window.utauttsTestWorker.addEventListener("message", event => {
      const data = event.data;
      if (data.type !== "callResult") return;
      const callback = pending.get(data.id);
      if (!callback) return;
      pending.delete(data.id);
      callback(data.response);
    });
    window.utauttsTestCall = (method, request) => new Promise((resolve, reject) => {
      const id = ++serial;
      const timer = setTimeout(() => {
        pending.delete(id);
        reject(new Error(method + " timed out; engine status: " + window.utauttsEngineStatus));
      }, 120000);
      pending.set(id, result => { clearTimeout(timer); resolve(result); });
      window.utauttsCallAsync(method, JSON.stringify(request || {}), id);
    });
  });
}

async function closeStartupWindow(page) {
  await page.locator("#qt-shadow-container .qt-window").evaluateAll(windows => {
    for (const window of windows) {
      if (window.querySelector(".window-name")?.textContent === "UtauTTS") continue;
      if (getComputedStyle(window).display === "none") continue;
      const buttons = window.querySelectorAll(".title-bar .action-button");
      buttons[buttons.length - 1]?.click();
    }
  });
}

async function expectMobileLayout(page, mobile) {
  try {
    await page.waitForSelector('#screen[data-layout="' + (mobile ? "mobile" : "desktop") + '"]');
  } catch (error) {
    console.error("layout marker:", await page.locator("#screen").getAttribute("data-layout"));
    await page.screenshot({ path: path.join(output, "layout-failure.png") });
    throw error;
  }
}

async function projectSnapshot(page) {
  await closeStartupWindow(page);
  await page.evaluate(() => {
    const shadow = document.querySelector("#qt-shadow-container").shadowRoot;
    const main = Array.from(shadow.querySelectorAll(".qt-window"))
      .find(value => value.querySelector(".window-name")?.textContent === "UtauTTS");
    main.querySelector("canvas").focus();
  });
  const download = page.waitForEvent("download", { timeout: 30000 });
  await page.keyboard.press("Control+s");
  const stream = await (await download).createReadStream();
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

async function checkMobilePagesAndDrag(page) {
  await page.setViewportSize({ width: 390, height: 844 });
  await expectMobileLayout(page, true);
  await page.mouse.click(354, 28);
  await page.waitForTimeout(250);
  await page.mouse.click(140, 320); // Settings in the 16px-inset Drawer.
  await page.waitForTimeout(250);
  await page.screenshot({ path: path.join(output, "mobile-settings-insets.png") });
  await page.mouse.click(348, 24); // Common page header's close button.
  await page.mouse.click(354, 28);
  await page.waitForTimeout(250);
  await page.mouse.click(140, 376); // Dictionary settings.
  await page.waitForTimeout(250);
  await page.mouse.click(100, 748); // Add an entry in the compact action grid.
  await page.waitForTimeout(100);
  await page.screenshot({ path: path.join(output, "mobile-dictionary-insets.png") });
  await page.mouse.click(348, 24);
  await page.mouse.click(354, 28);
  await page.waitForTimeout(250);
  await page.mouse.click(140, 432); // License page.
  await page.waitForTimeout(250);
  const license = await page.locator("#qt-shadow-container").screenshot();
  await page.screenshot({ path: path.join(output, "mobile-license.png") });
  assert.ok(license.length > 0);
  await page.mouse.click(348, 24);
  const before = await projectSnapshot(page);
  const session = await page.context().newCDPSession(page);
  await session.send("Input.dispatchTouchEvent", {
    type: "touchStart", touchPoints: [{ x: 33, y: 91 }],
  });
  for (const y of [101, 111, 121, 131, 141]) {
    await session.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: 33, y }] });
    await page.waitForTimeout(40);
  }
  await session.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await session.detach();
  const after = await projectSnapshot(page);
  assert.deepEqual(after.utterances.map(row => row.text), before.utterances.map(row => row.text).reverse(),
    "mobile touch drag must reorder utterances through their voicebank images");
  await page.screenshot({ path: path.join(output, "mobile-scrollbar-position.png") });
  console.log("Mobile settings/dictionary pages and touch-only reordering verified");
  await checkMobileSaveAll(page);
}

async function checkMobileSaveAll(page) {
  const downloads = [];
  page.on("download", download => downloads.push(download.suggestedFilename()));
  await page.mouse.click(354, 28); // Mobile navigation.
  await page.waitForTimeout(250);
  await page.mouse.click(140, 264); // Save all WAV.
  // One utterance has no reading yet; batch export must analyze it before saving.
  for (let i = 0; i < 360 && downloads.length < 2; ++i) await page.waitForTimeout(500);
  if (downloads.length < 2) {
    console.error("save-all trace:", await page.evaluate(() => window.utauttsCallTrace));
    await page.screenshot({ path: path.join(output, "mobile-save-all-failure.png") });
  }
  assert.equal(downloads.length, 2, "save all must download one WAV per utterance and analyze text first");
  assert.ok(downloads.every(name => name.endsWith(".wav")), "save all must produce WAV files");
  console.log("Mobile save-all-WAV analyzed text and downloaded:", downloads.join(", "));
}

async function checkResponsiveState(page) {
  await installTestCalls(page);
  const voices = await page.evaluate(() => window.utauttsTestCall("voicebanks"));
  assert.ok(voices.ok && voices.result.voicebanks.length);
  const utterance = {
    text: "こんにちは。", language: "ja", phonemizer: "ja-kana",
    voicebank_id: voices.result.voicebanks[0].id, model_id: "none", renderer_id: "utautts-world-phrase",
    alias_policy: "cv-only", tone: "D4", color: "", mora_duration_ms: 120, pause_duration_ms: 180,
    intonation: 1.4, apply_pitch: true, speech_timing: false,
    pitch_points: [75, -40, 20, 0, 0, 0], pitch_frames: [40, -60, 15],
    mora_durations_ms: [120, 130, 110, 140, 150, 180],
    mora_positions_ms: [0, 120, 250, 360, 500, 650],
    manual_pitch_edited: true, manual_mora_duration_edited: true,
    phoneme_overrides: [{ unit_index: 2, pitch_factor: 1.7, resampler_volume: 80 }],
  };
  const project = { format: "utautts-project", format_version: 5, selected_index: 1,
    utterances: [{ ...utterance, text: "最初の発話です。" }, utterance] };
  await page.evaluate(value => {
    window.utauttsMountFile("/tmp/utautts-open-project.utautts", new TextEncoder().encode(JSON.stringify(value)));
    window.utauttsQtModule._utauttsProjectFilePicked();
    const shadow = document.querySelector("#qt-shadow-container").shadowRoot;
    window.utauttsOriginalCanvas = Array.from(shadow.querySelectorAll(".qt-window"))
      .find(value => value.querySelector(".window-name")?.textContent === "UtauTTS").querySelector("canvas");
  }, project);
  await page.evaluate(() => window.utauttsTestCall("models"));
  await page.waitForFunction(() => window.utauttsCallTrace.some(call =>
    call.method === "predictProsody" && call.ok && call.reading), null, { timeout: 120000 });
  const before = await projectSnapshot(page);
  assert.equal(before.utterances.length, 2);
  assert.equal(before.selected_index, 1);
  assert.ok(before.utterances[1].analysis_cache.reading, "selected utterance must be analyzed before playback");
  assert.deepEqual(before.utterances[1].phoneme_overrides, utterance.phoneme_overrides);
  const fields = ["text", "language", "phonemizer", "voicebank_id", "model_id", "renderer_id", "tone",
    "intonation", "alias_policy", "pitch_points", "pitch_frames", "mora_durations_ms", "mora_positions_ms",
    "manual_pitch_edited", "manual_mora_duration_edited", "phoneme_overrides"];
  function editable(value) {
    return { selected_index: value.selected_index,
      utterances: value.utterances.map(row => Object.fromEntries(fields.map(key => [key, row[key]]))) };
  }
  for (const width of [767, 768, 390, 1024]) {
    await page.setViewportSize({ width, height: 844 });
    await expectMobileLayout(page, width < 768);
    const after = await projectSnapshot(page);
    assert.deepEqual(editable(after), editable(before), "resize must preserve utterances and manual edits at " + width);
    assert.equal(await page.evaluate(() => window.utauttsOriginalCanvas.isConnected), true);
    assert.equal(await page.evaluate(() => window.utauttsTestWorkerCount), 1);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expectMobileLayout(page, true);
  await page.mouse.click(362, 141); // Second utterance's edit button.
  await page.waitForTimeout(250); // Qt popup entrance animation.
  await page.screenshot({ path: path.join(output, "responsive-sheet.png") });
  await page.setViewportSize({ width: 1024, height: 844 });
  await expectMobileLayout(page, false);
  assert.deepEqual(editable(await projectSnapshot(page)), editable(before));
  await page.setViewportSize({ width: 390, height: 844 });
  await expectMobileLayout(page, true);
  await page.mouse.click(354, 28); // Mobile navigation.
  await page.waitForTimeout(250);
  await page.screenshot({ path: path.join(output, "responsive-drawer.png") });
  await page.setViewportSize({ width: 1024, height: 844 });
  await expectMobileLayout(page, false);
  assert.deepEqual(editable(await projectSnapshot(page)), editable(before));
  const previousCalls = await page.evaluate(() => window.utauttsSynthesisCallCount || 0);
  const previousAudio = await page.evaluate(() => window.utauttsAudioUrl || "");
  await page.mouse.click(32, 820, { delay: 80 }); // Actual Qt playback control.
  try {
    await page.waitForFunction(count => (window.utauttsSynthesisCallCount || 0) > count, previousCalls);
  } catch (error) {
    console.error("Qt call trace:", await page.evaluate(() => window.utauttsCallTrace));
    await page.screenshot({ path: path.join(output, "synthesis-failure.png") });
    throw error;
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expectMobileLayout(page, true);
  await page.setViewportSize({ width: 1024, height: 844 });
  await expectMobileLayout(page, false);
  await page.waitForFunction(previous => window.utauttsAudio && window.utauttsAudioUrl !== previous
    && window.utauttsAudio.readyState >= 1 && window.utauttsAudio.duration > 0, previousAudio, { timeout: 120000 });
  await page.waitForFunction(() => window.utauttsPlaybackEnded === true, null, { timeout: 30000 });
  assert.deepEqual(editable(await projectSnapshot(page)), editable(before));
  assert.equal(await page.evaluate(() => window.utauttsTestWorkerCount), 1);
  console.log("Responsive state preserved across 767 / 768 / 390 / 1024 px");
  console.log("Qt synthesis/playback preserved while switching layouts");
  await checkMobilePagesAndDrag(page);
}

(async () => {
  const assets = staticServer(path.join(output, "r2"), true);
  const pages = staticServer(path.join(output, "pages"), false);
  let browser;
  try {
    const assetURL = await listen(assets);
    const pagesURL = await listen(pages);
    const revision = execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim();
    execFileSync(process.env.PYTHON || "python", ["qt/wasm/cloudflare.py", "package",
      "--output", output, "--public-url", assetURL, "--deployment-id", "browser-smoke",
      "--version", "smoke", "--revision", revision, "--channel", "previews"],
    { cwd: root, stdio: "inherit" });
    manifest = JSON.parse(fs.readFileSync(path.join(output, "upload-manifest.json"), "utf8"));
    browser = await chromium.launch({ headless: true, args: [
      "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader",
    ] });
    for (const mode of ["desktop", "mobile", "responsive", "automatic-mobile"]) {
      const mobile = mode === "mobile" || mode === "automatic-mobile";
      const page = await browser.newPage({ locale: "ja-JP",
        viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 } });
      const errors = [];
      page.on("pageerror", error => { errors.push(error.message); console.error("browser exception:", error.stack); });
      page.on("console", message => {
        if (message.type() === "error") console.error("browser:", message.text());
        if (/ReferenceError|TypeError|Binding loop detected|Detected anchors/.test(message.text()))
          errors.push(message.text());
      });
      await page.addInitScript(() => {
        const NativeWorker = window.Worker;
        let audio;
        Object.defineProperty(window, "utauttsAudio", {
          configurable: true,
          get: () => audio,
          set: value => {
            audio = value;
            value.addEventListener("ended", () => { window.utauttsPlaybackEnded = true; });
          },
        });
        const methods = new Map();
        window.utauttsCallTrace = [];
        window.Worker = class extends NativeWorker {
          constructor(url, options) {
            super(url, options);
            if (String(url).includes("engine-worker.js")) {
              window.utauttsTestWorker = this;
              window.utauttsTestWorkerCount = (window.utauttsTestWorkerCount || 0) + 1;
              this.addEventListener("message", event => {
                if (event.data.type !== "callResult") return;
                const response = event.data.response;
                window.utauttsCallTrace.push({ method: methods.get(event.data.id), ok: response.ok,
                  error: response.error, reading: response.result?.reading });
              });
            }
          }
          postMessage(message, ...options) {
            if (message.type === "call") methods.set(message.id, message.method);
            if (message.type === "call" && message.method === "synthesize")
              window.utauttsSynthesisCallCount = (window.utauttsSynthesisCallCount || 0) + 1;
            return super.postMessage(message, ...options);
          }
        };
      });
      await page.goto(pagesURL + (mode === "responsive" || mode === "automatic-mobile" ? "/" : mobile ? "/?mobile=1" : "/?mobile=0"));
      try {
        await page.waitForSelector("#loading.hidden", { state: "attached", timeout: 180000 });
      } catch (error) {
        throw new Error("Qt startup failed: " + await page.locator("#status").textContent(), { cause: error });
      }
      await page.waitForFunction(() => window.utauttsQtModule && window.utauttsQtModule._utauttsCallCompleted);
      assert.deepEqual(errors, [], "startup JavaScript errors");
      await page.waitForFunction(() => Array.from(document.fonts).some(font =>
        font.family === "LINE Seed JP" && font.status === "loaded"));
      const titleFont = await page.locator("#qt-shadow-container .window-name").first()
        .evaluate(element => getComputedStyle(element).fontFamily);
      assert.ok(titleFont.includes("LINE Seed JP"), "Qt's shadow-root window title must use LINE Seed JP");
      console.log("Qt booted:", mode);
      await closeStartupWindow(page);
      await expectMobileLayout(page, mobile);
      if (mode === "responsive") await checkResponsiveState(page);
      else if (mode === "automatic-mobile") {
        await page.setViewportSize({ width: 1024, height: 844 });
        await expectMobileLayout(page, false);
        await closeStartupWindow(page);
        await page.setViewportSize({ width: 390, height: 844 });
        await expectMobileLayout(page, true);
      } else {
        await page.setViewportSize({ width: mobile ? 1024 : 390, height: 844 });
        await expectMobileLayout(page, mobile); // URL overrides remain fixed while resizing.
      }
      if (mode === "mobile") {
        await installTestCalls(page);
        const result = await page.evaluate(async () => {
          const voices = await window.utauttsTestCall("voicebanks");
          const models = await window.utauttsTestCall("models");
          if (!voices.ok || !models.ok) throw new Error(JSON.stringify({ voices, models }));
          const voice = voices.result.voicebanks[0];
          const model = models.result.models.find(value => String(value.id).includes("v9.1")) || models.result.models[0];
          const request = { text: "こんにちは。", language: "ja", phonemizer: "ja-kana",
            voicebank_id: voice.id, model_id: model.id, renderer: "utautts-world-phrase", tone: "C4" };
          const prosody = await window.utauttsTestCall("predictProsody", request);
          if (!prosody.ok) throw new Error(JSON.stringify(prosody));
          const audio = await window.utauttsTestCall("synthesize", { ...request, output_path: "/tmp/deploy-smoke.wav" });
          if (!audio.ok) throw new Error(JSON.stringify(audio));
          const bytes = window.utauttsFs.readFile("/tmp/deploy-smoke.wav");
          return { wavSize: bytes && bytes.length,
            header: bytes && String.fromCharCode(...bytes.slice(0, 4)),
            units: audio.result.units.length,
            sourceWavsMirrored: audio.result.units.some(unit => unit.source && window.utauttsFs.exists(unit.source)) };
        });
        assert.equal(result.header, "RIFF");
        assert.ok(result.wavSize > 44);
        assert.ok(result.units > 0);
        assert.equal(result.sourceWavsMirrored, false);
        assert.deepEqual(errors, [], "synthesis JavaScript errors");
        console.log("Worker prosody and synthesis:", result);
      }
      assert.deepEqual(errors, [], "layout JavaScript/QML errors");
      await page.close();
    }
    assert.ok(requests.includes("utautts.wasm"), "Qt wasm must load from the asset origin");
    assert.ok(requests.includes("web/dist/openjtalk/dict/sys.dic"));
    assert.ok(requests.some(file => file.startsWith("web/dist/voice/") && file.endsWith(".wav")));
    console.log("Pages/R2 browser smoke test passed");
  } finally {
    if (browser) await browser.close();
    await Promise.all([assets, pages].map(server => new Promise(resolve => server.close(resolve))));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
