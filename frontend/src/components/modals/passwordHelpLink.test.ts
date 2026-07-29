import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(
	new URL("./PasswordSetupModalBody.svelte", import.meta.url),
	"utf8",
);

// The allowlist in openExternalUrl is what decides whether a link opens at all.
// A URL outside it fails silently from the user's point of view, so the help
// link's host is pinned here rather than discovered in the wild.
const allowedHosts = readFileSync(
	new URL("../../lib/openExternalUrl.ts", import.meta.url),
	"utf8",
);

describe("password setup help link", () => {
	const url = source.match(/"(https:\/\/[^"]+steam-guard-passwords[^"]*)"/)?.[1];

	it("points at the documentation page", () => {
		expect(url).toBeDefined();
		expect(url).toContain("/docs/steam-guard-passwords-and-factors.md");
	});

	it("uses a host openExternalUrl will actually open", () => {
		const host = new URL(url as string).hostname;
		expect(allowedHosts).toContain(`"${host}"`);
	});

	// sanitizeHtml stamps target="_blank" on anchors and the navigation guard
	// then refuses them, so the link must be a handler rather than markup in
	// the injected body HTML.
	it("opens through openExternalUrl rather than an injected anchor", () => {
		expect(source).toContain("openExternalUrl(PASSWORD_HELP_URL)");
		expect(source).not.toMatch(/setupBodyHtml\s*=\s*`[^`]*<a\s/);
	});
});
