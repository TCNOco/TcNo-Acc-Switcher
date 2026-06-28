<script lang="ts">
  import { activeModal, dismissModal, cancelActiveModal } from "../stores/modal";
  import { t } from "../stores/i18n";
  import ModalShell from "./modals/ModalShell.svelte";
  import AlertModalBody from "./modals/AlertModalBody.svelte";
  import ConfirmModalBody from "./modals/ConfirmModalBody.svelte";
  import PromptModalBody from "./modals/PromptModalBody.svelte";
  import PasswordSetupModalBody from "./modals/PasswordSetupModalBody.svelte";
  import FolderPickerModalBody from "./modals/FolderPickerModalBody.svelte";
  import FeedbackModalBody from "./modals/FeedbackModalBody.svelte";
  import CrashReportModalBody from "./modals/CrashReportModalBody.svelte";
  import UpdateModalBody from "./modals/UpdateModalBody.svelte";

  $: m = $activeModal;

  $: modalTitle = (() => {
    if (!m) return "";
    if (m.kind === "update") return $t("Heading_UpdateAvailable");
    if (m.kind === "feedback") return m.mode === "issue" ? $t("Feedback_Issue_Title") : $t("Feedback_Suggestion_Title");
    if (m.kind === "crashReport") return $t("Modal_CrashReport_Title");
    return m.title;
  })();

  function onResolveAlert(): void {
    dismissModal();
  }

  function onResolve(e: CustomEvent<unknown>): void {
    dismissModal(e.detail);
  }
</script>

{#if m}
  <ModalShell
    kind={m.kind}
    title={modalTitle}
    modalId={m.id}
    on:cancel={cancelActiveModal}
  >
    {#key m.id}
      {#if m.kind === "alert" || m.kind === "alertNoButton"}
        <AlertModalBody
          dismissLabel={m.kind === "alert" ? (m.dismissLabel ?? $t("Ok")) : undefined}
          html={m.body}
          component={m.bodyComponent}
          componentProps={m.bodyProps}
          on:resolve={onResolveAlert}
        />
      {:else if m.kind === "confirm"}
        <ConfirmModalBody
          html={m.body}
          component={m.bodyComponent}
          componentProps={m.bodyProps}
          positiveLabel={m.positiveLabel ?? (m.style === "yesno" ? $t("Yes") : $t("Ok"))}
          negativeLabel={m.negativeLabel ?? $t("No")}
          style={m.style}
          on:resolve={onResolve}
        />
      {:else if m.kind === "prompt"}
        <PromptModalBody
          html={m.body}
          component={m.bodyComponent}
          componentProps={m.bodyProps}
          initialValue={m.initialValue ?? ""}
          positiveLabel={m.positiveLabel ?? $t("Ok")}
          multiline={m.multiline ?? false}
          inputType={m.inputType}
          on:resolve={onResolve}
        />
      {:else if m.kind === "passwordSetup"}
        <PasswordSetupModalBody
          positiveLabel={m.positiveLabel ?? $t("Security_SetAppPassword")}
          negativeLabel={m.negativeLabel ?? $t("Button_Cancel")}
          on:resolve={onResolve}
        />
      {:else if m.kind === "folder"}
        <FolderPickerModalBody
          html={m.body}
          component={m.bodyComponent}
          componentProps={m.bodyProps}
          initialPath={m.initialPath ?? ""}
          dirsOnly={m.dirsOnly ?? true}
          soughtFilename={m.soughtFilename ?? ""}
          positiveLabel={m.positiveLabel ?? (!(m.dirsOnly ?? true) ? $t("Modal_Button_Select") : $t("Modal_SetUserdata_ChooseFolder"))}
          showPortableButton={m.showPortableButton ?? false}
          on:resolve={onResolve}
        />
      {:else if m.kind === "feedback"}
        <FeedbackModalBody
          mode={m.mode}
          platform={m.platform ?? ""}
          on:resolve={onResolve}
        />
      {:else if m.kind === "crashReport"}
        <CrashReportModalBody
          on:resolve={onResolve}
        />
      {:else if m.kind === "update"}
        <UpdateModalBody
          message={m.message}
          downloadUrl={m.downloadUrl}
        />
      {/if}
    {/key}
  </ModalShell>
{/if}
