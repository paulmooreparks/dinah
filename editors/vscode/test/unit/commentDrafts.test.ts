// dinah-506: the draft an item comment is composed in, and what may delete it.
//
// Only two things delete a draft: a post the tool answered ok, and Discard.
// Not a closed tab, not a window reload, not a refusal, not a timeout, not a
// spawn failure and not an age. Most of what follows is that sentence driven
// one branch at a time, because every branch that ends a run without sending
// anything is a branch that could plausibly have tidied up after itself.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { SpawnOptions, SpawnOutcome, Spawner } from "../../src/cli";
import {
	DRAFTS_DIRNAME,
	DRAFT_SUFFIX,
	discardCommentDraft,
	draftPathFor,
	postCommentDraft,
	sweepDraftIndex,
} from "../../src/commentDrafts";
import type { DraftEntry } from "../../src/commentDrafts";
import { claimCard } from "../../src/cardCommands";
import { fingerprint } from "../../src/fingerprint";
import { COMMAND_COMMENT_ON_ITEM } from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import type { CheckResults, DraftLog, HostLog } from "../support/rows";
import {
	EXE,
	FOLDER,
	ROOT,
	STORAGE_ROOT,
	draftHost,
	emptyDraftLog,
	emptyLog,
	itemRow,
	ok,
	refused,
	wiringFor,
} from "../support/rows";

const ITEM = "dinah-506/questions/1";

/** Where this item's draft lands, recomputed rather than written out. */
function pathFor(item = ITEM, root = ROOT): string {
	return [
		STORAGE_ROOT,
		DRAFTS_DIRNAME,
		fingerprint(root),
		`${item.split("/").join(".")}${DRAFT_SUFFIX}`,
	].join("/");
}

/** The entry a Comment on this item writes. */
function entryFor(item = ITEM): DraftEntry {
	return { root: ROOT, folder: FOLDER, argv: ["comment", item, "-"], target: item };
}

/** A recording spawner, keeping the options object it was handed. */
function recorder(answer: SpawnOutcome = ok()): {
	readonly spawner: Spawner;
	readonly calls: string[][];
	readonly options: SpawnOptions[];
} {
	const calls: string[][] = [];
	const options: SpawnOptions[] = [];
	return {
		calls,
		options,
		spawner: async (_exe, argv, spawnOptions) => {
			calls.push([...argv]);
			options.push(spawnOptions);
			return answer;
		},
	};
}

/** Drives the Comment command through the table the editor registers from. */
async function comment(
	drafts: DraftLog,
	log: HostLog = emptyLog(),
): Promise<{ readonly calls: string[][] }> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === COMMAND_COMMENT_ON_ITEM);
	assert.notEqual(entry, undefined);
	const spawner = recorder();
	const results: CheckResults = { applied: [] };
	await (entry as (typeof ROW_COMMAND_TABLE)[number]).invoke(
		[itemRow({ ref: ITEM })],
		wiringFor(log, spawner.spawner, results, drafts),
	);
	return { calls: spawner.calls };
}

// ---------------------------------------------------------------------------
// dinah-506/criteria/21, 22: what Comment does, and what a second one does not
// ---------------------------------------------------------------------------

test("Comment writes a draft, opens it, and spawns nothing", async () => {
	const drafts = emptyDraftLog();
	const run = await comment(drafts);
	assert.deepEqual(run.calls, [], "Comment spawned dinah");
	assert.equal(drafts.written.length, 1, "writeDraft was not called exactly once");
	assert.equal(drafts.written[0].path, pathFor());
	assert.equal(drafts.written[0].text, "", "the new buffer was not created empty");
	assert.deepEqual(drafts.opened, [pathFor()]);
	assert.deepEqual(Object.keys(drafts.index), [pathFor()]);
	assert.deepEqual(drafts.index[pathFor()].argv, ["comment", ITEM, "-"]);
});

test("a second Comment reopens the draft and overwrites nothing", async () => {
	const paragraphs = "One.\n\nTwo.\n\nThree.\n";
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), paragraphs);
	drafts.index = { [pathFor()]: entryFor() };
	await comment(drafts);
	assert.deepEqual(drafts.written, [], "a draft already on disk was written over");
	assert.deepEqual(drafts.opened, [pathFor()]);
	assert.equal(drafts.disk.get(pathFor()), paragraphs);
});

