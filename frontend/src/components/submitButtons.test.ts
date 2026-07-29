import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const srcDir = fileURLToPath(new URL("..", import.meta.url));

function svelteFiles(dir: string): string[] {
	return readdirSync(dir).flatMap((entry) => {
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) return svelteFiles(full);
		return full.endsWith(".svelte") ? [full] : [];
	});
}

// installNavigationGuard cancels native form submission app-wide, so a
// type="submit" button does nothing when clicked. Keyboard Enter still works
// through the inputs' own handlers, which is why this reads as "the button is
// dead but Enter works" rather than as an obviously broken screen.
describe("action buttons", () => {
	it("never rely on native form submission", () => {
		const offenders = svelteFiles(srcDir)
			.filter((file) => readFileSync(file, "utf8").includes('type="submit"'))
			.map((file) => file.slice(srcDir.length));
		expect(offenders).toEqual([]);
	});
});
