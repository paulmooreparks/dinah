// What each contributed command does with a selection of several rows.
//
// dinah-490 AC-6 through AC-14, AC-18, AC-19, AC-22, AC-30, AC-33 and AC-35.
// Every run below goes through the entry the editor itself registers from,
// looked up by command id in ROW_COMMAND_TABLE, so the subject of each
// assertion is the value activate() iterates rather than a function a table
// here claims corresponds to it.
//
// What none of this proves is the last link: that VS Code invokes a registered
// handler with the element and the selection TreeViewOptions.canSelectMany
// declares. Nothing in the unit layer can, because proving it means running an
// extension host. test/unit/wiring.test.ts narrows the unproven part to the
// editor's own behaviour by reading what extension.ts binds.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

import type { BulkReport } from "../../src/bulk";
import { summaryFor } from "../../src/bulk";
import { refusalMessage } from "../../src/cardCommands";
import type { CliOutcome, SpawnOutcome, Spawner } from "../../src/cli";
import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import {
	COMMAND_ARCHIVE_CARD,
	COMMAND_BLOCK,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_CLAIM,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_DELETE_ATTACHMENT,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_MOVE,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_COMMENT,
	COMMAND_OPEN_ITEM,
	COMMAND_PULL,
} from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import { REPORT_CHANNELS } from "../../src/reporter";
import { SELECTION_POLICIES } from "../../src/selection";
import type { TreeElement } from "../../src/tree";
import type { CheckResults, HostLog } from "../support/rows";
import {
	FOLDER,
	OTHER_ROOT,
	columnView,
	ROOT,
	attachmentRowFor,
	cardRow,
	columnRowFor,
	emptyLog,
	commentRow,
	itemRow,
	noteRow,
	ok,
	refused,
	wiringFor,
	workbenchRow,
} from "../support/rows";

// This file is compiled to out/test/unit/, so the extension root is three up.
const srcDir = join(__dirname, "..", "..", "..", "src");

/** A second workspace folder, so a run can span two of them. */
const OTHER_FOLDER = "C:/elsewhere";

/** What one driven run recorded. */
interface Run {
	readonly report: BulkReport;
	readonly log: HostLog;
	readonly calls: string[][];
	readonly results: CheckResults;
}

/** Drives the entry the editor registers for this command id. */
async function invoke(
	id: string,
	elements: readonly TreeElement[],
	options: {
		readonly log?: HostLog;
		readonly answer?: (argv: readonly string[], index: number) => SpawnOutcome;
	} = {},
): Promise<Run> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === id);
	assert.notEqual(entry, undefined, `no table entry carries the id ${id}`);
	const log = options.log ?? emptyLog();
	const calls: string[][] = [];
	const answer = options.answer ?? ((): SpawnOutcome => ok());
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return answer(argv, calls.length - 1);
	};
	const results: CheckResults = { applied: [] };
	const report = await (entry as { invoke: (typeof ROW_COMMAND_TABLE)[number]["invoke"] }).invoke(
		elements,
		wiringFor(log, spawner, results),
	);
	return { report, log, calls, results };
}

/** How many messages of any level reached the reader. */
function shown(log: HostLog): number {
	return log.errors.length + log.infos.length + log.warnings.length;
}

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(root: string, ...args: string[]): string[] {
	return ["--json", "--workbench", root, ...args];
}

// ---------------------------------------------------------------------------
// AC-6, AC-7: Archive asks once and archives each card
// ---------------------------------------------------------------------------

