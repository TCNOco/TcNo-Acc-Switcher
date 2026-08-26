import { readFile } from "node:fs/promises";
import { parse as parseYaml } from "yaml";
import type { Plugin } from "vite";

/**
 * Loads .yaml files as pre-parsed JSON modules, keeping the `yaml` package and a
 * parse-per-theme out of the browser.
 *
 * Shared by vite.config.ts and vitest.config.ts so the two cannot drift: a test
 * that imports a theme has to see the same module shape the build produces.
 */
export function yamlAsJson(): Plugin {
  return {
    name: "yaml-as-json",
    enforce: "pre",
    async load(id: string) {
      const file = id.split("?")[0];
      if (!file.endsWith(".yaml")) return null;
      return `export default ${JSON.stringify(parseYaml(await readFile(file, "utf8")))};`;
    },
  };
}
