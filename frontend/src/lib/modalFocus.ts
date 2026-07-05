type FocusTrapOptions = {
  enabled?: boolean;
  initialFocus?: () => HTMLElement | null | undefined;
  onEscape?: () => void;
};

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(", ");

function isFocusable(element: HTMLElement): boolean {
  if (element.hidden) return false;
  if (element.getAttribute("aria-hidden") === "true") return false;
  return element.getClientRects().length > 0;
}

function getFocusableElements(node: HTMLElement): HTMLElement[] {
  return Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(isFocusable);
}

function restoreFocus(target: HTMLElement | null): void {
  if (!target || !target.isConnected) return;
  if ("disabled" in target && target.disabled) return;
  requestAnimationFrame(() => {
    target.focus({ preventScroll: true });
  });
}

function focusInitialElement(node: HTMLElement, options: FocusTrapOptions): void {
  const target = options.initialFocus?.() ?? getFocusableElements(node)[0] ?? node;
  requestAnimationFrame(() => {
    target.focus({ preventScroll: true });
  });
}

export function modalFocus(node: HTMLElement, initialOptions: FocusTrapOptions = {}) {
  let options = initialOptions;
  const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;

  if (!node.hasAttribute("tabindex")) {
    node.tabIndex = -1;
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (options.enabled === false) return;

    if (event.key === "Escape") {
      event.preventDefault();
      options.onEscape?.();
      return;
    }

    if (event.key !== "Tab") return;

    const focusable = getFocusableElements(node);
    if (focusable.length === 0) {
      event.preventDefault();
      node.focus({ preventScroll: true });
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;

    if (!active || !node.contains(active)) {
      event.preventDefault();
      first.focus({ preventScroll: true });
      return;
    }

    if (event.shiftKey && active === first) {
      event.preventDefault();
      last.focus({ preventScroll: true });
      return;
    }

    if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus({ preventScroll: true });
    }
  }

  node.addEventListener("keydown", handleKeydown);

  if (options.enabled !== false) {
    focusInitialElement(node, options);
  }

  return {
    update(nextOptions: FocusTrapOptions = {}) {
      const wasEnabled = options.enabled !== false;
      options = nextOptions;
      if (!wasEnabled && options.enabled !== false) {
        focusInitialElement(node, options);
      }
    },
    destroy() {
      node.removeEventListener("keydown", handleKeydown);
      restoreFocus(previousFocus);
    },
  };
}