test("Archive over three cards asks once and spawns one archive per card, in order", async () => {
	const log = emptyLog();
	log.confirmed = true;
	const run = await invoke(
		COMMAND_ARCHIVE_CARD,
		[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
		{ log },
	);
	assert.equal(log.confirmations.length, 1);
	assert.deepEqual(run.calls, [
		pinned(ROOT, "archive", "wb-1"),
		pinned(ROOT, "archive", "wb-2"),
		pinned(ROOT, "archive", "wb-3"),
	]);
});

test("a declined archive confirmation spawns nothing and cancels the whole run", async () => {
	// The declining half is the accepting case's partner: an entry that never
	// spawned at all would satisfy this and fail the test above.
	const log = emptyLog();
	log.confirmed = false;
	const run = await invoke(
		COMMAND_ARCHIVE_CARD,
		[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
		{ log },
	);
	assert.deepEqual(run.calls, []);
	assert.equal(run.report.selected, 3);
	assert.equal(run.report.entries.length, 3);
	assert.deepEqual(
		run.report.entries.map((entry) => entry.outcome.kind),
		["skipped", "skipped", "skipped"],
	);
	assert.equal(run.report.cancelled, true);
	// The cancelled report reconciles like any other, so this cannot throw.
	assert.equal(summaryFor(run.report, ENGLISH).level, "none");
});

test("the archive confirmation names one card's reference and several cards' count", async () => {
	const one = emptyLog();
	one.confirmed = true;
	await invoke(COMMAND_ARCHIVE_CARD, [cardRow("wb-1")], { log: one });
	assert.equal(one.confirmations.length, 1);
	assert.equal(
		one.confirmations[0].message,
		ENGLISH("dialog.archive.confirm.one", { ref: "wb-1" }),
	);
	assert.equal(one.confirmations[0].label, ENGLISH("dialog.archive.action"));

	// Two card rows and a row that resolves to no card, so the count the copy
	// names is the resolved one rather than the targeted one.
	const many = emptyLog();
	many.confirmed = true;
	await invoke(
		COMMAND_ARCHIVE_CARD,
		[cardRow("wb-1"), cardRow("wb-2"), noteRow()],
		{ log: many },
	);
	assert.equal(many.confirmations.length, 1);
	assert.equal(
		many.confirmations[0].message,
		ENGLISH("dialog.archive.confirm.many", { count: "2" }),
	);
	assert.equal(many.confirmations[0].label, ENGLISH("dialog.archive.action"));
});

// ---------------------------------------------------------------------------
// AC-8: a refusal in the middle does not stop the run
// ---------------------------------------------------------------------------

test("a five-card run whose third card refuses still runs the fourth and fifth", async () => {
	const outcome: CliOutcome = {
		kind: "refused",
		refusal: "dinah.not-ready",
		detail: "wb-3",
	};
	const log = emptyLog();
	log.confirmed = true;
	const run = await invoke(
		COMMAND_CLAIM,
		["wb-1", "wb-2", "wb-3", "wb-4", "wb-5"].map((ref) => cardRow(ref)),
		{
			log,
			answer: (_argv, index) =>
				index === 2 ? refused("dinah.not-ready", "wb-3") : ok(),
		},
	);
	assert.equal(run.calls.length, 5);
	assert.deepEqual(run.calls[3], pinned(ROOT, "claim", "wb-4"));
	assert.deepEqual(run.calls[4], pinned(ROOT, "claim", "wb-5"));
	assert.equal(run.report.selected, 5);
	assert.equal(run.report.entries.length, 5);
	assert.deepEqual(
		run.report.entries.map((entry) => entry.outcome),
		[
			{ kind: "done" },
			{ kind: "done" },
			{ kind: "failed", failure: refusalMessage(outcome) },
			{ kind: "done" },
			{ kind: "done" },
		],
	);
});

// ---------------------------------------------------------------------------
// AC-9: one message per run, and the counts it names
// ---------------------------------------------------------------------------

test("a five-card run with two refusals shows one warning and writes two lines", async () => {
	const run = await invoke(
		COMMAND_CLAIM,
		["wb-1", "wb-2", "wb-3", "wb-4", "wb-5"].map((ref) => cardRow(ref)),
		{
			answer: (_argv, index) =>
				index === 1 || index === 3 ? refused("dinah.not-ready") : ok(),
		},
	);
	// The zero on showError is what proves the collecting host is in the path
	// rather than merely constructed.
	assert.deepEqual(run.log.errors, []);
	assert.equal(run.log.warnings.length, 1);
	assert.equal(
		run.log.warnings[0],
		ENGLISH("dialog.bulk.partial", {
			succeeded: "3",
			selected: "5",
			refused: "2",
			skipped: "0",
		}),
	);
	assert.equal(run.log.lines.length, 2);
	assert.ok(run.log.lines[0].startsWith("wb-2:"), run.log.lines[0]);
	assert.ok(run.log.lines[1].startsWith("wb-4:"), run.log.lines[1]);
});

test("a five-card run with no refusals shows one information toast and writes nothing", async () => {
	const run = await invoke(
		COMMAND_CLAIM,
		["wb-1", "wb-2", "wb-3", "wb-4", "wb-5"].map((ref) => cardRow(ref)),
	);
	assert.deepEqual(run.log.infos, [
		ENGLISH("dialog.bulk.allSucceeded", { count: "5" }),
	]);
	assert.deepEqual(run.log.warnings, []);
	assert.deepEqual(run.log.lines, []);
	// The count came from the run's own done count rather than from selected:
	// a report whose selected has been forced no longer adds up, and the
	// summary refuses it rather than naming 6.
	const forged: BulkReport = { ...run.report, selected: 6 };
	assert.throws(() => summaryFor(forged, ENGLISH));
});

// ---------------------------------------------------------------------------
// AC-10: the single-row invocation is unchanged
// ---------------------------------------------------------------------------

test("a single-row invocation shows the refusal itself and checkpoints once", async () => {
	const outcome: CliOutcome = {
		kind: "refused",
		refusal: "dinah.not-ready",
		detail: "wb-1",
	};
	const run = await invoke(COMMAND_CLAIM, [cardRow("wb-1")], {
		answer: () => refused("dinah.not-ready", "wb-1"),
	});
	assert.deepEqual(run.log.errors, [refusalMessage(outcome)]);
	assert.deepEqual(run.log.infos, []);
	assert.deepEqual(run.log.warnings, []);
	assert.deepEqual(run.log.checkpoints, [FOLDER]);
});

// ---------------------------------------------------------------------------
// AC-11: a mixed selection is counted by what the reader aimed at
// ---------------------------------------------------------------------------

test("two card rows and a column row claim twice and report three", async () => {
	const run = await invoke(COMMAND_CLAIM, [
		cardRow("wb-1"),
		cardRow("wb-2"),
		columnRowFor("doing"),
	]);
	assert.equal(run.calls.length, 2);
	assert.deepEqual(run.calls[0], pinned(ROOT, "claim", "wb-1"));
	assert.deepEqual(run.calls[1], pinned(ROOT, "claim", "wb-2"));
	assert.equal(run.report.selected, 3);
	assert.deepEqual(
		run.report.entries.map((entry) => entry.outcome.kind),
		["done", "done", "skipped"],
	);
	assert.deepEqual(run.log.warnings, [
		ENGLISH("dialog.bulk.partial", {
			succeeded: "2",
			selected: "3",
			refused: "0",
			skipped: "1",
		}),
	]);
});

test("three column rows handed to Claim attempt nothing and still answer", async () => {
	// A run that attempted nothing and reported nothing is the "reports
	// success because it never looked" failure, and this is the assertion that
	// makes it impossible.
	const run = await invoke(COMMAND_CLAIM, [
		columnRowFor("a"),
		columnRowFor("b"),
		columnRowFor("c"),
	]);
	assert.deepEqual(run.calls, []);
	assert.equal(run.report.selected, 3);
	assert.deepEqual(
		run.report.entries.map((entry) => entry.outcome.kind),
		["skipped", "skipped", "skipped"],
	);
	assert.deepEqual(run.log.warnings, [
		ENGLISH("dialog.bulk.partial", {
			succeeded: "0",
			selected: "3",
			refused: "0",
			skipped: "3",
		}),
	]);
});

test("one refusing card among two column rows produces exactly one message", async () => {
	// The host switch reads the targeted count, which is the same number the
	// summary's single-row case reads, so the per-row toast and the summary
	// cannot both speak.
	const run = await invoke(
		COMMAND_CLAIM,
		[cardRow("wb-1"), columnRowFor("a"), columnRowFor("b")],
		{ answer: () => refused("dinah.not-ready", "wb-1") },
	);
	assert.deepEqual(run.log.errors, []);
	assert.equal(run.log.warnings.length, 1);
	assert.equal(shown(run.log), 1);
});

// ---------------------------------------------------------------------------
// AC-12: one checkpoint per distinct folder
// ---------------------------------------------------------------------------

test("five cards across two folders checkpoint twice, even when every one refuses", async () => {
	// runVerb checkpoints on a refusal because a refusal often means the board
	// moved under the reader, and that reasoning has to survive the change.
	const run = await invoke(
		COMMAND_CLAIM,
		[
			cardRow("wb-1"),
			cardRow("wb-2"),
			cardRow("ot-1", OTHER_ROOT, columnView(), OTHER_FOLDER),
			cardRow("wb-3"),
			cardRow("ot-2", OTHER_ROOT, columnView(), OTHER_FOLDER),
		],
		{ answer: () => refused("dinah.not-ready") },
	);
	assert.equal(run.calls.length, 5);
	// One per distinct folder rather than one per row: a run of five cards in
	// one folder checkpoints once.
	assert.equal(run.log.checkpoints.length, 2);
	assert.deepEqual(
		[...new Set(run.log.checkpoints)].sort(),
		[FOLDER, OTHER_FOLDER].sort(),
	);
});

// ---------------------------------------------------------------------------
// AC-13: Move asks once, over the destinations every card shares
// ---------------------------------------------------------------------------

/** The legal moves a card answers, as `instructions` reports them. */
function moves(...columns: string[]): SpawnOutcome {
	return ok({
		legal_moves: columns.map((column) => ({
			column,
			ref: column,
			title: column,
			direction: "forward",
		})),
	});
}

test("Move over three cards sharing one column asks once and moves all three", async () => {
	const log = emptyLog();
	log.picked = { label: "review", value: "review" };
	const perCard = [moves("review", "done"), moves("review"), moves("review", "intake")];
	const run = await invoke(
		COMMAND_MOVE,
		[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
		{
			log,
			answer: (argv, index) => (argv.includes("instructions") ? perCard[index] : ok()),
		},
	);
	assert.equal(log.offered.length, 1);
	assert.equal(log.offered[0].value, "review");
	assert.deepEqual(log.placeholders, [
		ENGLISH("dialog.move.placeholder.many", { count: "3" }),
	]);
	const moveCalls = run.calls.filter((argv) => argv.includes("move"));
	assert.equal(moveCalls.length, 3);
	for (const argv of moveCalls) {
		assert.equal(argv[argv.length - 1], "review");
	}
});

test("Move over one card names that card in the placeholder, as it does today", async () => {
	const log = emptyLog();
	log.picked = { label: "review", value: "review" };
	await invoke(COMMAND_MOVE, [cardRow("wb-1")], {
		log,
		answer: (argv) => (argv.includes("instructions") ? moves("review") : ok()),
	});
	assert.deepEqual(log.placeholders, [
		ENGLISH("dialog.move.placeholder", { ref: "wb-1" }),
	]);
});

test("three cards sharing no destination are refused without a prompt", async () => {
	const log = emptyLog();
	const perCard = [moves("review"), moves("done"), moves("intake")];
	const run = await invoke(
		COMMAND_MOVE,
		[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
		{
			log,
			answer: (argv, index) => (argv.includes("instructions") ? perCard[index] : ok()),
		},
	);
	assert.deepEqual(log.offered, []);
	assert.deepEqual(log.errors, [
		ENGLISH("dialog.move.noSharedDestination", { count: "3" }),
	]);
	assert.deepEqual(
		run.calls.filter((argv) => argv.includes("move")),
		[],
	);
});

test("one card with no legal moves keeps today's own sentence", async () => {
	const log = emptyLog();
	await invoke(COMMAND_MOVE, [cardRow("wb-1")], {
		log,
		answer: (argv) => (argv.includes("instructions") ? moves() : ok()),
	});
	assert.deepEqual(log.errors, [
		ENGLISH("dialog.move.noLegalMoves", { ref: "wb-1" }),
	]);
});

test("the refusal counts the cards that resolved, not the rows that were selected", async () => {
	const log = emptyLog();
	const perCard = [moves("review"), moves("done")];
	const run = await invoke(
		COMMAND_MOVE,
		[cardRow("wb-1"), cardRow("wb-2"), columnRowFor("a"), columnRowFor("b")],
		{
			log,
			answer: (argv, index) => (argv.includes("instructions") ? perCard[index] : ok()),
		},
	);
	assert.deepEqual(log.errors, [
		ENGLISH("dialog.move.noSharedDestination", { count: "2" }),
	]);
	assert.equal(run.report.selected, 4);
});

test("a selection with no card at all answers exactly as Claim does over it", async () => {
	const log = emptyLog();
	const run = await invoke(
		COMMAND_MOVE,
		[columnRowFor("a"), columnRowFor("b"), columnRowFor("c")],
		{ log },
	);
	assert.deepEqual(run.calls, []);
	assert.deepEqual(log.offered, []);
	assert.deepEqual(log.errors, []);
	assert.equal(run.report.selected, 3);
	assert.deepEqual(
		run.report.entries.map((entry) => entry.outcome.kind),
		["skipped", "skipped", "skipped"],
	);
	const summary = summaryFor(run.report, ENGLISH);
	assert.equal(summary.level, "warning");
	assert.equal(
		summary.message,
		ENGLISH("dialog.bulk.partial", {
			succeeded: "0",
			selected: "3",
			refused: "0",
			skipped: "3",
		}),
	);
});

// ---------------------------------------------------------------------------
// AC-14: Block asks once, and the same reason reaches every card
// ---------------------------------------------------------------------------

test("Block over three cards asks once, about three cards, and sends one reason", async () => {
	const log = emptyLog();
	log.typed = "  waiting on the printer  ";
	const run = await invoke(
		COMMAND_BLOCK,
		[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
		{ log },
	);
	assert.deepEqual(log.prompts, [
		ENGLISH("dialog.block.reasonPrompt.many", { count: "3" }),
	]);
	assert.equal(run.calls.length, 3);
	for (const argv of run.calls) {
		assert.equal(argv[argv.length - 1], "waiting on the printer");
	}
});

test("Block over one card keeps today's own prompt", async () => {
	const log = emptyLog();
	log.typed = "because";
	await invoke(COMMAND_BLOCK, [cardRow("wb-1")], { log });
	assert.deepEqual(log.prompts, [ENGLISH("dialog.block.reasonPrompt")]);
});

test("a blank reason blocks nothing, whatever the row count", async () => {
	for (const typed of [undefined, "", "   "]) {
		const log = emptyLog();
		log.typed = typed;
		const run = await invoke(
			COMMAND_BLOCK,
			[cardRow("wb-1"), cardRow("wb-2"), cardRow("wb-3")],
			{ log },
		);
		assert.deepEqual(run.calls, []);
	}
});

// ---------------------------------------------------------------------------
// AC-18: New Card refuses a selection rather than narrowing it
// ---------------------------------------------------------------------------

test("New Card over two columns refuses, opens no prompt and files nothing", async () => {
	const log = emptyLog();
	const run = await invoke(COMMAND_NEW_CARD, [columnRowFor("a"), columnRowFor("b")], {
		log,
	});
	assert.deepEqual(run.calls, []);
	assert.deepEqual(log.prompts, []);
	assert.deepEqual(log.errors, [ENGLISH("dialog.bulk.oneRowOnly")]);
	assert.equal(log.errors.length, 1);
});

test("New Card over one column behaves as it does today", async () => {
	// The accepting partner: an implementation refusing everything would
	// satisfy the clause above on its own.
	const log = emptyLog();
	log.typed = "Fix the thing";
	const run = await invoke(COMMAND_NEW_CARD, [columnRowFor("a")], { log });
	assert.equal(log.prompts.length, 1);
	assert.equal(run.calls.length, 1);
	assert.deepEqual(run.calls[0], pinned(ROOT, "add", "Fix the thing", "--column", "a"));
});

// ---------------------------------------------------------------------------
// AC-30: Delete Attachment counts what resolved
// ---------------------------------------------------------------------------

test("Delete Attachment over two attachments and a column row asks about two", async () => {
	const log = emptyLog();
	log.confirmed = true;
	const run = await invoke(
		COMMAND_DELETE_ATTACHMENT,
		[attachmentRowFor("a1"), attachmentRowFor("a2"), columnRowFor("a")],
		{ log },
	);
	assert.equal(log.confirmations.length, 1);
	assert.equal(
		log.confirmations[0].message,
		ENGLISH("dialog.attachment.delete.confirm.many", { count: "2" }),
	);
	assert.equal(
		log.confirmations[0].label,
		ENGLISH("dialog.attachment.delete.action"),
	);
	assert.equal(run.calls.length, 2);
});

test("Delete Attachment over one attachment keeps the sentence that ships today", async () => {
	const log = emptyLog();
	log.confirmed = true;
	const run = await invoke(
		COMMAND_DELETE_ATTACHMENT,
		[attachmentRowFor("a1"), columnRowFor("a"), columnRowFor("b")],
		{ log },
	);
	assert.equal(log.confirmations.length, 1);
	assert.equal(
		log.confirmations[0].message,
		ENGLISH("dialog.attachment.delete.confirm", {
			filename: "spec.pdf",
			ref: "tr-1/attachments/1",
		}),
	);
	assert.equal(run.calls.length, 1);
});

test("a declined attachment confirmation deletes nothing and cancels the run", async () => {
	for (const rows of [
		[attachmentRowFor("a1"), attachmentRowFor("a2"), columnRowFor("a")],
		[attachmentRowFor("a1"), columnRowFor("a"), columnRowFor("b")],
	]) {
		const log = emptyLog();
		log.confirmed = false;
		const run = await invoke(COMMAND_DELETE_ATTACHMENT, rows, { log });
		assert.deepEqual(run.calls, []);
		assert.equal(run.report.cancelled, true);
		assert.equal(run.report.selected, 3);
	}
});

// ---------------------------------------------------------------------------
// AC-33: an informational per-row path still produces one message
// ---------------------------------------------------------------------------

test("five queue columns of which four are empty produce one message and four notes", async () => {
	const empty = ok({
		outcome: "ok",
		message_values: { upstream: "Queue", destination: "Doing" },
	});
	const pulled = ok({ outcome: "ok", card: { id: "abc" } });
	const run = await invoke(
		COMMAND_PULL,
		["a", "b", "c", "d", "e"].map((id) => columnRowFor(id)),
		{ answer: (_argv, index) => (index === 0 ? pulled : empty) },
	);
	assert.equal(shown(run.log), 1);
	assert.equal(run.log.infos.length, 1);
	assert.equal(
		run.log.infos[0],
		ENGLISH("dialog.bulk.allSucceededNotes", { count: "5", notes: "4" }),
	);
	assert.equal(run.report.notes.length, 4);
	assert.ok(run.report.notes.every((note) => note.level === "info"));
	assert.equal(run.log.lines.length, 4);
	for (const line of run.log.lines) {
		assert.equal(line, ENGLISH("dialog.pull.empty", { from: "Queue", into: "Doing" }));
	}
});

test("three workbench checks that found defects produce one warning and three notes", async () => {
	const findings = {
		code: 5,
		stdout: JSON.stringify({
			outcome: "findings",
			findings: [{ Path: "cards/wb-1", Key: "dinah.orphan", Detail: "" }],
		}),
		stderr: "",
	};
	const run = await invoke(
		COMMAND_CHECK_WORKBENCH,
		[workbenchRow(ROOT), workbenchRow(OTHER_ROOT), workbenchRow("C:/work/third")],
		{ answer: () => findings },
	);
	assert.equal(shown(run.log), 1);
	assert.equal(run.log.warnings.length, 1);
	assert.ok(
		run.log.warnings[0].startsWith(
			ENGLISH("dialog.bulk.allSucceededNotes", { count: "3", notes: "3" }),
		),
		run.log.warnings[0],
	);
	// The channel stays reachable through the summary's own action, so
	// revealing it three times during the loop is what does not happen.
	assert.equal(run.log.revealed, 0);
	assert.equal(run.report.notes.length, 3);
	assert.ok(run.report.notes.every((note) => note.level === "warning"));
	// The Problems panel is updated from the answer each check already
	// fetched, once per row.
	assert.equal(run.results.applied.length, 3);
});

// ---------------------------------------------------------------------------
// AC-35: the copy family writes once and speaks once
// ---------------------------------------------------------------------------

test("Copy Reference over two cards makes one clipboard write and one message", async () => {
	const run = await invoke(COMMAND_COPY_CARD_REF, [cardRow("wb-1"), cardRow("wb-2")]);
	assert.deepEqual(run.log.copied, ["wb-1\nwb-2"]);
	assert.equal(shown(run.log), 1);
	assert.deepEqual(run.log.infos, [
		ENGLISH("dialog.card.copiedRef.many", { count: "2" }),
	]);
});

test("Copy Reference over one card keeps the sentence that ships today", async () => {
	const run = await invoke(COMMAND_COPY_CARD_REF, [cardRow("wb-1")]);
	assert.deepEqual(run.log.copied, ["wb-1"]);
	assert.deepEqual(run.log.infos, [
		ENGLISH("dialog.card.copiedRef", { ref: "wb-1" }),
	]);
});

test("Copy Path behaves the same way over workbench rows", async () => {
	const many = await invoke(COMMAND_COPY_WORKBENCH_PATH, [
		workbenchRow(ROOT),
		workbenchRow(OTHER_ROOT),
	]);
	assert.deepEqual(many.log.copied, [`${ROOT}\n${OTHER_ROOT}`]);
	assert.equal(shown(many.log), 1);
	assert.deepEqual(many.log.infos, [
		ENGLISH("dialog.workbench.copiedPath.many", { count: "2" }),
	]);

	const one = await invoke(COMMAND_COPY_WORKBENCH_PATH, [workbenchRow(ROOT)]);
	assert.deepEqual(one.log.copied, [ROOT]);
	assert.deepEqual(one.log.infos, [
		ENGLISH("dialog.workbench.copiedPath", { path: ROOT }),
	]);
});

// ---------------------------------------------------------------------------
// AC-22: a row is failed when its outcome is anything other than ok
// ---------------------------------------------------------------------------

/** Every `kind` the CliOutcome union declares, read off cli.ts at test time. */
function outcomeKinds(): string[] {
	const file = join(srcDir, "cli.ts");
	const source = ts.createSourceFile(
		file,
		readFileSync(file, "utf8"),
		ts.ScriptTarget.ES2022,
		true,
		ts.ScriptKind.TS,
	);
	const kinds: string[] = [];
	const visit = (node: ts.Node): void => {
		if (
			ts.isTypeAliasDeclaration(node) &&
			node.name.text === "CliOutcome" &&
			ts.isUnionTypeNode(node.type)
		) {
			for (const member of node.type.types) {
				if (!ts.isTypeLiteralNode(member)) {
					continue;
				}
				for (const property of member.members) {
					if (
						ts.isPropertySignature(property) &&
						ts.isIdentifier(property.name) &&
						property.name.text === "kind" &&
						property.type !== undefined &&
						ts.isLiteralTypeNode(property.type) &&
						ts.isStringLiteral(property.type.literal)
					) {
						kinds.push(property.type.literal.text);
					}
				}
			}
		}
		ts.forEachChild(node, visit);
	};
	visit(source);
	return kinds;
}

/** The spawn answer that produces one non-ok outcome of the kind given. */
function answerOfKind(kind: string): SpawnOutcome {
	if (kind === "refused") {
		return refused("dinah.not-ready", "wb-3");
	}
	if (kind === "stale") {
		return { code: 3, stdout: JSON.stringify({ detail: "cursor is behind" }), stderr: "" };
	}
	if (kind === "unreachable") {
		return { code: 4, stdout: JSON.stringify({ detail: "no workbench" }), stderr: "" };
	}
	if (kind === "spawn-failed") {
		return {
			code: null,
			stdout: "",
			stderr: "",
			spawnError: { code: "ENOENT", message: "dinah is not on the path" },
		};
	}
	// not-json, and any arm a later card adds whose answer is not readable.
	return { code: 0, stdout: "this is not json", stderr: "" };
}

test("every non-ok outcome the union declares is one failed row and one warning", async () => {
	const collected = outcomeKinds();
	// The floor is what stops a walk that matched nothing reporting a clean
	// sweep, and it is a floor rather than an equality so that a later card
	// adding an arm does not redden a card with nothing to do with it.
	assert.ok(
		collected.length >= 6,
		`the CliOutcome walk collected ${String(collected.length)} kinds`,
	);
	assert.ok(collected.includes("ok"), collected.join(", "));
	const driven: string[] = [];
	for (const kind of collected) {
		if (kind === "ok") {
			continue;
		}
		const run = await invoke(
			COMMAND_CLAIM,
			["wb-1", "wb-2", "wb-3", "wb-4", "wb-5"].map((ref) => cardRow(ref)),
			{ answer: (_argv, index) => (index === 2 ? answerOfKind(kind) : ok()) },
		);
		const failures = run.report.entries.filter(
			(entry) => entry.outcome.kind === "failed",
		);
		assert.equal(failures.length, 1, kind);
		const summary = summaryFor(run.report, ENGLISH);
		assert.equal(summary.level, "warning", kind);
		assert.equal(
			summary.message,
			ENGLISH("dialog.bulk.partial", {
				succeeded: "4",
				selected: "5",
				refused: "1",
				skipped: "0",
			}),
			kind,
		);
		driven.push(kind);
	}
	// The set driven is compared against the set collected rather than against
	// a table written here, so a new arm goes red until somebody drives it.
	assert.deepEqual(
		[...driven].sort(),
		collected.filter((kind) => kind !== "ok").sort(),
	);
});

test("a five-row run whose every spawn answers ok reports every row finished", async () => {
	// The accepting partner: an implementation calling every row failed would
	// satisfy the refusing halves above on its own.
	const run = await invoke(
		COMMAND_CLAIM,
		["wb-1", "wb-2", "wb-3", "wb-4", "wb-5"].map((ref) => cardRow(ref)),
	);
	const summary = summaryFor(run.report, ENGLISH);
	assert.equal(summary.level, "info");
	assert.equal(summary.message, ENGLISH("dialog.bulk.allSucceeded", { count: "5" }));
});

// ---------------------------------------------------------------------------
// AC-19: every fanOut entry is driven, and produces the effect it declares
// ---------------------------------------------------------------------------

/** Two actionable rows of the kind the command handles. */
function rowsFor(id: string): readonly TreeElement[] {
	if (
		id === COMMAND_CHECK_WORKBENCH ||
		id === COMMAND_COPY_WORKBENCH_PATH ||
		id === COMMAND_EDIT_WORKBENCH_DEFINITION
	) {
		return [workbenchRow(ROOT), workbenchRow(OTHER_ROOT)];
	}
	if (id === COMMAND_OPEN_ATTACHMENT) {
		return [attachmentRowFor("a1", "one.pdf"), attachmentRowFor("a2", "two.pdf")];
	}
	if (id === COMMAND_EDIT_COLUMN_INSTRUCTIONS || id === COMMAND_PULL) {
		return [columnRowFor("a"), columnRowFor("b")];
	}
	if (id === COMMAND_OPEN_ITEM) {
		return [
			itemRow({ ref: "tr-1/questions/1" }),
			itemRow({ ref: "tr-1/questions/2", id: "b00000000002", ordinal: 2 }),
		];
	}
	if (id === COMMAND_OPEN_COMMENT) {
		return [
			commentRow({ ref: "tr-1/comments/1" }),
			commentRow({ ref: "tr-1/comments/2", id: "c00000000002" }),
		];
	}
	return [cardRow("wb-1"), cardRow("wb-2")];
}

/**
 * The per-row effects one run produced, counted per kind of effect.
 *
 * A spawning command's effects are its spawns; a non-spawning one's are the
 * tabs, files and clipboard writes its act performed. They are counted apart
 * rather than summed, because Open Card does both, so a sum would read two
 * rows as four effects and a perRow expectation of two would be unreachable
 * for it.
 */
function perRowEffects(run: Run): readonly number[] {
	return [
		run.calls.length,
		run.log.opened.length,
		run.log.files.length,
		run.log.served.length,
		run.log.copied.length,
	].filter((count) => count > 0);
}

test("every fanOut command is driven over two rows and produces the effect it declares", async () => {
	const subjects = Object.entries(SELECTION_POLICIES).filter(
		([, entry]) => entry.policy === "fanOut",
	);
	// Asserted against a literal so that re-declaring a command out of the
	// family reddens this run rather than shrinking the sweep in silence.
	// Fourteen since dinah-597, which moved Unblock to oneInput so it can ask
	// why the block is lifted.
	assert.equal(subjects.length, 14);
	let exercised = 0;
	for (const [id, entry] of subjects) {
		const table = ROW_COMMAND_TABLE.find((row) => row.id === id);
		assert.notEqual(table, undefined, `no table entry carries the id ${id}`);
		const run = await invoke(id, rowsFor(id), {
			answer: (argv) =>
				argv.includes("show")
					? ok({ path: "C:/work/bench/cards/wb-1/card.md" })
					: argv.includes("path")
						? ok({ path: "C:/work/bench/columns/a/column.md" })
						: argv.includes("check")
							? ok({ outcome: "ok" })
							: ok({ outcome: "ok", card: { id: "abc" } }),
		});
		// The one message the card promises, stated over what the reader was
		// shown rather than over how many times the host was called.
		assert.equal(
			shown(run.log),
			1,
			`${id} showed ${String(shown(run.log))} messages over two rows`,
		);
		assert.deepEqual([...REPORT_CHANNELS], ["showError", "showInfo", "showWarning"]);
		if (entry.effect === "perRow") {
			const effects = perRowEffects(run);
			assert.ok(effects.length > 0, `${id} produced no per-row effect at all`);
			for (const count of effects) {
				assert.equal(
					count,
					2,
					`${id} produced ${String(count)} of one kind of effect over two rows`,
				);
			}
		} else {
			// A oneCall entry spawns nothing per row, performs one effect in
			// finish, and carries both rows' values newline-joined.
			assert.equal(run.calls.length, 0, id);
			assert.equal(run.log.copied.length, 1, id);
			assert.ok(run.log.copied[0].includes("\n"), run.log.copied[0]);
		}
		exercised += 1;
	}
	// No exemption list and no skip mechanism: every fanOut entry is driven or
	// this run is red.
	assert.equal(exercised, subjects.length);
});
