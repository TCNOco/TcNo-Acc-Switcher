import { describe, expect, it } from "vitest";
import { folderNameMatches } from "./folderNamePattern";

const backups = "TcNo-Acc-Switcher-SteamGuard*";

describe("folderNameMatches", () => {
  it("matches the folders a verified backup creates", () => {
    expect(folderNameMatches("TcNo-Acc-Switcher-SteamGuard-Backup-20260728-150405", backups)).toBe(true);
    expect(folderNameMatches("TcNo-Acc-Switcher-SteamGuard-Backup-20260728-150405-01", backups)).toBe(true);
  });

  it("matches the bare prefix with nothing after it", () => {
    expect(folderNameMatches("TcNo-Acc-Switcher-SteamGuard", backups)).toBe(true);
  });

  it("ignores case", () => {
    expect(folderNameMatches("tcno-acc-switcher-steamguard-backup-1", backups)).toBe(true);
  });

  it("does not match unrelated folders", () => {
    expect(folderNameMatches("Documents", backups)).toBe(false);
    expect(folderNameMatches("SteamGuard", backups)).toBe(false);
    expect(folderNameMatches("My-TcNo-Acc-Switcher-SteamGuard", backups)).toBe(false);
  });

  it("treats everything but the star as literal", () => {
    expect(folderNameMatches("bacXup", "bac.up")).toBe(false);
    expect(folderNameMatches("bac.up", "bac.up")).toBe(true);
  });

  it("matches anything against a lone star, and nothing without a pattern", () => {
    expect(folderNameMatches("anything", "*")).toBe(true);
    expect(folderNameMatches("anything", "")).toBe(false);
    expect(folderNameMatches("", backups)).toBe(false);
  });
});
