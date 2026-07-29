/** One enrolled way into the vault. `requires` lists the factors needed together. */
export type VaultWayIn = { requires: string[] };

export type VaultFactorSummary = {
  factors: VaultWayIn[];
  /** Whether some way in needs nothing but the password. */
  passwordOpens: boolean;
};

/**
 * The factors a caller has to collect on top of the password before it can open
 * the vault. Empty when the password alone is enough.
 *
 * Anything that re-derives a slot's key - unlocking, backing up, restoring,
 * enrolling or removing a factor - needs every factor that slot lists, so the
 * way in chosen here is the one the user is most likely able to satisfy: one
 * that includes the password they are already being asked for. A security key
 * is never listed: the backend asks the device itself.
 */
export function extraFactorsNeeded(status: VaultFactorSummary | null): string[] {
  if (!status || status.passwordOpens || status.factors.length === 0) return [];
  const wayIn = status.factors.find((factor) => factor.requires.includes("password")) ?? status.factors[0];
  return wayIn.requires.filter((kind) => kind !== "password" && kind !== "securitykey");
}

/** Whether the password prompt is worth showing at all. */
export function passwordIsUsed(status: VaultFactorSummary | null): boolean {
  if (!status || status.factors.length === 0) return true;
  return status.factors.some((factor) => factor.requires.includes("password"));
}
