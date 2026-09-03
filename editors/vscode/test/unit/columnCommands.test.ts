// What a column row's one act sends, and what a reader is told afterwards.
//
// Two of the assertions here are a pair rather than a repetition, and the pair
// is the point of the file. One pins the field the guard reads when the status
// join found no view for this column, which is a race columnsOf documents. The
// other pins the field it reads on the ordinary path, where a view is present
// and its raw id differs from the slug the node carries. Either one alone
// passes on a build that reads the two fields in the wrong order.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import type {
	ColumnCommandContext,
	ColumnCommandHost,
} from "../../src/columnCommands";
import {
	OPEN_OUTPUT,
	contextForColumn,
	editColumnInstructions,
} from "../../src/columnCommands";
import { COMMAND_EDIT_COLUMN_INSTRUCTIONS } from "../../src/identity";
import type { RootRow, TreeElement } from "../../src/tree";
import type { ColumnView } from "../../src/wire";

const BENCH = "C:/work/board";

/** The column's own raw identifier, which is what resolves precisely. */
const COLUMN_ID = "c0100000000a";

/** The column's slug, which is what its tree node carries as a ref. */
const COLUMN_SLUG = "spec";

/** What `path <column-ref>` answers, as PathAnswer marshals it. */
const RESOLVED: SpawnOutcome = {
	code: 0,
	stdout: JSON.stringify({
		path: `${BENCH}/columns/${COLUMN_ID}/column.md`,
		workbench_source: "search",
	}),
	stderr: "",
};

interface Recorder {
	readonly host: ColumnCommandHost;
	context: ColumnCommandContext;
	/** Every argv the spawner was handed, in call order. */
	readonly calls: string[][];
	readonly warnings: string[];
	readonly appended: string[];
	readonly opened: string[];
	readonly logged: string[];
	/** How many times the channel was revealed. */
	revealed: number;
	/** What the warning toast's action returns, undefined meaning dismissed. */
	answer?: string;
}

function recorder(outcome: SpawnOutcome = RESOLVED): Recorder {
	const spawner: Spawner = async (_exe, argv) => {
		r.calls.push([...argv]);
		return outcome;
	};
	const r: Recorder = {
		calls: [],
		warnings: [],
		appended: [],
		opened: [],
		logged: [],
		revealed: 0,
		host: {
			showWarning: async (message, actions) => {
				r.warnings.push(message);
				// The action list is asserted here rather than in each test,
				// because a toast offering a button the handler does not read
				// would be a dead end a reader clicks once and never again.
				assert.deepEqual([...actions], [OPEN_OUTPUT]);
				return r.answer;
			},
			appendLines: (lines) => {
				r.appended.push(...lines);
			},
			revealOutput: () => {
				r.revealed += 1;
			},
			openDocument: async (path) => {
				r.opened.push(path);
			},
			log: (line) => {
				r.logged.push(line);
			},
		},
		// Filled in below, because the context holds the host the object above
		// composes and neither can be spelled before the other.
		context: undefined as unknown as ColumnCommandContext,
	};
	r.context = {
		spawner,
		exe: "dinah",
		host: r.host,
		root: BENCH,
		columnRef: COLUMN_ID,
		label: "Spec",
	};
	return r;
}

const silentHost: ColumnCommandHost = {
	showWarning: async () => undefined,
	appendLines: () => undefined,
	revealOutput: () => undefined,
	openDocument: async () => undefined,
	log: () => undefined,
};

const silentSpawner: Spawner = async () => RESOLVED;

function columnView(overrides: Partial<ColumnView> = {}): ColumnView {
	return {
		id: COLUMN_ID,
		slug: COLUMN_SLUG,
		title: "Spec",
		kind: "flow",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: true,
		count: 2,
		...overrides,
	};
}

function rootRow(overrides: Partial<RootRow> = {}): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: "C:/work",
		folderName: "work",
		description: "",
		sole: true,
		data: {
			path: BENCH,
			title: "Work",
			columns: new Map(),
			cards: new Map(),
		},
		...overrides,
	};
}

/** A column row, with whatever view and node value the case under test needs. */
function columnElement(
	view: ColumnView | undefined,
	value: string | undefined,
	row: RootRow = rootRow(),
): TreeElement {
	return {
		kind: "column",
		row,
		node: { kind: "column", axis: "column", value, title: "Spec", count: 2 },
		view,
	};
}

// contextForColumn
// ---------------------------------------------------------------------------

test("contextForColumn resolves through view.id when the status join found a view", async () => {
	// dinah-332 AC-10, the ordinary path and the half AC-9 cannot see.
	//
	// The view's id and the node's value are deliberately different strings
	// here, because they differ on every slugged column in a real workbench:
	// tree.go fills a column node's Value with the column's own Ref(), which is
	// the slug when there is one. A build reading `node.value ?? view?.id`
	// still passes the race assertion below while answering this, the common
	// case, with the slug instead of the identifier.
	const element = columnElement(columnView(), COLUMN_SLUG);
	const target = contextForColumn(element, "dinah", silentHost, silentSpawner);
	assert.notEqual(target, undefined, "a column row with a view composed no context");
	assert.equal(target?.columnRef, COLUMN_ID);
	assert.notEqual(
		COLUMN_ID,
		COLUMN_SLUG,
		"the two fields carry the same string, so this case proved nothing",
	);

	// And the ref the guard read is the ref that reaches the binary.
	const r = recorder();
	const viewTarget = contextForColumn(element, "dinah", r.host, r.context.spawner);
	await editColumnInstructions(viewTarget as ColumnCommandContext);
	assert.deepEqual(r.calls, [["--json", "--workbench", BENCH, "path", COLUMN_ID]]);
});

