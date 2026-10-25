import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';

import './theme/theme.css';
import { initTheme } from './theme';
import App from './App.vue';
import router from './router';
import { i18nPlugin } from './i18n';

// Initialize Theme (light / dark)
initTheme();

const app = createApp(App);

// Register i18n
app.use(i18nPlugin);

// Register Pinia
const pinia = createPinia();
app.use(pinia);

// Register Router
app.use(router);

// Register Element Plus
app.use(ElementPlus);

// Register Element Plus Icons globally
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.mount('#app');
