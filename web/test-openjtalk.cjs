// Node上でOpen JTalk wasmのフロントエンドを検証する。web/build-openjtalk.ps1 実行後に使う。
"use strict";

const fs = require("fs");
const path = require("path");

const FACTORY = path.join(__dirname, "dist", "openjtalk", "utautts-openjtalk.js");
const DICT_DIR =
  process.env.OPENJTALK_DICT ||
  path.join(__dirname, "..", ".tmp-openjtalk", "pyopenjtalk", "open_jtalk_dic_utf_8-1.11");

(async () => {
  if (!fs.existsSync(DICT_DIR)) {
    throw new Error("dictionary not found: " + DICT_DIR);
  }
  const createUtauTTSOpenJTalk = require(FACTORY);
  const Module = await createUtauTTSOpenJTalk();

  // 辞書を仮想FSへ書き込む。ブラウザではfetch→FS.writeFileで同じことをする。
  Module.FS.mkdir("/dict");
  let copied = 0;
  for (const name of fs.readdirSync(DICT_DIR)) {
    const full = path.join(DICT_DIR, name);
    if (!fs.statSync(full).isFile()) continue;
    Module.FS.writeFile("/dict/" + name, new Uint8Array(fs.readFileSync(full)));
    copied++;
  }
  console.log("dictionary files:", copied);

  const init = Module.cwrap("UtauTTSOpenJTalkInit", "number", ["string"]);
  if (!init("/dict")) {
    const error = Module.cwrap("UtauTTSOpenJTalkError", "string", [])();
    throw new Error("UtauTTSOpenJTalkInit failed: " + error);
  }

  const run = Module.cwrap("UtauTTSOpenJTalkRun", "number", ["string", "number", "number"]);
  const capacity = 1 << 18;
  const outPtr = Module._malloc(capacity);
  const text = process.argv[2] || "こんにちは、今日はいい天気です。";
  const written = run(text, outPtr, capacity);
  if (written <= 0) {
    const error = Module.cwrap("UtauTTSOpenJTalkError", "string", [])();
    throw new Error("UtauTTSOpenJTalkRun failed: " + error);
  }
  const tsv = Module.UTF8ToString(outPtr, written);
  Module._free(outPtr);

  const reportPath = path.join(__dirname, "dist", "openjtalk", "njd.tsv");
  fs.writeFileSync(reportPath, tsv, "utf8");

  const lines = tsv.split("\n").filter((line) => line.length > 0);
  const fieldCounts = new Set(lines.map((line) => line.split("\t").length));
  console.log("nodes:", lines.length, "fieldCounts:", [...fieldCounts].join(","));
  console.log("wrote:", reportPath);
  process.exit(0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
