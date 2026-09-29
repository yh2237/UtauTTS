"use strict";

// 配信先で辞書を差し替えられるようにする設定。
// Cloudflare等ではR2のURLを指定し、リポジトリ/Pagesには102MBの辞書を置かない。
globalThis.UtauTTSConfig = {
  dictBaseURL: "./openjtalk/dict/",
  dictManifestURL: "./openjtalk/dict-manifest.json",
};
