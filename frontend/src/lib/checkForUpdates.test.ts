import { beforeEach, expect, it, vi } from "vitest";
import { checkForUpdatesManually } from "./checkForUpdates";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { pushToast } from "../stores/toast";

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => ({
  CheckForUpdatesManually: vi.fn(),
}));
vi.mock("../stores/toast", () => ({ pushToast: vi.fn() }));
vi.mock("../stores/i18n", () => ({
  t: { subscribe: (run: (value: (key: string) => string) => void) => {
    run((key) => key);
    return () => {};
  } },
}));

beforeEach(() => vi.clearAllMocks());

it("explains an offline manual update check without reporting success or an error", async () => {
  vi.mocked(PlatformService.CheckForUpdatesManually).mockResolvedValue("offline");
  await checkForUpdatesManually();
  expect(pushToast).toHaveBeenLastCalledWith(expect.objectContaining({
    type: "info", message: "Toast_UpdateCheckOffline",
  }));
  expect(vi.mocked(pushToast).mock.calls.every(([toast]) => toast.type === "info")).toBe(true);
});

it("allows another update check after a connection failure", async () => {
  vi.mocked(PlatformService.CheckForUpdatesManually)
    .mockResolvedValueOnce("offline").mockResolvedValueOnce("up-to-date");
  await checkForUpdatesManually();
  await checkForUpdatesManually();
  expect(PlatformService.CheckForUpdatesManually).toHaveBeenCalledTimes(2);
  expect(pushToast).toHaveBeenLastCalledWith(expect.objectContaining({
    type: "success", message: "Toast_UpToDate",
  }));
});
