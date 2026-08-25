import { describe, expect, it } from "vitest";
import { formatLastLoginForLocale } from "./formatLastLogin";

const INSTANT = "2026-01-15T10:30:00Z";

describe("formatLastLoginForLocale", () => {
  // The formatter is cached per language, so the cache key has to be the
  // language - a shared formatter would render every locale the first way.
  it("keeps locales apart", () => {
    const us = formatLastLoginForLocale(INSTANT, "en-US");
    const de = formatLastLoginForLocale(INSTANT, "de-DE");
    expect(us).not.toBe("");
    expect(de).not.toBe("");
    expect(formatLastLoginForLocale(INSTANT, "en-US")).toBe(us);
    expect(formatLastLoginForLocale(INSTANT, "de-DE")).toBe(de);
  });

  it("treats an empty locale as en-US", () => {
    expect(formatLastLoginForLocale(INSTANT, "")).toBe(
      formatLastLoginForLocale(INSTANT, "en-US"),
    );
  });

  it("returns the input unchanged when it is not a date", () => {
    expect(formatLastLoginForLocale("not a date", "en-US")).toBe("not a date");
    expect(formatLastLoginForLocale("   ", "en-US")).toBe("");
  });

  // An unusable language tag must not take the account list down with it.
  it("falls back to the raw value when the locale is rejected", () => {
    expect(formatLastLoginForLocale(INSTANT, "!!not-a-locale!!")).toBe(INSTANT);
  });
});
