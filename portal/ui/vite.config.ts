import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/web_client/portal/",
  build: {
    // Generate .vite/manifest.json so the Go backend can build index.html.
    manifest: true,
    rollupOptions: {
      // JS entry only — no index.html emitted.
      input: "src/main.tsx",
    },
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    // Allow the Go backend (different origin) to load assets during dev.
    cors: true,
  },
});
