import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies the engine API (REST + WebSocket) so the app can use
// relative URLs and avoid CORS. Override the target via the ENGINE_TARGET env.
const target = process.env.ENGINE_TARGET ?? "http://localhost:8050";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target,
        changeOrigin: true,
        ws: true,
      },
    },
  },
});
