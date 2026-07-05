import { writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export const controllerSupportEnabled = writable(false);

export function normalizeControllerSupportEnabled(value: unknown): boolean {
  return typeof value === "boolean" ? value : true;
}

export function applyControllerSupportEnabled(value: unknown): boolean {
  const enabled = normalizeControllerSupportEnabled(value);
  controllerSupportEnabled.set(enabled);
  return enabled;
}

export async function loadControllerSupportEnabled(): Promise<boolean> {
  try {
    const settings = await PlatformService.ReadSettings();
    return applyControllerSupportEnabled(settings.controllerSupportEnabled);
  } catch {
    controllerSupportEnabled.set(true);
    return true;
  }
}

export async function setControllerSupportEnabled(enabled: boolean): Promise<void> {
  await PlatformService.UpdateSettings({ controllerSupportEnabled: enabled });
  controllerSupportEnabled.set(enabled);
}
