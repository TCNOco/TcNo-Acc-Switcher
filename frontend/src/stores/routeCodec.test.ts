import { describe, expect, it } from "vitest";
import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
import { applySinglePlatformStartupRoute, type Route } from "./routeCodec";

function startup(overrides: Partial<PlatformStartup> = {}): PlatformStartup {
  return {
    homePlatformOrder: ["Steam"],
    allPlatformNames: ["Steam"],
    disabledPlatformNames: [],
    platformsFileMissing: false,
    ...overrides,
  } as PlatformStartup;
}

describe("applySinglePlatformStartupRoute", () => {
  it("opens the only enabled platform from the startup home route", () => {
    expect(applySinglePlatformStartupRoute({ page: "home" }, startup())).toEqual({
      page: "platform",
      platformName: "Steam",
    });
  });

  it.each([
    startup({ homePlatformOrder: [] }),
    startup({
      homePlatformOrder: ["Steam", "Epic Games"],
      allPlatformNames: ["Steam", "Epic Games"],
    }),
    startup({ platformsFileMissing: true }),
  ])("keeps home when automatic navigation is not applicable", (state) => {
    expect(applySinglePlatformStartupRoute({ page: "home" }, state)).toEqual({ page: "home" });
  });

  it("preserves an explicit startup route", () => {
    const route: Route = { page: "settings" };
    expect(applySinglePlatformStartupRoute(route, startup())).toBe(route);
  });
});
