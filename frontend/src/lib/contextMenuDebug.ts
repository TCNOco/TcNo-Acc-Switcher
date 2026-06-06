/** Verbose `[ctx-menu]` logs */
const CTX_MENU_DEBUG = false;

export function ctxMenuLog(...args: unknown[]): void {
  if (!CTX_MENU_DEBUG) {
    return;
  }
  console.log("[ctx-menu]", ...args);
}
