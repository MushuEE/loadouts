import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,      // Listen on all addresses, including LAN and public IPs
    port: 5173,      // (Optional) Explicitly set the port
    // Hosts permitted to reach the dev server. Vite rejects anything else with a 403 as
    // DNS-rebinding protection, which otherwise blocks reaching this workstation by name.
    // A leading dot matches the domain and all subdomains.
    allowedHosts: [
      ".googlers.com",
      "b2607f8b048001000006913b5ac133b8d1435000000000000000001.proxy.googlers.com"
    ],
    // Forward backend traffic so the whole app is reachable over this one port. Without
    // this, a browser on another machine has to reach 8080 directly, which fails whenever
    // only 5173 is exposed. Same-origin also means no CORS preflight.
    // /healthz and /sandbox are separate because the backend serves them outside /api/v1:
    // the former is a bare liveness probe, the latter serves HTML plugin frames.
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/healthz': { target: 'http://localhost:8080', changeOrigin: true },
      '/sandbox': { target: 'http://localhost:8080', changeOrigin: true }
    }
  }
})
