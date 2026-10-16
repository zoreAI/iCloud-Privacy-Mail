import type { NextConfig } from 'next';
const backend = process.env.GO_BACKEND_URL || 'http://127.0.0.1:8787';
const config: NextConfig = {
  async rewrites() { return [{ source: '/api/:path*', destination: `${backend}/api/:path*` }]; },
};
export default config;
