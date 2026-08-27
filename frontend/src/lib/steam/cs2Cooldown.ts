export type Cs2CooldownUnit = "day" | "hour" | "minute";

export type Cs2Cooldown =
  | { permanent: true }
  | { permanent: false; unit: Cs2CooldownUnit; value: number };

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

type Translate = (key: string, vars?: Record<string, string | number>) => string;

/**
 * The largest whole unit still remaining, or null when there is nothing to show.
 *
 * Null covers absent, unparseable and already-expired alike: none of them has a
 * line, and telling them apart on the tile would say nothing useful. A cooldown
 * is stored as an absolute instant, so this stays correct however long ago it
 * was fetched — which is what lets collection happen only on a vault unlock.
 */
export function cs2CooldownRemaining(
  expiresAt: string | null | undefined,
  permanent: boolean | undefined,
  nowMs: number,
): Cs2Cooldown | null {
  if (permanent === true) return { permanent: true };
  const raw = (expiresAt ?? "").trim();
  if (!raw) return null;
  const expiry = Date.parse(raw);
  if (Number.isNaN(expiry)) return null;
  const remaining = expiry - nowMs;
  if (remaining <= 0) return null;
  if (remaining >= DAY) return { permanent: false, unit: "day", value: Math.floor(remaining / DAY) };
  if (remaining >= HOUR) return { permanent: false, unit: "hour", value: Math.floor(remaining / HOUR) };
  // Under a minute still reads "1m": rounding to zero would print a line that
  // contradicts itself for the last 59 seconds.
  return { permanent: false, unit: "minute", value: Math.max(1, Math.floor(remaining / MINUTE)) };
}

const PHRASE_KEY: Record<Cs2CooldownUnit, [one: string, other: string]> = {
  day: ["Steam_Cs2Cooldown_Time_Day", "Steam_Cs2Cooldown_Time_Days"],
  hour: ["Steam_Cs2Cooldown_Time_Hour", "Steam_Cs2Cooldown_Time_Hours"],
  minute: ["Steam_Cs2Cooldown_Time_Minute", "Steam_Cs2Cooldown_Time_Minutes"],
};

/** Tooltip, and the screen-reader status line. Spelled out and pluralised. */
export function cs2CooldownTooltip(cooldown: Cs2Cooldown, tr: Translate): string {
  if (cooldown.permanent) return tr("Tooltip_SteamCs2CooldownPermanent");
  const [one, other] = PHRASE_KEY[cooldown.unit];
  const time = tr(cooldown.value === 1 ? one : other, { count: cooldown.value });
  return tr("Tooltip_SteamCs2Cooldown", { time });
}
