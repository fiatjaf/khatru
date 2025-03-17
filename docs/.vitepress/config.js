export default {
  lang: 'en-US',
  title: 'khatru',
  description: 'a framework for making Nostr relays',
  themeConfig: {
    logo: '/logo.png',
    nav: [
      {text: 'Home', link: '/'},
      {text: 'Why', link: '/why'},
      {text: 'Docs', link: '/getting-started'},
      {text: 'Source', link: 'https://github.com/fiatjaf/khatru'}
    ],
    sidebar: [
      {
        text: 'Core Concepts',
        items: [
          { text: 'Event Storage', link: '/core/eventstore' },
          { text: 'Authentication', link: '/core/auth' },
          { text: 'HTTP Integration', link: '/core/embed' },
          { text: 'Request Routing', link: '/core/routing' },
          { text: 'Management API', link: '/core/management' },
          { text: 'Media Storage (Blossom)', link: '/core/blossom' },
        ]
      },
      {
        text: 'Cookbook',
        items: [
          { text: 'Search', link: '/cookbook/search' },
          { text: 'Dynamic Relays', link: '/cookbook/dynamic' },
          { text: 'Generating Events Live', link: '/cookbook/custom-live-events' },
          { text: 'Custom Stores', link: '/cookbook/custom-stores' },
          { text: 'Using something like Google Drive', link: '/cookbook/google-drive' },
        ]
      }
    ],
    editLink: {
      pattern: 'https://github.com/fiatjaf/khatru/edit/master/docs/:path'
    }
  },
  head: [['link', {rel: 'icon', href: '/logo.png'}]],
  cleanUrls: true
}
