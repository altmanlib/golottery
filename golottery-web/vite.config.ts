import { fileURLToPath, URL } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  base: "./",
  plugins: [react()],
  define: {
    "process.env.NODE_ENV": JSON.stringify(
      process.env.NODE_ENV ?? "development",
    ),
  },
  resolve: {
    alias: { "#": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": { target: "http://127.0.0.1:5568", changeOrigin: true },
      "/healthz": { target: "http://127.0.0.1:5568", changeOrigin: true },
      "/openapi.json": { target: "http://127.0.0.1:5568", changeOrigin: true },
      "/openapi.yaml": { target: "http://127.0.0.1:5568", changeOrigin: true },
      "/readyz": { target: "http://127.0.0.1:5568", changeOrigin: true },
    },
  },
  build: {
    chunkSizeWarningLimit: 2048,
  },
  test: {
    globals: true,
    environment: "happy-dom",
    setupFiles: ["./src/test/setup.ts"],
    exclude: ["e2e/**", "node_modules/**"],
    coverage: { provider: "v8", reporter: ["text", "html"] },
  },
} as Record<string, unknown>);
