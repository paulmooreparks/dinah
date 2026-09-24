// scripts/verify-package.mjs's own attention-icon assertion, dinah-599
// criteria/12: the packaged archive carries every composed attention icon and
// the notice attributing them, or the check fails and names what is missing.
//
// The function under test is pure over an archive's already-listed entry
// names, so this drives it with plain string arrays rather than building a
// real zip. scripts/verify-package.mjs guards its own top-level run behind an
// import.meta check (the same guard scripts/package.mjs already uses), so
// importing it here reads no real archive and does not exit the process. The
// script is a plain ECMAScript module and this suite compiles to CommonJS, so
// it is reached through a dynamic import in a `before` hook.

import assert from "node:assert/strict";
import { join } from "node:path";
import { before, test } from "node:test";
import { pathToFileURL } from "node:url";

// The type is resolved against this source file's own location, which is
// where scripts/verify-package.d.mts sits beside its sibling .mjs, two
// directories up. The module itself is loaded at run time from an absolute
// path instead (below), because this suite compiles to out/test/unit, one
// directory deeper than the source tree, and a literal relative specifier
// cannot resolve correctly for both trees at once.
type VerifyPackage = typeof import(
	"../../scripts/verify-package.mjs",
	{ with: { "resolution-mode": "import" } }
);
let verifyPackage: VerifyPackage;

before(async () => {
	const extensionRoot = join(__dirname, "..", "..", "..");
	const modulePath = join(extensionRoot, "scripts", "verify-package.mjs");
	verifyPackage = (await import(pathToFileURL(modulePath).href)) as VerifyPackage;
});

test("the attention glyph list is not empty, so the two tests below are not vacuous", () => {
	const { ATTENTION_GLYPH_IDS, attentionIconEntries } = verifyPackage;
	assert.ok(ATTENTION_GLYPH_IDS.length > 0);
	assert.equal(attentionIconEntries.length, ATTENTION_GLYPH_IDS.length * 2);
});

test("a complete archive raises no attention problem", () => {
	const { ATTENTION_NOTICE_ENTRY, attentionIconEntries, attentionProblems } = verifyPackage;
	const entries = [ATTENTION_NOTICE_ENTRY, ...attentionIconEntries];
	assert.deepEqual(attentionProblems("universal", entries), []);
});

// Arming: the two red cases below fail by name when either check is removed,
// because a complete archive passes and each incomplete one is complete but
// for the one thing it lacks.
test("an archive missing the notice fails, naming it", () => {
	const { ATTENTION_NOTICE_ENTRY, attentionIconEntries, attentionProblems } = verifyPackage;
	const entries = [...attentionIconEntries];
	const problems = attentionProblems("universal", entries);
	assert.equal(problems.length, 1);
	assert.match(
		problems[0],
		new RegExp(ATTENTION_NOTICE_ENTRY.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")),
	);
});

test("an archive missing one icon fails, naming it", () => {
	const { ATTENTION_NOTICE_ENTRY, attentionIconEntries, attentionProblems } = verifyPackage;
	const missing = attentionIconEntries[0];
	assert.ok(missing !== undefined);
	const entries = [
		ATTENTION_NOTICE_ENTRY,
		...attentionIconEntries.filter((entry) => entry !== missing),
	];
	const problems = attentionProblems("universal", entries);
	assert.equal(problems.length, 1);
	assert.match(problems[0], new RegExp(missing.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
});

test("an archive missing several icons and the notice fails both ways", () => {
	const { attentionIconEntries, attentionProblems } = verifyPackage;
	const entries = attentionIconEntries.slice(2);
	const problems = attentionProblems("universal", entries);
	assert.equal(problems.length, 2);
});
