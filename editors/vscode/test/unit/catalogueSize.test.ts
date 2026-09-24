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
//
// dinah-519 AC-14 added the third and fourth tests. The totals alone admit an
// implementation that adds a seventh key and drops a ninth, reaching the same
// number by substitution, so the keys this card moves are named one at a time
// beside the counts.
//
// dinah-515 adds the fifth test, on the same reasoning, for the four settings
// the language server contributes.
//
// dinah-517 adds the sixth and seventh, for the two runtime keys its item row
// needs and the three manifest keys its filing commands need. Its counts are
// derived from the merged tree rather than carried forward from the spec,
// because three criteria on that card named totals that had gone stale before
// anybody read them.
//
// dinah-550 adds the test naming the five runtime keys Delete Comment shows.
//
// dinah-599 adds the test naming the four tree.attention keys the operator
// attention indicator's hovers draw from, and it removes item.row.commentCount,
// the count dinah-517 put at the head of an item row's label, on the
// operator's ruling that the tree carries no numbers.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");

/**
 * The six runtime keys dinah-519 adds: the collection row over a thread, the
 * two strings a comment row composes itself out of, and the three sentences a
 * refused read shows. The third of those three replaces an untranslated
 * English literal in the arm this card rewrites.
 */
const RUNTIME_ADDED: readonly string[] = [
	"tree.collection.comments",
	"comment.row.description",
	"comment.row.author",
	"tree.contents.unreadable",
	"tree.comments.unreadable",
	"tree.attachments.unreadable",
];

/**
 * The eight runtime keys dinah-519 removes, which is the whole `item.document`
 * family.
 *
 * Six are the composed document's invented headings. The other two go with the
 * document itself: the title was the served-text tab's own, and an anchor
 * file's tab is titled by its filename, and the fallback body was what a
 * document with no readable ItemView showed, and there is no document to fall
 * back.
 */
const RUNTIME_REMOVED: readonly string[] = [
	"item.document.heading.status",
	"item.document.heading.text",
	"item.document.heading.note",
	"item.document.heading.comments",
	"item.document.comment.heading",
	"item.document.commentsEmpty",
	"item.document.title",
	"item.document.textUnavailable",
];

/** The one manifest key dinah-519 adds, which titles Open Comment. */
const MANIFEST_ADDED = "manifest.command.dinah.tree.openComment.title";

/** The four manifest keys dinah-515 adds, one per language-server setting. */
const MANIFEST_ADDED_515: readonly string[] = [
	"manifest.configuration.dinah.lsp.enabled.markdownDescription",
	"manifest.configuration.dinah.lsp.annotateProse.markdownDescription",
	"manifest.configuration.dinah.lsp.pollIntervalSeconds.markdownDescription",
	"manifest.configuration.dinah.lsp.trace.server.markdownDescription",
];

function runtimeKeys(): Set<string> {
	const catalogue = JSON.parse(
		readFileSync(join(extensionRoot, "src", "locales", "en.json"), "utf8"),
	) as { entries: Record<string, unknown> };
	return new Set(Object.keys(catalogue.entries));
}

function manifestKeys(): Set<string> {
	const catalogue = JSON.parse(
		readFileSync(join(extensionRoot, "package.nls.json"), "utf8"),
	) as Record<string, unknown>;
	return new Set(Object.keys(catalogue));
}

// The count this card found on its base commit, and what it does to it.
//
// Written as a base and a movement rather than as the total, because the total
// is the answer rather than the claim. dinah-525 branched from bddf4c16, where
// the runtime catalogue carried 194 entries. It drops thirteen: dinah-506's
// twelve draft keys, whose apparatus it deletes, and the label for the note an
// item's answer stopped being. It adds seven: three for composing a comment,
// one for a comment already diverged when a session opens it, and three for
// rendering an item's answer. A card that lands before this one and adds a
// key of its own moves the base: the fix is to write the base it found here
// and say so in its handoff, not to guess at a new total.
const CATALOGUE_BASE = 194;
const CATALOGUE_REMOVED_BY_THIS_CARD = 13;
const CATALOGUE_ADDED_BY_THIS_CARD = 7;

/**
 * The two runtime keys dinah-517 adds: the count drawn at the head of an item
 * row's label, and the label before the state its tooltip now carries.
 */
