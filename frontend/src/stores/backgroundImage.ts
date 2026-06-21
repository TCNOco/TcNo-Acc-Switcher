import { derived, writable } from "svelte/store";
import type { AppBackgroundInfo } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export type { AppBackgroundInfo };

const DEFAULT_BG: AppBackgroundInfo = { hasImage: false, imageUrl: "", opacity: 0.6, blur: 6.0, themeBgOverride: false };

/** App-wide background image state (updated reactively from backend). */
export const appBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });

/** Platform-specific background image state for the currently active platform. */
export const platformBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });

/**
 * True when the user has set their own background or explicitly cleared it,
 * overriding the active theme's bundled background image.
 * Derived directly from the backend-sourced appBgInfo — no localStorage involved.
 */
export const userOverriddenAppBg = derived(appBgInfo, ($bg) => $bg.themeBgOverride);

/**
 * Persist the override flag to the settings file via the backend and update the
 * appBgInfo store so all derived state reacts immediately.
 */
export async function setUserOverride(val: boolean): Promise<void> {
  await PlatformService.SetThemeBgOverride(val);
  appBgInfo.update((bg) => ({ ...bg, themeBgOverride: val }));
}

