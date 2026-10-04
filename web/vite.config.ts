import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const API_PROXY_TARGET = process.env.API_PROXY_TARGET ?? "http://localhost:8090";
const API_SECRET = process.env.API_SECRET ?? "";

// Dev proxy: /api/* -> API_PROXY_TARGET/* with the bearer secret injected
// server-side, so the secret never enters the browser bundle.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: API_PROXY_TARGET,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
        headers: { Authorization: `Bearer ${API_SECRET}` },
      },
    },
  },
});
