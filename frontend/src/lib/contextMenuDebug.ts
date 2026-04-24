/** Verbose `[ctx-menu]` logs */
export const CTX_MENU_DEBUG = false;

export function ctxMenuLog(...args: unknown[]): void {
  if (!CTX_MENU_DEBUG) {
    return;
  }
  console.log("[ctx-menu]", ...args);
}
