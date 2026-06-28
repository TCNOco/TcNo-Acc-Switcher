import { describe, expect, it, vi } from "vitest";

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => ({
  ReadSettings: vi.fn(),
  UpdateSettings: vi.fn(),
}));

vi.mock("./toast", () => ({
  pushToast: vi.fn(),
}));

vi.mock("./i18n", () => ({
  t: {
    subscribe(run: (translator: (key: string) => string) => void) {
      run((key: string) => key);
      return () => {};
    },
  },
}));

vi.mock("../lib/formatWailsError", () => ({
  formatToastWithError: vi.fn(),
}));

import {
  eventMatchesCommandPaletteHotkey,
  formatCommandPaletteHotkeyEvent,
  normalizeCommandPaletteHotkey,
} from "./commandPalette";

function keyEvent(key: string, opts: Partial<Pick<KeyboardEvent, "ctrlKey" | "altKey" | "shiftKey" | "metaKey">> = {}): KeyboardEvent {
  return {
    key,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    metaKey: false,
    ...opts,
  } as KeyboardEvent;
}

describe("command palette hotkeys", () => {
  it("normalizes saved values", () => {
    expect(normalizeCommandPaletteHotkey(null)).toBe("Ctrl+K");
    expect(normalizeCommandPaletteHotkey("Control + p")).toBe("Ctrl+P");
    expect(normalizeCommandPaletteHotkey("ctrl+shift+k")).toBe("Ctrl+Shift+K");
  });

  it("formats captured keyboard events", () => {
    expect(formatCommandPaletteHotkeyEvent(keyEvent("k", { ctrlKey: true, shiftKey: true }))).toBe("Ctrl+Shift+K");
    expect(formatCommandPaletteHotkeyEvent(keyEvent("Control", { ctrlKey: true }))).toBeNull();
    expect(formatCommandPaletteHotkeyEvent(keyEvent("k"))).toBeNull();
  });

  it("matches exact modifier combinations", () => {
    expect(eventMatchesCommandPaletteHotkey(keyEvent("k", { ctrlKey: true }), "Ctrl+K")).toBe(true);
    expect(eventMatchesCommandPaletteHotkey(keyEvent("k", { ctrlKey: true, shiftKey: true }), "Ctrl+K")).toBe(false);
    expect(eventMatchesCommandPaletteHotkey(keyEvent("K", { ctrlKey: true, shiftKey: true }), "Ctrl+Shift+K")).toBe(true);
  });
});
