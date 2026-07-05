import type { Action } from "svelte/action";
import { openContextMenu, openContextMenuAtRect, type ContextMenuAnchorRect, type MenuItemDef } from "../../stores/contextMenu";

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

function isKeyboardContextMenuEvent(ev: KeyboardEvent): boolean {
  return (ev.shiftKey && ev.key === "F10") || ev.key === "ContextMenu";
}

function rectFromElement(el: HTMLElement): ContextMenuAnchorRect {
  const rect = el.getBoundingClientRect();
  return {
    left: rect.left,
    right: rect.right,
    top: rect.top,
    bottom: rect.bottom,
    width: rect.width,
    height: rect.height,
  };
}

function openMenu(getter: ContextMenuItemsGetter, beforeOpen: (() => void) | undefined, open: (items: MenuItemDef[]) => void): void {
  beforeOpen?.();
  const items = getter?.() ?? [];
  if (!items.length) {
    return;
  }
  open(items);
}

/** Right-click: optionally run beforeOpen, then open global context menu from getter. */
export const contextMenu: Action<HTMLElement, ContextMenuBinding> = (node, binding) => {
  let getter: ContextMenuItemsGetter;
  let beforeOpen: (() => void) | undefined;
  ({ getter, beforeOpen } = normalize(binding));
  const keyboardHost = node.closest("[data-dnd-cell]");
  const listenerNodes = keyboardHost instanceof HTMLElement && keyboardHost !== node ? [node, keyboardHost] : [node];

  const onCtx = (ev: MouseEvent): void => {
    if (ev.ctrlKey) {
      return;
    }
    ev.preventDefault();
    ev.stopPropagation();
    openMenu(getter, beforeOpen, (items) => openContextMenu(ev.clientX, ev.clientY, items));
  };

  const onKeyDown = (ev: KeyboardEvent): void => {
    if (!isKeyboardContextMenuEvent(ev)) {
      return;
    }
    const target = ev.target instanceof HTMLElement ? ev.target : node;
    ev.preventDefault();
    ev.stopPropagation();
    openMenu(getter, beforeOpen, (items) => openContextMenuAtRect(rectFromElement(target), items));
  };

  listenerNodes.forEach((listenerNode) => {
    listenerNode.addEventListener("contextmenu", onCtx);
    listenerNode.addEventListener("keydown", onKeyDown);
  });
  return {
    update(next: ContextMenuBinding) {
      ({ getter, beforeOpen } = normalize(next));
    },
    destroy() {
      listenerNodes.forEach((listenerNode) => {
        listenerNode.removeEventListener("contextmenu", onCtx);
        listenerNode.removeEventListener("keydown", onKeyDown);
      });
    },
  };
};
