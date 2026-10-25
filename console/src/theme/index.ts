import { ref } from 'vue';

export type ThemeMode = 'light' | 'dark';

const THEME_STORAGE_KEY = 'agentpay_theme';

function getInitialTheme(): ThemeMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY) as ThemeMode | null;
    if (saved === 'light' || saved === 'dark') {
      return saved;
    }
    if (typeof window !== 'undefined' && window.matchMedia) {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      return prefersDark ? 'dark' : 'light';
    }
  } catch {
    // fallback
  }
  return 'dark';
}

export const currentTheme = ref<ThemeMode>(getInitialTheme());

export function applyTheme(mode: ThemeMode) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  if (mode === 'dark') {
    root.classList.add('dark');
    root.classList.remove('light');
  } else {
    root.classList.remove('dark');
    root.classList.add('light');
  }
}

export function setTheme(mode: ThemeMode) {
  currentTheme.value = mode;
  try {
    localStorage.setItem(THEME_STORAGE_KEY, mode);
  } catch {
    // ignore
  }
  applyTheme(mode);
}

export function toggleTheme() {
  const nextMode: ThemeMode = currentTheme.value === 'dark' ? 'light' : 'dark';
  setTheme(nextMode);
}

export function initTheme() {
  applyTheme(currentTheme.value);
}

export function useTheme() {
  return {
    theme: currentTheme,
    setTheme,
    toggleTheme,
  };
}
