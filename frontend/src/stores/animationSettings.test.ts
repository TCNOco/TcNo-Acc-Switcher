import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";

const platformService = vi.hoisted(() => ({
  GetAnimationsEnabled: vi.fn(),
  SetAnimationsEnabled: vi.fn(),
}));

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => platformService);

import { animationsEnabled, setAnimationsEnabled } from "./animationSettings";

describe("animation settings store", () => {
  beforeEach(() => {
    animationsEnabled.set(false);
    platformService.GetAnimationsEnabled.mockReset();
    platformService.SetAnimationsEnabled.mockReset();
  });

  it("persists animation changes before updating the store", async () => {
    platformService.SetAnimationsEnabled.mockResolvedValue(undefined);

    await setAnimationsEnabled(true);

    expect(platformService.SetAnimationsEnabled).toHaveBeenCalledWith(true);
    expect(get(animationsEnabled)).toBe(true);
  });

  it("updates the local store when the backend save is unavailable", async () => {
    const error = new Error("Wails runtime unavailable");
    platformService.SetAnimationsEnabled.mockRejectedValue(error);

    await expect(setAnimationsEnabled(true)).rejects.toBe(error);

    expect(platformService.SetAnimationsEnabled).toHaveBeenCalledWith(true);
    expect(get(animationsEnabled)).toBe(true);
  });
});
