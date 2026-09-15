// Dragging a card onto a column: what a drag carries, what a drop resolves to,
// and what each verdict does.
//
// The negative cases are the point of this file. A guard that ignores its
// argument passes every positive assertion here, so each resolver is driven
// with the rows it must decline as well as the ones it must accept, and the
// two verdicts that act on nothing are asserted by counting calls that never
// happened rather than by watching the tree.
//
// That last shape is what the sidebar's own contract forces. The tree renders
// the effect of a drop rather than the drop itself, so a refused move and a
// drag that missed its target both leave the card where it stood. The message
// is the only difference between them, and a test asserting that the card did
// not move would pass on a drag that never started.

import assert from "node:assert/strict";
import { test } from "node:test";

import { ENGLISH } from "../../src/l10n";

import type { CommandHost, PickItem } from "../../src/cardCommands";
import { askMoveDestination, moveCardTo, refusalMessage } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import type { DragPayload, DragRow, DropTarget } from "../../src/dragAndDrop";
import {
	applyDropVerdicts,
	classifyDrop,
	dragRowsFor,
	dragRowsFrom,
	dropColumnFor,
	offerDrag,
} from "../../src/dragAndDrop";
import { DRAG_MIME_TYPE, VIEW_ID } from "../../src/identity";
import type { RootRow, TreeElement } from "../../src/tree";
import type { AttachmentView, ColumnView, TreeNode } from "../../src/wire";

const FOLDER = "C:\\work\\bench";
const ROOT = "C:\\work\\bench";
const OTHER_ROOT = "C:\\work\\other";

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

function refused(refusal: string, detail?: string): SpawnOutcome {
	return { code: 2, stdout: JSON.stringify({ refusal, detail }), stderr: "" };
}

function column(id: string, slug?: string, title = "Doing"): ColumnView {
	return {
		id,
		slug,
		title,
		kind: "work",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: true,
		count: 0,
	};
}

function row(path = ROOT): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: FOLDER,
		folderName: "bench",
		description: "",
		sole: true,
		data: {
			path,
			title: "Bench",
			columns: new Map(),
			cards: new Map(),
			holding: [],
		},
	};
}

/** A row whose workbench did not resolve, so it carries no data at all. */
function unresolvedRow(): RootRow {
	return {
		rowKind: "deadEnd",
		folder: FOLDER,
		folderName: "bench",
		description: "",
		sole: true,
	};
}

function attachment(): AttachmentView {
	return {
		id: "at-1",
		ordinal: 1,
		ref: "tr-4",
		filename: "a.md",
		provenance: "attached",
		path: "C:\\work\\bench\\a.md",
	};
}

function node(ref: string | undefined): TreeNode {
	return { kind: "card", ref, count: 0 };
}

/**
 * A card row, complete unless a field is named as missing.
 *
 * The missing fields are named rather than passed as undefined overrides,
 * because an override of undefined is indistinguishable from an absent one and
 * every negative case here would quietly build the complete row instead.
 */
function cardRow(missing: "ref" | "emptyRef" | "root" | "column" | "" = ""): TreeElement {
	return {
		kind: "card",
		row: missing === "root" ? unresolvedRow() : row(),
		node: node(
			missing === "ref" ? undefined : missing === "emptyRef" ? "" : "tr-4",
		),
		column: missing === "column" ? undefined : column("c-doing", "doing"),
	};
}

/** The four fields a drag carries for the card fixture above. */
function payload(overrides: Partial<DragPayload> = {}): DragPayload {
	return {
		ref: "tr-4",
		root: ROOT,
		folder: FOLDER,
		columnId: "c-doing",
		...overrides,
	};
}

interface Watcher {
	readonly host: CommandHost;
	readonly spawner: Spawner;
	readonly calls: string[][];
	readonly errors: string[];
	readonly infos: string[];
	readonly checkpoints: string[];
	answer: SpawnOutcome;
	picked?: PickItem;
}

