import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getInputModality,
  installInputModalityTracking,
  markControllerInput,
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

  it("marks controller modality explicitly", () => {
    const { root } = installDom();

    markControllerInput();

    expect(getInputModality()).toBe("controller");
    expect(root.classList.contains("input-modality-controller")).toBe(true);
    expect(root.dataset.inputModality).toBe("controller");
  });
});
