import { beforeEach, describe, expect, it } from "vitest";
import {
  avatarEpochOf,
  avatarEpochs,
  currentPlatformAvatarEpochs,
  resetAvatarEpochs,
  setPlatformAvatarEpochs,
} from "./avatarEpoch";
import { get } from "svelte/store";

describe("avatarEpochs", () => {
  beforeEach(() => {
    resetAvatarEpochs();
  });

  // The Steam Guard vault reads the counter the account list writes; a view with
  // its own counters draws the URL from before the refresh.
  it("hands a counter written by one view to another", () => {
    setPlatformAvatarEpochs("Steam", { "76561198000000001": 3 });

    expect(avatarEpochOf(get(avatarEpochs), "Steam", "76561198000000001")).toBe(3);
    expect(currentPlatformAvatarEpochs("Steam")).toEqual({ "76561198000000001": 3 });
  });

  it("keeps platforms apart", () => {
    setPlatformAvatarEpochs("Steam", { shared: 2 });
    setPlatformAvatarEpochs("Epic", { shared: 7 });

    expect(avatarEpochOf(get(avatarEpochs), "Steam", "shared")).toBe(2);
    expect(avatarEpochOf(get(avatarEpochs), "Epic", "shared")).toBe(7);
  });

  it("reports zero for an account nothing has refreshed", () => {
    expect(avatarEpochOf(get(avatarEpochs), "Steam", "unknown")).toBe(0);
  });
});
