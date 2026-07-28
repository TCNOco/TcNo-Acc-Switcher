/**
 * Matches a folder name against a pattern where `*` stands for any run of
 * characters, as in `TcNo-Acc-Switcher-SteamGuard*`. Everything else is
 * literal, and matching ignores case: this only decides what to highlight, so
 * being generous costs nothing and a folder that fails still selects.
 */
export function folderNameMatches(name: string, pattern: string): boolean {
  const trimmedPattern = pattern.trim();
  const trimmedName = name.trim();
  if (!trimmedPattern || !trimmedName) return false;
  const source = trimmedPattern
    .split("*")
    .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    .join(".*");
  return new RegExp(`^${source}$`, "i").test(trimmedName);
}
