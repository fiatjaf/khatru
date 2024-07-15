export default {
  lang: 'en-US',
  title: 'khatru',
  description: 'a framework for making Nostr relays',
  themeConfig: {
    logo: '/logo.png',
    nav: [
      {text: 'Home', link: '/'},
      {text: 'Why', link: '/why'},
      {text: 'Use Cases', link: '/use-cases'},
      {text: 'Get Started', link: '/getting-started'},
      {text: 'Cookbook', link: '/cookbook'},
      {text: 'Source', link: 'https://github.com/fiatjaf/khatru'}
    ],
    editLink: {
      pattern: 'https://github.com/fiatjaf/khatru/edit/master/docs/:path'
    }
  },
  head: [['link', {rel: 'icon', href: '/logo.png'}]],
  cleanUrls: true
}