function watcher(answer: SpawnOutcome = ok({})): Watcher {
	const calls: string[][] = [];
	const errors: string[] = [];
	const infos: string[] = [];
	const checkpoints: string[] = [];
	const state = { calls, errors, checkpoints, infos, answer } as Watcher;
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		if (argv.includes("instructions")) {
			return ok({
				legal_moves: [
					{ column: "review", ref: "review", title: "Review", direction: "forward" },
				],
			});
		}
		return state.answer;
	};
	const host: CommandHost = {
		t: ENGLISH,
		showError: (message) => errors.push(message),
		showInfo: (message) => infos.push(message),
		// The three CommandHost gained with ReporterHost (dinah-490 D-25).
		// A single-row drop reaches none of them: the summary stays silent at
		// one row, and the channel line is the run's own.
		showWarning: async () => undefined,
		appendLines: () => {},
		revealOutput: () => {},
		copyToClipboard: async () => {},
		pick: async () => state.picked,
		input: async () => undefined,
		openDocument: async () => {},
		openFile: async () => {},
		pickFile: async () => undefined,
		openServedText: async () => undefined,
		confirmDestructive: async () => false,
		checkpoint: async (folder) => {
			checkpoints.push(folder);
		},
		log: () => {},
	};
	(state as { host: CommandHost }).host = host;
	(state as { spawner: Spawner }).spawner = spawner;
	return state;
}

// ---------------------------------------------------------------------------
// AC-1: the mime type is composed from the view's own id
// ---------------------------------------------------------------------------

test("the drag mime type is composed from VIEW_ID rather than typed twice", () => {
	// Recomputed from the live export rather than compared against a literal,
	// so renaming the view moves both halves together. The literal is asserted
	// too, because a recomputation alone would pass if VIEW_ID itself went
	// wrong, and VS Code recommends this exact spelling.
	assert.equal(DRAG_MIME_TYPE, `application/vnd.code.tree.${VIEW_ID.toLowerCase()}`);
	assert.equal(DRAG_MIME_TYPE, "application/vnd.code.tree.dinah.workbenchview");
});

// ---------------------------------------------------------------------------
// AC-3: only a card is dragged, and only a complete one
// ---------------------------------------------------------------------------

test("a card row carries its reference, its workbench, its folder and its column", () => {
	assert.deepEqual(dragRowsFor([cardRow()]), [
		{
			kind: "card",
			ref: "tr-4",
			payload: {
				ref: "tr-4",
				root: ROOT,
				folder: FOLDER,
				columnId: "c-doing",
			},
		},
	]);
});

test("no other row kind is draggable", () => {
	const owner = row();
	const kinds: TreeElement[] = [
		{ kind: "root", row: owner },
		{ kind: "note", owner, text: "", tooltip: "" },
		{ kind: "column", row: owner, node: { kind: "column", count: 0 }, view: column("c-doing") },
		{ kind: "group", row: owner, node: { kind: "group", count: 0 }, column: column("c-doing") },
		{
			kind: "collection",
			row: owner,
			root: ROOT,
			holder: "tr-4",
			holderKind: "card",
			memberKind: "attachment",
			members: [],
		},
		{ kind: "attachment", row: owner, root: ROOT, owner: "tr-4", view: attachment() },
	];
	for (const element of kinds) {
		// dinah-490 carries such a row rather than dropping it, so the drop can
		// name the rows it could not act on. What it must not do is compose a
		// payload, and the kind is what says so.
		const rows = dragRowsFor([element]);
		assert.equal(rows.length, 1);
		assert.equal(
			rows[0].kind,
			"other",
			`a ${element.kind} row offered a drag payload`,
		);
	}
	// The roster is asserted rather than trusted, so a TreeElement kind added
	// later is not silently left unexercised by a loop over five of six.
	assert.equal(kinds.length, 6);
});

test("a card missing anything the move needs carries nothing", () => {
	// Three fields, three cases. A card whose reference did not reach the row
	// names nothing to move, one whose workbench did not resolve names nowhere
	// to run, and one whose column the status join missed cannot say where it
	// started, so a drop back onto that column would read as a real move.
	for (const missing of ["ref", "emptyRef", "root", "column"] as const) {
		assert.equal(
			dragRowsFor([cardRow(missing)])[0].kind,
			"other",
			`a card missing its ${missing} still composed a payload`,
		);
	}
	// The same fixture with nothing named missing does carry a payload, so the
	// four assertions above cannot be passing because the fixture is broken.
	assert.equal(dragRowsFor([cardRow()])[0].kind, "card");
});

test("an empty drag carries nothing", () => {
	assert.deepEqual(dragRowsFor([]), []);
});

// ---------------------------------------------------------------------------
// AC-10: a drag that starts on any other row sets no mime entry at all
// ---------------------------------------------------------------------------

test("a drag starting on a row that is not a card sets no mime entry", () => {
	const owner = row();
	const kinds: TreeElement[] = [
		{ kind: "column", row: owner, node: { kind: "column", count: 0 }, view: column("c-doing") },
		{ kind: "group", row: owner, node: { kind: "group", count: 0 }, column: column("c-doing") },
		{ kind: "root", row: owner },
		{ kind: "attachment", row: owner, root: ROOT, owner: "tr-4", view: attachment() },
	];
	for (const element of kinds) {
		const sets: string[] = [];
		offerDrag([element], DRAG_MIME_TYPE, { set: (mime) => sets.push(mime) }, (rows) => rows);
		assert.deepEqual(sets, [], `a ${element.kind} row set a mime entry`);
	}
});