test("a draft the index does not know is adopted rather than truncated", async () => {
	// The half an index-first implementation fails. The file-present,
	// entry-absent direction is reachable between two windows on one
	// workbench, because nothing VS Code documents says when a Memento write
	// in one becomes visible in the other.
	const paragraphs = "One.\n\nTwo.\n\nThree.\n";
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), paragraphs);
	assert.deepEqual(drafts.index, {});
	await comment(drafts);
	assert.deepEqual(drafts.written, [], "unsent prose was written over");
	assert.equal(drafts.disk.get(pathFor()), paragraphs);
	assert.deepEqual(Object.keys(drafts.index), [pathFor()]);
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/23 to 27, 32: the post
// ---------------------------------------------------------------------------

test("the post saves before it reads, and sends the saved bytes verbatim", async () => {
	const paragraphs = "One.\n\nTwo.\n\nThree.\n";
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), paragraphs);
	drafts.index = { [pathFor()]: entryFor() };
	const spawner = recorder();
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	const saved = drafts.order.indexOf(`saveDocument ${pathFor()}`);
	const read = drafts.order.indexOf(`readDraft ${pathFor()}`);
	assert.notEqual(saved, -1, "the post never saved the document");
	assert.notEqual(read, -1, "the post never read the file");
	assert.ok(saved < read, `the post read at ${String(read)} and saved at ${String(saved)}`);
	assert.equal(spawner.calls.length, 1);
	// The trailing newline included, because the store keeps what it is given
	// and the only way the comment reads as its author wrote it is to send
	// what its author wrote.
	assert.equal(spawner.options[0].stdin, paragraphs);
});

test("a refused post deletes nothing, keeps the entry, and records one error", async () => {
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), "three paragraphs");
	drafts.index = { [pathFor()]: entryFor() };
	const spawner = recorder(refused("dinah.unknown-card"));
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(drafts.deleted, [], "a refused post deleted the draft");
	assert.deepEqual(drafts.index[pathFor()].argv, ["comment", ITEM, "-"]);
	assert.equal(drafts.errors.length, 1, `the host recorded ${String(drafts.errors.length)} errors`);
	// The reader is told the words are safe, and that is an info rather than a
	// second red toast competing with the refusal they have to act on.
	assert.deepEqual(drafts.infos, [ENGLISH("draft.post.keptAfterRefusal", { path: pathFor() })]);
});

test("a successful post deletes the file, drops the entry and names the item", async () => {
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), "three paragraphs");
	drafts.index = { [pathFor()]: entryFor() };
	const spawner = recorder(ok({ outcome: "ok" }));
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(drafts.deleted, [pathFor()]);
	assert.equal(drafts.index[pathFor()], undefined, "the index still carries the posted draft");
	assert.equal(drafts.infos.length, 1);
	assert.ok(drafts.infos[0].includes(ITEM), drafts.infos[0]);
	assert.deepEqual(drafts.checkpoints, [FOLDER]);
});

test("an empty draft is refused locally and nothing is spawned", async () => {
	// Zero spawns is the assertion, because a run that spawned and reported
	// the tool's own malformed refusal would pass a weaker one.
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), " \n\t\n");
	drafts.index = { [pathFor()]: entryFor() };
	const spawner = recorder();
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(spawner.calls, []);
	assert.deepEqual(drafts.deleted, []);
	assert.equal(drafts.errors.length, 1);
	assert.equal(drafts.errors[0], ENGLISH("draft.post.empty", { path: pathFor() }));
});

test("a path the index does not know is refused by name and costs nothing", async () => {
	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), "three paragraphs");
	assert.deepEqual(drafts.index, {});
	const spawner = recorder();
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(spawner.calls, []);
	assert.deepEqual(drafts.deleted, []);
	assert.equal(drafts.errors.length, 1);
	assert.ok(drafts.errors[0].includes(pathFor()), drafts.errors[0]);
});

test("a post whose draft file has gone missing refuses by name and keeps the entry", async () => {
	// The entry surviving is half the assertion. The post is not the
	// housekeeper: a command that sent nothing removes nothing, and the
	// activation sweep is the one place a fileless entry is reaped.
	const drafts = emptyDraftLog();
	drafts.index = { [pathFor()]: entryFor() };
	assert.equal(drafts.disk.get(pathFor()), undefined);
	const spawner = recorder();
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(spawner.calls, []);
	assert.deepEqual(drafts.deleted, []);
	assert.deepEqual(drafts.index[pathFor()].argv, ["comment", ITEM, "-"]);
	assert.equal(drafts.errors.length, 1);
	assert.ok(drafts.errors[0].includes(pathFor()), drafts.errors[0]);
});

