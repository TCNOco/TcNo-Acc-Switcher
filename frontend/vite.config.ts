import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";
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
  plugins: [yamlAsJson(), svelte(), wails("./bindings")],
  json: {
    // Nothing imports a named export from a .json file - the locales come in
    // through import.meta.glob and are read off .default - so emitting one
    // export per key only inflates every locale chunk. stringify lets the
    // browser parse them with JSON.parse instead of evaluating an object
    // literal, which is markedly faster at this size.
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
