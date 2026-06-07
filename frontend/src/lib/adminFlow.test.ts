import { describe, it, expect, vi } from "vitest";

// adminFlow.ts imports Wails bindings and Svelte stores — mock them all
vi.mock("./formatWailsError", () => ({
  formatWailsError: vi.fn(),
  formatToastWithError: vi.fn(),
}));

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => ({
  CheckAdminForPlatform: vi.fn(),
  RestartAsAdmin: vi.fn(),
}));

vi.mock("../stores/i18n", () => ({
  t: { subscribe: vi.fn() },
}));

vi.mock("../stores/modal", () => ({
  openConfirm: vi.fn(),
}));

vi.mock("../stores/toast", () => ({
  pushToast: vi.fn(),
}));

import { isNeedsAdminError } from "./adminFlow";
import { formatWailsError } from "./formatWailsError";

describe("isNeedsAdminError", () => {
  it("returns true for NEEDS_ADMIN: marker", () => {
    vi.mocked(formatWailsError).mockReturnValue("NEEDS_ADMIN: requires elevation");
    expect(isNeedsAdminError({})).toBe(true);
  });

  it('returns true for Wails JSON with "Err":740 (elevation required)', () => {
    vi.mocked(formatWailsError).mockReturnValue(`{"Err": 740, "message": "access denied"}`);
    expect(isNeedsAdminError({})).toBe(true);
  });

  it("returns false for unrelated errors", () => {
    vi.mocked(formatWailsError).mockReturnValue("file not found");
    expect(isNeedsAdminError({})).toBe(false);
  });

  it("returns false for Err:7401 (wrong Windows error code)", () => {
    vi.mocked(formatWailsError).mockReturnValue(`{"Err":7401}`);
    expect(isNeedsAdminError({})).toBe(false);
  });
});
