import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { localeValueArrays } from "./vite-plugin-locale-values";
import { yamlAsJson } from "./vite-plugin-yaml";

export default defineConfig({
  plugins: [yamlAsJson(), localeValueArrays(), svelte()],
  test: {
    include: ["src/**/*.test.ts"],
  },
});
