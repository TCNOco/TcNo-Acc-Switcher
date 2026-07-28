export type PasswordPolicyError = "empty";

export function validateNewPassword(password: string): PasswordPolicyError | null {
	if (password.length === 0) return "empty";
  return null;
}

export function passwordPolicyMessage(error: PasswordPolicyError): string {
  switch (error) {
		case "empty":
			return "Enter a password.";
  }
}
