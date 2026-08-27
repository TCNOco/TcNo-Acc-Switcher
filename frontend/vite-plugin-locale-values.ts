import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Plugin } from "vite";

const VIRTUAL_ID = "virtual:locale-keys";
const RESOLVED_ID = `\0${VIRTUAL_ID}`;

const RESOURCES_DIR = fileURLToPath(new URL("./src/Resources", import.meta.url));

type Messages = Record<string, string>;

/** One read of the Resources directory: locales and the index built from them. */
type Snapshot = { locales: Record<string, Messages>; keys: string[] };

/**
 * The shared key index: the union of every locale's keys, not just en-US's, so a
 * key that only a translated locale carries still gets a slot instead of being
 * silently dropped. Plain lexicographic sort keeps it reproducible.
 */
export function buildKeyIndex(locales: Record<string, Messages>): string[] {
  const union = new Set<string>();
  for (const messages of Object.values(locales)) {
    for (const key of Object.keys(messages)) union.add(key);
  }
  return [...union].sort();
}

/**
 * Projects one locale onto the index. Missing keys become null rather than a
 * hole, so the array round-trips through JSON.stringify with its positions
 * intact, and the consumer can tell "absent" from "empty string".
 *
 * Throws rather than emitting an array the index cannot explain: a desynced
 * array shows wrong strings at runtime instead of failing. Array length is not
 * the invariant worth asserting - it is keys.length by construction. A key the
 * index lacks is, and it means this locale was read from a newer snapshot.
 */
export function localeValues(
  name: string,
  messages: Messages,
  keys: readonly string[],
): (string | null)[] {
  const indexed = new Set(keys);
  const unindexed = Object.keys(messages).filter((key) => !indexed.has(key));
  if (unindexed.length > 0) {
    throw new Error(
      `locale-values: ${name} has keys the shared index lacks (${unindexed.slice(0, 5).join(", ")}); the index is stale`,
    );
  }
  return keys.map((key) =>
    Object.hasOwn(messages, key) ? (messages[key] ?? null) : null,
  );
}

/**
 * Rewrites each locale JSON into a positional array of values and publishes the
 * shared key list once as `virtual:locale-keys`, so the 1,170 key strings ship
 * once instead of once per locale. Runs in dev and build alike so the runtime
 * never has to branch on the module shape.
 *
 * A locale still loads as JSON text, so vite:json downstream keeps wrapping it in
 * JSON.parse; only the virtual index has to spell that out.
 */
export function localeValueArrays(): Plugin {
  // Vite's ids and this file's URL can disagree on drive-letter case on Windows.
  const dirKey = RESOURCES_DIR.replace(/\\/g, "/").toLowerCase();
  // One cached promise, not two fields: every concurrent load resolves to the
  // same snapshot object, so a locale array and the index it is positioned
  // against can never come from different reads of the directory.
  let snapshot: Promise<Snapshot> | null = null;

  function localeName(id: string): string | null {
    const file = (id.split("?")[0] ?? "").replace(/\\/g, "/");
    if (!file.endsWith(".json")) return null;
    const slash = file.lastIndexOf("/");
    if (file.slice(0, slash).toLowerCase() !== dirKey) return null;
    return file.slice(slash + 1, -".json".length);
  }

  async function read(): Promise<Snapshot> {
    const files = (await readdir(RESOURCES_DIR)).filter((f) =>
      f.endsWith(".json"),
    );
    const locales: Record<string, Messages> = Object.create(null);
    for (const file of files) {
      locales[file.slice(0, -".json".length)] = JSON.parse(
        await readFile(path.join(RESOURCES_DIR, file), "utf8"),
      ) as Messages;
    }
    return { locales, keys: buildKeyIndex(locales) };
  }

  function load(): Promise<Snapshot> {
    // A rejected read must not be cached, or one malformed save poisons the
    // server until it restarts.
    snapshot ??= read().catch((err) => {
      snapshot = null;
      throw err;
    });
    return snapshot;
  }

  return {
    name: "locale-value-arrays",
    enforce: "pre",
    resolveId(id) {
      return id === VIRTUAL_ID ? RESOLVED_ID : null;
    },
    async load(id) {
      if (id === RESOLVED_ID) {
        const index = JSON.stringify((await load()).keys);
        return `export default JSON.parse(${JSON.stringify(index)});`;
      }
      const name = localeName(id);
      if (!name) return null;
      const state = await load();
      const messages = state.locales[name];
      // Falling through to vite:json here would serve the raw object while the
      // index still describes an array, and zipMessages would quietly return {}
      // - the locale renders as pure en-US with no error. Fail instead.
      if (!messages) {
        throw new Error(
          `locale-values: ${name}.json is not in the current snapshot; restart the dev server`,
        );
      }
      return JSON.stringify(localeValues(name, messages, state.keys));
    },
    configureServer(server) {
      // handleHotUpdate only fires for changes, so an added or deleted locale
      // would leave a stale index paired with fresh arrays.
      const invalidate = (file: string) => {
        if (!localeName(file)) return;
        snapshot = null;
        // A shifted key restages every locale's positions, so the index and all
        // already-transformed siblings go, not just the file that changed.
        for (const mod of server.moduleGraph.idToModuleMap.values()) {
          if (mod.id === RESOLVED_ID || (mod.id && localeName(mod.id))) {
            server.moduleGraph.invalidateModule(mod);
          }
        }
        server.hot.send({ type: "full-reload" });
      };
      for (const event of ["add", "change", "unlink"] as const) {
        server.watcher.on(event, invalidate);
      }
    },
  };
}
