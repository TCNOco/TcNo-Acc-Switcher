export type BattleNetAccount = {
  serial: string;
  label: string;
  pending: boolean;
  hasRestoreCode: boolean;
  accountLink: string;
  requiresAccountSetup?: boolean;
};

export type BattleNetScreen = "home" | "add" | "enroll" | "import" | "restore" | "export" | "recovery" | "rename";

export type BattleNetRecovery = { serial: string; restoreCode: string; uri: string; qrImage: string };

export function battleNetAccountLabel(account: Pick<BattleNetAccount, "label" | "serial">): string {
  const label = account.label.trim();
  return label && label !== account.serial ? `${label} (${account.serial})` : account.serial;
}

export function matchingBattleNetAuthenticators(accounts: BattleNetAccount[], accountId: string): BattleNetAccount[] {
  const id = accountId.trim().toLowerCase();
  return id ? accounts.filter(account => account.accountLink.trim().toLowerCase() === id) : [];
}

export function emptyBattleNetForm() {
  return {
    source: "", sourcePassword: "", selected: [] as string[], replaceExisting: false,
    accountIdentifier: "", restoreSerial: "", restoreCode: "",
    password: "", keyfile: "", backupKey: "", exportPassword: "", accountLabel: "",
  };
}

export type BattleNetForm = ReturnType<typeof emptyBattleNetForm>;

export type BattleNetActions = {
  navigate: (screen: BattleNetScreen) => void;
  selectAccount: () => void;
  pickImport: () => void;
  previewImport: () => void;
  importSelected: () => void;
  enroll: () => void;
  cancelEnrollment: () => void;
  openEnrollmentAccount: () => void;
  restore: () => void;
  recover: () => void;
  exportSelected: () => void;
  showRecovery: () => void;
  acknowledgeBackup: () => void;
  copyCode: () => void;
  copyRecovery: (field: "serial" | "restoreCode" | "uri") => void;
  browse: () => void;
  rename: () => void;
  link: () => void;
  pickKeyfile: () => Promise<string>;
};
