import * as SteamService from "../../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
import type { Settings } from "../../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";

export type SteamSwitchTransport = {
  saveSettings: (settings: Settings) => Promise<void>;
  swapToAccount: (steamId64: string, personaState: number, extraArgs: string[]) => Promise<void>;
  loginAndLaunchGame: (steamId64: string, personaState: number, appId: string) => Promise<void>;
  addNew: () => Promise<void>;
  launchSteam: () => Promise<void>;
};

export function createSteamSwitchCoordinator(transport: SteamSwitchTransport) {
  let saveTail: Promise<void> = Promise.resolve();
  let queuedSaves = 0;

  function beginSave(settings: Settings): Promise<void> {
    try {
      return Promise.resolve(transport.saveSettings(settings));
    } catch (error) {
      return Promise.reject(error);
    }
  }

  function saveSettings(settings: Settings): Promise<void> {
    const previousSave = saveTail;
    queuedSaves += 1;
    const save = queuedSaves === 1
      ? beginSave(settings)
      : previousSave.catch(() => undefined).then(() => beginSave(settings));
    saveTail = save.finally(() => { queuedSaves -= 1; });
    return saveTail;
  }

  async function afterCurrentSave<T>(operation: () => Promise<T>): Promise<T> {
    await saveTail;
    return operation();
  }

  return {
    saveSettings,
    swapToAccount: (steamId64: string, personaState: number, extraArgs: string[]) =>
      afterCurrentSave(() => transport.swapToAccount(steamId64, personaState, extraArgs)),
    loginAndLaunchGame: (steamId64: string, personaState: number, appId: string) =>
      afterCurrentSave(() => transport.loginAndLaunchGame(steamId64, personaState, appId)),
    addNew: () => afterCurrentSave(() => transport.addNew()),
    launchSteam: () => afterCurrentSave(() => transport.launchSteam()),
  };
}

export const steamSwitchCoordinator = createSteamSwitchCoordinator({
  saveSettings: SteamService.SaveSteamSettings,
  swapToAccount: SteamService.SwapToSteamAccount,
  loginAndLaunchGame: SteamService.LoginAndLaunchGame,
  addNew: SteamService.SteamAddNew,
  launchSteam: SteamService.LaunchSteam,
});
