import { describe, expect, it } from "vitest";
import { platformArtworkName, platformDisplayLabels } from "./platformName";

describe("platformArtworkName", () => {
  it("shares one icon across install methods", () => {
    expect(platformArtworkName("Discord (Flatpak)")).toBe("Discord");
    expect(platformArtworkName("OBS Studio (Flatpak)")).toBe("OBS Studio");
  });

  it("leaves names that are not qualified alone", () => {
    expect(platformArtworkName("Discord Canary")).toBe("Discord Canary");
  });
});

describe("platformDisplayLabels", () => {
  it("drops the install method when only one is on screen", () => {
    const labels = platformDisplayLabels(["Discord (Flatpak)", "Steam"]);
    expect(labels.get("Discord (Flatpak)")).toBe("Discord");
    expect(labels.get("Steam")).toBe("Steam");
  });

  it("keeps both apart when two methods are installed", () => {
    const labels = platformDisplayLabels(["Discord (Flatpak)", "Discord (Snap)"]);
    expect(labels.get("Discord (Flatpak)")).toBe("Discord (Flatpak)");
    expect(labels.get("Discord (Snap)")).toBe("Discord (Snap)");
  });

  it("counts the un-suffixed name, so a native install still qualifies the others", () => {
    const labels = platformDisplayLabels(["Discord", "Discord (Flatpak)"]);
    expect(labels.get("Discord")).toBe("Discord");
    expect(labels.get("Discord (Flatpak)")).toBe("Discord (Flatpak)");
  });

  it("keeps a parenthetical that names a store rather than an install method", () => {
    const labels = platformDisplayLabels(["Heroic (Epic)"]);
    expect(labels.get("Heroic (Epic)")).toBe("Heroic (Epic)");
  });
});
