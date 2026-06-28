import type { AccountCommands, AccountRowProjection, SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { MenuItemDef } from "../../stores/contextMenu";
import type { TagDefRow } from "../accountTagsContext";
import { buildTagsSectionMenuItem } from "../accountTagsContext";
import { openConfirm, openPrompt } from "../../stores/modal";
import { pushToast } from "../../stores/toast";
import { formatToastWithError } from "../formatWailsError";
import * as Shortcuts from "wails-shortcuts-service";

export interface ContextMenuContext {
  name: string;
  adapter: AccountCommands & Pick<AccountRowProjection<unknown>, "name" | "imageUrl" | "manualProfileImage" | "tags" | "accountLogin" | "savedDataBroken">;
  isActionBusy: boolean;
  hasGameStatsSupport: boolean;
  tr: (key: string, vars?: Record<string, string | number>) => string;
  tagDefs: TagDefRow[];
  openImagePick: (rowId: string) => void;
  swapToLogin: () => Promise<void>;
  loadAccounts: () => Promise<void>;
  scheduleAccountsRefresh: () => void;
  loadTagDefs: () => Promise<void>;
  openGameStatsModal: (rowId: string) => void;
  onSelectedIdChanged: (id: string) => void;
}

export function buildSharedItems(
  acc: unknown,
  rowId: string,
  ctx: ContextMenuContext,
): SharedMenuItems {
  const { adapter, isActionBusy, tr, name, hasGameStatsSupport } = ctx;
  const imgUrl = (adapter.imageUrl(acc) ?? "").trim();
  const manual = adapter.manualProfileImage(acc);
  const savedDataBroken = adapter.savedDataBroken?.(acc) === true;
  const openPick = () => { ctx.onSelectedIdChanged(rowId); ctx.openImagePick(rowId); };

  return {
    swapTo: {
      label: savedDataBroken ? tr("Security_AccountDataBroken") : tr("Context_SwapTo"),
      disabled: isActionBusy || savedDataBroken,
      action: () => {
        if (isActionBusy || savedDataBroken) return;
        ctx.onSelectedIdChanged(rowId);
        void ctx.swapToLogin();
      },
    },
    changeName: {
      label: tr("Context_ChangeName"),
      action: async () => {
        const next = await openPrompt({
          title: tr("Context_ChangeName"), body: "",
          positiveLabel: tr("Ok"), negativeLabel: tr("Button_Cancel"),
          initialValue: adapter.name(acc) ?? "",
        });
        if (next === null || !String(next).trim()) return;
        try {
          await adapter.rename(rowId, String(next).trim());
          await ctx.loadAccounts();
          pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
        }
      },
    },
    createShortcut: {
      label: tr("Context_CreateShortcut"),
      action: async () => {
        try {
          const p = await Shortcuts.CreateAccountShortcut(
            name, rowId, adapter.name(acc) ?? rowId, "", "", adapter.accountLogin(acc) ?? "",
          );
          pushToast({ type: "success", message: `${tr("Toast_ShortcutCreated")}\n${p}`, duration: 6000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
        }
      },
    },
    changeImage: (!imgUrl || !manual)
      ? { label: tr("Context_ChangeImage"), action: openPick }
      : {
          label: tr("Context_ChangeImage"), action: openPick,
          children: [
            { label: tr("Context_ChooseProfileImage"), action: openPick },
            {
              label: tr("Context_RemoveProfileImage"),
              action: async () => {
                try {
                  await adapter.clearManualImage(rowId);
                  ctx.scheduleAccountsRefresh();
                  pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
                } catch (e) {
                  pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
                }
              },
            },
          ],
        },
    forget: {
      label: tr("Forget"),
      action: async () => {
        const ok = await openConfirm({
          title: tr("Forget"), body: tr("Prompt_ForgetAccount", { platform: name }), style: "yesno",
        });
        if (!ok) return;
        try {
          await adapter.forget(rowId);
          await ctx.loadAccounts();
          pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
        }
      },
    },
    notes: {
      label: tr("Notes"),
      action: async () => {
        const cur = await adapter.getNote(rowId);
        const note = await openPrompt({
          title: tr("Notes"),
          body: tr("Modal_Title_AccountNotes", { accountName: adapter.name(acc) ?? rowId }),
          positiveLabel: tr("Ok"), negativeLabel: tr("Button_Cancel"),
          initialValue: cur ?? "", multiline: true,
        });
        if (note === null) return;
        try {
          await adapter.setNote(rowId, String(note));
          await ctx.loadAccounts();
          pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
        }
      },
    },
    tags: buildTagsSectionMenuItem({
      platformKey: name, uniqueId: rowId,
      assignedTags: (adapter.tags(acc) ?? []) as TagDefRow[],
      tagDefs: ctx.tagDefs, tr,
      afterChange: async () => { await ctx.loadAccounts(); await ctx.loadTagDefs(); },
      onSuccess: () => pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 2500 }),
      onError: (e) => pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 }),
    }),
    gameStats: hasGameStatsSupport
      ? { label: tr("Context_ManageGameStats"), action: () => ctx.openGameStatsModal(rowId) }
      : null,
  };
}
