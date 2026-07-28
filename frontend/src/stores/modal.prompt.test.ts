import { get } from "svelte/store";
import { describe, expect, it } from "vitest";
import {
	activeModal,
	dismissModal,
	openPromptWithCheckbox,
} from "./modal";

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
