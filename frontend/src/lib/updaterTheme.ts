import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

/**
 * Maps app theme CSS variables (shared across all themes via _editor_palette
 * or legacy :root blocks) onto Wails updater window variables.
 * @see https://v3.wails.io/guides/updater/
 */
const UPDATER_VAR_SOURCES: ReadonlyArray<{ updater: string; app: readonly string[] }> = [
  { updater: "--bg", app: ["--mainContentBackground", "--document-canvas-bg", "--base-surface"] },
  { updater: "--surface", app: ["--surface-panel-bg", "--surface0"] },
  { updater: "--surface-2", app: ["--surface1", "--button-bg-hover"] },
  { updater: "--fg", app: ["--white", "--whiteSecondary"] },
  { updater: "--fg-dim", app: ["--text-muted-gray", "--text-body-muted"] },
  { updater: "--fg-faint", app: ["--text-subtle-gray", "--text-dim-gray"] },
  { updater: "--border", app: ["--form-checkbox-border", "--input-number-border"] },
  { updater: "--accent", app: ["--accent"] },
  { updater: "--success", app: ["--green", "--notification-success-border"] },
  { updater: "--error", app: ["--red", "--window-close-hover"] },
];

const UPDATER_DEFAULTS: Readonly<Record<string, string>> = {
  "--bg": "#1a1a1c",
  "--surface": "#232326",
  "--surface-2": "#2c2c30",
  "--fg": "#f5f5f7",
  "--fg-dim": "#b0b0b8",
  "--fg-faint": "#7a7a82",
  "--border": "#3a3a3e",
  "--accent": "#0a84ff",
  "--success": "#34c759",
  "--error": "#ff3b30",
};

function readCssVar(root: CSSStyleDeclaration, names: readonly string[]): string {
  for (const name of names) {
    const value = root.getPropertyValue(name).trim();
    if (!value || value.includes("gradient(")) {
      continue;
    }
    return value;
  }
  return "";
}

function normalizeHexColor(value: string): string | null {
  const trimmed = value.trim();
  const match = trimmed.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!match) {
    return null;
  }
  const hex = match[1];
  if (hex.length === 3) {
    return `#${hex
      .split("")
      .map((ch) => `${ch}${ch}`)
      .join("")
      .toLowerCase()}`;
  }
  return `#${hex.toLowerCase()}`;
}

function accentForeground(accent: string): string {
  const hex = normalizeHexColor(accent);
  if (!hex) {
    return "#ffffff";
  }
  const r = Number.parseInt(hex.slice(1, 3), 16);
  const g = Number.parseInt(hex.slice(3, 5), 16);
  const b = Number.parseInt(hex.slice(5, 7), 16);
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return lum > 0.62 ? "#1d1d1f" : "#ffffff";
}

export function buildUpdaterThemeCSS(): string {
  if (typeof document === "undefined") {
    return "";
  }
  const root = getComputedStyle(document.documentElement);
  const lines: string[] = [];

  for (const { updater, app } of UPDATER_VAR_SOURCES) {
    const value = readCssVar(root, app) || UPDATER_DEFAULTS[updater] || "";
    if (value) {
      lines.push(`${updater}: ${value}`);
    }
  }

  const accent = readCssVar(root, ["--accent"]) || UPDATER_DEFAULTS["--accent"] || "#0a84ff";
  lines.push(`--accent-fg: ${accentForeground(accent)}`);
  lines.push("--radius: 10px");

  return `:root {\n  ${lines.join(";\n  ")};\n}`;
}

let syncHandle = 0;

/** Push the active app theme to the Wails updater window (after styles have painted). */
export function scheduleUpdaterThemeSync(): void {
  if (typeof window === "undefined") {
    return;
  }
  if (syncHandle) {
    cancelAnimationFrame(syncHandle);
  }
  syncHandle = requestAnimationFrame(() => {
    syncHandle = requestAnimationFrame(() => {
      syncHandle = 0;
      void syncUpdaterTheme();
    });
  });
}

export async function syncUpdaterTheme(): Promise<void> {
  const css = buildUpdaterThemeCSS();
  if (!css) {
    return;
  }
  try {
    await PlatformService.SetUpdaterThemeCSS(css);
  } catch {
    /* backend not ready */
  }
}