const RUNTIME_ADDED_517: readonly string[] = [
	"item.row.commentCount",
	"item.stateLabel",
];

/**
 * The one runtime key dinah-517 removes. The kind quick pick is gone, because
 * each filing command carries its own kind, so the placeholder over that pick
 * is a string nobody renders.
 */
const RUNTIME_REMOVED_517 = "form.file.kind.placeholder";

/** What dinah-517 does to the runtime total: two keys in, one out. */
const CATALOGUE_ADDED_BY_517 = 2;
const CATALOGUE_REMOVED_BY_517 = 1;

/**
 * The five runtime keys dinah-550 adds for deleting a comment: the first
 * confirmation for one comment and for several, its action label, and the
 * second confirmation a comment that is an item's answer of record raises,
 * with its own action label.
 */
const RUNTIME_ADDED_550: readonly string[] = [
	"dialog.comment.delete.confirm",
	"dialog.comment.delete.confirm.many",
	"dialog.comment.delete.action",
	"dialog.comment.delete.designated.confirm",
	"dialog.comment.delete.designated.action",
];

/**
 * The one runtime key dinah-545 adds: the heading above the listing of a
 * column's attachments in the served instructions tab.
 */
const RUNTIME_ADDED_545: readonly string[] = ["servedText.heading.columnAttachments"];

/**
 * The three runtime keys dinah-597 adds: the prompt Unblock raises over one
 * card and over several, and the history row for a lift that said why.
 */
const RUNTIME_ADDED_597: readonly string[] = [
	"dialog.unblock.reasonPrompt",
	"dialog.unblock.reasonPrompt.many",
	"history.event.unblocked.reason",
];

/**
 * The four runtime keys dinah-599 adds: the heading and the "and others" line
 * of the attention hover every row but two draws, and the two sentences the
 * two rows with a hover of their own draw as their first line.
 */
const RUNTIME_ADDED_599: readonly string[] = [
	"tree.attention.heading",
	"tree.attention.others",
	"tree.attention.blocked",
	"tree.attention.answer",
];

/**
 * The one runtime key dinah-599 removes: the comment count dinah-517 put at
 * the head of an item row's label, gone on the operator's ruling that the
 * tree carries no numbers anywhere.
 */
const RUNTIME_REMOVED_599 = "item.row.commentCount";

/** The three manifest keys dinah-517 adds, one title per filing command. */
const MANIFEST_ADDED_517: readonly string[] = [
	"manifest.command.dinah.tree.raiseQuestion.title",
	"manifest.command.dinah.tree.recordDecision.title",
	"manifest.command.dinah.tree.addCriterion.title",
];

/** The one manifest key dinah-517 removes with the command it titled. */
const MANIFEST_REMOVED_517 = "manifest.command.dinah.tree.fileItem.title";

test("the English runtime catalogue carries the base count and this card's additions", () => {
	// The count is taken over `entries` rather than over the file's top level,
	// which carries two members: the tag and the entries themselves.
	//
	// 192 before dinah-519, less its eight removals, plus its six additions,
	// plus the one skip reason dinah-518 adds for a row that names no column.
	// dinah-515 adds no runtime key, its four strings being manifest ones.
	// dinah-536 then adds Questions, Criteria and Decisions, and dinah-525
	// deletes dinah-506's draft apparatus with the twelve keys it rendered
	// and the label for the note an item's answer stopped being, and adds the
	// one the second review cycle needed for a comment already diverged when
	// a session opens it. dinah-550 adds the five strings Delete Comment
	// shows, dinah-545 the heading above a column's attachments, and
	// dinah-597 the two Unblock prompts and the reasoned unblock row, and
	// dinah-599 the four tree.attention strings less the one comment-count
	// key it removes.
	assert.equal(
		runtimeKeys().size,
		CATALOGUE_BASE -
			CATALOGUE_REMOVED_BY_THIS_CARD +
			CATALOGUE_ADDED_BY_THIS_CARD +
			CATALOGUE_ADDED_BY_517 -
			CATALOGUE_REMOVED_BY_517 +
			RUNTIME_ADDED_550.length +
			RUNTIME_ADDED_545.length +
			RUNTIME_ADDED_597.length +
			RUNTIME_ADDED_599.length -
			1,
	);
});

