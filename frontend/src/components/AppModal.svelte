<script lang="ts">
  import { activeModal, dismissModal, cancelActiveModal } from "../stores/modal";
  import { t } from "../stores/i18n";
  import ModalShell from "./modals/ModalShell.svelte";
  import AlertModalBody from "./modals/AlertModalBody.svelte";
  import ConfirmModalBody from "./modals/ConfirmModalBody.svelte";
  import PromptModalBody from "./modals/PromptModalBody.svelte";
  import PasswordSetupModalBody from "./modals/PasswordSetupModalBody.svelte";
  import TagExpiryModalBody from "./modals/TagExpiryModalBody.svelte";
  import FolderPickerModalBody from "./modals/FolderPickerModalBody.svelte";
  import FeedbackModalBody from "./modals/FeedbackModalBody.svelte";
  import CrashReportModalBody from "./modals/CrashReportModalBody.svelte";
  import UpdateModalBody from "./modals/UpdateModalBody.svelte";
  import SteamGuardModalBody from "./modals/SteamGuardModalBody.svelte";

  $: m = $activeModal;

  // What the open modal renders from. Only ever assigned a real modal, so the
  // markup below can dereference it freely.
  //
  // `m` alone is not safe to read inside the block: when a modal body changes
  // its own state in the same tick that dismisses the modal, Svelte flushes the
  // still-mounted slot after the store has gone null, and a dereference there
  // throws out of the update loop - which stops every component on the page
  // updating, not just this one. The page then looks frozen rather than broken.
  let shown: NonNullable<typeof m> | null = null;
  $: if (m) shown = m;

  $: modalTitle = (() => {
    if (!shown) return "";
    if (shown.kind === "update") return $t("Heading_UpdateAvailable");
    if (shown.kind === "feedback") return shown.mode === "issue" ? $t("Feedback_Issue_Title") : $t("Feedback_Suggestion_Title");
    if (shown.kind === "crashReport") return $t("Modal_CrashReport_Title");
    return shown.title;
  })();

  function onResolveAlert(): void {
    dismissModal();
  }

  function onResolve(e: CustomEvent<unknown>): void {
    dismissModal(e.detail);
  }
</script>

{#if m && shown}
  <ModalShell
    kind={shown.kind}
    title={modalTitle}
    modalId={shown.id}
    on:cancel={cancelActiveModal}
  >
    {#key shown.id}
      {#if shown.kind === "alert" || shown.kind === "alertNoButton"}
        <AlertModalBody
          dismissLabel={shown.kind === "alert" ? (shown.dismissLabel ?? $t("Ok")) : undefined}
          html={shown.body}
          component={shown.bodyComponent}
          componentProps={shown.bodyProps}
          on:resolve={onResolveAlert}
        />
      {:else if shown.kind === "confirm"}
        <ConfirmModalBody
          html={shown.body}
          component={shown.bodyComponent}
          componentProps={shown.bodyProps}
          positiveLabel={shown.positiveLabel ?? (shown.style === "yesno" ? $t("Yes") : $t("Ok"))}
          negativeLabel={shown.negativeLabel ?? $t("No")}
          style={shown.style}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "prompt"}
        <PromptModalBody
          html={shown.body}
          component={shown.bodyComponent}
          componentProps={shown.bodyProps}
          initialValue={shown.initialValue ?? ""}
          positiveLabel={shown.positiveLabel ?? $t("Ok")}
          multiline={shown.multiline ?? false}
          inputType={shown.inputType}
          checkboxLabel={shown.checkboxLabel}
          checkboxInitial={shown.checkboxInitial ?? false}
          returnCheckboxResult={shown.returnCheckboxResult ?? false}
          modalId={shown.id}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "passwordSetup"}
        <PasswordSetupModalBody
          positiveLabel={shown.positiveLabel ?? $t("Security_SetAppPassword")}
          negativeLabel={shown.negativeLabel ?? $t("Button_Cancel")}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "tagExpiry"}
        <TagExpiryModalBody
          tagName={shown.tagName}
          initialScope={shown.initialScope ?? "account"}
          initialDate={shown.initialDate ?? ""}
          initialTime={shown.initialTime ?? ""}
          positiveLabel={shown.positiveLabel ?? $t("Ok")}
          negativeLabel={shown.negativeLabel ?? $t("Button_Cancel")}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "folder"}
        <FolderPickerModalBody
          html={shown.body}
          component={shown.bodyComponent}
          componentProps={shown.bodyProps}
          initialPath={shown.initialPath ?? ""}
          dirsOnly={shown.dirsOnly ?? true}
          soughtFilename={shown.soughtFilename ?? ""}
          suggestedFolder={shown.suggestedFolder ?? ""}
          positiveLabel={shown.positiveLabel ?? (!(shown.dirsOnly ?? true) ? $t("Modal_Button_Select") : $t("Modal_SetUserdata_ChooseFolder"))}
          showPortableButton={shown.showPortableButton ?? false}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "feedback"}
        <FeedbackModalBody
          mode={shown.mode}
          platform={shown.platform ?? ""}
          on:resolve={onResolve}
        />
      {:else if shown.kind === "crashReport"}
        <CrashReportModalBody
          on:resolve={onResolve}
        />
      {:else if shown.kind === "update"}
        <UpdateModalBody
          message={shown.message}
          downloadUrl={shown.downloadUrl}
        />
      {:else if shown.kind === "steamGuard"}
        <SteamGuardModalBody
          account={shown.account}
          entry={shown.entry}
          knownAccounts={shown.knownAccounts}
          controller={shown.controller}
        />
      {/if}
    {/key}
  </ModalShell>
{/if}
