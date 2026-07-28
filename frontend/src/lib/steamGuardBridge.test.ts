import { beforeEach, describe, expect, it, vi } from "vitest";

const service = vi.hoisted(() => ({
	GetSettingsStatus: vi.fn(),
	PickMaFiles: vi.fn(),
	Initialize: vi.fn(),
	SetFeatureEnabled: vi.fn(),
	ImportMaFiles: vi.fn(),
}));

const steamService = vi.hoisted(() => ({
	GetSteamAccountsList: vi.fn(),
}));

const modal = vi.hoisted(() => ({
	openAlert: vi.fn(),
	openPrompt: vi.fn(),
	openPromptWithCheckbox: vi.fn(),
}));

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/steamguard/service.js", () => service);
vi.mock("../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js", () => steamService);
vi.mock("../stores/modal", () => modal);

import { mergeSteamGuardAccountRows, runImport } from "./steamGuardBridge";

const status = {
	vaultConfigured: true,
	rememberPasswordForSession: false,
	unlocked: false,
	folderPath: "C:\\SteamGuard",
	savedAccountDataEncrypted: false,
};

describe("Steam Guard maFile import", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		service.GetSettingsStatus.mockResolvedValue(status);
		service.SetFeatureEnabled.mockResolvedValue(undefined);
		steamService.GetSteamAccountsList.mockResolvedValue([]);
		modal.openAlert.mockResolvedValue(undefined);
	});

	it("offers an unchecked Remember Me option and honors the selected session unlock", async () => {
		modal.openPromptWithCheckbox.mockResolvedValueOnce({
			value: "vault password",
			checked: true,
		});
		service.ImportMaFiles.mockResolvedValueOnce([
			{ path: "C:\\plain.maFile", imported: true, steamId64: "1" },
		]);

		await runImport(["C:\\plain.maFile"]);

		expect(service.ImportMaFiles).toHaveBeenCalledWith(
			["C:\\plain.maFile"],
			"vault password",
			"",
			true,
		);
		expect(modal.openPromptWithCheckbox).toHaveBeenCalledWith(expect.objectContaining({
			title: "SteamGuard_Unlock_PromptTitle",
			checkboxLabel: "SteamGuard_RememberMe",
			checkboxInitial: false,
		}));
		expect(modal.openPrompt).not.toHaveBeenCalled();
	});

	it("imports through an existing session unlock without asking for the password", async () => {
		service.GetSettingsStatus.mockResolvedValue({
			...status,
			unlocked: true,
			rememberPasswordForSession: true,
		});
		service.ImportMaFiles.mockResolvedValueOnce([
			{ path: "C:\\plain.maFile", imported: true, steamId64: "1" },
		]);

		await runImport(["C:\\plain.maFile"]);

		expect(modal.openPromptWithCheckbox).not.toHaveBeenCalled();
		expect(service.ImportMaFiles).toHaveBeenCalledWith(
			["C:\\plain.maFile"],
			"",
			"",
			true,
		);
	});

	it("reports the current switcher name for the imported Steam account", async () => {
		modal.openPromptWithCheckbox.mockResolvedValueOnce({
			value: "vault password",
			checked: false,
		});
		service.ImportMaFiles.mockResolvedValueOnce([
			{
				path: "C:\\account.maFile",
				imported: true,
				steamId64: "76561198000000001",
				accountName: "name_from_file",
			},
		]);
		steamService.GetSteamAccountsList.mockResolvedValueOnce([
			{
				steamId64: "76561198000000001",
				displayName: "Current <Persona>",
				personaName: "Older Persona",
				accountName: "login_name",
			},
		]);

		await runImport(["C:\\account.maFile"]);

		expect(modal.openAlert).toHaveBeenCalledWith(expect.objectContaining({
			body: expect.stringContaining("SteamGuard_Import_AddedToAccount<br>Current &lt;Persona&gt;"),
		}));
		expect(modal.openAlert).toHaveBeenCalledWith(expect.objectContaining({
			body: expect.not.stringContaining("SteamGuard_Import_AddedToAccount<br>name_from_file"),
		}));
	});

	it("falls back to the maFile account name when Steam is not in the switcher", async () => {
		modal.openPromptWithCheckbox.mockResolvedValueOnce({
			value: "vault password",
			checked: false,
		});
		service.ImportMaFiles.mockResolvedValueOnce([
			{
				path: "C:\\account.maFile",
				imported: true,
				steamId64: "76561198000000002",
				accountName: "name_from_file",
			},
		]);

		await runImport(["C:\\account.maFile"]);

		expect(modal.openAlert).toHaveBeenCalledWith(expect.objectContaining({
			body: expect.stringContaining("SteamGuard_Import_AddedToAccount<br>name_from_file"),
		}));
	});

	it("prompts only after SDA decryption fails and retries only those files", async () => {
		modal.openPromptWithCheckbox.mockResolvedValueOnce({
			value: "vault password",
			checked: false,
		});
		modal.openPrompt.mockResolvedValueOnce("SDA password");
		service.ImportMaFiles
			.mockResolvedValueOnce([
				{ path: "C:\\plain.maFile", imported: true, steamId64: "1" },
				{
					path: "C:\\encrypted.maFile",
					imported: false,
					errorCode: "legacy_wrong_password_or_corrupt",
				},
			])
			.mockResolvedValueOnce([
				{ path: "C:\\encrypted.maFile", imported: true, steamId64: "2" },
			]);

		await runImport(["C:\\plain.maFile", "C:\\encrypted.maFile"]);

		expect(service.ImportMaFiles).toHaveBeenNthCalledWith(
			1,
			["C:\\plain.maFile", "C:\\encrypted.maFile"],
			"vault password",
			"",
			false,
		);
		expect(service.ImportMaFiles).toHaveBeenNthCalledWith(
			2,
			["C:\\encrypted.maFile"],
			"vault password",
			"SDA password",
			false,
		);
		expect(modal.openPrompt).toHaveBeenCalledWith(expect.objectContaining({
			title: "SteamGuard_Import_LegacyTitle",
		}));
		expect(modal.openAlert).toHaveBeenCalledWith(expect.objectContaining({
			body: expect.stringContaining("SteamGuard_Import_CountMany"),
		}));
	});
});

