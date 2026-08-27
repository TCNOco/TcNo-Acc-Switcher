import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import localeKeys from "virtual:locale-keys";
import deDE from "../Resources/de-DE.json";
import { buildKeyIndex, localeValues } from "../../vite-plugin-locale-values";
import { zipMessages } from "./localeMessages";

// The plugin has already turned this into a positional array; resolveJsonModule
// still types it as the source object.
const deDEValues = deDE as unknown as (string | null)[];

describe("zipMessages", () => {
  it("reconstructs a real locale exactly as authored", () => {
    const source = JSON.parse(
      readFileSync(
        fileURLToPath(new URL("../Resources/de-DE.json", import.meta.url)),
        "utf8",
      ),
    );
    expect(zipMessages(localeKeys, deDEValues)).toEqual(source);
  });

  it("leaves a hole absent so the en-US merge supplies it", () => {
    const keys = ["a", "b"];
    const en = zipMessages(keys, ["A", "B"]);
    const partial = zipMessages(keys, ["Ä", null]);
    expect("b" in partial).toBe(false);
    expect({ ...en, ...partial }).toEqual({ a: "Ä", b: "B" });
  });

  it("keeps an empty string, which is not a hole", () => {
    expect(zipMessages(["a"], [""])).toEqual({ a: "" });
  });
});

describe("buildKeyIndex", () => {
  it("indexes a key only a translated locale carries", () => {
    const keys = buildKeyIndex({
      "en-US": { b: "B" },
      "de-DE": { a: "A", b: "B" },
    });
    expect(keys).toEqual(["a", "b"]);
    expect(zipMessages(keys, localeValues("de-DE", { a: "A", b: "B" }, keys))
      .a).toBe("A");
  });
});

describe("localeValues", () => {
  it("refuses to emit against an index missing one of its keys", () => {
    expect(() => localeValues("de-DE", { a: "A", z: "Z" }, ["a"])).toThrow(/z/);
  });
});
