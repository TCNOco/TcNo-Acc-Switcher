import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";

const platformService = vi.hoisted(() => ({
  ReadSettings: vi.fn(),
  UpdateSettings: vi.fn(),
}));

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => platformService);

import {
  controllerSupportEnabled,
  loadControllerSupportEnabled,
  setControllerSupportEnabled,
} from "./controllerSupport";

describe("controller support settings store", () => {
  beforeEach(() => {
    controllerSupportEnabled.set(false);
    platformService.ReadSettings.mockReset();
    platformService.UpdateSettings.mockReset();
  });

  it("defaults controller support on when the backend setting is absent", async () => {
    platformService.ReadSettings.mockResolvedValue({});

    await expect(loadControllerSupportEnabled()).resolves.toBe(true);
    expect(get(controllerSupportEnabled)).toBe(true);
  });

  it("preserves an explicit disabled setting", async () => {
    platformService.ReadSettings.mockResolvedValue({ controllerSupportEnabled: false });

    await expect(loadControllerSupportEnabled()).resolves.toBe(false);
    expect(get(controllerSupportEnabled)).toBe(false);
  });

  it("persists setting changes before updating the store", async () => {
    platformService.UpdateSettings.mockResolvedValue(undefined);

    await setControllerSupportEnabled(true);

    expect(platformService.UpdateSettings).toHaveBeenCalledWith({ controllerSupportEnabled: true });
    expect(get(controllerSupportEnabled)).toBe(true);
  });
});
