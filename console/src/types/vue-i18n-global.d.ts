export {};

declare module 'vue' {
  interface ComponentCustomProperties {
    $t: (key: string, params?: Record<string, any>) => string;
  }
}
