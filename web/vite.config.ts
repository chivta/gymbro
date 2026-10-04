import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const API_PROXY_TARGET = process.env.API_PROXY_TARGET ?? "http://localhost:8090";

// Dev proxy: /api/* -> API_PROXY_TARGET/*. The browser authenticates with the
// API's session cookie; the API sets cookies without Domain and with Path /,
// so they bind to the page origin (localhost:5173) and pass through unchanged.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: API_PROXY_TARGET,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
