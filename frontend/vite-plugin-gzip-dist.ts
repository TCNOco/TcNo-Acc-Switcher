import { constants, gzipSync } from "node:zlib";
import { readdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import type { Plugin, ResolvedConfig } from "vite";

/**
 * Replaces each built asset with a maximum-compression `.gz` sibling and deletes
 * the original, but only where compressing actually shrinks it — WebP, PNG and
 * woff2 all grow. Go embeds the result and inflates transparently on read.
 *
 * Gated on the resolved mode rather than an env var so it cannot disagree with
 * the script that invoked it: `build` is --mode production, `build:dev` is not.
 */
export function gzipDist(): Plugin {
  let outDir = "";
  let enabled = false;
  return {
    name: "gzip-dist",
    apply: "build",
    configResolved(config: ResolvedConfig) {
      outDir = path.resolve(config.root, config.build.outDir);
      enabled = config.mode === "production";
    },
    async closeBundle() {
      if (!enabled) return;

      let files: string[];
      try {
        files = (await readdir(outDir, { recursive: true, withFileTypes: true }))
          .filter((e) => e.isFile())
          .map((e) => path.join(e.parentPath, e.name));
      } catch {
        return;
      }
      // gzipfs resolves `x.gz` as the stored form of `x`, so a genuine .gz asset
      // would vanish from ReadDir under its own name and be inflated twice.
      const preCompressed = files.find((f) => f.endsWith(".gz"));
      if (preCompressed) {
        throw new Error(
          `gzip-dist: ${path.relative(outDir, preCompressed)} is already .gz; rename it (Go resolves .gz as a compressed sibling)`,
        );
      }

      let compressed = 0;
      let raw = 0;
      let before = 0;
      let after = 0;
      for (const file of files) {
        const input = await readFile(file);
        const gz = gzipSync(input, { level: constants.Z_BEST_COMPRESSION });
        before += input.byteLength;
        if (gz.byteLength < input.byteLength) {
          await writeFile(`${file}.gz`, gz);
          await rm(file);
          compressed++;
          after += gz.byteLength;
        } else {
          raw++;
          after += input.byteLength;
        }
      }
      this.info(
        `${compressed} compressed, ${raw} left raw: ${before} -> ${after} bytes`,
      );
    },
  };
}
