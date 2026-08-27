import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";
import { gzipDist } from "./vite-plugin-gzip-dist";
import { yamlAsJson } from "./vite-plugin-yaml";

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
  plugins: [yamlAsJson(), svelte(), wails("./bindings"), gzipDist()],
  build: {
    // WebView2 is Evergreen Chromium, so downlevel transpilation is dead weight.
    target: "esnext",
  },
  json: {
    // Locales arrive through import.meta.glob and are read off .default, so a
    // named export per key only inflates each locale chunk; stringify lets the
    // browser JSON.parse them instead of evaluating a huge object literal.
    namedExports: false,
    stringify: true,
  },
  css: {
    preprocessorOptions: {
      scss: {
        api: "modern", // or "modern-compiler"
      },
    },
  },
});
