import * as AppEvents from "../../bindings/events/index.js";
import { Call, Events } from "@wailsio/runtime";
import { get, writable } from "svelte/store";
import type { BattleNetAccount } from "./battleNetAuthenticator";

export const battleNetCall = (method: string, ...args: unknown[]) => Call.ByName(`TcNo-Acc-Switcher/internal/steamguard.Service.${method}`, ...args);
const state = writable<{ unlocked: boolean; accounts: BattleNetAccount[] }>({ unlocked: false, accounts: [] });
export const battleNetAuthenticators = { subscribe: state.subscribe };

export function publishBattleNetAuthenticators(accounts: BattleNetAccount[]): void {
  state.set({ unlocked: true, accounts });
}

export function requestBattleNetView(): Promise<{ capability: string; lease: string }> {
  const requestId = crypto.randomUUID();
  return new Promise((resolve, reject) => {
    const off = Events.On(AppEvents.SteamguardSensitiveViewGrant, event => {
      const data = event.data;
      if (data?.requestId !== requestId || data.accountId !== "battlenet-authenticators") return;
      clearTimeout(timer);
      off();
      resolve(data);
    });
    const timer = setTimeout(() => { off(); reject(new Error("Authenticator view unavailable")); }, 5000);
    void battleNetCall("RequestBattleNetView", requestId).catch(cause => { clearTimeout(timer); off(); reject(cause); });
  });
}

export async function withBattleNetView<T>(action: (token: string) => Promise<T>): Promise<T> {
  const grant = await requestBattleNetView();
  try {
    return await action(grant.capability);
  } finally {
    await battleNetCall("EndSensitiveView", grant.capability, grant.lease);
  }
}

let refreshing: Promise<void> | null = null;
export async function refreshBattleNetAuthenticators(): Promise<void> {
  if (refreshing) return refreshing;
  refreshing = (async () => {
    try {
      const status = await battleNetCall("GetSettingsStatus");
      if (!status.unlocked) {
        state.set({ unlocked: false, accounts: [] });
        return;
      }
      const accounts = await withBattleNetView(token => battleNetCall("ListBattleNetAccounts", token));
      publishBattleNetAuthenticators(accounts);
    } catch {
      state.set({ unlocked: false, accounts: [] });
    }
  })();
  try { await refreshing; } finally { refreshing = null; }
}

export function battleNetAuthenticatorsNow() { return get(state); }
