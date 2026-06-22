import { get, writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { pushToast } from "./toast";
import { t } from "./i18n";
import { formatToastWithError } from "../lib/formatWailsError";

export type CommandPaletteHotkey = "Ctrl+K" | "Ctrl+P";

const DEFAULT_HOTKEY: CommandPaletteHotkey = "Ctrl+K";

export const commandPaletteHotkey = writable<CommandPaletteHotkey>(DEFAULT_HOTKEY);

export function normalizeCommandPaletteHotkey(value: string | null | undefined): CommandPaletteHotkey {
  const normalized = (value ?? "").trim().toLowerCase().replace(/\s+/g, "");
  return normalized === "ctrl+p" || normalized === "control+p" ? "Ctrl+P" : DEFAULT_HOTKEY;
}

export function eventMatchesCommandPaletteHotkey(e: KeyboardEvent, hotkey: CommandPaletteHotkey): boolean {
  if (!e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) {
    return false;
  }
  const key = e.key.toLowerCase();
  return hotkey === "Ctrl+P" ? key === "p" : key === "k";
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
