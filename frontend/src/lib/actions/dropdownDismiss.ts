import type { Action } from "svelte/action";

/**
 * Dispatches `dismiss` when an open dropdown should close: a pointer landing
 * anywhere outside it, or Escape.
 *
 * Apply to the `.dropdown` wrapper, not the menu — the toggle has to count as
 * "inside" so its own click still closes rather than closing and reopening.
 */
export const dropdownDismiss: Action<
  HTMLElement,
  boolean,
  { "on:dismiss": (e: CustomEvent<void>) => void }
> = (node, open = false) => {
  let listening = false;

  function dismiss(): void {
    node.dispatchEvent(new CustomEvent("dismiss"));
  }

  function onPointerDown(e: PointerEvent): void {
    if (!node.contains(e.target as Node)) dismiss();
  }

  function onKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") return;
    // Swallowed so the settings page does not also clear its search or navigate
    // back on the same keypress.
    e.preventDefault();
    e.stopPropagation();
    dismiss();
  }

  function listen(next: boolean): void {
    if (next === listening) return;
    listening = next;
    // Capture, so a menu that stops propagation on its own can't strand us open.
    if (next) {
      document.addEventListener("pointerdown", onPointerDown, true);
      document.addEventListener("keydown", onKeyDown, true);
    } else {
      document.removeEventListener("pointerdown", onPointerDown, true);
      document.removeEventListener("keydown", onKeyDown, true);
    }
  }

  listen(open);

  return {
    update(next: boolean): void {
      listen(next);
    },
    destroy(): void {
      listen(false);
    },
  };
};
