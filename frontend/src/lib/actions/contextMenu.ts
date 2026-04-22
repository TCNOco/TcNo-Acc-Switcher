import type { Action } from "svelte/action";
import { openContextMenu, type MenuItemDef } from "../../stores/contextMenu";

/** Right-click: open global context menu with items from the getter. */
export const contextMenu: Action<HTMLElement, () => MenuItemDef[]> = (node, getItems) => {
  let getter: () => MenuItemDef[] = getItems;
  const onCtx = (ev: MouseEvent): void => {
    ev.preventDefault();
    ev.stopPropagation();
    const items = getter?.() ?? [];
    if (!items.length) {
      return;
    }
    openContextMenu(ev.clientX, ev.clientY, items);
  };
  node.addEventListener("contextmenu", onCtx);
  return {
    update(next: () => MenuItemDef[]) {
      getter = next;
    },
    destroy() {
      node.removeEventListener("contextmenu", onCtx);
    },
  };
};
