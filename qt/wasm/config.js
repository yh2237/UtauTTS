"use strict";

// 開発時はリポジトリ基準。配布時にサイト相対パスまたはR2の世代別URLへ置き換える。
globalThis.UtauTTSConfig = {
  engineBaseURL: "../../web/dist/",
  rendererBaseURL: "../../renderer/",
};
