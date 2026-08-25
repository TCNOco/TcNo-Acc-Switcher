import { readFile } from "node:fs/promises";
import { parse as parseYaml } from "yaml";
import type { Plugin } from "vite";

/**
 * Loads .yaml files as pre-parsed JSON modules.
 *
 * The themes are authored as YAML, and parsing them in the browser meant
 * shipping the whole `yaml` package and running a parse per theme at module
 * evaluation - before the first paint. Doing it here costs nothing at runtime.
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
