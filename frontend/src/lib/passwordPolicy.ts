/** Mirrors internal/passwordpolicy. Keep both in step. */
export const MIN_PASSWORD_LENGTH = 5;

export type PasswordPolicyError = "empty" | "too-short";

type Translate = (key: string, vars?: Record<string, string | number>) => string;

export function validateNewPassword(
	password: string,
): PasswordPolicyError | null {
	if (password.length === 0) return "empty";
	// Spread counts code points, matching Go's utf8.RuneCountInString. Plain
	// .length counts UTF-16 units, so an emoji would count double.
	if ([...password].length < MIN_PASSWORD_LENGTH) return "too-short";
	return null;
}

export function passwordPolicyMessage(
	error: PasswordPolicyError,
	translate: Translate,
): string {
	switch (error) {
		case "empty":
			return translate("Security_PasswordRequired");
		case "too-short":
			return translate("Security_PasswordTooShort", {
				count: MIN_PASSWORD_LENGTH,
			});
	}
}
