import { get } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { t } from "../stores/i18n";
import { pushToast } from "../stores/toast";
import { formatToastWithError } from "./formatWailsError";

let checking = false;

export function formatAppVersion(version: string): string {
  const trimmed = version.trim();
  if (!trimmed) {
    return "v0.0.0";
  }
  return trimmed.startsWith("v") ? trimmed : `v${trimmed}`;
}

export async function checkForUpdatesManually(): Promise<void> {
  if (checking) {
    return;
  }
  checking = true;
  pushToast({
    type: "info",
    message: `${get(t)("Updater_Checking")}…`,
    duration: 4000,
  });
  try {
    const result = await PlatformService.CheckForUpdatesManually();
    if (result === "failed" || result === "offline") {
      pushToast({
        type: "error",
        message: get(t)("Toast_FailedUpdateCheck"),
        duration: 8000,
      });
    }
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(get(t)("Toast_FailedUpdateCheck"), e),
      duration: 8000,
    });
  } finally {
    checking = false;
  }
}
