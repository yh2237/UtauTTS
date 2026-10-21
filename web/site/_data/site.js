export const title = 'utautts.yh.contact'

export const linkbar = [
  {
    label: 'PAGES',
    href: '/',
    items: [
      { label: 'ABOUT', href: '/#about' },
      { label: 'DOWNLOADS', href: '/#downloads' },
      { label: 'TERMS', href: '/terms' },
    ],
  },
  {
    label: 'DESKTOP',
    href: '/desktop',
    items: [
      { label: 'ABOUT', href: '/desktop#about' },
      { label: 'INSTALL', href: '/desktop#install' },
      { label: 'USAGE', href: '/desktop#usage' },
      { label: 'VOICEBANKS', href: '/desktop#voicebanks' },
      { label: 'DOCS', href: '/desktop#docs' }
    ],
  },
  {
    label: 'WEB',
    href: '/web',
    items: [
      { label: 'ABOUT', href: '/web#about' },
      { label: 'USAGE', href: '/web#usage' },
      { label: 'VOICEBANKS', href: '/web#voicebanks' },
      { label: 'SAVE', href: '/web#save' },
    ],
  },
  {
    label: 'CONTACT',
    href: '/contact',
    items: [
      { label: 'EMAIL', href: 'mailto:hi@yh.contact' },
      { label: 'TWITTER', href: 'https://twitter.com/2237yh', external: true },
      { label: 'GITHUB', href: 'https://github.com/yh2237', external: true },
      { label: 'BLUESKY', href: 'https://bsky.app/profile/0yh.dev', external: true },
    ],
  },
]

export const footerLinks = [
  { label: 'PAGES', href: '/' },
  { label: 'DESKTOP', href: '/desktop' },
  { label: 'WEB', href: '/web' },
  { label: 'CONTACT', href: '/contact' },
  { label: 'TERMS', href: '/terms' },
]

export const copyright = 'yh'

export default {
  title,
  linkbar,
  footerLinks,
  copyright,
}
