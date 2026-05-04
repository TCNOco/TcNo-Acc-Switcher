import { writable } from "svelte/store";
import { ctxMenuLog } from "../lib/contextMenuDebug";

export type MenuItemDef = {
  /** Ignored when type is search/separator */
  label?: string;
  type?: "item" | "search" | "separator";
  /** Leaf click, or with `children` a parent row click (see ContextMenuNest). */
  action?: () => void;
  children?: MenuItemDef[];
  /** When true, the row is visible but not clickable (no action, menu stays open on click). */
  disabled?: boolean;
  /**
   * On a `type: "search"` row in a submenu: show the filter input even when the sibling list has
   * five or fewer items (default submenu search only appears above that threshold).
   */
  alwaysShowSearch?: boolean;
  /** On `type: "search"`: invoked when Enter is pressed with a non-empty trimmed query. */
  onSearchEnter?: (query: string) => void;
  /** On `type: "search"`: when true, show a synthetic `Create: <query>` row. */
  onSearchCanCreate?: (query: string) => boolean;
  /** On `type: "search"`: action for clicking synthetic `Create: <query>` row. */
  onSearchCreate?: (query: string) => void;
};

export type ContextMenuState =
  | null
  | {
      x: number;
      y: number;
      items: MenuItemDef[];
    };

export const contextMenu = writable<ContextMenuState>(null);

/** Indices from root — which branches are expanded (e.g. [4, 0] = root item 4, then child 0). */
export const submenuOpenPath = writable<number[]>([]);

/**
 * False until the menu has painted (tick + rAF). Blocks pointerenter in the same frame as mount
 * so a submenu doesn’t pop open when the menu first appears under the cursor.
 */
export const submenuExpandEnabled = writable(false);

export function openContextMenu(x: number, y: number, items: MenuItemDef[]): void {
  ctxMenuLog("openContextMenu", { x, y, itemCount: items.length });
  submenuOpenPath.set([]);
  submenuExpandEnabled.set(false);
  contextMenu.set({ x, y, items });
}

export function closeContextMenu(): void {
  ctxMenuLog("closeContextMenu");
  submenuOpenPath.set([]);
  submenuExpandEnabled.set(false);
  contextMenu.set(null);
}
