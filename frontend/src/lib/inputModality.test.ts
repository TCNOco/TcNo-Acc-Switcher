import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getControllerGlyphScheme,
  getInputModality,
  inferControllerGlyphScheme,
  installInputModalityTracking,
  markControllerInput,
  markSyntheticControllerKeyEvent,
  setInputModality,
} from "./inputModality";

afterEach(() => {
  vi.unstubAllGlobals();
});

class FakeClassList {
  private classes = new Set<string>();

  add(...tokens: string[]): void {
    tokens.forEach((token) => this.classes.add(token));
  }

  remove(...tokens: string[]): void {
    tokens.forEach((token) => this.classes.delete(token));
  }

  contains(token: string): boolean {
    return this.classes.has(token);
  }
}

function keyboardEvent(key: string): KeyboardEvent {
  const event = new Event("keydown") as KeyboardEvent;
  Object.defineProperty(event, "key", { value: key });
  return event;
}

function installDom(): { window: EventTarget; root: HTMLElement } {
  const root = { classList: new FakeClassList(), dataset: {} };
  const fakeDocument = { documentElement: root, body: null };
  const fakeWindow = new EventTarget();
  vi.stubGlobal("document", fakeDocument);
  vi.stubGlobal("window", fakeWindow);
  return { window: fakeWindow, root: root as unknown as HTMLElement };
}

describe("input modality", () => {
  it("marks pointer and keyboard modality on global input", () => {
    const { window, root } = installDom();
    const cleanup = installInputModalityTracking();

    window.dispatchEvent(new Event("mousedown"));
    expect(getInputModality()).toBe("pointer");
    expect(root.classList.contains("input-modality-pointer")).toBe(true);

    window.dispatchEvent(keyboardEvent("Tab"));
    expect(getInputModality()).toBe("keyboard");
    expect(root.classList.contains("input-modality-keyboard")).toBe(true);
    expect(root.dataset.inputModality).toBe("keyboard");

    cleanup();
  });

  it("ignores non-navigation typing keys", () => {
    const { window } = installDom();
    const cleanup = installInputModalityTracking();

    setInputModality("pointer");
    window.dispatchEvent(keyboardEvent("a"));
    expect(getInputModality()).toBe("pointer");

    cleanup();
  });

  it("does not treat synthetic controller arrows as keyboard input", () => {
    const { window } = installDom();
    const cleanup = installInputModalityTracking();
    const event = keyboardEvent("ArrowRight");

    markControllerInput("xbox");
    markSyntheticControllerKeyEvent(event);
    window.dispatchEvent(event);

    expect(getInputModality()).toBe("controller");
    expect(getControllerGlyphScheme()).toBe("xbox");

    cleanup();
  });

  it("marks controller modality explicitly", () => {
    const { root } = installDom();

    markControllerInput("playstation");

    expect(getInputModality()).toBe("controller");
    expect(getControllerGlyphScheme()).toBe("playstation");
    expect(root.classList.contains("input-modality-controller")).toBe(true);
    expect(root.dataset.inputModality).toBe("controller");
    expect(root.dataset.controllerScheme).toBe("playstation");
  });

  it("infers controller glyph schemes from common gamepad ids", () => {
    expect(inferControllerGlyphScheme("Xbox Wireless Controller")).toBe("xbox");
    expect(inferControllerGlyphScheme("Sony DualSense Wireless Controller")).toBe("playstation");
    expect(inferControllerGlyphScheme("Nintendo Switch Pro Controller")).toBe("nintendo");
    expect(inferControllerGlyphScheme("Unknown USB Controller")).toBe("generic");
  });
});
