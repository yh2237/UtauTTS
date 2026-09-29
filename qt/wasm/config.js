"use strict";

// Local build/qt-wasm and qt/wasm both sit two directories below the repo root.
// Release assembly replaces this with paths relative to the assembled site.
// Cloudflare packaging replaces it with an immutable assetBaseURL on R2.
globalThis.UtauTTSConfig = {
  engineBaseURL: "../../web/dist/",
  rendererBaseURL: "../../renderer/",
};
