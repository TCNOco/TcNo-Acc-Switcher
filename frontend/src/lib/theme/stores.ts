import { writable } from "svelte/store";
import { DEFAULT_THEME_ID } from "./types";

export const currentThemeId = writable<string>(DEFAULT_THEME_ID);
export const currentThemeBgUrl = writable<string>("");
export const currentThemeAccentKey = writable<string>("");
export const currentThemeCustomAccentColor = writable<string>("");
export const currentWindowsThemeAccentColor = writable<string>("");