describe("Steam Guard account picker rows", () => {
	const summaries = [
		{ steamId64: "76561198000000001", accountName: "vault_user" },
		{ steamId64: "76561198000000002", accountName: "other_user" },
	];
	const switcherAccounts = [{
		steamId64: "76561198000000001",
		accountName: "vault_user",
		displayName: "Vault Display",
		personaName: "Vault Persona",
	}];
	const enrichment = [{
		steamId64: "76561198000000001",
		imageUrl: "/img/avatar.png",
		staticImageUrl: "/img/avatar-static.png",
		vac: true,
		ltd: true,
		showVac: true,
		showLimited: false,
	}];

	it("joins the switcher avatar and display name by steamId64", () => {
		const rows = mergeSteamGuardAccountRows({
			summaries,
			switcherAccounts,
			enrichment,
			vaultUnlocked: true,
			activeAccountId: "76561198000000001",
		});

		expect(rows[0]).toMatchObject({
			id: "76561198000000001",
			username: "vault_user",
			displayName: "Vault Display",
			imageUrl: "/img/avatar.png",
			staticImageUrl: "/img/avatar-static.png",
			vac: true,
			limited: false,
		});
	});

	it("leaves accounts that are not in the switcher without an avatar", () => {
		const rows = mergeSteamGuardAccountRows({
			summaries,
			switcherAccounts,
			enrichment,
			vaultUnlocked: true,
			activeAccountId: "76561198000000001",
		});

		expect(rows[1]).toMatchObject({ id: "76561198000000002", displayName: "other_user" });
		expect(rows[1].imageUrl).toBeUndefined();
		expect(rows[1].staticImageUrl).toBeUndefined();
	});

	it("reports every account as ready while the vault holds a session unlock", () => {
		const rows = mergeSteamGuardAccountRows({
			summaries,
			vaultUnlocked: true,
			activeAccountId: "76561198000000001",
		});

		expect(rows.map((row) => row.locked)).toEqual([false, false]);
	});

	it("locks accounts other than the held capability when the vault is not session-unlocked", () => {
		const rows = mergeSteamGuardAccountRows({
			summaries,
			vaultUnlocked: false,
			activeAccountId: "76561198000000001",
		});

		expect(rows.map((row) => row.locked)).toEqual([false, true]);
	});
});
