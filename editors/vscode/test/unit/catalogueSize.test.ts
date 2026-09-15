// The two catalogues carry the entries this build expects, exactly.
//
// dinah-490 AC-16. The l10n suite's own guards hold the eight runtime
// catalogues and the eight manifest ones against each other, so a key added to
// one and forgotten in another already reddens. What none of them holds is the
// total: a key deleted from all sixteen files at once leaves every one of
// those guards green, and the string it rendered simply stops appearing.
//
// The floors here are real numbers rather than a more-than-zero check, and a
// card that adds or removes a key edits them. That is the intended cost.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");

test("the English runtime catalogue carries exactly 192 entries", () => {
	// The count is taken over `entries` rather than over the file's top level,
	// which carries two members: the tag and the entries themselves.
	const catalogue = JSON.parse(
		readFileSync(join(extensionRoot, "src", "locales", "en.json"), "utf8"),
	) as { entries: Record<string, unknown> };
	assert.equal(Object.keys(catalogue.entries).length, 192);
});

test("the base manifest catalogue carries exactly 51 keys", () => {
	const catalogue = JSON.parse(
		readFileSync(join(extensionRoot, "package.nls.json"), "utf8"),
	) as Record<string, unknown>;
	assert.equal(Object.keys(catalogue).length, 51);
});
