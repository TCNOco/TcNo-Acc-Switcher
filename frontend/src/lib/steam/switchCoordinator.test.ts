import { describe, expect, it, vi } from "vitest";
import { createSteamSwitchCoordinator, type SteamSwitchTransport } from "./switchCoordinator";
import type { Settings } from "../../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

describe("Steam switch coordinator", () => {
  it("does not begin a switch until the current settings save resolves", async () => {
    const pendingSave = deferred<void>();
    const saveSettings = vi.fn(() => pendingSave.promise);
    const swapToAccount = vi.fn().mockResolvedValue(undefined);
    const loginAndLaunchGame = vi.fn().mockResolvedValue(undefined);
    const addNew = vi.fn().mockResolvedValue(undefined);
    const launchSteam = vi.fn().mockResolvedValue(undefined);
    const coordinator = createSteamSwitchCoordinator({
      saveSettings,
      swapToAccount,
      loginAndLaunchGame,
      addNew,
      launchSteam,
    });

    const saving = coordinator.saveSettings({} as Settings);
    const switching = coordinator.swapToAccount("76561198000000000", -1, []);
    const adding = coordinator.addNew();
    const launching = coordinator.launchSteam();

    await Promise.resolve();
    expect(saveSettings).toHaveBeenCalledOnce();
    expect(swapToAccount).not.toHaveBeenCalled();
    expect(addNew).not.toHaveBeenCalled();
    expect(launchSteam).not.toHaveBeenCalled();

    pendingSave.resolve();
    await saving;
    await switching;
    await adding;
    await launching;
    expect(swapToAccount).toHaveBeenCalledWith("76561198000000000", -1, []);
    expect(addNew).toHaveBeenCalledOnce();
    expect(launchSteam).toHaveBeenCalledOnce();
  });

  it("propagates a failed save to the waiting switch and accepts later saves", async () => {
    const failure = new Error("save failed");
    const saveSettings = vi
      .fn<SteamSwitchTransport["saveSettings"]>()
      .mockRejectedValueOnce(failure)
      .mockResolvedValue(undefined);
    const swapToAccount = vi.fn().mockResolvedValue(undefined);
    const loginAndLaunchGame = vi.fn().mockResolvedValue(undefined);
    const addNew = vi.fn().mockResolvedValue(undefined);
    const launchSteam = vi.fn().mockResolvedValue(undefined);
    const coordinator = createSteamSwitchCoordinator({
      saveSettings,
      swapToAccount,
      loginAndLaunchGame,
      addNew,
      launchSteam,
    });

    const failedSave = coordinator.saveSettings({} as Settings);
    const blockedSwitch = coordinator.swapToAccount("76561198000000000", -1, []);
    await expect(failedSave).rejects.toBe(failure);
    await expect(blockedSwitch).rejects.toBe(failure);
    expect(swapToAccount).not.toHaveBeenCalled();

    await expect(coordinator.saveSettings({} as Settings)).resolves.toBeUndefined();
    await expect(coordinator.loginAndLaunchGame("76561198000000000", -1, "440")).resolves.toBeUndefined();
    expect(loginAndLaunchGame).toHaveBeenCalledWith("76561198000000000", -1, "440");
  });
});
