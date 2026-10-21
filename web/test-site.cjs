"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const output = path.join(__dirname, "site", "dist");

function files(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const file = path.join(directory, entry.name);
    return entry.isDirectory() ? files(file) : [file];
  });
}

test("built website includes its entry pages and resolves local links and anchors", () => {
  for (const name of ["index.html", "404.html"]) {
    assert.ok(fs.readFileSync(path.join(output, name), "utf8").trim(), `${name} is empty`);
  }
  for (const file of files(output).filter(file => file.endsWith(".html"))) {
    const html = fs.readFileSync(file, "utf8");
    const page = path.relative(output, file).split(path.sep).join("/");
    for (const [, reference] of html.matchAll(/(?:href|src)="([^"]+)"/g)) {
      const url = new URL(reference, `https://site.invalid/${page}`);
      if (url.origin !== "https://site.invalid") continue;
      // The editor is packaged separately under this route.
      if (url.pathname === "/editor" || url.pathname.startsWith("/editor/")) continue;
      const target = path.join(output, decodeURIComponent(url.pathname));
      const resolved = [target, target + ".html", path.join(target, "index.html")]
        .find(candidate => fs.existsSync(candidate) && fs.statSync(candidate).isFile());
      assert.ok(resolved, `${page}: missing local target ${reference}`);
      if (url.hash && resolved.endsWith(".html")) {
        const destination = fs.readFileSync(resolved, "utf8");
        const ids = [...destination.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]);
        assert.ok(ids.includes(decodeURIComponent(url.hash.slice(1))), `${page}: missing anchor ${reference}`);
      }
    }
  }
});

test("reference sources and build tooling are not published", () => {
  for (const name of ["vitepress", "node_modules", "_includes", "_data", "package.json", "eleventy.config.js"]) {
    assert.ok(!fs.existsSync(path.join(output, name)), `${name} must not be in the website output`);
  }
});
