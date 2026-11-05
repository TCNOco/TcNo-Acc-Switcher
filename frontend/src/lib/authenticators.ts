import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { openAlertNoButton } from "../stores/modal";
import type { BattleNetScreen } from "./battleNetAuthenticator";
import { refreshBattleNetAuthenticators } from "./battleNetAuthenticatorBridge";

export async function openAuthenticator(provider: string, accountLink = "", suggestedLabel = "", initialSerial = "", initialScreen: BattleNetScreen = "home"): Promise<void> {
  if (provider !== "battlenet") return;
  const { default: bodyComponent } = await import("../components/modals/BattleNetAuthenticatorModal.svelte");
  await openAlertNoButton({
    authenticatorLayout: true,
    title: get(t)("Authenticator_Title"),
    bodyComponent,
    bodyProps: { accountLink, suggestedLabel, initialSerial, initialScreen },
  });
  await refreshBattleNetAuthenticators();
}