test("a drag starting on a card sets the one mime entry, carrying the payload", () => {
	// The sibling above asserts a call that never happens, which passes just as
	// well when nothing sets an entry at all, so this is what keeps it honest.
	const sets: [string, readonly DragRow[]][] = [];
	offerDrag(
		[cardRow()],
		DRAG_MIME_TYPE,
		{ set: (mime, item: readonly DragRow[]) => sets.push([mime, item]) },
		(rows) => rows,
	);
	assert.equal(sets.length, 1);
	assert.equal(sets[0][0], DRAG_MIME_TYPE);
	assert.equal(sets[0][1].length, 1);
	assert.equal(sets[0][1][0].ref, "tr-4");
});

// ---------------------------------------------------------------------------
// AC-4: a drop resolves to the column it landed under
// ---------------------------------------------------------------------------

test("a column, a state group and a card under one column all resolve to it", () => {
	const view = column("c-review", "review", "Review");
	const owner = row();
	const targets: TreeElement[] = [
		{ kind: "column", row: owner, node: { kind: "column", count: 0 }, view },
		{ kind: "group", row: owner, node: { kind: "group", count: 0 }, column: view },
		{ kind: "card", row: owner, node: node("tr-9"), column: view },
	];
	for (const target of targets) {
		assert.deepEqual(
			dropColumnFor(target),
			{ view, root: ROOT },
			`a ${target.kind} row did not resolve to the column that owns it`,
		);
	}
});

test("a row naming no column resolves to nothing", () => {
	const owner = row();
	const targets: (TreeElement | undefined)[] = [
		undefined,
		{ kind: "root", row: owner },
		{ kind: "note", owner, text: "", tooltip: "" },
		{
			kind: "collection",
			row: owner,
			root: ROOT,
			holder: "tr-4",
			holderKind: "card",
			memberKind: "attachment",
			members: [],
		},
		{ kind: "attachment", row: owner, root: ROOT, owner: "tr-4", view: attachment() },
		// The status/tree join missed this column, so the row carries no view.
		{ kind: "column", row: owner, node: { kind: "column", count: 0 } },
		// The join found the column, but the workbench itself did not resolve.
		{
			kind: "column",
			row: unresolvedRow(),
			node: { kind: "column", count: 0 },
			view: column("c-review"),
		},
	];
	for (const target of targets) {
		assert.equal(dropColumnFor(target), undefined);
	}
});

// ---------------------------------------------------------------------------
// AC-5: what a resolved drop means
// ---------------------------------------------------------------------------

test("a drop resolving to no column does nothing", () => {
	assert.deepEqual(classifyDrop(payload(), undefined), { kind: "ignore" });
});

test("a drop back onto the card's own column does nothing", () => {
	const drop: DropTarget = { view: column("c-doing", "doing"), root: ROOT };
	assert.deepEqual(classifyDrop(payload(), drop), { kind: "ignore" });
});

test("a drop onto another column is a move, named by the column's own ref", () => {
	const drop: DropTarget = { view: column("c-review", "review", "Review"), root: ROOT };
	assert.deepEqual(classifyDrop(payload(), drop), {
		kind: "act",
		destinationRef: "review",
	});
});

test("a column with no slug is named by its identifier", () => {
	const drop: DropTarget = { view: column("c-review"), root: ROOT };
	assert.deepEqual(classifyDrop(payload(), drop), {
		kind: "act",
		destinationRef: "c-review",
	});
});

test("a drop into another workbench is reported, even at the same column id", () => {
	// The workbench check runs before the same-column check, and this is what
	// pins the order: two workbenches can name a column the same thing, and
	// running the same-column check first would answer ignore and say nothing.
	const drop: DropTarget = { view: column("c-doing", "doing"), root: OTHER_ROOT };
	assert.deepEqual(classifyDrop(payload(), drop), { kind: "crossWorkbench" });
});

// ---------------------------------------------------------------------------
// AC-8: a verdict of ignore touches nothing and says nothing
// ---------------------------------------------------------------------------

