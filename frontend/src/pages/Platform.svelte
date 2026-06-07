<script lang="ts">
  import { get } from "svelte/store";
  import PlatformAccountsBase from "../components/PlatformAccountsBase.svelte";
  import type { PlatformAccountAdapter } from "../components/PlatformAccountAdapter";
  import type { TagDefRow } from "../lib/accountTagsContext";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { pushToast } from "../stores/toast";
  import { t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { AccountDTO, AccountImagePatch } from "../../bindings/TcNo-Acc-Switcher/internal/basic/models.js";
  import { LaunchPlatform } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { offerRestartIfNeedsAdmin, isNeedsAdminError } from "../lib/adminFlow";
  import { openPrompt } from "../stores/modal";
  import "../styles/platformAccountsShared.scss";

  const PROFILE_FALLBACK = "/img/BasicDefault.webp";

  type BasicRow = InstanceType<typeof AccountDTO> & {
    currentSession?: boolean;
    avatarPending?: boolean;
    tags?: TagDefRow[];
    manualProfileImage?: boolean;
    showLastUsed?: boolean;
    note?: string;
    lastUsed?: string;
  };

  export let name: string;

  $: adapter = {
    platformKey: name,
    profileFallback: PROFILE_FALLBACK,

    id: (a: BasicRow) => a.uniqueId,
    name: (a: BasicRow) => a.displayName ?? "",
    imageUrl: (a: BasicRow) => a.imageUrl,
    imagePending: (a: BasicRow) => a.avatarPending ?? false,
    currentSession: (a: BasicRow) => a.currentSession ?? false,
    manualProfileImage: (a: BasicRow) => a.manualProfileImage ?? false,
    tags: (a: BasicRow) => a.tags,
    note: (a: BasicRow) => a.note ?? "",
    shouldShowNote: (a: BasicRow) => !!(a.note ?? "").trim(),
    shouldShowLastUsed: (a: BasicRow) => a.showLastUsed === true && !!(a.lastUsed ?? "").trim(),
    lastUsed: (a: BasicRow) => a.lastUsed ?? "",
    accountLogin: () => "",

    loadAccounts: () => BasicService.GetAccounts(name) as Promise<BasicRow[]>,
    swapTo: (id: string) => BasicService.SwapToAccount(name, id, []),
    saveOrder: (ids: string[]) => BasicService.SaveAccountOrder(name, ids),
    addNew: () => BasicService.AddNew(name),
    forget: (id: string) => BasicService.ForgetAccount(name, id),
    rename: (id: string, newName: string) => BasicService.RenameAccount(name, id, newName),
    changeImage: (id: string, path: string) => BasicService.ChangeAccountImage(name, id, path),
    clearManualImage: (id: string) => BasicService.ClearManualAccountProfileImage(name, id),
    getNote: (id: string) => BasicService.GetAccountNote(name, id),
    setNote: (id: string, note: string) => BasicService.SetAccountNote(name, id, note),
    launch: () => LaunchPlatform(name),

    buildMenu: (_a, shared) => [
      shared.swapTo,
      shared.changeName,
      shared.createShortcut,
      shared.changeImage,
      shared.forget,
      shared.notes,
      shared.tags,
      shared.gameStats,
    ].filter((x): x is MenuItemDef => x != null),

    updateEventName: "basic-account-image-updated",
    buildPatch: (raw: unknown) =>
      raw instanceof AccountImagePatch
        ? raw
        : AccountImagePatch.createFrom(raw as Record<string, unknown>),
    patchTargetId: (patch: unknown) => {
      const p = patch as { platformKey?: string; uniqueId?: string };
      return (p.uniqueId ?? "").trim();
    },
    applyPatch: (patch: unknown, account: BasicRow) => {
      const p = patch as {
        platformKey?: string; uniqueId?: string;
        imageUrl?: string | null; avatarPending?: boolean;
        manualProfileImage?: boolean;
      };
      const prevUrl = (account.imageUrl ?? "").trim();
      const nextUrl = p.imageUrl != null ? String(p.imageUrl).trim() : prevUrl;
      const nextPending = typeof p.avatarPending === "boolean" ? p.avatarPending : (account.avatarPending ?? false);
      const nextManual = typeof p.manualProfileImage === "boolean" ? p.manualProfileImage : (account.manualProfileImage ?? false);
      return { ...account, imageUrl: nextUrl, avatarPending: nextPending, manualProfileImage: nextManual } as BasicRow;
    },

    searchHay: (a: BasicRow, _q: string) => [a.displayName, a.uniqueId, a.note ?? ""].join("\n"),

    saveCurrent: async () => {
      let suggestedName = "";
      try {
        suggestedName = await BasicService.SuggestedSaveAccountName(name);
      } catch (e) {
        await offerRestartIfNeedsAdmin(e, name);
        if (isNeedsAdminError(e)) return;
      }
      const displayName = await openPrompt({
        title: get(t)("Modal_SaveCurrent_Title"),
        body: get(t)("Modal_SaveCurrent_Body"),
        positiveLabel: get(t)("Button_SaveCurrent"),
        negativeLabel: get(t)("Button_Cancel"),
        initialValue: suggestedName || "",
      });
      if (displayName === null || !String(displayName).trim()) return;
      try {
        await BasicService.SaveCurrent(name, String(displayName).trim());
        pushToast({ type: "success", message: get(t)("Toast_AccountSaved"), duration: 4000 });
      } catch (e) {
        await offerRestartIfNeedsAdmin(e, name);
        if (isNeedsAdminError(e)) return;
        pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SaveFailed"), e), duration: 8000 });
      }
    },

    suggestedSaveName: async () => {
      try { return await BasicService.SuggestedSaveAccountName(name); }
      catch { return ""; }
    },
  } satisfies PlatformAccountAdapter<BasicRow>;
</script>

<PlatformAccountsBase {name} {adapter} />
