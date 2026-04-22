import { get } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { t } from "../stores/i18n";
import { openConfirm } from "../stores/modal";
import { pushToast } from "../stores/toast";
import { formatWailsError } from "./formatWailsError";

const needsAdminMarker = "NEEDS_ADMIN:";

/** True when the Go backend returned [winutil.NeedsAdminPrefix] / ErrNeedsAdmin. */
export function isNeedsAdminError(err: unknown): boolean {
  const s = formatWailsError(err);
  return s.includes(needsAdminMarker);
}

/** Proactive check when opening a platform page; may restart elevated or toast if user declines. */
export async function preflightAdminForPlatform(platformKey: string): Promise<void> {
  const key = String(platformKey ?? "").trim();
  if (!key) {
    return;
  }
  try {
    const r = await PlatformService.CheckAdminForPlatform(key);
    if (!r.needsAdmin) {
      return;
    }
    const tr = get(t);
    const ok = await openConfirm({
      title: tr("Modal_Title_ConfirmAction"),
      body: tr("Prompt_RestartAsAdmin"),
      style: "yesno",
      positiveLabel: tr("Ok"),
      negativeLabel: tr("No"),
    });
    if (ok) {
      await PlatformService.RestartAsAdmin([`--page=${key}`]);
      return;
    }
    pushToast({
      type: "info",
      title: "",
      message: tr("Toast_RestartAsAdmin"),
      duration: 8000,
    });
  } catch (e) {
    pushToast({
      type: "error",
      title: "",
      message: formatWailsError(e) || String(e),
      duration: 6000,
    });
  }
}

/** After a failed swap/launch, offer restart-as-admin when the error is NEEDS_ADMIN. */
export async function offerRestartIfNeedsAdmin(err: unknown, platformKey: string): Promise<void> {
  if (!isNeedsAdminError(err)) {
    return;
  }
  const key = String(platformKey ?? "").trim();
  const tr = get(t);
  const ok = await openConfirm({
    title: tr("Modal_Title_ConfirmAction"),
    body: tr("Prompt_RestartAsAdmin"),
    style: "yesno",
    positiveLabel: tr("Ok"),
    negativeLabel: tr("No"),
  });
  if (ok && key) {
    await PlatformService.RestartAsAdmin([`--page=${key}`]);
  }
}
