import { describe, expect, it } from "vitest";
import {
  normalizeGameSearchKey,
  safeShortcutFolderName,
  shortcutIconIndexes,
  steamGameIconUrl,
} from "./shortcutAssets";

describe("shortcutAssets", () => {
  it("normalizes platform folder names", () => {
    expect(safeShortcutFolderName("Battle.net / Test")).toBe("battlenet___test");
    expect(safeShortcutFolderName("")).toBe("unknown");
  });

  it("normalizes game search keys", () => {
    expect(normalizeGameSearchKey("Portal™  2: Game of the Year")).toBe("portal 2 game of the year");
  });

  it("indexes shortcut icons by numeric app id and normalized stem", () => {
    const indexes = shortcutIconIndexes(
      [
        { fileName: "123.lnk", iconUrl: "/custom/123.png" },
        { FileName: "Portal 2.url" },
      ],
      "Steam",
      "/fallback.svg",
      false,
    );

    expect(indexes.byAppId["123"]).toBe("/custom/123.png");
    expect(indexes.byStemKey["portal 2"]).toBe("/img/shortcuts/steam/portal 2.png");
  });

  it("resolves Steam game icons from indexes before using conventional paths", () => {
    const indexes = {
      byAppId: { "123": "/custom/123.png" },
      byStemKey: { "portal 2": "/custom/portal.png" },
    };

    expect(steamGameIconUrl({ appId: "123", name: "Other" }, "Steam", indexes, "/fallback.svg", false)).toBe("/custom/123.png");
    expect(steamGameIconUrl({ appId: "999", name: "Portal® 2" }, "Steam", indexes, "/fallback.svg", false)).toBe("/custom/portal.png");
    expect(steamGameIconUrl({ appId: "456", name: "Missing" }, "Steam", indexes, "/fallback.svg", false)).toBe("/img/shortcuts/steam/456.png");
  });
});
