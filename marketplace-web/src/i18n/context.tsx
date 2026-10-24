'use client';

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react';
import type { SupportedLocale, Translations } from './types';
import { zh } from './locales/zh';
import { en } from './locales/en';

interface I18nContextValue {
  locale: SupportedLocale;
  setLocale: (locale: SupportedLocale) => void;
  t: Translations;
}

const translationsMap: Record<SupportedLocale, Translations> = {
  zh,
  en,
};

const I18nContext = createContext<I18nContextValue>({
  locale: 'en',
  setLocale: () => {},
  t: en,
});

export function I18nProvider({
  children,
  initialLocale = 'en',
}: {
  children: React.ReactNode;
  initialLocale?: SupportedLocale;
}) {
  const [locale, setLocaleState] = useState<SupportedLocale>(initialLocale);

  // Sync from localStorage on client mount if available
  useEffect(() => {
    try {
      const stored = localStorage.getItem('NEXT_LOCALE') as SupportedLocale | null;
      if (stored && (stored === 'zh' || stored === 'en')) {
        setLocaleState(stored);
        document.documentElement.lang = stored === 'zh' ? 'zh-CN' : 'en';
        return;
      }

      // Default language is English
      setLocaleState('en');
      document.documentElement.lang = 'en';
    } catch {
      // Ignore in non-browser environments
    }
  }, []);

  const setLocale = useCallback((newLocale: SupportedLocale) => {
    setLocaleState(newLocale);
    try {
      localStorage.setItem('NEXT_LOCALE', newLocale);
      document.cookie = `NEXT_LOCALE=${newLocale}; path=/; max-age=31536000; SameSite=Lax`;
      document.documentElement.lang = newLocale === 'zh' ? 'zh-CN' : 'en';
    } catch {
      // Ignore
    }
  }, []);

  const value = {
    locale,
    setLocale,
    t: translationsMap[locale] || en,
  };

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error('useI18n must be used within an I18nProvider');
  }
  return context;
}
