import { describe, expect, it } from "vitest";
import { renderReleaseNotes } from "./releaseNotes";

describe("renderReleaseNotes", () => {
	it("renders GitHub generated release notes", () => {
		const notes = [
			"## What's Changed",
			"* Fix updater by @TCNOco in https://github.com/TCNOco/TcNo-Acc-Switcher/pull/123",
			"* Another **bold** fix",
			"",
			"**Full Changelog**: https://github.com/TCNOco/TcNo-Acc-Switcher/compare/v4.0.4...v4.0.6",
		].join("\n");

		expect(renderReleaseNotes(notes)).toBe(
			"<p><strong>What&#39;s Changed</strong></p>" +
				"<ul>" +
				'<li>Fix updater by @TCNOco in <a href="https://github.com/TCNOco/TcNo-Acc-Switcher/pull/123">https://github.com/TCNOco/TcNo-Acc-Switcher/pull/123</a></li>' +
				"<li>Another <strong>bold</strong> fix</li>" +
				"</ul>" +
				'<p><strong>Full Changelog</strong>: <a href="https://github.com/TCNOco/TcNo-Acc-Switcher/compare/v4.0.4...v4.0.6">https://github.com/TCNOco/TcNo-Acc-Switcher/compare/v4.0.4...v4.0.6</a></p>',
		);
	});

	it("renders explicit links, inline code, italics and ordered lists", () => {
		const notes = ["1. See [the docs](https://tcno.co/docs) for `SwapTo`", "2. Now *stable*"].join("\n");
		expect(renderReleaseNotes(notes)).toBe(
			"<ol>" +
				'<li>See <a href="https://tcno.co/docs">the docs</a> for <code>SwapTo</code></li>' +
				"<li>Now <em>stable</em></li>" +
				"</ol>",
		);
	});

	it("splits paragraphs on blank lines and keeps single newlines as breaks", () => {
		expect(renderReleaseNotes("line one\nline two\n\nsecond paragraph")).toBe(
			"<p>line one<br>line two</p><p>second paragraph</p>",
		);
	});

	it("escapes HTML instead of rendering it", () => {
		expect(renderReleaseNotes('<script>alert(1)</script> & <img src=x onerror=alert(1)>')).toBe(
			"<p>&lt;script&gt;alert(1)&lt;/script&gt; &amp; &lt;img src=x onerror=alert(1)&gt;</p>",
		);
	});

	it("does not linkify non-https schemes", () => {
		const out = renderReleaseNotes("[click](javascript:alert(1)) or ftp://host");
		expect(out).not.toContain("<a");
	});

	it("keeps trailing sentence punctuation out of bare links", () => {
		expect(renderReleaseNotes("See https://tcno.co/download.")).toBe(
			'<p>See <a href="https://tcno.co/download">https://tcno.co/download</a>.</p>',
		);
	});

	it("drops horizontal rules and unwraps blockquotes", () => {
		expect(renderReleaseNotes("> quoted note\n---\nafter")).toBe("<p>quoted note</p><p>after</p>");
	});
});
