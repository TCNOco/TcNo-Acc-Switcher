import { derived, writable } from "svelte/store";
import type { AppBackgroundInfo } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export type { AppBackgroundInfo };

/**
 * Brightness for a background nobody has measured — either there is no image, or
 * it was chosen before the app started sampling them. Consumers treat this as
 * "unknown" and fall back to sampling the loaded image on a canvas.
 */
export const UNMEASURED_LUMA = { measured: false } as const;

const DEFAULT_BG: AppBackgroundInfo = {
  hasImage: false,
  imageUrl: "",
  opacity: 0.6,
  blur: 6.0,
  alignment: "center",
  fit: "cover",
  themeBgOverride: false,
  luma: UNMEASURED_LUMA,
};

/** App-wide background image state (updated reactively from backend). */
export const appBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });

/** Platform-specific background image state for the currently active platform. */
export const platformBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });

/**
 * True when the user has set their own background or explicitly cleared it,
 * overriding the active theme's bundled background image.
 */
export const userOverriddenAppBg = derived(appBgInfo, ($bg) => $bg.themeBgOverride);

/** Persists the override flag to the settings file via the backend. */
export async function setUserOverride(val: boolean): Promise<void> {
  await PlatformService.SetThemeBgOverride(val);
  appBgInfo.update((bg) => ({ ...bg, themeBgOverride: val }));
}

