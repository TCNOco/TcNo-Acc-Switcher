/**
 * The Linux and macOS catalogs name one app once per install method: "Discord",
 * "Discord (Flatpak)" and "Discord (Snap)" are three platforms with three
 * separate sets of login files.
 *
 * Only an install-method suffix may be dropped. "Heroic (Epic)" and "Heroic
 * (GOG)" are different account stores behind one launcher, so the allowlist
 * below is what keeps those two cases apart.
 */

const INSTALL_METHODS = new Set([
  "flatpak",
  "snap",
  "appimage",
  "native",
  "deb",
  "rpm",
  "aur",
  "homebrew",
]);

const TRAILING_PARENTHETICAL = /^(.*?)\s*\(([^()]+)\)$/;

/**
 * The name artwork is filed under: any trailing parenthetical removed, because
 * every install method of an app shares one icon.
 */
export function platformArtworkName(name: string): string {
  const trimmed = name.trim();
  return TRAILING_PARENTHETICAL.exec(trimmed)?.[1]?.trim() || trimmed;
}

/** The app name behind an install-method suffix, or null if that isn't what the suffix is. */
function installMethodBase(name: string): string | null {
  const match = TRAILING_PARENTHETICAL.exec(name.trim());
  if (!match) return null;
  const base = match[1].trim();
  if (!base) return null;
  return INSTALL_METHODS.has(match[2].trim().toLowerCase()) ? base : null;
}

/**
 * Labels for a set of platforms displayed together, keyed by platform name.
 * The install method shows only where the same app appears more than once in
 * that set - counting the un-suffixed name too, or a native Discord beside a
 * Flatpak one would leave two tiles both reading "Discord".
 */
export function platformDisplayLabels(names: readonly string[]): Map<string, string> {
  const perBase = new Map<string, number>();
  for (const name of names) {
    const base = installMethodBase(name) ?? name.trim();
    perBase.set(base, (perBase.get(base) ?? 0) + 1);
  }

  const labels = new Map<string, string>();
  for (const name of names) {
    const base = installMethodBase(name);
    labels.set(name, base !== null && perBase.get(base) === 1 ? base : name);
  }
  return labels;
}