test("contextForColumn resolves through node.value when no view was joined", async () => {
	// dinah-332 AC-9, the race columnsOf documents: a column deleted between
	// the status and the tree calls of one checkpoint leaves the node with no
	// view beside it, and the row self-heals on the next checkpoint.
	//
	// Asserting only that the guard declines to throw here would pass on a
	// fallback reading node.id, which is undefined on every column node, so
	// what is asserted is that the resolved ref actually equals node.value.
	const element = columnElement(undefined, COLUMN_SLUG);
	const target = contextForColumn(element, "dinah", silentHost, silentSpawner);
	assert.notEqual(target, undefined, "the race composed no context at all");
	assert.equal(target?.columnRef, COLUMN_SLUG);

	const r = recorder();
	const raceTarget = contextForColumn(element, "dinah", r.host, r.context.spawner);
	await editColumnInstructions(raceTarget as ColumnCommandContext);
	assert.deepEqual(r.calls, [["--json", "--workbench", BENCH, "path", COLUMN_SLUG]]);
});

test("contextForColumn answers undefined for an absent element", () => {
	// dinah-332 AC-4. The palette hands a command no argument at all. This
	// command is hidden from it, but a keybinding and another extension can
	// still aim one here, and reading a field off undefined throws before the
	// handler's own wrong-row branch could run.
	assert.equal(contextForColumn(undefined, "dinah", silentHost, silentSpawner), undefined);
});

test("contextForColumn answers undefined for every element kind that is not a column", () => {
	// dinah-332 AC-4. Each of these rows carries a resolvable workbench path,
	// so each reaches the kind check on its own rather than being turned away
	// by the path check standing behind it.
	const row = rootRow();
	const elements: [string, TreeElement][] = [
		["a workbench root", { kind: "root", row }],
		[
			"a state group",
			{
				kind: "group",
				row,
				node: { kind: "group", axis: "state", value: "ready", count: 2 },
			},
		],
		[
			"a card",
			{
				kind: "card",
				row,
				node: { kind: "card", id: "f00000000009", ref: "work-1", count: 0 },
			},
		],
		[
			"a note",
			{ kind: "note", owner: row, text: "did not answer", tooltip: "malformed" },
		],
	];
	for (const [what, element] of elements) {
		assert.equal(
			contextForColumn(element, "dinah", silentHost, silentSpawner),
			undefined,
			`${what} composed a context`,
		);
	}
});

test("contextForColumn answers undefined for a column row that names no column", () => {
	// dinah-332 AC-4. Both rows below carry a resolvable workbench path, so
	// each one reaches the reference check rather than being turned away by the
	// path check for a reason that has nothing to do with what it tests.
	const cases: [string, TreeElement][] = [
		["no view and no node value", columnElement(undefined, undefined)],
		["no view and an empty node value", columnElement(undefined, "")],
	];
	for (const [what, element] of cases) {
		assert.equal(
			contextForColumn(element, "dinah", silentHost, silentSpawner),
			undefined,
			`a column row with ${what} composed a context`,
		);
	}
});

test("contextForColumn answers undefined for a column row under an unresolvable workbench", () => {
	// dinah-332 AC-4. This row carries a view, so its reference resolves and
	// the case reaches the path check that it exists to exercise. Without the
	// view it would be refused one line earlier and would test nothing.
	const cases: [string, RootRow][] = [
		["neither data nor a candidate", rootRow({ data: undefined })],
		[
			"a candidate whose path is empty",
			rootRow({
				rowKind: "workbenchCandidate",
				data: undefined,
				candidate: { path: "", title: "Other" },
			}),
		],
	];
	for (const [what, row] of cases) {
		const element = columnElement(columnView(), COLUMN_SLUG, row);
		assert.equal(
			contextForColumn(element, "dinah", silentHost, silentSpawner),
			undefined,
			`a row carrying ${what} composed a context`,
		);
	}
});

test("contextForColumn reads an unexpanded candidate's own workbench path", () => {
	// The companion the absence assertions above need. A guard that answered
	// undefined for every candidate row would satisfy them while offering the
	// menu to nobody on a workbench that has not been expanded yet.
	const element = columnElement(
		columnView(),
		COLUMN_SLUG,
		rootRow({
			rowKind: "workbenchCandidate",
			data: undefined,
			candidate: { path: "C:/work/other", title: "Other" },
		}),
	);
	const target = contextForColumn(element, "dinah", silentHost, silentSpawner);
	assert.equal(target?.root, "C:/work/other");
	assert.equal(target?.columnRef, COLUMN_ID);
});

