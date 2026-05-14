import { writable } from "svelte/store";
import type { AppBackgroundInfo } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

export type { AppBackgroundInfo };

const DEFAULT_BG: AppBackgroundInfo = { hasImage: false, imageUrl: "", opacity: 0.6, blur: 6.0 };

/** App-wide background image state (updated reactively from backend). */
export const appBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });

/** Platform-specific background image state for the currently active platform. */
export const platformBgInfo = writable<AppBackgroundInfo>({ ...DEFAULT_BG });
