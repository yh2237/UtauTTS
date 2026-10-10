import { defineConfig } from 'vitepress'

// 日本語は語の間に空白がないので、検索の語の区切りにIntl.Segmenterを使う（索引と検索の両方で使うため外部の値を参照しない）。
function tokenize(text: string): string[] {
  const words: string[] = []
  for (const part of new Intl.Segmenter('ja', { granularity: 'word' }).segment(text)) {
    if (part.isWordLike) words.push(part.segment)
  }
  return words
}

// 既定のslugifyはNFKDで分解して濁点（U+3099）を分けたままにするので、「ド」などを含む見出しへのリンクが合わない。
// NFCのまま、記号だけを外して見出しのIDにする。
function slugify(text: string): string {
  return text
    .normalize('NFC')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\p{L}\p{N}\p{M}_-]/gu, '')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '')
}

const desktopSidebar = [
  {
    text: 'デスクトップ版',
    items: [
      { text: 'インストール', link: '/desktop/' },
      { text: '基本の使い方', link: '/desktop/basics' },
      { text: '抑揚と長さの編集', link: '/desktop/editing' },
      { text: '音源の追加', link: '/desktop/voicebanks' },
      { text: '書き出し', link: '/desktop/export' },
      { text: '設定', link: '/desktop/settings' },
      { text: '英語・中国語の読み上げ', link: '/desktop/languages' },
      { text: '困ったとき', link: '/desktop/troubleshooting' },
    ],
  },
  { text: '利用条件とライセンス', link: '/terms' },
]

const webSidebar = [
  {
    text: 'Web版',
    items: [
      { text: 'はじめに', link: '/web/' },
      { text: '基本の使い方', link: '/web/basics' },
      { text: '抑揚と長さの編集', link: '/web/editing' },
      { text: 'スマートフォン・タブレット', link: '/web/mobile' },
      { text: '困ったとき', link: '/web/troubleshooting' },
    ],
  },
  { text: '利用条件とライセンス', link: '/terms' },
]

export default defineConfig({
  lang: 'ja',
  title: 'UtauTTS',
  description: 'UTAU音源で文章を読み上げるTTSの使い方',
  cleanUrls: true,
  outDir: './dist',
  srcExclude: ['README.md', 'shared/**'],
  // 型はdarkしか許さないが、initialValueは保存された設定がないときの表示に使われる。ライトを既定にする。
  appearance: { initialValue: 'light' } as unknown as { initialValue: 'dark' },
  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/favicon.png' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=LINE+Seed+JP:wght@400;700&display=swap' }],
  ],
  ignoreDeadLinks: [/^\/editor\//],
  markdown: { anchor: { slugify } },
  themeConfig: {
    logo: '/favicon.png',
    nav: [
      { text: 'デスクトップ版', link: '/desktop/', activeMatch: '^/desktop/' },
      { text: 'Web版', link: '/web/', activeMatch: '^/web/' },
      { text: 'エディタを開く', link: '/editor/', target: '_self' },
    ],
    sidebar: {
      '/desktop/': desktopSidebar,
      '/web/': webSidebar,
      '/': [
        { text: 'デスクトップ版', link: '/desktop/' },
        { text: 'Web版', link: '/web/' },
        { text: '利用条件とライセンス', link: '/terms' },
      ],
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/yh2237/UtauTTS' }],
    search: {
      provider: 'local',
      options: {
        miniSearch: { options: { tokenize } },
        translations: {
          button: { buttonText: '検索', buttonAriaLabel: '検索' },
          modal: {
            displayDetails: '詳細を表示',
            resetButtonTitle: '検索をリセット',
            backButtonTitle: '検索を閉じる',
            noResultsText: '見つかりませんでした',
            footer: { selectText: '選択', navigateText: '移動', closeText: '閉じる' },
          },
        },
      },
    },
    outline: { label: 'このページの内容' },
    docFooter: { prev: '前のページ', next: '次のページ' },
    darkModeSwitchLabel: '表示',
    lightModeSwitchTitle: 'ライトモードにする',
    darkModeSwitchTitle: 'ダークモードにする',
    sidebarMenuLabel: 'メニュー',
    returnToTopLabel: 'ページの先頭へ',
    notFound: { title: 'ページが見つかりません', quote: 'URLを確かめるか、トップページから探してください。', linkText: 'トップページへ' },
  },
})
