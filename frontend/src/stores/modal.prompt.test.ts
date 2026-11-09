import { get } from "svelte/store";
import { describe, expect, it } from "vitest";
import {
	activeModal,
	dismissModal,
	openPrompt,
	openPromptWithCheckbox,
} from "./modal";

describe("text prompt dismissal", () => {
	it.each([undefined, null])("returns null when dismissed with %s", async (result) => {
		const pending = openPrompt({ title: "Save current session" });
		dismissModal(result);
		await expect(pending).resolves.toBeNull();
	});

	it("preserves submitted text", async () => {
		const pending = openPrompt({ title: "Save current session" });
		dismissModal("My account");
		await expect(pending).resolves.toBe("My account");
	});
});

describe("prompt checkbox", () => {
	it("starts unchecked and resolves the entered value with the checkbox state", async () => {
		const pending = openPromptWithCheckbox({
			title: "Unlock Steam Guard",
			inputType: "password",
			checkboxLabel: "Remember Me",
		});

		expect(get(activeModal)).toMatchObject({
			kind: "prompt",
			checkboxLabel: "Remember Me",
			checkboxInitial: false,
			returnCheckboxResult: true,
		});

		dismissModal({ value: "vault password", checked: true });
		await expect(pending).resolves.toEqual({
			value: "vault password",
			checked: true,
		});
	});
});
