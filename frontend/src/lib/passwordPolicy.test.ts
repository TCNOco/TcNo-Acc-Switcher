import { describe, expect, it } from "vitest";
import {
	passwordPolicyMessage,
	validateNewPassword,
} from "./passwordPolicy";

describe("new password policy", () => {
	it("accepts any non-empty password without length or strength rules", () => {
		expect(validateNewPassword("x")).toBeNull();
		expect(validateNewPassword("password")).toBeNull();
		expect(validateNewPassword("correct horse battery staple")).toBeNull();
		expect(validateNewPassword(" ")).toBeNull();
		expect(validateNewPassword("🔐".repeat(10_000))).toBeNull();
	});

	it("still requires a password value", () => {
		expect(validateNewPassword("")).toBe("empty");
		expect(passwordPolicyMessage("empty")).toBe("Enter a password.");
	});
});
