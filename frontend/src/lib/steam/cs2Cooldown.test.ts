import { describe, expect, it } from "vitest";
import { cs2CooldownRemaining, cs2CooldownTooltip } from "./cs2Cooldown";

const NOW = Date.parse("2026-08-07T12:00:00Z");
const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

function at(offsetMs: number): string {
  return new Date(NOW + offsetMs).toISOString();
}

const labels: Record<string, string> = {
  Tooltip_SteamCs2Cooldown: "Counter-Strike 2 cooldown remains for another {time}",
  Tooltip_SteamCs2CooldownPermanent: "Counter-Strike 2 cooldown is permanent",
  Steam_Cs2Cooldown_Time_Day: "{count} day",
  Steam_Cs2Cooldown_Time_Days: "{count} days",
  Steam_Cs2Cooldown_Time_Hour: "{count} hour",
  Steam_Cs2Cooldown_Time_Hours: "{count} hours",
  Steam_Cs2Cooldown_Time_Minute: "{count} minute",
  Steam_Cs2Cooldown_Time_Minutes: "{count} minutes",
};

const tr = (key: string, vars?: Record<string, string | number>) =>
  (labels[key] ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(vars?.[name] ?? ""));

describe("cs2CooldownRemaining", () => {
  it("picks the largest whole unit, rounding down", () => {
    const cases: [offset: number, unit: string, value: number][] = [
      [7 * DAY, "day", 7],
      [6 * DAY + 12 * HOUR, "day", 6],
      [DAY + HOUR, "day", 1],
      [DAY, "day", 1],
      [DAY - MINUTE, "hour", 23],
      [23 * HOUR, "hour", 23],
      [HOUR, "hour", 1],
      [HOUR - MINUTE, "minute", 59],
      [16 * MINUTE + 30_000, "minute", 16],
      [MINUTE, "minute", 1],
    ];
    for (const [offset, unit, value] of cases) {
      const got = cs2CooldownRemaining(at(offset), false, NOW);
      expect(got, `offset ${offset}`).toEqual({ permanent: false, unit, value });
    }
  });

  it("still reads one minute in the final seconds", () => {
    // Rounding to zero would print a line that contradicts itself while a
    // cooldown is genuinely still running.
    expect(cs2CooldownRemaining(at(30_000), false, NOW)).toEqual({ permanent: false, unit: "minute", value: 1 });
    expect(cs2CooldownRemaining(at(1_000), false, NOW)).toEqual({ permanent: false, unit: "minute", value: 1 });
  });

  it("returns null once there is nothing to show", () => {
    // Absent, unparseable and expired all mean "no line"; distinguishing them on
    // the tile would say nothing useful.
    expect(cs2CooldownRemaining(at(0), false, NOW)).toBeNull();
    expect(cs2CooldownRemaining(at(-MINUTE), false, NOW)).toBeNull();
    expect(cs2CooldownRemaining("", false, NOW)).toBeNull();
    expect(cs2CooldownRemaining(undefined, false, NOW)).toBeNull();
    expect(cs2CooldownRemaining(null, false, NOW)).toBeNull();
    expect(cs2CooldownRemaining("not a date", false, NOW)).toBeNull();
  });

  it("reports a permanent cooldown regardless of the timestamp", () => {
    expect(cs2CooldownRemaining("", true, NOW)).toEqual({ permanent: true });
    expect(cs2CooldownRemaining(at(-DAY), true, NOW)).toEqual({ permanent: true });
  });
});

describe("cs2Cooldown display", () => {
  it("spells out the tooltip", () => {
    const threeDays = cs2CooldownRemaining(at(3 * DAY), false, NOW)!;
    expect(cs2CooldownTooltip(threeDays, tr)).toBe("Counter-Strike 2 cooldown remains for another 3 days");
  });

  it("uses the singular phrase for a count of one", () => {
    const oneHour = cs2CooldownRemaining(at(HOUR), false, NOW)!;
    expect(cs2CooldownTooltip(oneHour, tr)).toBe("Counter-Strike 2 cooldown remains for another 1 hour");

    const oneMinute = cs2CooldownRemaining(at(MINUTE), false, NOW)!;
    expect(cs2CooldownTooltip(oneMinute, tr)).toBe("Counter-Strike 2 cooldown remains for another 1 minute");
  });

  it("names a permanent cooldown instead of counting it down", () => {
    const permanent = cs2CooldownRemaining("", true, NOW)!;
    expect(cs2CooldownTooltip(permanent, tr)).toBe("Counter-Strike 2 cooldown is permanent");
  });
});
