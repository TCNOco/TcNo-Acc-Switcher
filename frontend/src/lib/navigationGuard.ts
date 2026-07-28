export type NavigationKind = "internal" | "external" | "blocked";

export function classifyNavigationHref(rawHref: string, currentHref: string): NavigationKind {
  let target: URL;
  let current: URL;
  try {
    current = new URL(currentHref);
    target = new URL(rawHref, current);
  } catch {
    return "blocked";
  }
  if (target.username || target.password) return "blocked";
  if (target.protocol !== "http:" && target.protocol !== "https:") return "blocked";
  if (target.origin === current.origin) return "internal";
  return target.protocol === "https:" ? "external" : "blocked";
}

function anchorFromEvent(event: Event): HTMLAnchorElement | null {
  const path = typeof event.composedPath === "function" ? event.composedPath() : [];
  for (const node of path) {
    if (node instanceof HTMLAnchorElement) return node;
  }
  return event.target instanceof Element ? event.target.closest("a[href]") : null;
}

export function installNavigationGuard(): void {
  const intercept = (event: MouseEvent): void => {
    const anchor = anchorFromEvent(event);
    if (!anchor) return;

    const kind = classifyNavigationHref(anchor.href, window.location.href);
    const opensNewContext = anchor.target !== "" && anchor.target.toLowerCase() !== "_self";
    if (kind === "internal" && !opensNewContext && event.button === 0) return;

    event.preventDefault();
    if (kind === "external" && event.button === 0) {
      return;
    }
    event.stopImmediatePropagation();
  };

  document.addEventListener("click", intercept, true);
  document.addEventListener("auxclick", intercept, true);
  document.addEventListener("submit", preventFormNavigation, true);

  Object.defineProperty(window, "open", {
    value: () => null,
    configurable: false,
    writable: false,
  });
}

/** Prevents native form navigation without blocking application submit handlers. */
export function preventFormNavigation(event: Event): void {
  event.preventDefault();
}
