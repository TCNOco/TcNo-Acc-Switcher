import { describe, expect, it } from "vitest";
import {
	MIN_PASSWORD_LENGTH,
	passwordPolicyMessage,
	validateNewPassword,
} from "./passwordPolicy";

const translate = (key: string, vars?: Record<string, string | number>) =>
	vars ? `${key}:${JSON.stringify(vars)}` : key;

describe("new password policy", () => {
	it("accepts any password of at least the minimum length", () => {
		expect(validateNewPassword("abcde")).toBeNull();
		expect(validateNewPassword("password")).toBeNull();
		expect(validateNewPassword("correct horse battery staple")).toBeNull();
	});

	it("applies no composition rules", () => {
		expect(validateNewPassword("aaaaa")).toBeNull();
		expect(validateNewPassword("     ")).toBeNull();
	});

	it("rejects passwords below the minimum length", () => {
		expect(validateNewPassword("abcd")).toBe("too-short");
		expect(validateNewPassword("x")).toBe("too-short");
		expect(validateNewPassword(" ")).toBe("too-short");
	});

	it("still requires a password value", () => {
		expect(validateNewPassword("")).toBe("empty");
	});

	// Must match Go's utf8.RuneCountInString, not UTF-16 length: "🔐".length is 2.
	it("counts code points, so emoji are not double counted", () => {
		expect(validateNewPassword("🔐".repeat(4))).toBe("too-short");
		expect(validateNewPassword("🔐".repeat(MIN_PASSWORD_LENGTH))).toBeNull();
	});

	it("resolves messages through the supplied translator", () => {
		expect(passwordPolicyMessage("empty", translate)).toBe(
			"Security_PasswordRequired",
		);
		expect(passwordPolicyMessage("too-short", translate)).toBe(
			`Security_PasswordTooShort:{"count":${MIN_PASSWORD_LENGTH}}`,
		);
	});
});
