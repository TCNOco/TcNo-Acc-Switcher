import { get } from "svelte/store";
import * as Shortcuts from "wails-shortcuts-service";
import type { MenuItemDef } from "../stores/contextMenu";
import { t } from "../stores/i18n";
import { pushToast } from "../stores/toast";
import {
  selectedAccount,
  platformActionBusy,
} from "../stores/platformPage";
import { LaunchPlatformAs } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { reportLaunchFailure } from "./adminFlow";
import { formatToastWithError } from "./formatWailsError";

function shortcutProgramLabel(fileName: string, displayName?: string): string {
  const label = String(displayName ?? "").trim();
  if (label) return label;
  return fileName.replace(/\.(lnk|url)$/i, "").trim() || fileName;
}

export async function runShortcut(
  platformName: string,
  fileName: string,
  admin: boolean,
  isUrl: boolean,
  onAccountsRefresh: () => void,
  displayName?: string,
): Promise<void> {
  const busy = get(platformActionBusy);
  if (busy.busy) return;

  let a = admin;
  const tr = get(t);
  if (a && isUrl) {
    pushToast({ type: "warning", message: tr("Toast_UrlAdminErr"), duration: 8000 });
    a = false;
  }
  try {
    const sel = get(selectedAccount);
    const uid =
      sel.platformKey === platformName
        ? String(sel.uniqueId ?? "").trim()
        : "";
    await Shortcuts.RunShortcut(platformName, fileName, a, uid);
    onAccountsRefresh();
    pushToast({
      type: "success",
      message: tr("Toast_StartedGame", {
        program: shortcutProgramLabel(fileName, displayName),
      }),
      duration: 4000,
    });
  } catch (e) {
    await reportLaunchFailure(e, platformName);
  }
}

export async function hideShortcut(
  platformName: string,
  fileName: string,
  onRefresh: () => Promise<void>,
): Promise<void> {
  const tr = get(t);
  try {
    await Shortcuts.HideShortcut(platformName, fileName);
    await onRefresh();
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(tr("Toast_SwitchFailed"), e),
      duration: 8000,
    });
  }
}

export async function openShortcutFolder(
  platformName: string,
): Promise<void> {
  const tr = get(t);
  try {
    await Shortcuts.OpenShortcutFolder(platformName);
    pushToast({
      type: "info",
      message: tr("Toast_PlaceShortcutFiles"),
      duration: 8000,
    });
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(tr("Toast_LaunchFailed"), e),
      duration: 8000,
    });
  }
}

export function buildShortcutContextMenu(opts: {
  platformName: string;
  fileName: string;
  swapLabel: string;
  onRunAsAdmin: () => void;
  onHide: () => void;
  reorder?: () => {
    canMoveLeft: boolean;
    canMoveRight: boolean;
    canPin: boolean;
    canUnpin: boolean;
    canMoveToPinned: boolean;
    canMoveToDropdown: boolean;
    onMoveLeft: () => void;
    onMoveRight: () => void;
    onPin: () => void;
    onUnpin: () => void;
    onMoveToPinned: () => void;
    onMoveToDropdown: () => void;
  };
}): () => MenuItemDef[] {
  return () => {
    const tr = get(t);
    const sel = get(selectedAccount);
    const busy = get(platformActionBusy).busy;
    const hasSel =
      sel.platformKey === opts.platformName &&
      String(sel.uniqueId ?? "").trim() !== "";
    const swapName =
      opts.swapLabel ||
      String(sel.displayName ?? "").trim() ||
      sel.uniqueId;
    const swapLabel = hasSel
      ? tr("Context_CreateShortcut_SwapTo").replace("{name}", swapName)
      : tr("Context_CreateShortcut_SelectAccount");
    const reorder = opts.reorder?.();
    const reorderItems: MenuItemDef[] = reorder
      ? [
          {
            label: tr("Context_Reorder"),
            children: [
              { label: tr("Context_MoveLeft"), disabled: !reorder.canMoveLeft || busy, action: reorder.onMoveLeft },
              { label: tr("Context_MoveRight"), disabled: !reorder.canMoveRight || busy, action: reorder.onMoveRight },
              { label: tr("Context_Pin"), disabled: !reorder.canPin || busy, action: reorder.onPin },
              { label: tr("Context_Unpin"), disabled: !reorder.canUnpin || busy, action: reorder.onUnpin },
              { label: tr("Context_MoveToPinned"), disabled: !reorder.canMoveToPinned || busy, action: reorder.onMoveToPinned },
              { label: tr("Context_MoveToDropdown"), disabled: !reorder.canMoveToDropdown || busy, action: reorder.onMoveToDropdown },
            ],
          },
        ]
      : [];

    return [
      {
        label: swapLabel,
        disabled: !hasSel || busy,
        action: async () => {
          if (!hasSel) return;
          try {
            const p = await Shortcuts.CreateGameAccountShortcut(
              opts.platformName,
              sel.uniqueId,
              sel.displayName,
              sel.accountLogin,
              opts.fileName,
            );
            pushToast({
              type: "success",
              message: `${tr("Toast_ShortcutCreated")}\n${p}`,
              duration: 8000,
            });
          } catch (e: unknown) {
            pushToast({
              type: "error",
              message: formatToastWithError(
                tr("Toast_CreateShortcutFailed"),
                e,
              ),
              duration: 8000,
            });
          }
        },
      },
      {
        label: tr("Context_RunAdmin"),
        disabled: busy,
        action: opts.onRunAsAdmin,
      },
      ...reorderItems,
      {
        label: tr("Context_Hide"),
        disabled: busy,
        action: opts.onHide,
      },
    ];
  };
}

export function buildPlatformContextMenu(
  platformName: string,
): () => MenuItemDef[] {
  return () => {
    const tr = get(t);
    const busy = get(platformActionBusy).busy;
    return [
      {
        label: tr("Context_RunAdmin"),
        disabled: busy,
        action: () => {
          void LaunchPlatformAs(platformName, true).catch((e: unknown) => {
            void reportLaunchFailure(e, platformName);
          });
        },
      },
    ];
  };
}
