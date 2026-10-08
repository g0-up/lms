import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  // VITE_API_PROXY from the shell or .env.local (see .env.example); only the dev/preview proxy reads it.
  const apiTarget = loadEnv(mode, import.meta.dirname, "VITE_").VITE_API_PROXY || "http://localhost:8080";
  // Cùng origin với API ở dev: không cần CORS, cookie phiên chạy như production.
  const proxy = { "/api": { target: apiTarget, xfwd: true } };

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: { "@": path.resolve(import.meta.dirname, "src") },
    },
    server: { port: 5173, strictPort: true, proxy },
    preview: { port: 4173, strictPort: true, proxy },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id: string) {
            if (/node_modules\/(react|react-dom|scheduler|react-router)\//.test(id)) return "react-vendor";
            return undefined;
          },
        },
      },
    },
  };
});
