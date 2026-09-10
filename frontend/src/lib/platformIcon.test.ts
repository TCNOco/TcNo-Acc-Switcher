import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { platformIconFgHref, platformIconSrc } from "./platformIcon";

describe("shared platform artwork", () => {
  it.each([
    ["Spotify (Windows Store)", "Spotify"],
    ["Spotify (Snap)", "Spotify"],
    ["Spotify (xyz)", "Spotify"],
    ["Discord (Flatpak)", "Discord"],
    ["OBS Studio (AppImage)", "OBS Studio"],
  ])("resolves %s to the bundled %s logo", (variant, base) => {
    const src = `/img/platform/${encodeURIComponent(base)}.svg`;
    expect(platformIconSrc(variant)).toBe(src);
    expect(platformIconFgHref(variant)).toBe(`${src}#FG`);
    const asset = readFileSync(new URL(`../../public${src}`, import.meta.url), "utf8");
    expect(asset).toContain('id="FG"');
  });
});
