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
    for (const mobile of [false, true]) {
      const page = await browser.newPage({ viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 } });
      const errors = [];
      page.on("pageerror", error => errors.push(error.message));
      page.on("console", message => {
        if (message.type() === "error") console.error("browser:", message.text());
      });
      await page.addInitScript(() => {
        const NativeWorker = window.Worker;
        window.Worker = class extends NativeWorker {
          constructor(url, options) {
            super(url, options);
            if (String(url).includes("engine-worker.js")) window.utauttsTestWorker = this;
          }
        };
      });
      await page.goto(pagesURL + (mobile ? "/?mobile=1" : "/?mobile=0"));
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
      console.log("Qt booted:", mobile ? "mobile" : "desktop");
      if (mobile) {
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
