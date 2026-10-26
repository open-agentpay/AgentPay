/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  poweredByHeader: false,
  env: {
    NEXT_PUBLIC_SITE_URL: process.env.NEXT_PUBLIC_SITE_URL || 'https://openagentpay.dev',
    NEXT_PUBLIC_GATEWAY_URL: process.env.NEXT_PUBLIC_GATEWAY_URL || 'https://gate.openagentpay.dev',
    NEXT_PUBLIC_CONSOLE_URL: process.env.NEXT_PUBLIC_CONSOLE_URL || 'https://console.openagentpay.dev',
    NEXT_PUBLIC_DOCS_URL: process.env.NEXT_PUBLIC_DOCS_URL || '/docs/',
  },
};

export default nextConfig;
