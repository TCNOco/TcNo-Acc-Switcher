import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { pushToast } from "../stores/toast";
import { openConfirm } from "../stores/modal";
import { formatToastWithError } from "./formatWailsError";
import { escapeHtml } from "./html";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

/**
 * Offers to delete the files the old C# version left in the install folder.
 * Only asks when removing them needs administrator rights - anything the app can
 * delete on its own is already gone by the time the window opens.
 */
export async function runLegacyInstallPromptIfNeeded(): Promise<void> {
  const info = await PlatformService.PendingLegacyInstall();
  if (!info.found) return;

  const tr = get(t);
  const ok = await openConfirm({
    title: tr("Modal_LegacyInstall_Title"),
    body:
      tr("Modal_LegacyInstall_Body", { entries: info.entries, size: info.size }) +
      `<br><code>${escapeHtml(info.dir)}</code>`,
    positiveLabel: tr("Modal_LegacyInstall_Remove"),
    negativeLabel: tr("Modal_LegacyInstall_Keep"),
    style: "yesno",
  });

  if (!ok) {
    await PlatformService.DismissLegacyInstallPrompt();
    return;
  }

  try {
    const res = await PlatformService.CleanLegacyInstall();
    if (res.declined) return; // UAC dismissed; the offer returns next launch.
    if (res.failed > 0) {
      pushToast({
        type: "warning",
        message: tr("Toast_LegacyInstallPartial", { freed: res.freed, failed: res.failed }),
        duration: 10000,
      });
      return;
    }
    pushToast({
      type: "success",
      message: tr("Toast_LegacyInstallCleaned", { freed: res.freed }),
      duration: 6000,
    });
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(tr("Toast_LegacyInstallFailed"), e),
      duration: 8000,
    });
  }
}
