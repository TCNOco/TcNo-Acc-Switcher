import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const promptSource = readFileSync(new URL("./PromptModalBody.svelte", import.meta.url), "utf8");
const steamGuardSource = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

function expectStyledCheckbox(source: string, inputMarker: string, labelMarker: string): void {
  const inputIndex = source.indexOf(inputMarker);
  expect(inputIndex).toBeGreaterThanOrEqual(0);
  const fragment = source.slice(inputIndex, inputIndex + 500);
  expect(fragment).toContain('type="checkbox"');
  expect(fragment).toContain('class="form-check-input"');
  const inputEnd = fragment.indexOf("/>");
  const visualLabel = fragment.indexOf(`<label class="form-check-label" ${labelMarker}`);
  expect(inputEnd).toBeGreaterThanOrEqual(0);
  expect(visualLabel).toBeGreaterThan(inputEnd);
  expect(fragment.slice(inputEnd + 2, visualLabel).trim()).toBe("");
}

describe("modal checkbox markup", () => {
  it("uses the app checkbox input and adjacent label contract in prompts", () => {
    expectStyledCheckbox(promptSource, "id={checkboxId}", "for={checkboxId}");
  });

  it("uses visible app checkboxes for both Steam Guard Remember Me controls", () => {
    expectStyledCheckbox(
      steamGuardSource,
      'id="steam-guard-remember-session"',
      'for="steam-guard-remember-session"',
    );
    expectStyledCheckbox(
      steamGuardSource,
      'id="steam-enrollment-remember-session"',
      'for="steam-enrollment-remember-session"',
    );
  });
});