test("the three runtime keys dinah-597 adds are present", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED_597.length, 3);
	assert.deepEqual(
		RUNTIME_ADDED_597.filter((key) => !keys.has(key)),
		[],
		"each key above is a string Unblock asks with or a history row it writes",
	);
});

test("the base manifest catalogue carries exactly 59 keys", () => {
	// 51 before dinah-519, plus Open Comment, which it put on the comment row,
	// the one command dinah-518 puts on a column row, and the four settings
	// dinah-515's language server contributes, less the two draft commands
	// dinah-525 retires with the apparatus behind them, plus dinah-517's
	// three filing commands less the one command they replace, plus the one
	// command dinah-549 puts on a card row, plus the one command dinah-550
	// puts on a comment row.
	assert.equal(manifestKeys().size, 59);
});

test("the runtime key dinah-545 adds is present", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED_545.length, 1);
	assert.deepEqual(
		RUNTIME_ADDED_545.filter((key) => !keys.has(key)),
		[],
		"the key above is the heading the served instructions tab draws over a column's attachments",
	);
});

test("the five runtime keys dinah-550 adds are present", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED_550.length, 5);
	assert.deepEqual(
		RUNTIME_ADDED_550.filter((key) => !keys.has(key)),
		[],
		"each key above is a string Delete Comment shows",
	);
});

test("dinah-517's item.stateLabel is present and item.row.commentCount, which dinah-599 removed, is gone", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED_517.length, 2);
	assert.ok(
		keys.has("item.stateLabel"),
		"item.stateLabel is dinah-517's own record of an item's state, which dinah-599 leaves standing",
	);
	assert.equal(
		keys.has("item.row.commentCount"),
		false,
		"item.row.commentCount was dinah-517's own count at the head of an item row's label, and dinah-599 removed it: the tree carries no numbers",
	);
	assert.equal(
		keys.has(RUNTIME_REMOVED_517),
		false,
		"the kind quick pick is gone, so its placeholder renders nowhere",
	);
});

test("the three manifest keys dinah-517 adds are present and the one they replace is gone", () => {
	const keys = manifestKeys();
	assert.equal(MANIFEST_ADDED_517.length, 3);
	assert.deepEqual(
		MANIFEST_ADDED_517.filter((key) => !keys.has(key)),
		[],
		"each key above titles one of the three filing commands",
	);
	assert.equal(
		keys.has(MANIFEST_REMOVED_517),
		false,
		"File Checklist Item is removed rather than retitled",
	);
});

test("the six runtime keys dinah-519 adds are present and its eight removals are gone", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED.length, 6);
	assert.equal(RUNTIME_REMOVED.length, 8);
	assert.deepEqual(
		RUNTIME_ADDED.filter((key) => !keys.has(key)),
		[],
		"each key above was added by dinah-519 and is not in the catalogue",
	);
	assert.deepEqual(
		RUNTIME_REMOVED.filter((key) => keys.has(key)),
		[],
		"each key above belongs to the composed item document, which dinah-519 deleted",
	);
});

test("the one manifest key dinah-519 adds is present", () => {
	assert.ok(
		manifestKeys().has(MANIFEST_ADDED),
		`${MANIFEST_ADDED} is what titles Open Comment`,
	);
});

test("the four manifest keys dinah-515 adds are present", () => {
	const keys = manifestKeys();
	assert.equal(MANIFEST_ADDED_515.length, 4);
	assert.deepEqual(
		MANIFEST_ADDED_515.filter((key) => !keys.has(key)),
		[],
		"each key above describes one of the language server's settings",
	);
});

test("the four runtime keys dinah-599 adds are present", () => {
	const keys = runtimeKeys();
	assert.equal(RUNTIME_ADDED_599.length, 4);
	assert.deepEqual(
		RUNTIME_ADDED_599.filter((key) => !keys.has(key)),
		[],
		"each key above is a line of the attention hover a tree row draws when something beneath it waits on the operator",
	);
	assert.equal(
		keys.has(RUNTIME_REMOVED_599),
		false,
		"item.row.commentCount is gone: the tree carries no numbers",
	);
});
