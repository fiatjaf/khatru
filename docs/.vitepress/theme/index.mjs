import DefaultTheme from 'vitepress/theme'
import NostrifyLayout from './Layout.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  Layout: NostrifyLayout
}
