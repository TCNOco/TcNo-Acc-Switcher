import type { PlatformAccountAdapter } from "../../components/PlatformAccountAdapter";
import { pushToast } from "../../stores/toast";
import { platformActionBusy } from "../../stores/platformPage";
import { actionBarStatus } from "../../stores/fileDrop";
import { formatToastWithError } from "../formatWailsError";
import { isNeedsAdminError, offerRestartIfNeedsAdmin, reportLaunchFailure } from "../adminFlow";
import { get } from "svelte/store";
import { t } from "../../stores/i18n";

export interface AccountActionsContext {
  name: string;
  adapter: PlatformAccountAdapter<unknown>;
  selectedId: string;
  isActionBusyValue: boolean;
  accountById: (id: string) => unknown | undefined;
  scheduleAccountsRefresh: () => void;
  touchStatus: () => void;
  setIsActionBusy: (v: boolean) => void;
}

export async function swapToLogin(ctx: AccountActionsContext): Promise<void> {
  if (!ctx.selectedId) return;
  try {
    await ctx.adapter.swapTo(ctx.selectedId);
    ctx.scheduleAccountsRefresh();
    pushToast({ type: "success", message: get(t)("Toast_AccountSwitched"), duration: 4000 });
  } catch (e) { await reportSwitchFailure(e, ctx); }
}

async function reportSwitchFailure(e: unknown, ctx: AccountActionsContext): Promise<void> {
  await offerRestartIfNeedsAdmin(e, ctx.name);
  if (isNeedsAdminError(e)) return;
  pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SwitchFailed"), e), duration: 8000 });
}

async function reportSaveFailure(e: unknown, ctx: AccountActionsContext): Promise<void> {
  await offerRestartIfNeedsAdmin(e, ctx.name);
  if (isNeedsAdminError(e)) return;
  pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SaveFailed"), e), duration: 8000 });
}

export async function launchPlatformForSelection(ctx: AccountActionsContext): Promise<void> {
  const selected = ctx.accountById(ctx.selectedId);
  if (ctx.selectedId && selected && !ctx.adapter.currentSession(selected)) {
    try { await ctx.adapter.swapTo(ctx.selectedId); }
    catch (e) { await reportSwitchFailure(e, ctx); return; }
  }
  try {
    await ctx.adapter.launch();
    ctx.scheduleAccountsRefresh();
  } catch (e) { await reportLaunchFailure(e, ctx.name); }
}

export async function runPlatformActionLocked(
  work: () => Promise<void>,
  ctx: AccountActionsContext & { setIsActionBusy: (v: boolean) => void },
): Promise<void> {
  if (ctx.isActionBusyValue) return;
  ctx.setIsActionBusy(true);
  platformActionBusy.set({ busy: true, platformKey: ctx.name });
  try { await work(); }
  finally {
    ctx.setIsActionBusy(false);
    platformActionBusy.set({ busy: false, platformKey: "" });
    ctx.touchStatus();
  }
}

export async function handlePlatformActionKind(
  kind: "login" | "addNew" | "launch" | "saveCurrent",
  ctx: AccountActionsContext,
): Promise<void> {
  await runPlatformActionLocked(async () => {
    if (kind === "launch") { await launchPlatformForSelection(ctx); return; }
    if (kind === "addNew") {
      try {
        await ctx.adapter.addNew();
        ctx.scheduleAccountsRefresh();
        pushToast({ type: "success", message: get(t)("Toast_AccountSwitched"), duration: 4000 });
      } catch (e) { await reportSwitchFailure(e, ctx); }
      return;
    }
    if (kind === "saveCurrent") {
      if (ctx.adapter.saveCurrent) {
        actionBarStatus.set(get(t)("Status_ActionBar_PreparingSave"));
        const saved = await ctx.adapter.saveCurrent();
        if (saved) ctx.scheduleAccountsRefresh();
      }
      return;
    }
    if (kind === "login") { await swapToLogin(ctx); }
  }, ctx);
}
