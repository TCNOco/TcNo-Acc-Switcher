export { WINDOWS_THEME_ACCENT_KEY } from "./theme/types";
export type { ThemeAccentOption, ResolvedThemeAccent, ThemeOption } from "./theme/types";
export { listThemes } from "./theme/catalog";
export { supportsWindowsThemeAccent } from "./theme/dom";
export {
  currentThemeId,
  currentThemeBgUrl,
  currentThemeAccentKey,
  currentThemeCustomAccentColor,
  currentWindowsThemeAccentColor,
  resolveThemeAccent,
  initTheme,
  setUserTheme,
  setUserThemeAccentPreset,
  setUserThemeAccentCustom,
} from "./theme/persistence";