test("an ignored drop runs no dinah, checkpoints nothing and shows nothing", async () => {
	const w = watcher();
	await applyDropVerdicts(dragRowsFor([cardRow()]), undefined, "dinah", w.host, w.spawner);
	assert.deepEqual(w.calls, []);
	assert.deepEqual(w.checkpoints, []);
	assert.deepEqual(w.errors, []);
	assert.deepEqual(w.infos, []);
});

// ---------------------------------------------------------------------------
// AC-7: a cross-workbench drop is told, and nothing is run
// ---------------------------------------------------------------------------

test("a cross-workbench drop names the card and the destination, and runs nothing", async () => {
	const w = watcher();
	const drop: DropTarget = { view: column("c-doing", "doing", "Doing"), root: OTHER_ROOT };
	await applyDropVerdicts(dragRowsFor([cardRow()]), drop, "dinah", w.host, w.spawner);
	assert.equal(w.errors.length, 1);
	assert.ok(w.errors[0].includes("tr-4"), w.errors[0]);
	assert.ok(w.errors[0].includes("Doing"), w.errors[0]);
	assert.ok(w.errors[0].includes("different workbench"), w.errors[0]);
	assert.deepEqual(w.calls, []);
	assert.deepEqual(w.checkpoints, []);
});

// ---------------------------------------------------------------------------
// AC-6 and AC-9: an act runs the move verb, and a refusal reads as Move's does
// ---------------------------------------------------------------------------

test("an act runs move, pinned to the workbench, and checkpoints the folder", async () => {
	const w = watcher(ok({}));
	await applyDropVerdicts(
		dragRowsFor([cardRow()]),
		{ view: column("c-review", "review"), root: ROOT },
		"dinah",
		w.host,
		w.spawner,
	);
	assert.deepEqual(w.calls, [
		["--json", "--workbench", ROOT, "move", "tr-4", "review"],
	]);
	assert.deepEqual(w.checkpoints, [FOLDER]);
	assert.deepEqual(w.errors, []);
	// A drag is never a pull. `dinah pull` takes a destination and picks its
	// own card, so it could move a card other than the one that was dragged.
	assert.ok(!w.calls[0].includes("pull"));
});

test("a refused drop shows the sentence the Move command shows for that refusal", async () => {
	// The refusal is the only thing separating a workbench that said no from a
	// drag that missed, so it is checked against the sentence a reader already
	// knows rather than against a shape composed for this path.
	for (const [refusal, detail] of [
		["not-operator", "only the operator moves a card out of this column"],
		["at-capacity", "doing"],
	] as const) {
		const outcome = refused(refusal, detail);

		const dragged = watcher(outcome);
		await applyDropVerdicts(
			dragRowsFor([cardRow()]),
			{ view: column("c-review", "review"), root: ROOT },
			"dinah",
			dragged.host,
			dragged.spawner,
		);

		const picked = watcher(outcome);
		picked.picked = { label: "Review", value: "review" };
		const context = {
			spawner: picked.spawner,
			exe: "dinah",
			host: picked.host,
			folder: FOLDER,
			root: ROOT,
			ref: "tr-4",
		};
		const destination = await askMoveDestination([context], picked.host);
		assert.equal(typeof destination, "string");
		await moveCardTo(context, destination ?? "");

		assert.equal(dragged.errors.length, 1);
		assert.deepEqual(dragged.errors, picked.errors);
		assert.equal(
			dragged.errors[0],
			refusalMessage({ kind: "refused", refusal, detail }),
		);
		// The board moved under the reader, so the read that follows is what
		// shows them why. It runs on a refusal exactly as it runs on success.
		assert.deepEqual(dragged.checkpoints, [FOLDER]);
	}
});

// ---------------------------------------------------------------------------
// dinah-490 AC-15: the drag round trip, driven end to end
// ---------------------------------------------------------------------------
//
// Nothing in the chain below is typed by the test. offerDrag sets an entry
// into a recording sink, dragRowsFrom reads back the value that sink was
// handed, and applyDropVerdicts acts on THAT answer. Round 4 of this card
// shipped a criterion that drove applyDropVerdicts with a hand-built list,
// which went green over a list the shipping wiring never builds.
//
// What this does not prove is that extension.ts wires the two halves this way.
// test/unit/wiring.test.ts asserts that separately over the AST.

/** A card row carrying a reference of its own, for a multi-row drag. */
function namedCardRow(ref: string): TreeElement {
	return {
		kind: "card",
		row: row(),
		node: node(ref),
		column: column("c-doing", "doing"),
	};
}

/** A column row, which is a dragged row composing no payload. */
function draggedColumnRow(): TreeElement {
	return {
		kind: "column",
		row: row(),
		node: { kind: "column", value: "doing", count: 0 },
		view: column("c-doing", "doing", "Doing"),
	};
}

