# Webサイト

[profile](https://github.com/yh2237/profile/tree/c57767a6cdd45689aae1c2a9cd0cc6b3e0ac9408/src)のデザインをもとにしたUtauTTSのサイトです。Eleventyで静的HTMLを生成します。

## 開発とビルド

Node.js 18以上を使います。リポジトリのルートで実行してください。

```sh
npm ci --prefix web/site
npm run dev --prefix web/site
```

`dev`はEleventyで静的HTMLを生成し、ローカルプレビューを起動します。

```sh
npm run build --prefix web/site
npm test --prefix web/site
```

出力先は`web/site/dist/`です。Cloudflare Pagesへの公開では、この内容をルートに置き、Web版エディタは従来どおり`/editor/`に配置します。

## 編集する場所

- 各ページ: `index.njk`、`desktop/`、`web/`、`terms/`、`contact/`、`404.njk`
- 共通レイアウト: `_includes/`
- ナビゲーションなどの共通データ: `_data/site.js`
- スタイル・スクリプト・画像: `css/`、`js/`、`img/`

旧VitePressの本文・設定・画像は`vitepress/`に退避しています。参考用で、ビルド・公開の対象には含めません。
