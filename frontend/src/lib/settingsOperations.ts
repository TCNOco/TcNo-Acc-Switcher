import { get } from "svelte/store";
import type { Writable } from "svelte/store";
import { t } from "../stores/i18n";
import { showUserDataMoveOverlay, hideUserDataMoveOverlay } from "../stores/userDataMove";
import { pushToast } from "../stores/toast";
import { formatToastWithError } from "./formatWailsError";
import { parentDisplayPath } from "./fsPaths";
import { checkForUpdatesManually } from "./checkForUpdates";
import { FOLDER_PICKER_APPDATA, FOLDER_PICKER_PORTABLE, openAlertNoButton, openFolderPicker } from "../stores/modal";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import StatsReportModalBody from "../components/modals/StatsReportModalBody.svelte";

export async function openUserDataFolder(): Promise<void> {
  try {
    await PlatformService.OpenUserDataFolder();
  } catch (e) {
    pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SaveFailed"), e), duration: 8000 });
  }
}

export async function runUserDataMove(
  action: () => Promise<void>,
  loading: Writable<boolean>,
): Promise<void> {
  if (get(loading)) return;
  loading.set(true);
  showUserDataMoveOverlay();
  try {
    await action();
  } catch (e) {
    hideUserDataMoveOverlay();
    pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SaveFailed"), e), duration: 8000 });
    loading.set(false);
  }
}

export async function openMoveUserDataModal(
  loading: Writable<boolean>,
  userDataPath: string,
): Promise<void> {
  if (get(loading)) return;
  const picked = await openFolderPicker({
    title: get(t)("Modal_Title_MoveUserdata"),
    body: get(t)("Modal_SetUserdata"),
    initialPath: parentDisplayPath(userDataPath),
    dirsOnly: true,
    showPortableButton: true,
    positiveLabel: get(t)("Modal_SetUserdata_Button"),
  });
  if (!picked) return;
  if (picked === FOLDER_PICKER_PORTABLE) {
    await runUserDataMove(() => PlatformService.MoveUserDataPortable(), loading);
    return;
  }
  if (picked === FOLDER_PICKER_APPDATA) {
    await runUserDataMove(() => PlatformService.MoveUserDataAppData(), loading);
    return;
  }
  await runUserDataMove(() => PlatformService.MoveUserDataTo(picked), loading);
}

export async function onCheckForUpdates(loading: Writable<boolean>): Promise<void> {
  if (get(loading)) return;
  loading.set(true);
  try {
    await checkForUpdatesManually();
  } finally {
    loading.set(false);
  }
}

export async function openStatsModal(): Promise<void> {
  try {
    const report = await PlatformService.GetStatsReport();
    void openAlertNoButton({
      title: get(t)("Settings_ViewStats"),
      bodyComponent: StatsReportModalBody,
      bodyProps: { initialReport: report },
    });
  } catch (e) {
    pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_StatsReportFailed"), e), duration: 8000 });
  }
}
