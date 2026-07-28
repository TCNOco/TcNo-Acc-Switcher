import { afterEach, describe, expect, it, vi } from "vitest";
import {
  configureSteamGuardDropAdapter,
  handleSteamGuardDrop,
  setSteamGuardDropTarget,
  type SteamGuardDropAdapter,
} from "./steamGuardDrop";

function adapter(): SteamGuardDropAdapter & {
  importMaFiles: ReturnType<typeof vi.fn>;
  decodeQrScreenshot: ReturnType<typeof vi.fn>;
  reportError: ReturnType<typeof vi.fn>;
} {
  return {
    importMaFiles: vi.fn().mockResolvedValue(undefined),
    decodeQrScreenshot: vi.fn().mockResolvedValue(undefined),
    reportError: vi.fn(),
  };
}

afterEach(() => {
  configureSteamGuardDropAdapter(null);
});
describe("Steam Guard file drops", () => {
  it("prioritizes absolute maFiles and ignores unrelated paths", async () => {
    const service = adapter();
    configureSteamGuardDropAdapter(service);

    await expect(handleSteamGuardDrop([
      "C:\\auth\\one.maFile",
      "relative.maFile",
      "C:\\images\\avatar.png",
    ])).resolves.toBe(true);
    expect(service.importMaFiles).toHaveBeenCalledWith(["C:\\auth\\one.maFile"]);
    expect(service.decodeQrScreenshot).not.toHaveBeenCalled();
  });

  it("only sends an image to QR decoding while the QR target is active", async () => {
    const service = adapter();
    configureSteamGuardDropAdapter(service);

    await expect(handleSteamGuardDrop(["C:\\shot.png"])).resolves.toBe(false);
    setSteamGuardDropTarget("qr");
    await expect(handleSteamGuardDrop(["C:\\shot.png"])).resolves.toBe(true);
    expect(service.decodeQrScreenshot).toHaveBeenCalledWith("C:\\shot.png");
  });

  it("consumes ambiguous QR image drops without choosing one", async () => {
    const service = adapter();
    configureSteamGuardDropAdapter(service);
    setSteamGuardDropTarget("qr");

    await expect(handleSteamGuardDrop(["C:\\one.png", "C:\\two.jpg"])).resolves.toBe(true);
    expect(service.decodeQrScreenshot).not.toHaveBeenCalled();
    expect(service.reportError).toHaveBeenCalledOnce();
  });

	it("routes the active modal drop through its account-bound handler", async () => {
		const service = adapter();
		const handler = vi.fn().mockResolvedValue(undefined);
		configureSteamGuardDropAdapter(service);
		setSteamGuardDropTarget("qr", handler);

		await expect(handleSteamGuardDrop(["C:\\active-login.jpg"])).resolves.toBe(true);
		expect(handler).toHaveBeenCalledWith("C:\\active-login.jpg");
		expect(service.decodeQrScreenshot).not.toHaveBeenCalled();
	});
});
