import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,      // Listen on all addresses, including LAN and public IPs
    port: 5173,      // (Optional) Explicitly set the port
    // Add the cloud proxy host to the allowed list to bypass the security check
    allowedHosts: [
      "b2607f8b048001000006913b5ac133b8d1435000000000000000001.proxy.googlers.com"
    ]
  }
})
