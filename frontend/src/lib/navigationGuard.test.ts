import { describe, expect, it, vi } from "vitest";
import { classifyNavigationHref, preventFormNavigation } from "./navigationGuard";

describe("classifyNavigationHref", () => {
  const current = "http://wails.localhost/#/home";

  it("allows only same-origin application navigation", () => {
    expect(classifyNavigationHref("#/steam", current)).toBe("internal");
    expect(classifyNavigationHref("/img/avatar.webp", current)).toBe("internal");
  });

  it("routes HTTPS links outside the WebView", () => {
    expect(classifyNavigationHref("https://github.com/TCNOCo", current)).toBe("external");
  });

  it("blocks unsafe schemes, credentials, and cleartext remote links", () => {
    expect(classifyNavigationHref("javascript:alert(1)", current)).toBe("blocked");
    expect(classifyNavigationHref("https://user:secret@example.com", current)).toBe("blocked");
    expect(classifyNavigationHref("http://example.com", current)).toBe("blocked");
  });

  it("prevents native form navigation without blocking application handlers", () => {
    const preventDefault = vi.fn();
    const stopImmediatePropagation = vi.fn();

    preventFormNavigation({
      preventDefault,
      stopImmediatePropagation,
    } as unknown as Event);

    expect(preventDefault).toHaveBeenCalledOnce();
    expect(stopImmediatePropagation).not.toHaveBeenCalled();
  });
});
