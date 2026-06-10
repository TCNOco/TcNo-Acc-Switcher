import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { offlineMode } from "../stores/offlineMode";
import { pushToast } from "../stores/toast";
import { openCrashReportPrompt, type CrashReportChoice } from "../stores/modal";
import { formatToastWithError } from "./formatWailsError";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

async function applyCrashReportChoice(choice: CrashReportChoice): Promise<void> {
  if (choice === "no") {
    await PlatformService.DiscardPendingCrashReport();
    return;
  }

  if (choice === "always") {
    await PlatformService.SetCrashReportAutoSubmit(true);
  }

  try {
    const submitted = await PlatformService.SubmitPendingCrashReport();
    if (submitted) {
      pushToast({
        type: "success",
        message: get(t)("Toast_CrashReported"),
        duration: 6000,
      });
    }
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(get(t)("Toast_CrashReportSubmitFailed"), e),
      duration: 8000,
    });
  }
}

/** Prompts to submit a pending crash dump when auto-submit is off and online. */
export async function runCrashReportPromptIfNeeded(): Promise<void> {
  if (get(offlineMode)) {
    return;
  }

  const [pending, autoSubmit] = await Promise.all([
    PlatformService.HasPendingCrashReport(),
    PlatformService.GetCrashReportAutoSubmit(),
  ]);

  if (!pending || autoSubmit) {
    return;
  }

  await applyCrashReportChoice(await openCrashReportPrompt());
}
