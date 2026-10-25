import { ref, computed, type App } from 'vue';
import zhCnLocale from 'element-plus/es/locale/lang/zh-cn';
import enLocale from 'element-plus/es/locale/lang/en';
import zhCN from './locales/zh-cn';
import en from './locales/en';
import { enPhrases } from './locales/phrases';

export type LocaleKey = 'zh-cn' | 'en';

const LOCALE_STORAGE_KEY = 'agentpay_locale';

function getInitialLocale(): LocaleKey {
  try {
    if (typeof window !== 'undefined' && window.location) {
      const params = new URLSearchParams(window.location.search);
      const urlLang = params.get('lang') || params.get('locale');
      if (urlLang === 'en' || urlLang === 'zh-cn') {
        localStorage.setItem(LOCALE_STORAGE_KEY, urlLang);
        return urlLang;
      }
    }
    const saved = localStorage.getItem(LOCALE_STORAGE_KEY);
    if (saved === 'en' || saved === 'zh-cn') {
      return saved;
    }
  } catch {
    // ignore storage or window errors
  }
  return 'en'; // default to English
}

const currentLocale = ref<LocaleKey>(getInitialLocale());
if (typeof document !== 'undefined') {
  document.documentElement.lang = currentLocale.value;
}

const messages: Record<LocaleKey, Record<string, any>> = {
  'zh-cn': zhCN,
  en: en
};

export function setLocale(lang: LocaleKey) {
  currentLocale.value = lang;
  localStorage.setItem(LOCALE_STORAGE_KEY, lang);
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lang;
  }
}

function formatParams(str: string, params?: Record<string, any>): string {
  if (!params) return str;
  return str.replace(/\{(\w+)\}/g, (_, key) => {
    return params[key] !== undefined ? String(params[key]) : `{${key}}`;
  });
}

export function t(path: string, params?: Record<string, any>): string {
  if (!path) return '';
  const lang = currentLocale.value;

  // 1. Try dot-path in structured dictionaries if it contains dots
  if (path.includes('.')) {
    const dict = messages[lang] || messages['zh-cn'];
    const keys = path.split('.');
    let current: any = dict;
    for (const k of keys) {
      if (current && typeof current === 'object' && k in current) {
        current = current[k];
      } else {
        current = undefined;
        break;
      }
    }
    if (typeof current === 'string') {
      return formatParams(current, params);
    }
    // Fallback to zh-cn
    if (lang !== 'zh-cn') {
      let fallback: any = messages['zh-cn'];
      for (const k of keys) {
        if (fallback && typeof fallback === 'object' && k in fallback) {
          fallback = fallback[k];
        } else {
          fallback = undefined;
          break;
        }
      }
      if (typeof fallback === 'string') {
        return formatParams(fallback, params);
      }
    }
  }

  // 2. Direct phrase lookup for English
  if (lang === 'en') {
    if (enPhrases[path]) {
      return formatParams(enPhrases[path], params);
    }
    const trimmed = path.trim();
    if (enPhrases[trimmed]) {
      const leadingSpace = path.match(/^\s*/)?.[0] || '';
      const trailingSpace = path.match(/\s*$/)?.[0] || '';
      return leadingSpace + formatParams(enPhrases[trimmed], params) + trailingSpace;
    }
  }

  // 3. If lang is zh-cn, return path as is (it's already Chinese)
  if (lang === 'zh-cn') {
    return formatParams(path, params);
  }

  // 4. Fallback to phrase map or original text
  return formatParams(enPhrases[path] || path, params);
}

export const elLocale = computed(() => {
  return currentLocale.value === 'en' ? enLocale : zhCnLocale;
});

export function useI18n() {
  return {
    locale: currentLocale,
    setLocale,
    t,
    elLocale
  };
}

export const i18nPlugin = {
  install(app: App) {
    app.config.globalProperties.$t = t;
    app.config.globalProperties.$locale = currentLocale;
    app.provide('i18n', {
      locale: currentLocale,
      setLocale,
      t,
      elLocale
    });
  }
};