/** The four rows every clause below drags: three cards and a column header. */
function mixedDrag(): TreeElement[] {
	return [
		namedCardRow("tr-1"),
		namedCardRow("tr-2"),
		namedCardRow("tr-3"),
		draggedColumnRow(),
	];
}

/** A sink that records what it was set, with the identity as its wrapper. */
function recordingSink(): {
	readonly sink: { set: (mime: string, item: readonly DragRow[]) => void };
	readonly entries: [string, readonly DragRow[]][];
} {
	const entries: [string, readonly DragRow[]][] = [];
	return {
		sink: { set: (mime, item) => entries.push([mime, item]) },
		entries,
	};
}

test("a mixed drag answers one row per dragged element, cards and header alike", () => {
	const rows = dragRowsFor(mixedDrag());
	assert.equal(rows.length, 4);
	assert.deepEqual(
		rows.map((dragged: DragRow) => dragged.kind),
		["card", "card", "card", "other"],
	);
	// The header's entry names the row rather than being a synthetic hole, so
	// the report can say what it could not act on.
	assert.equal(rows[3].ref, "Doing");
});

test("the whole gesture crosses the mime entry, and the drop acts on what came back", async () => {
	const recorder = recordingSink();
	offerDrag(mixedDrag(), DRAG_MIME_TYPE, recorder.sink, (rows) => rows);
	assert.equal(recorder.entries.length, 1);
	assert.equal(recorder.entries[0][0], DRAG_MIME_TYPE);

	const readBack = dragRowsFrom(recorder.entries[0][1]);
	assert.notEqual(readBack, undefined);
	assert.equal(readBack?.length, 4);
	assert.equal(
		readBack?.filter((dragged: DragRow) => dragged.kind === "other").length,
		1,
	);

	const w = watcher(ok({}));
	const report = await applyDropVerdicts(
		readBack ?? [],
		{ view: column("c-review", "review", "Review"), root: ROOT },
		"dinah",
		w.host,
		w.spawner,
	);
	const moved = w.calls.filter((argv) => argv.includes("move"));
	assert.equal(moved.length, 3);
	for (const argv of moved) {
		assert.equal(argv[argv.length - 1], "review");
	}
	assert.equal(report.selected, 4);
	assert.deepEqual(
		report.entries.map((entry) => entry.outcome.kind),
		["done", "done", "done", "skipped"],
	);
	// The ref on the skipped entry is what catches a synthetic fourth entry:
	// an implementation padding the report to make it reconcile fails here.
	assert.equal(report.entries[3].ref, "Doing");
});

test("a drag holding no card row sets no entry, and no shape without one reads back", () => {
	const recorder = recordingSink();
	offerDrag([draggedColumnRow()], DRAG_MIME_TYPE, recorder.sink, (rows) => rows);
	assert.deepEqual(recorder.entries, []);
	// The two halves of that rule are written once on each side, so they
	// cannot drift apart.
	assert.equal(dragRowsFrom(payload()), undefined);
	assert.equal(dragRowsFrom([payload(), payload()]), undefined);
	assert.equal(dragRowsFrom([{ kind: "other", ref: "Doing" }]), undefined);
	assert.equal(dragRowsFrom(undefined), undefined);
	assert.equal(dragRowsFrom([]), undefined);
	assert.equal(dragRowsFrom("a string an unrelated extension set"), undefined);
});

test("a drop where every verdict is ignore spawns nothing and calls no member of the host", async () => {
	// A drag is a gesture that can miss, and a gesture that missed has to go
	// on looking like a gesture that missed. This is the one place the drop
	// path's reporting differs from a menu command's, and the assertion is on
	// the whole host rather than on the spawn count, because an absence of
	// spawns is what a run that reported loudly would also show.
	const recorder = recordingSink();
	offerDrag(mixedDrag(), DRAG_MIME_TYPE, recorder.sink, (rows) => rows);
	const readBack = dragRowsFrom(recorder.entries[0][1]);
	const w = watcher(ok({}));
	const report = await applyDropVerdicts(
		readBack ?? [],
		{ view: column("c-doing", "doing", "Doing"), root: ROOT },
		"dinah",
		w.host,
		w.spawner,
	);
	assert.deepEqual(w.calls, []);
	assert.deepEqual(w.errors, []);
	assert.deepEqual(w.infos, []);
	assert.deepEqual(w.checkpoints, []);
	assert.equal(report.selected, 4);
	assert.ok(report.entries.every((entry) => entry.outcome.kind === "skipped"));
});
