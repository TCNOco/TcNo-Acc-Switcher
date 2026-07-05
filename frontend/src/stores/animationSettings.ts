import { writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export const animationsEnabled = writable<boolean>(true);

export async function loadAnimationsEnabled(): Promise<void> {
  try {
    const v = await PlatformService.GetAnimationsEnabled();
    animationsEnabled.set(v);
  } catch {
    animationsEnabled.set(true);
  }
}

export async function setAnimationsEnabled(enabled: boolean): Promise<void> {
  try {
    await PlatformService.SetAnimationsEnabled(enabled);
  } catch (e) {
    animationsEnabled.set(enabled);
    throw e;
  }
  animationsEnabled.set(enabled);
}
