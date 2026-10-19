import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';

import App from './App.vue';
import router from './router';

const app = createApp(App);

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
