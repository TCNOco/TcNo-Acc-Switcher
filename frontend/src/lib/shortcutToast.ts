import type { ShortcutResult } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
import { pushToast } from "../stores/toast";

type Translate = (key: string, vars?: Record<string, string | number>) => string;

/** Toasts the outcome of a shortcut creation, distinguishing a fresh write from a re-click. */
export function pushShortcutCreatedToast(
  result: ShortcutResult,
  tr: Translate,
  duration = 6000,
): void {
  const head = tr(result.alreadyExisted ? "Toast_ShortcutAlreadyExists" : "Toast_ShortcutCreated");
  const path = String(result.path ?? "").trim();
  pushToast({
    type: "success",
    message: path ? `${head}\n${path}` : head,
    duration,
  });
}
