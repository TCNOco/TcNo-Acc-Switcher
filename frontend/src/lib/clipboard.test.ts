import { describe, it, expect, vi, afterEach } from "vitest";
import { copyText } from "./clipboard";

// These tests run without a DOM, so the fallback's document is stubbed the same
// way the other suites stub navigator.
const realDocument = (globalThis as { document?: unknown }).document;
const realNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");

function setNavigator(value: unknown): void {
  Object.defineProperty(globalThis, "navigator", { value, configurable: true });
}

/** Minimal document recording what the fallback did to the element it made. */
function fakeDocument(copySucceeds: boolean) {
  const appended: unknown[] = [];
  const removed: unknown[] = [];
  const el = {
    value: "",
    tabIndex: 0,
    style: {} as Record<string, string>,
    setAttribute: vi.fn(),
    select: vi.fn(),
    remove: vi.fn(() => removed.push(el)),
  };
  return {
    el,
    appended,
    removed,
    doc: {
      createElement: vi.fn(() => el),
      execCommand: vi.fn(() => copySucceeds),
      body: { appendChild: vi.fn((n: unknown) => appended.push(n)) },
    },
  };
}

afterEach(() => {
  if (realNavigator) Object.defineProperty(globalThis, "navigator", realNavigator);
  (globalThis as { document?: unknown }).document = realDocument;
  vi.restoreAllMocks();
});

describe("copyText", () => {
  it("uses the async clipboard when the webview has one", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setNavigator({ clipboard: { writeText } });
    await copyText("hello");
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  // A WebKitGTK webview outside a secure context has no navigator.clipboard.
  // The old copyPath returned silently there, so the button did nothing and
  // reported nothing.
  it("falls back to execCommand and removes the element it added", async () => {
    setNavigator({});
    const f = fakeDocument(true);
    (globalThis as { document?: unknown }).document = f.doc;

    await copyText("fallback");

    expect(f.el.value).toBe("fallback");
    expect(f.doc.execCommand).toHaveBeenCalledWith("copy");
    expect(f.appended).toHaveLength(1);
    expect(f.removed).toHaveLength(1);
  });

  it("throws when neither route works, so the caller can report it", async () => {
    setNavigator({});
    const f = fakeDocument(false);
    (globalThis as { document?: unknown }).document = f.doc;

    await expect(copyText("nope")).rejects.toThrow();
    // Still cleaned up on the failure path.
    expect(f.removed).toHaveLength(1);
  });
});
