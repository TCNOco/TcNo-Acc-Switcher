import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";

const runtimeMock = vi.hoisted(() => {
  const handlers = new Map<string, (event: { data: unknown }) => void>();
  return {
    handlers,
    Events: {
      On: vi.fn((name: string, handler: (event: { data: unknown }) => void) => {
        handlers.set(name, handler);
        return () => handlers.delete(name);
      }),
    },
  };
});

const updaterThemeMock = vi.hoisted(() => ({
  scheduleUpdaterThemeSync: vi.fn(),
}));

const platformServiceMock = vi.hoisted(() => ({
  GetTheme: vi.fn(),
  GetThemeAccentCustom: vi.fn(),
  GetThemeAccentPreset: vi.fn(),
  SetTheme: vi.fn(),
  SetThemeAccentCustom: vi.fn(),
  SetThemeAccentPreset: vi.fn(),
}));

vi.mock("@wailsio/runtime", () => runtimeMock);
vi.mock("../updaterTheme", () => updaterThemeMock);
vi.mock("../../styles/modal-primary.scss?inline", () => ({
  default: "/* modal primary */",
}));
vi.mock("../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => platformServiceMock);

type LoadedThemeModules = {
  dom: typeof import("./dom");
  persistence: typeof import("./persistence");
  stores: typeof import("./stores");
  types: typeof import("./types");
};

type FakeElement = {
  id: string;
  tagName: string;
  textContent: string;
  attributes: Map<string, string>;
  parentNode: FakeHead | null;
  remove: () => void;
  setAttribute: (name: string, value: string) => void;
};

type FakeHead = {
  children: FakeElement[];
  appendChild: (node: FakeElement) => FakeElement;
};

function createFakeDocument(): Document {
  const head: FakeHead = {
    children: [],
    appendChild(node) {
      const existingIndex = this.children.indexOf(node);
      if (existingIndex >= 0) {
        this.children.splice(existingIndex, 1);
      }
      node.parentNode = this;
      this.children.push(node);
      return node;
    },
  };

  const createElement = (tagName: string): FakeElement => ({
    id: "",
    tagName: tagName.toLowerCase(),
    textContent: "",
    attributes: new Map(),
    parentNode: null,
    remove() {
      if (!this.parentNode) {
        return;
      }
      const index = this.parentNode.children.indexOf(this);
      if (index >= 0) {
        this.parentNode.children.splice(index, 1);
      }
      this.parentNode = null;
    },
    setAttribute(name: string, value: string) {
      this.attributes.set(name, value);
    },
  });

  const querySelectorAll = (selector: string): FakeElement[] => {
    const match = selector.match(/^([a-z]+)\[([^\]]+)\]$/i);
    if (!match) {
      return [];
    }
    const [, tagName, attrName] = match;
    return head.children.filter((node) => node.tagName === tagName && node.attributes.has(attrName));
  };

  const documentLike = {
    head,
    createElement,
    getElementById(id: string) {
      return head.children.find((node) => node.id === id) ?? null;
    },
    querySelectorAll,
  };

  return documentLike as unknown as Document;
}

function installFakeBrowserGlobals(): void {
  Object.defineProperty(globalThis, "navigator", {
    value: { userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", platform: "Win32" },
    configurable: true,
  });
  Object.defineProperty(globalThis, "document", {
    value: createFakeDocument(),
    configurable: true,
  });
}

function cssVar(css: string, name: string): string {
  const match = css.match(new RegExp(`${name}:\\s*([^;]+);`));
  if (!match) {
    throw new Error(`Missing CSS variable ${name}`);
  }
  return match[1].trim();
}

function getAccentOverlayCss(): string {
  const style = (document as unknown as { getElementById: (id: string) => FakeElement | null }).getElementById(
    "tcno-theme-accent-overlay",
  );
  expect(style).not.toBeNull();
  return style?.textContent ?? "";
}

function emitWindowsAccent(data: unknown): void {
  const handler = runtimeMock.handlers.get("windows-accent-changed");
  if (!handler) {
    throw new Error("windows-accent-changed handler not registered");
  }
  handler({ data });
}

async function loadThemeModules(): Promise<LoadedThemeModules> {
  return {
    dom: await import("./dom"),
    persistence: await import("./persistence"),
    stores: await import("./stores"),
    types: await import("./types"),
  };
}

describe("Windows accent live updates", () => {
  beforeEach(() => {
    runtimeMock.handlers.clear();
    runtimeMock.Events.On.mockClear();
    updaterThemeMock.scheduleUpdaterThemeSync.mockClear();
    Object.defineProperty(globalThis, "document", {
      value: undefined,
      configurable: true,
    });
    vi.resetModules();
    installFakeBrowserGlobals();
  });

  it("reapplies the Windows accent overlay and resolved accent for dark and light updates", async () => {
    const { dom, persistence, stores, types } = await loadThemeModules();
    const { applyResolvedAccent, ensureWindowsAccentSubscription } = dom;
    const { resolveThemeAccent } = persistence;
    const { currentThemeAccentKey, currentThemeCustomAccentColor, currentThemeId, currentWindowsThemeAccentColor } =
      stores;
    const { DEFAULT_THEME_ID, WINDOWS_THEME_ACCENT_KEY } = types;

    currentThemeId.set(DEFAULT_THEME_ID);
    currentThemeAccentKey.set("");
    currentThemeCustomAccentColor.set("");
    currentWindowsThemeAccentColor.set("");

    applyResolvedAccent(DEFAULT_THEME_ID, WINDOWS_THEME_ACCENT_KEY, "");
    expect(cssVar(getAccentOverlayCss(), "--accent")).toBe("#80ffea");

    ensureWindowsAccentSubscription();
    ensureWindowsAccentSubscription();
    expect(runtimeMock.Events.On).toHaveBeenCalledTimes(1);

    emitWindowsAccent("#123456");

    let overlayCss = getAccentOverlayCss();
    expect(get(currentWindowsThemeAccentColor)).toBe("#123456");
    expect(cssVar(overlayCss, "--accent")).toBe("#123456");
    expect(cssVar(overlayCss, "--text-on-bright-bg")).toBe("#f8f8f2");
    expect(resolveThemeAccent(DEFAULT_THEME_ID).color).toBe("#123456");
    expect(resolveThemeAccent(DEFAULT_THEME_ID).id).toBe(WINDOWS_THEME_ACCENT_KEY);

    emitWindowsAccent("#f5d76e");

    overlayCss = getAccentOverlayCss();
    expect(get(currentWindowsThemeAccentColor)).toBe("#f5d76e");
    expect(cssVar(overlayCss, "--accent")).toBe("#f5d76e");
    expect(cssVar(overlayCss, "--text-on-bright-bg")).toBe("#111111");
    expect(resolveThemeAccent(DEFAULT_THEME_ID).color).toBe("#f5d76e");
    expect(resolveThemeAccent(DEFAULT_THEME_ID).id).toBe(WINDOWS_THEME_ACCENT_KEY);
  });
});
