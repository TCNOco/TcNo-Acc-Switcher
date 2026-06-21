import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      "wails-shortcuts-service": path.resolve(
        __dirname,
        "bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js",
      ),
    },
  },
  plugins: [svelte(), wails("./bindings")],
  css: {
    preprocessorOptions: {
      scss: {
        api: "modern", // or "modern-compiler"
      },
    },
  },
});
