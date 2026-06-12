export const DEFAULT_THEME_ID = "default";
export const CUSTOM_THEME_ACCENT_KEY = "custom";
export const WINDOWS_THEME_ACCENT_KEY = "windows";

export type ThemeAccentOption = {
  id: string;
  label: string;
  color: string;
};

export type ResolvedThemeAccent = ThemeAccentOption & {
  isCustom: boolean;
};

export type ThemeOption = {
  id: string;
  label: string;
  googleFontsCss: string | null;
  backgroundUrl: string | null;
  defaultAccentColor: string;
  defaultAccentKey: string;
  accents: ThemeAccentOption[];
};

export const DEFAULT_THEME_OPTION: ThemeOption = {
  id: DEFAULT_THEME_ID,
  label: "Dracula Cyan (Default)",
  googleFontsCss: null,
  backgroundUrl: null,
  defaultAccentColor: "#80ffea",
  defaultAccentKey: "cyan",
  accents: [
    { id: "cyan", label: "Cyan", color: "#80ffea" },
    { id: "green", label: "Green", color: "#8aff80" },
    { id: "orange", label: "Orange", color: "#ffca80" },
    { id: "pink", label: "Pink", color: "#ff80bf" },
    { id: "purple", label: "Purple", color: "#9580ff" },
    { id: "red", label: "Red", color: "#ff9580" },
    { id: "yellow", label: "Yellow", color: "#ffff80" },
  ],
};
