import { derived, get, writable } from "svelte/store";
import * as SecurityService from "../../bindings/TcNo-Acc-Switcher/internal/security/securityservice.js";
import { InterruptedRestoreInfo, QuarantineInfo, Status } from "../../bindings/TcNo-Acc-Switcher/internal/security/models.js";

export type SecurityStatus = InstanceType<typeof Status>;
export type SecurityQuarantineInfo = InstanceType<typeof QuarantineInfo>;
export type SecurityInterruptedRestoreInfo = InstanceType<typeof InterruptedRestoreInfo>;

function defaultStatus(): SecurityStatus {
  return new Status({
    appPasswordSet: false,
    appLocked: false,
    savedAccountDataEncrypted: false,
    operationBusy: false,
    quarantineCount: 0,
    interruptedRestorePending: false,
  });
}

export const securityStatus = writable<SecurityStatus>(defaultStatus());
export const securityStatusLoaded = writable(false);
export const securityProgressMessage = writable("");
export const securityProgressActive = derived(
  [securityStatus, securityProgressMessage],
  ([$securityStatus, $securityProgressMessage]) =>
    $securityStatus.operationBusy || $securityProgressMessage.trim() !== "",
);

export async function loadSecurityStatus(): Promise<SecurityStatus> {
  const status = await SecurityService.GetSecurityStatus();
  securityStatus.set(status);
  securityStatusLoaded.set(true);
  return status;
}

export async function setAppPassword(password: string): Promise<void> {
  await SecurityService.SetAppPassword(password);
  await loadSecurityStatus();
}

export async function unlockApp(password: string): Promise<void> {
  await SecurityService.UnlockApp(password);
  await loadSecurityStatus();
}

export async function removeAppPassword(password: string): Promise<void> {
  await runSecurityProgress("Security_Progress_RemovePassword", async () => {
    await SecurityService.RemoveAppPassword(password);
  });
}

export async function enableSavedAccountEncryption(password: string): Promise<void> {
  await runSecurityProgress("Security_Progress_Encrypt", async () => {
    await SecurityService.EnableSavedAccountEncryption(password);
  });
}

export async function disableSavedAccountEncryption(password: string): Promise<void> {
  await runSecurityProgress("Security_Progress_Decrypt", async () => {
    await SecurityService.DisableSavedAccountEncryption(password);
  });
}

export async function deleteQuarantine(id: string): Promise<void> {
  await SecurityService.DeleteQuarantine(id);
  await loadSecurityStatus();
}

export async function listQuarantines(): Promise<SecurityQuarantineInfo[]> {
  return SecurityService.ListQuarantines();
}

export async function retryQuarantineImport(id: string, password: string): Promise<void> {
  await runSecurityProgress("Security_Progress_RetryQuarantine", async () => {
    await SecurityService.RetryQuarantineImport(id, password);
  });
}

export async function listInterruptedRestores(): Promise<SecurityInterruptedRestoreInfo[]> {
  return SecurityService.ListInterruptedRestores();
}

export async function repairInterruptedRestore(): Promise<void> {
  await runSecurityProgress("Security_Progress_RepairInterruptedRestore", async () => {
    await SecurityService.RepairInterruptedRestore();
  });
}

export function isWeakPassword(password: string): boolean {
  const p = password.trim().toLowerCase();
  if (p.length < 8) return true;
  return ["password", "123456", "12345678", "qwerty", "letmein"].includes(p);
}

export function currentSecurityStatus(): SecurityStatus {
  return get(securityStatus);
}

async function runSecurityProgress(key: string, fn: () => Promise<void>): Promise<void> {
  securityProgressMessage.set(key);
  try {
    await fn();
    await loadSecurityStatus();
  } finally {
    securityProgressMessage.set("");
  }
}
