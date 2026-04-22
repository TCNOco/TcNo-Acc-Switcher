import { writable } from "svelte/store";

export type MenuItemDef = {
  /** Ignored when type is search/separator */
  label?: string;
  type?: "item" | "search" | "separator";
  action?: () => void;
  children?: MenuItemDef[];
};

export type ContextMenuState =
  | null
  | {
      x: number;
      y: number;
      items: MenuItemDef[];
    };

export const contextMenu = writable<ContextMenuState>(null);

export function openContextMenu(x: number, y: number, items: MenuItemDef[]): void {
  contextMenu.set({ x, y, items });
}

export function closeContextMenu(): void {
  contextMenu.set(null);
}
