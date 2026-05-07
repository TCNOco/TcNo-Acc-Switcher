import { writable } from "svelte/store";

/**
 * Active while dragging file(s) that should show per-account "Set icon" cues (basic/Steam tabs),
 * instead of the fullscreen shortcut FileDropOverlay.
 */
export const accountProfileImageDropActive = writable(false);
