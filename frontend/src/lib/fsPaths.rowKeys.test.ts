import { describe, expect, it } from "vitest";
import { shortcutRowKey, treeRowKey } from "./fsPaths";

// The picker lists shortcuts to folders that also appear under their drive.
// When both rows shared an identity, scrolling and arrow keys landed on the
// shortcut, which is always near the top, instead of the row in the tree.
describe("path picker row keys", () => {
  it("tells a shortcut apart from the tree row for the same folder", () => {
    const path = "C:\\Users\\tcno\\Documents";
    expect(treeRowKey(path)).not.toBe(shortcutRowKey(path));
  });

  it("gives one folder one key however its path is written", () => {
    expect(treeRowKey("C:/Users//tcno")).toBe(treeRowKey("c:\\users\\tcno"));
    expect(shortcutRowKey("C:/Users//tcno")).toBe(shortcutRowKey("c:\\users\\tcno"));
  });

  it("keeps different folders apart", () => {
    expect(treeRowKey("C:\\Users\\tcno")).not.toBe(treeRowKey("C:\\Users\\tcno2"));
  });

  it("does not fold case away from paths that are case sensitive", () => {
    expect(treeRowKey("/home/tcno")).not.toBe(treeRowKey("/home/TCNO"));
  });
});