test("a save that did not happen ends the run with nothing spawned and nothing deleted", async () => {
	const drafts = emptyDraftLog();
	drafts.saves = false;
	drafts.disk.set(pathFor(), "three paragraphs");
	drafts.index = { [pathFor()]: entryFor() };
	const spawner = recorder();
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, pathFor());

	assert.deepEqual(spawner.calls, []);
	assert.deepEqual(drafts.deleted, []);
	assert.equal(drafts.errors.length, 1);
	assert.equal(drafts.errors[0], ENGLISH("draft.post.saveFailed", { path: pathFor() }));
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/28: Discard
// ---------------------------------------------------------------------------

test("Discard deletes only after the destructive confirmation", async () => {
	// The refusing case and the accepting case together, because a discard
	// that never deletes passes the first alone.
	const declined = emptyDraftLog();
	declined.confirmed = false;
	declined.disk.set(pathFor(), "three paragraphs");
	declined.index = { [pathFor()]: entryFor() };
	await discardCommentDraft(draftHost(declined), pathFor());
	assert.deepEqual(declined.deleted, []);
	assert.deepEqual(declined.index[pathFor()].argv, ["comment", ITEM, "-"]);
	assert.equal(declined.confirmations.length, 1);
	assert.equal(declined.confirmations[0].label, ENGLISH("draft.discard.confirmLabel"));

	const accepted = emptyDraftLog();
	accepted.confirmed = true;
	accepted.disk.set(pathFor(), "three paragraphs");
	accepted.index = { [pathFor()]: entryFor() };
	await discardCommentDraft(draftHost(accepted), pathFor());
	assert.deepEqual(accepted.deleted, [pathFor()]);
	assert.equal(accepted.index[pathFor()], undefined);
});

test("Discard refuses a path the index does not know, deleting nothing and asking nothing", async () => {
	// The data-loss route this guard exists to close. The manifest offers both
	// draft commands on any file whose name ends in DRAFT_SUFFIX, wherever it
	// sits, so an operator's own notes.dinah-comment.md can be the file in
	// front of the command. An earlier form of discardCommentDraft confirmed
	// and then deleted whatever path it was handed, which destroyed that file.
	const foreign = emptyDraftLog();
	foreign.confirmed = true;
	const outsider = `C:\\Users\\somebody\\notes${DRAFT_SUFFIX}`;
	foreign.disk.set(outsider, "an argument the operator was drafting");
	// The index knows a real draft, and it is not this path. An empty index
	// would pass a guard that refused every path, so the entry has to be here.
	foreign.index = { [pathFor()]: entryFor() };

	await discardCommentDraft(draftHost(foreign), outsider);

	assert.deepEqual(foreign.deleted, [], "Discard deleted a file the index does not know");
	assert.equal(foreign.disk.get(outsider), "an argument the operator was drafting");
	// No modal either. A confirmation for a delete that is not going to happen
	// has already told the reader something false about what is about to
	// happen, so the refusal comes first.
	assert.deepEqual(foreign.confirmations, []);
	assert.deepEqual(foreign.errors, [ENGLISH("draft.unknown", { path: outsider })]);
	// The real entry is still in the index, so the refusal dropped nothing.
	assert.notEqual(foreign.index[pathFor()], undefined);
});

test("Post and Discard refuse an unknown path through the one catalogue entry", async () => {
	// The two commands say the same thing on the same branch, so the entry is
	// the shared draft.unknown rather than a post-specific one. Driving both
	// is what keeps a later rename from splitting them silently.
	const posting = emptyDraftLog();
	posting.saves = true;
	const outsider = `C:\\Users\\somebody\\notes${DRAFT_SUFFIX}`;
	posting.disk.set(outsider, "an argument the operator was drafting");
	posting.index = { [pathFor()]: entryFor() };
	const spawner = recorder();
	await postCommentDraft(draftHost(posting), spawner.spawner, EXE, outsider);
	assert.deepEqual(spawner.calls, []);
	assert.deepEqual(posting.deleted, []);
	assert.deepEqual(posting.errors, [ENGLISH("draft.unknown", { path: outsider })]);

	const discarding = emptyDraftLog();
	discarding.confirmed = true;
	discarding.disk.set(outsider, "an argument the operator was drafting");
	discarding.index = { [pathFor()]: entryFor() };
	await discardCommentDraft(draftHost(discarding), outsider);
	assert.deepEqual(discarding.errors, posting.errors);
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/29: identity is the index, the filename is display
// ---------------------------------------------------------------------------

test("the draft path is the storage root, the workbench's fingerprint and the flattened reference", () => {
	assert.equal(draftPathFor(STORAGE_ROOT, ROOT, ITEM), pathFor());
	assert.ok(
		draftPathFor(STORAGE_ROOT, ROOT, ITEM).includes(fingerprint(ROOT)),
		"the path carries no fingerprint of the workbench root",
	);
	assert.ok(
		draftPathFor(STORAGE_ROOT, ROOT, ITEM).endsWith("dinah-506.questions.1.dinah-comment.md"),
		draftPathFor(STORAGE_ROOT, ROOT, ITEM),
	);
	// Two workbenches that both hold a dinah-506 do not collide.
	assert.notEqual(
		draftPathFor(STORAGE_ROOT, ROOT, ITEM),
		draftPathFor(STORAGE_ROOT, "C:/other/bench", ITEM),
	);
});

test("the post names the entry's target and never the item in the filename", async () => {
	const misnamed = pathFor("dinah-999/criteria/7");
	const drafts = emptyDraftLog();
	drafts.disk.set(misnamed, "three paragraphs");
	drafts.index = { [misnamed]: entryFor(ITEM) };
	const spawner = recorder(ok({ outcome: "ok" }));
	await postCommentDraft(draftHost(drafts), spawner.spawner, EXE, misnamed);

	assert.equal(spawner.calls.length, 1);
	assert.ok(
		spawner.calls[0].includes(ITEM),
		`the argv names ${spawner.calls[0].join(" ")} rather than the entry's target`,
	);
	assert.ok(
		!spawner.calls[0].includes("dinah-999/criteria/7"),
		"the argv names the item in the filename",
	);
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/30: runVerb's stdin parameter, both ways
// ---------------------------------------------------------------------------

test("runVerb's stdin parameter is additive in both directions", async () => {
	// The accepting case beside the refusing one, because an implementation
	// always setting stdin to the empty string passes the second half alone,
	// and Object.hasOwn is what tells an absent key from a present undefined.
	const log = emptyLog();
	const claiming = recorder();
	await claimCard({
		spawner: claiming.spawner,
		exe: EXE,
		host: {
			...(wiringFor(log, claiming.spawner).cardHost as object),
		} as Parameters<typeof claimCard>[0]["host"],
		folder: FOLDER,
		root: ROOT,
		ref: "wb-1",
	});
	assert.equal(claiming.options.length, 1);
	assert.equal(
		Object.hasOwn(claiming.options[0], "stdin"),
		false,
		"claim was handed a stdin key it never asked for",
	);

	const drafts = emptyDraftLog();
	drafts.disk.set(pathFor(), "three paragraphs");
	drafts.index = { [pathFor()]: entryFor() };
	const posting = recorder(ok({ outcome: "ok" }));
	await postCommentDraft(draftHost(drafts), posting.spawner, EXE, pathFor());
	assert.equal(posting.options.length, 1);
	assert.equal(Object.hasOwn(posting.options[0], "stdin"), true);
});

// ---------------------------------------------------------------------------
// The activation sweep, which drops entries and never files
// ---------------------------------------------------------------------------

test("the activation sweep drops an entry whose file is gone and keeps one whose file is there", async () => {
	const drafts = emptyDraftLog();
	const present = pathFor("dinah-506/questions/1");
	const absent = pathFor("dinah-506/questions/2");
	drafts.disk.set(present, "three paragraphs");
	drafts.index = {
		[present]: entryFor("dinah-506/questions/1"),
		[absent]: entryFor("dinah-506/questions/2"),
	};
	const dropped = await sweepDraftIndex(draftHost(drafts));
	assert.equal(dropped, 1);
	assert.deepEqual(Object.keys(drafts.index), [present]);
	assert.deepEqual(drafts.deleted, [], "the sweep deleted a file");
});
