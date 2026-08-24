import { describe, expect, it } from "vitest";
import { focusEscapedSurface } from "./focusOnShow";

const surface = (holds: boolean): Element =>
  ({ contains: () => holds }) as unknown as Element;
const element = {} as Element;

describe("focusEscapedSurface", () => {
  it("takes the caret back when nothing holds focus", () => {
    expect(focusEscapedSurface(surface(false), null)).toBe(true);
  });

  it("takes the caret back when focus left the dialog", () => {
    // What the account list's row does when the context menu that opened the
    // modal hands focus back to it.
    expect(focusEscapedSurface(surface(false), element)).toBe(true);
  });

  it("leaves focus the user moved within the dialog", () => {
    expect(focusEscapedSurface(surface(true), element)).toBe(false);
  });

  it("claims nothing when there is no dialog to judge against", () => {
    expect(focusEscapedSurface(null, element)).toBe(false);
  });
});
