import { defineConfig } from 'vitepress'

// 日本語は語の間に空白がないので、検索の語の区切りにIntl.Segmenterを使う（索引と検索の両方で使うため外部の値を参照しない）。
function tokenize(text: string): string[] {
  const words: string[] = []
  for (const part of new Intl.Segmenter('ja', { granularity: 'word' }).segment(text)) {
    if (part.isWordLike) words.push(part.segment)
  }
  return words
}

export default defineConfig({
  lang: 'ja',
  title: 'UtauTTS',
  description: 'UTAU音源で文章を読み上げる、ブラウザで動くTTSエディタ',
  cleanUrls: true,
  outDir: './dist',
  srcExclude: ['README.md'],
  head: [['link', { rel: 'icon', type: 'image/png', href: '/favicon.png' }]],
  ignoreDeadLinks: [/^\/editor\//],
  themeConfig: {
    logo: '/favicon.png',
    nav: [{ text: 'エディタを開く', link: '/editor/', target: '_self' }],
    sidebar: [
      {
        text: '使い方',
        items: [
          { text: 'はじめに', link: '/getting-started' },
          { text: '基本の使い方', link: '/basics' },
          { text: '抑揚と長さの編集', link: '/editing' },
          { text: 'スマートフォン・タブレット', link: '/mobile' },
        ],
      },
      { text: 'よくある質問と利用条件', link: '/faq' },
    ],
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
