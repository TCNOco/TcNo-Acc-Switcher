import { get, writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { pushToast } from "./toast";
import { t } from "./i18n";
import { formatToastWithError } from "../lib/formatWailsError";

export type CommandPaletteHotkey = string;

type ParsedCommandPaletteHotkey = {
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
  meta: boolean;
  key: string;
};

const DEFAULT_HOTKEY: CommandPaletteHotkey = "Ctrl+K";
const MODIFIER_ORDER = ["Ctrl", "Alt", "Shift", "Meta"] as const;
const MODIFIER_ALIASES: Record<string, (typeof MODIFIER_ORDER)[number]> = {
  ctrl: "Ctrl",
  control: "Ctrl",
  alt: "Alt",
  option: "Alt",
  shift: "Shift",
  meta: "Meta",
  cmd: "Meta",
  command: "Meta",
  win: "Meta",
  windows: "Meta",
  super: "Meta",
};
const KEY_ALIASES: Record<string, string> = {
  space: "Space",
  spacebar: "Space",
  enter: "Enter",
  return: "Enter",
  tab: "Tab",
  backspace: "Backspace",
  delete: "Delete",
  del: "Delete",
  insert: "Insert",
  ins: "Insert",
  home: "Home",
  end: "End",
  pageup: "PageUp",
  pgup: "PageUp",
  pagedown: "PageDown",
  pgdown: "PageDown",
  arrowup: "ArrowUp",
  up: "ArrowUp",
  arrowdown: "ArrowDown",
  down: "ArrowDown",
  arrowleft: "ArrowLeft",
  left: "ArrowLeft",
  arrowright: "ArrowRight",
  right: "ArrowRight",
};

export const commandPaletteHotkey = writable<CommandPaletteHotkey>(DEFAULT_HOTKEY);

function normalizeCommandPaletteKey(value: string): string | null {
  if (value === " ") {
    return "Space";
  }
  const compact = value.trim();
  if (!compact) {
    return null;
  }
  const lower = compact.toLowerCase();
  if (MODIFIER_ALIASES[lower] || lower === "escape" || lower === "esc") {
    return null;
  }
  const alias = KEY_ALIASES[lower];
  if (alias) {
    return alias;
  }
  const functionKey = /^f([1-9]|1[0-9]|2[0-4])$/i.exec(compact);
  if (functionKey) {
    return `F${functionKey[1]}`;
  }
  return compact.length === 1 ? compact.toUpperCase() : null;
}

function formatParsedCommandPaletteHotkey(parsed: ParsedCommandPaletteHotkey): CommandPaletteHotkey {
  const parts: string[] = [];
  for (const modifier of MODIFIER_ORDER) {
    const key = modifier.toLowerCase() as "ctrl" | "alt" | "shift" | "meta";
    if (parsed[key]) {
      parts.push(modifier);
    }
  }
  parts.push(parsed.key);
  return parts.join("+");
}

function parseCommandPaletteHotkey(value: string | null | undefined): ParsedCommandPaletteHotkey | null {
  const compact = (value ?? "").trim().replace(/\s+/g, "");
  if (!compact) {
    return null;
  }
  const parts = compact.split("+");
  if (parts.length < 2 || parts.some((part) => part === "")) {
    return null;
  }
  const parsed: ParsedCommandPaletteHotkey = {
    ctrl: false,
    alt: false,
    shift: false,
    meta: false,
    key: "",
  };
  for (const part of parts) {
    const modifier = MODIFIER_ALIASES[part.toLowerCase()];
    if (modifier) {
      if (parsed.key) {
        return null;
      }
      const key = modifier.toLowerCase() as "ctrl" | "alt" | "shift" | "meta";
      if (parsed[key]) {
        return null;
      }
      parsed[key] = true;
      continue;
    }
    if (parsed.key) {
      return null;
    }
    const key = normalizeCommandPaletteKey(part);
    if (!key) {
      return null;
    }
    parsed.key = key;
  }
  if (!parsed.key || (!parsed.ctrl && !parsed.alt && !parsed.shift && !parsed.meta)) {
    return null;
  }
  return parsed;
}

export function normalizeCommandPaletteHotkey(value: string | null | undefined): CommandPaletteHotkey {
  const parsed = parseCommandPaletteHotkey(value);
  return parsed ? formatParsedCommandPaletteHotkey(parsed) : DEFAULT_HOTKEY;
}

export function formatCommandPaletteHotkeyEvent(e: KeyboardEvent): CommandPaletteHotkey | null {
  const key = normalizeCommandPaletteKey(e.key);
  if (!key || (!e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey)) {
    return null;
  }
  return formatParsedCommandPaletteHotkey({
    ctrl: e.ctrlKey,
    alt: e.altKey,
    shift: e.shiftKey,
    meta: e.metaKey,
    key,
  });
}

export function eventMatchesCommandPaletteHotkey(e: KeyboardEvent, hotkey: CommandPaletteHotkey): boolean {
  const parsed = parseCommandPaletteHotkey(hotkey);
  if (!parsed) {
    return false;
  }
  return e.ctrlKey === parsed.ctrl
    && e.altKey === parsed.alt
    && e.shiftKey === parsed.shift
    && e.metaKey === parsed.meta
    && normalizeCommandPaletteKey(e.key) === parsed.key;
}

export async function loadCommandPaletteHotkey(): Promise<void> {
  try {
    const settings = await PlatformService.ReadSettings();
    commandPaletteHotkey.set(normalizeCommandPaletteHotkey(settings.commandPaletteHotkey));
  } catch {
    commandPaletteHotkey.set(DEFAULT_HOTKEY);
  }
}

export async function setCommandPaletteHotkey(value: CommandPaletteHotkey): Promise<void> {
  const next = normalizeCommandPaletteHotkey(value);
  const prev = get(commandPaletteHotkey);
  commandPaletteHotkey.set(next);
  try {
    await PlatformService.UpdateSettings({ commandPaletteHotkey: next });
    pushToast({
      type: "success",
      message: get(t)("Toast_SavedItem", { item: get(t)("Settings_CommandPaletteHotkey") }),
      duration: 3000,
    });
  } catch (e) {
    commandPaletteHotkey.set(prev);
    pushToast({
      type: "error",
      message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
      duration: 8000,
    });
  }
}
