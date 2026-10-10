// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	integrations: [
		starlight({
			title: 'UtauTTS',
			description: 'UTAU音源で文章を読み上げる、ブラウザで動くTTSエディタ',
			logo: { src: './src/assets/icon.png' },
			favicon: '/favicon.png',
			defaultLocale: 'root',
			locales: { root: { label: '日本語', lang: 'ja' } },
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/yh2237/UtauTTS' }],
			customCss: ['./src/styles/theme.css'],
			sidebar: [
				{ label: 'エディタを開く', link: '/editor/', attrs: { target: '_self' } },
				{
					label: '使い方',
					items: [
						{ label: 'はじめに', slug: 'getting-started' },
						{ label: '基本の使い方', slug: 'basics' },
						{ label: '抑揚と長さの編集', slug: 'editing' },
						{ label: 'スマートフォン・タブレット', slug: 'mobile' },
					],
				},
				{ label: 'よくある質問と利用条件', slug: 'faq' },
			],
		}),
	],
});
