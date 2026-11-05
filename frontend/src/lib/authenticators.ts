import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { openAlert } from "../stores/modal";

export async function openAuthenticator(provider: string, accountLink = "", suggestedLabel = ""): Promise<void> {
  if (provider !== "battlenet") return;
  const { default: bodyComponent } = await import("../components/modals/BattleNetAuthenticatorModal.svelte");
  await openAlert({
    title: get(t)("Authenticator_Title"),
    bodyComponent,
    bodyProps: { accountLink, suggestedLabel },
  });
}
