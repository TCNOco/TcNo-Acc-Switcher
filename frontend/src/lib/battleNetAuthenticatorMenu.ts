import type { MenuItemDef } from "../stores/contextMenu";
import { battleNetAccountLabel, matchingBattleNetAuthenticators, type BattleNetAccount, type BattleNetScreen } from "./battleNetAuthenticator";
import { steamGuardCodeRemaining as authenticatorCodeRemaining } from "./steam/steamGuardCodePeriod";

type Translate = (key: string) => string;
type Deps = {
  open: (serial?: string, screen?: BattleNetScreen) => void;
  copy: (serial: string) => void;
  browse: (serial: string) => void;
};

export function buildBattleNetAuthenticatorMenu(accountId: string, state: { unlocked: boolean; accounts: BattleNetAccount[] }, tr: Translate, deps: Deps): MenuItemDef {
  const matching = matchingBattleNetAuthenticators(state.accounts, accountId);
  const actions = (account: BattleNetAccount): MenuItemDef[] => [
    { label: tr("Authenticator_Copy"), action: () => deps.copy(account.serial), progress: authenticatorCodeRemaining },
    { label: tr("Authenticator_Browse"), action: () => deps.browse(account.serial) },
    { label: tr("Authenticator_Recovery"), action: () => deps.open(account.serial, "recovery") },
    { label: tr("Authenticator_Export"), action: () => deps.open(account.serial, "export") },
    { label: tr("Authenticator_Rename"), action: () => deps.open(account.serial, "rename") },
  ];
  const item: MenuItemDef = { label: tr("Authenticator_Title"), action: () => deps.open(matching.length === 1 ? matching[0].serial : undefined) };
  if (state.unlocked && matching.length) {
    item.children = matching.length === 1 ? actions(matching[0]) : matching.map(account => ({
      label: battleNetAccountLabel(account), action: () => deps.open(account.serial), children: actions(account),
    }));
  }
  return item;
}
