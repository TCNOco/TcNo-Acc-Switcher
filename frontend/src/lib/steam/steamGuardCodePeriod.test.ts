import { describe, expect, it } from "vitest";
import {
  steamGuardCodePeriodExpiry,
  steamGuardCodePeriodProgress,
} from "./steamGuardCodePeriod";

describe("Steam Guard code period", () => {
  it("ends at the next 30-second boundary counted off the epoch", () => {
    expect(steamGuardCodePeriodExpiry(0)).toBe(30_000);
    expect(steamGuardCodePeriodExpiry(29_999)).toBe(30_000);
    expect(steamGuardCodePeriodExpiry(30_000)).toBe(60_000);
  });

  it("counts a full window down to zero and resets", () => {
    expect(steamGuardCodePeriodProgress(30_000)).toBe(1);
    expect(steamGuardCodePeriodProgress(45_000)).toBe(0.5);
    expect(steamGuardCodePeriodProgress(59_999)).toBeCloseTo(0, 4);
    expect(steamGuardCodePeriodProgress(60_000)).toBe(1);
  });
});
