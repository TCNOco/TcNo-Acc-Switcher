/** Verbose `[ctx-menu]` logs; set `false` for release builds. */
export const CTX_MENU_DEBUG = true;

export function ctxMenuLog(...args: unknown[]): void {
  if (!CTX_MENU_DEBUG) {
    return;
  }
  console.log("[ctx-menu]", ...args);
}
