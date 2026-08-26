import { describe, it, expect, beforeEach } from "vitest";
import { get } from "svelte/store";
import { homeScreenData } from "./homeScreenData";
import { capabilities, currentOS, getCapabilities } from "./osCapabilities";
import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

function startup(over: Partial<PlatformStartup>): PlatformStartup {
  return over as PlatformStartup;
}

describe("osCapabilities", () => {
  beforeEach(() => homeScreenData.set(null));

  // Offering a control and then withdrawing it is worse than a late reveal.
  it("reports everything unsupported before startup data arrives", () => {
    const c = get(capabilities);
    expect(c.shortcuts).toBe(false);
    expect(c.processControl).toBe(false);
    expect(c.autostart).toBe(false);
    expect(get(currentOS)).toBe("");
  });

  it("reflects the backend capabilities once startup lands", () => {
    homeScreenData.set(
      startup({
        os: "linux",
        capabilities: { shortcuts: false, processControl: true, autostart: true } as never,
      }),
    );
    expect(get(capabilities).processControl).toBe(true);
    expect(get(capabilities).shortcuts).toBe(false);
    expect(get(currentOS)).toBe("linux");
  });

  it("distinguishes linux from darwin", () => {
    homeScreenData.set(startup({ os: "darwin", capabilities: { processControl: false } as never }));
    expect(getCapabilities().processControl).toBe(false);
    expect(get(currentOS)).toBe("darwin");

    homeScreenData.set(startup({ os: "linux", capabilities: { processControl: true } as never }));
    expect(getCapabilities().processControl).toBe(true);
  });

  it("falls back to unsupported when startup data omits capabilities", () => {
    homeScreenData.set(startup({ os: "linux" }));
    expect(get(capabilities).shortcuts).toBe(false);
  });
});
