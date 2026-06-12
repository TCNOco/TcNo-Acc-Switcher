import { get } from "svelte/store";
import { ImportDroppedShortcuts } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";
import { pushToast } from "../stores/toast";
import { t } from "../stores/i18n";
import { formatToastWithError, formatWailsError } from "./formatWailsError";
import { pathsAreOnlyProfileMedia } from "./profileImageDrop";
import type { FileDropAcceptor } from "../stores/fileDrop";

function isNoShortcutFilesDrop(err: unknown): boolean {
  const s = formatWailsError(err) || String(err);
  return (
    s.trim() === "Toast_ShortcutImportUnsupported" ||
    s.includes("Toast_ShortcutImportUnsupported")
  );
}

/** Creates a drop acceptor that imports shortcuts for a platform. */
export function createShortcutFileDropAcceptor(
  getPlatformName: () => string,
  onSuccess: (count: number) => void,
): FileDropAcceptor {
  return {
    labelKey: "DropOverlay_CopyShortcut",
    handle: async (paths: string[]) => {
      const tr = get(t);
      try {
        const n = await ImportDroppedShortcuts(getPlatformName(), paths);
        onSuccess(n);
        pushToast({
          type: "success",
          message: tr("Toast_ShortcutImported", { count: n }),
          duration: 6000,
        });
      } catch (e: unknown) {
        if (isNoShortcutFilesDrop(e)) {
          if (pathsAreOnlyProfileMedia(paths)) return;
          pushToast({
            type: "warning",
            message: tr("Toast_ShortcutImportUnsupported"),
            duration: 8000,
          });
        } else {
          pushToast({
            type: "error",
            message: formatToastWithError(
              tr("Toast_ShortcutImportFailed"),
              e,
            ),
            duration: 8000,
          });
        }
      }
    },
  };
}
