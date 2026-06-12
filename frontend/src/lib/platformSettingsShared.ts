export const ARG_SILENT = "-silent";
export const ARG_VGUI = "-vgui";

export const closingValues = ["Combined", "Close", "TaskKill", "Electron"] as const;

export const startingValues = ["Default", "Direct"] as const;

export function closingLabel(v: string): string {
  if (v === "Combined") return "Combined (Best)";
  if (v === "Close") return "Close";
  if (v === "TaskKill") return "TaskKill (Old)";
  if (v === "Electron") return "Electron / Discord (recommended)";
  return v;
}

export function startingLabel(v: string): string {
  if (v === "Default") return "Default (Best)";
  if (v === "Direct") return "Direct";
  return v;
}

export const overrideStates: { v: number; key: string }[] = [
  { v: -1, key: "NoDefault" },
  { v: 1, key: "Online" },
  { v: 7, key: "Invisible" },
  { v: 0, key: "Offline" },
  { v: 2, key: "Busy" },
  { v: 3, key: "Away" },
  { v: 4, key: "Snooze" },
  { v: 5, key: "LookingToTrade" },
  { v: 6, key: "LookingToPlay" },
];

export function launchArgTokens(line: string): string[] {
  return line.trim().split(/\s+/).filter((x) => x.length > 0);
}

export function hasLaunchArgFlag(line: string, flag: string): boolean {
  const f = flag.trim().toLowerCase();
  return launchArgTokens(line).some((t) => t.toLowerCase() === f);
}

export function withLaunchArgFlag(line: string, flag: string, on: boolean): string {
  const f = flag.trim();
  const lower = f.toLowerCase();
  const parts = launchArgTokens(line).filter((t) => t.toLowerCase() !== lower);
  if (on) parts.push(f);
  return parts.join(" ");
}

export function sanitizeSettingsPayload(raw: unknown): Record<string, unknown> {
  const source = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  const next: Record<string, unknown> = { ...source };
  if (!Array.isArray(next.Shortcuts)) {
    delete next.Shortcuts;
  }
  if (!next.AccountNotes || typeof next.AccountNotes !== "object" || Array.isArray(next.AccountNotes)) {
    delete next.AccountNotes;
  }
  return next;
}

export function isClosingMethodForcedPayload(raw: unknown): boolean {
  if (!raw || typeof raw !== "object") return false;
  const o = raw as Record<string, unknown>;
  return o.ClosingMethodForced === true || o.closingMethodForced === true;
}
