"use strict";

// Shared by the host page and the classic Worker. The app scripts stay on
// Pages; assetBaseURL can point at an immutable deployment prefix on R2.
function createUtauTTSAssetPaths(scriptURL, config = {}) {
  const directory = (value, base) => {
    const url = new URL(value, base);
    if (!url.pathname.endsWith("/")) url.pathname += "/";
    return url.href;
  };
  const appBaseURL = directory("./", scriptURL);
  const assetBaseURL = directory(config.assetBaseURL || "./", appBaseURL);
  const engineBaseURL = directory(config.engineBaseURL || "web/dist/", assetBaseURL);
  const rendererBaseURL = directory(config.rendererBaseURL || "renderer/", assetBaseURL);
  return Object.freeze({
    appBaseURL, assetBaseURL, engineBaseURL, rendererBaseURL,
    app: file => new URL(file, appBaseURL).href,
    asset: file => new URL(file, assetBaseURL).href,
    engine: file => new URL(file, engineBaseURL).href,
    renderer: file => new URL(file, rendererBaseURL).href,
    dictBaseURL: directory(config.dictBaseURL || "openjtalk/dict/", engineBaseURL),
    dictManifestURL: new URL(config.dictManifestURL || "openjtalk/dict-manifest.json", engineBaseURL).href,
  });
}

if (typeof module === "object" && module.exports)
  module.exports = { createUtauTTSAssetPaths };
