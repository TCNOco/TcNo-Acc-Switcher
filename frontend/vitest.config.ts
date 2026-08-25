import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { yamlAsJson } from "./vite-plugin-yaml";

export default defineConfig({
  plugins: [yamlAsJson(), svelte()],
  test: {
    include: ["src/**/*.test.ts"],
  },
});
