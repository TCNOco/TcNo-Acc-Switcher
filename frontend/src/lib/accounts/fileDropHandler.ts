import type { PlatformAccountAdapter } from "../../components/PlatformAccountAdapter";
import { firstProfileImagePath } from "../profileImageDrop";
import { pushToast } from "../../stores/toast";
import { formatToastWithError } from "../formatWailsError";

export interface FileDropContext {
  adapter: PlatformAccountAdapter<unknown>;
  imagePick: { open: boolean; accountId: string; displayName: string; manual: boolean };
  fileDragHoverRowId: string;
  tr: (key: string, vars?: Record<string, string | number>) => string;
  loadAccounts: () => Promise<void>;
  bumpAvatarEpoch: (id: string) => void;
  closeImagePick: () => void;
  clearFileDragHover: () => void;
}

export async function fileDropIntercept(
  paths: string[],
  ctx: FileDropContext,
): Promise<boolean> {
  const img = firstProfileImagePath(paths);
  if (!img) return false;
  try {
    if (ctx.imagePick.open && ctx.imagePick.accountId.trim()) {
      const target = ctx.imagePick.accountId.trim();
      await ctx.adapter.changeImage(target, img);
      await ctx.loadAccounts();
      ctx.bumpAvatarEpoch(target);
      pushToast({ type: "success", message: ctx.tr("Toast_AccountSaved"), duration: 4000 });
      ctx.closeImagePick();
      return true;
    }
    const hover = ctx.fileDragHoverRowId.trim();
    if (hover) {
      await ctx.adapter.changeImage(hover, img);
      await ctx.loadAccounts();
      ctx.bumpAvatarEpoch(hover);
      pushToast({ type: "success", message: ctx.tr("Toast_AccountSaved"), duration: 4000 });
      ctx.clearFileDragHover();
      return true;
    }
  } catch (e) {
    pushToast({ type: "error", message: formatToastWithError(ctx.tr("Toast_SaveFailed"), e), duration: 8000 });
    return true;
  }
  return false;
}