test("contextForColumn reuses the label the row already draws", () => {
	// The label is what every message below names, and composing it a second
	// time here would let the toast and the row disagree about what this
	// column is called.
	const withView = contextForColumn(
		columnElement(columnView({ title: "Agent Code Review" }), COLUMN_SLUG),
		"dinah",
		silentHost,
		silentSpawner,
	);
	assert.equal(withView?.label, "Agent Code Review");
	// With no view the row draws the node's own value, so the context does too.
	const raced = contextForColumn(
		columnElement(undefined, COLUMN_SLUG),
		"dinah",
		silentHost,
		silentSpawner,
	);
	assert.equal(raced?.label, COLUMN_SLUG);
});

// editColumnInstructions
// ---------------------------------------------------------------------------

test("editColumnInstructions asks path for the column and nothing else", async () => {
	// dinah-332 AC-5. The argv is the contract between this command and the
	// binary. A --migrate flag here would turn an act that reads into an act
	// that writes.
	const r = recorder();
	await editColumnInstructions(r.context);
	assert.deepEqual(r.calls, [["--json", "--workbench", BENCH, "path", COLUMN_ID]]);
});

test("editColumnInstructions hands the resolved path to the editor and says nothing", async () => {
	// dinah-332 AC-6. Opening the file is the whole answer, so a toast would
	// tell a reader something the editor has already shown them.
	const r = recorder();
	await editColumnInstructions(r.context);
	assert.deepEqual(r.opened, [`${BENCH}/columns/${COLUMN_ID}/column.md`]);
	assert.deepEqual(r.appended, []);
	assert.deepEqual(r.warnings, []);
	assert.equal(r.revealed, 0);
});

test("editColumnInstructions opens nothing when the answer carries no path", async () => {
	// dinah-332 AC-6's other half. An ok answer with no path is a binary this
	// build and dinah disagree about, not a file to open, and openDocument on
	// an empty string would ask the editor for the current directory.
	for (const [what, stdout] of [
		["an absent path", JSON.stringify({ workbench_source: "search" })],
		["an empty path", JSON.stringify({ path: "", workbench_source: "search" })],
	] as const) {
		const r = recorder({ code: 0, stdout, stderr: "" });
		await editColumnInstructions(r.context);
		assert.deepEqual(r.opened, [], `${what} still reached the editor`);
		assert.deepEqual(
			r.logged,
			[`${COMMAND_EDIT_COLUMN_INSTRUCTIONS} answered with no path`],
			`${what} logged the wrong line`,
		);
		assert.deepEqual(r.warnings, [], `${what} raised a toast`);
	}
});

test("editColumnInstructions reports every refusal and opens nothing", async () => {
	// dinah-332 AC-7. Each non-ok kind reaches the same arm, and each has to
	// say why. This command is explicitly not a recovery path for a broken
	// column.md (dinah-332 D-4), so the refusal a corrupted workbench produces
	// is what a reader meets here and it has to name itself.
	const cases: [string, SpawnOutcome, string][] = [
		[
			"refused",
			{
				code: 2,
				stdout: JSON.stringify({
					outcome: "refused",
					refusal: "dinah.malformed",
					detail: "column spec",
				}),
				stderr: "",
			},
			"dinah.malformed: column spec",
		],
		[
			"stale",
			{ code: 3, stdout: "", stderr: "the cursor is old" },
			"stale: the cursor is old",
		],
		[
			"unreachable",
			{ code: 4, stdout: "", stderr: "no such directory" },
			"unreachable: no such directory",
		],
		[
			"spawn-failed",
			{
				code: null,
				stdout: "",
				stderr: "",
				spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
			},
			"spawn-failed: spawn dinah ENOENT",
		],
		[
			"not-json",
			{ code: 0, stdout: "C:/work/board/columns/spec/column.md\n", stderr: "" },
			"not-json: this binary is not dinah, or is too old to answer `--json version`",
		],
	];
	for (const [kind, outcome, sentence] of cases) {
		const r = recorder(outcome);
		r.answer = OPEN_OUTPUT;
		await editColumnInstructions(r.context);
		assert.deepEqual(
			r.appended,
			[`Spec: could not open this column's instructions file. ${sentence}`],
			`the ${kind} outcome wrote the wrong line`,
		);
		assert.deepEqual(
			r.warnings,
			[
				"Spec: could not open this column's instructions file. See the Dinah output channel for details.",
			],
			`the ${kind} outcome told the reader the wrong thing`,
		);
		assert.deepEqual(r.opened, [], `the ${kind} outcome opened a file anyway`);
		assert.equal(r.revealed, 1, `the ${kind} outcome did not offer the channel`);
	}
});

test("a dismissed refusal toast leaves the channel unrevealed and the reason in it", async () => {
	// dinah-332 AC-7's action half. The button is only worth offering if
	// picking it reveals the channel and dismissing it does not, and the reason
	// is in the channel either way.
	const r = recorder({ code: 3, stdout: "", stderr: "the cursor is old" });
	r.answer = undefined;
	await editColumnInstructions(r.context);
	assert.equal(r.revealed, 0);
	assert.equal(r.appended.length, 1);
});
