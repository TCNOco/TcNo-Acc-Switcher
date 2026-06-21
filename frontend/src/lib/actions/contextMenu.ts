import type { Action } from "svelte/action";
import { openContextMenu, type MenuItemDef } from "../../stores/contextMenu";

/** Menu items getter, optionally with logic that runs before the menu opens (e.g. select row). */
export type ContextMenuItemsGetter = () => MenuItemDef[];

export type ContextMenuBinding =
  | ContextMenuItemsGetter
  | {
      items: ContextMenuItemsGetter;
      /** Runs after preventDefault/stopPropagation; before building items (e.g. sync selection). */
      beforeOpen?: () => void;
    };

function normalize(binding: ContextMenuBinding): {
  getter: ContextMenuItemsGetter;
  beforeOpen?: () => void;
} {
  if (typeof binding === "function") {
    return { getter: binding };
  }
  return { getter: binding.items, beforeOpen: binding.beforeOpen };
}

/** Right-click: optionally run beforeOpen, then open global context menu from getter. */
export const contextMenu: Action<HTMLElement, ContextMenuBinding> = (node, binding) => {
  let getter: ContextMenuItemsGetter;
  let beforeOpen: (() => void) | undefined;
  ({ getter, beforeOpen } = normalize(binding));

  const onCtx = (ev: MouseEvent): void => {
    if (ev.ctrlKey) {
      return;
    }
    ev.preventDefault();
    ev.stopPropagation();
    beforeOpen?.();
    const items = getter?.() ?? [];
    if (!items.length) {
      return;
    }
    openContextMenu(ev.clientX, ev.clientY, items);
  };
  node.addEventListener("contextmenu", onCtx);
  return {
    update(next: ContextMenuBinding) {
      ({ getter, beforeOpen } = normalize(next));
    },
    destroy() {
      node.removeEventListener("contextmenu", onCtx);
    },
  };
};
