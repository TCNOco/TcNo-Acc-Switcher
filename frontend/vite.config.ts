import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";
import { gzipDist } from "./vite-plugin-gzip-dist";
import { localeValueArrays } from "./vite-plugin-locale-values";
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
  plugins: [
    yamlAsJson(),
    localeValueArrays(),
    svelte(),
    wails("./bindings"),
    gzipDist(),
  ],
  build: {
    // Three webviews, not one: WebView2 is evergreen and WebKitGTK 6.0 is
    // recent, so macOS is the floor -- LSMinimumSystemVersion is 10.15, whose
    // WKWebView is Safari 13.1. Raise this only alongside that plist.
    target: ["chrome87", "safari13.1"],
  },
  json: {
    // Locales arrive through import.meta.glob and are read off .default, so a
    // named export per key only inflates each locale chunk; stringify lets the
    // browser JSON.parse them instead of evaluating a huge object literal.
    // localeValueArrays runs first and hands this the value array, still as JSON.
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
