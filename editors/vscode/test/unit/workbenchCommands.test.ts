// What a workbench row's two acts send, and what a reader is told afterwards.
//
// The check command's whole value is that its answer reaches somebody. A run
// that found nothing has to say so, or a reader cannot tell it from a run that
// never happened, and a run that could not happen at all has to say that too.
// Those three outcomes are what this file pins.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import { EXIT_READ_FINDINGS } from "../../src/cli";
import type { RootRow, TreeElement } from "../../src/tree";
import type {
	WorkbenchCommandContext,
	WorkbenchCommandHost,
} from "../../src/workbenchCommands";
import {
	OPEN_OUTPUT,
	checkWorkbench,
	contextForWorkbench,
	copyWorkbenchPath,
} from "../../src/workbenchCommands";

const BENCH = "C:/work/board";

/** What a clean check answers, as verb.CheckReport marshals a nil slice. */
const CLEAN: SpawnOutcome = {
	code: 0,
	stdout: JSON.stringify({ outcome: "ok", findings: null }),
	stderr: "",
};

/** What a check that found two defects answers, on the exit code dinah-346 minted. */
const DIRTY: SpawnOutcome = {
	code: EXIT_READ_FINDINGS,
	stdout: JSON.stringify({
		outcome: "findings",
		findings: [
			{
				Path: "C:/work/board/cards/f00000000009/card.md",
				Key: "check.dangling-workstream",
				Detail: "f00000000009",
			},
			{ Path: "C:/work/board/workbench.md", Key: "check.missing-slug", Detail: "" },
		],
	}),
	stderr: "",
};

interface Recorder {
	readonly host: WorkbenchCommandHost;
	context: WorkbenchCommandContext;
	/** Every argv the spawner was handed, in call order. */
	readonly calls: string[][];
	readonly infos: string[];
	readonly warnings: string[];
	readonly appended: string[];
	readonly copied: string[];
	readonly logged: string[];
	/** How many times the channel was revealed. */
	revealed: number;
	/** What the warning toast's action returns, undefined meaning dismissed. */
	answer?: string;
}

function recorder(outcome: SpawnOutcome = CLEAN): Recorder {
	const spawner: Spawner = async (_exe, argv) => {
		r.calls.push([...argv]);
		return outcome;
	};
	const r: Recorder = {
		calls: [],
		infos: [],
		warnings: [],
		appended: [],
		copied: [],
		logged: [],
		revealed: 0,
		host: {
			showInfo: (message) => {
				r.infos.push(message);
			},
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
			copyToClipboard: async (text) => {
				r.copied.push(text);
			},
			log: (line) => {
				r.logged.push(line);
			},
		},
		// Filled in below, because the context holds the host the object above
		// composes and neither can be spelled before the other.
		context: undefined as unknown as WorkbenchCommandContext,
	};
	r.context = {
		spawner,
		exe: "dinah",
		host: r.host,
		path: BENCH,
		label: "Work",
	};
	return r;
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

const silentHost: WorkbenchCommandHost = {
	showInfo: () => undefined,
	showWarning: async () => undefined,
	appendLines: () => undefined,
	revealOutput: () => undefined,
	copyToClipboard: async () => undefined,
	log: () => undefined,
};

const silentSpawner: Spawner = async () => CLEAN;

// contextForWorkbench
// ---------------------------------------------------------------------------

test("contextForWorkbench composes a context for each of the three resolved row kinds", () => {
	// The companion the absence assertions below need. A guard that answered
	// undefined for everything would satisfy every test in the next block
	// while offering the menu to nobody.
	for (const rowKind of [
		"workbenchRoot",
		"workbenchCandidate",
		"workbenchForest",
	] as const) {
		const element: TreeElement = { kind: "root", row: rootRow({ rowKind }) };
		const target = contextForWorkbench(element, "dinah", silentHost, silentSpawner);
		assert.notEqual(target, undefined, `a ${rowKind} row composed no context`);
		assert.equal(target?.path, BENCH);
	}
});

test("contextForWorkbench reads a candidate's own path before it has been expanded", () => {
	// A candidate row carries no data until somebody expands it, and the whole
	// point of offering Check there is that it can be run without expanding
	// first (dinah-330 D-4).
	const element: TreeElement = {
		kind: "root",
		row: rootRow({
			rowKind: "workbenchCandidate",
			data: undefined,
			candidate: { path: "C:/work/other", title: "Other" },
		}),
	};
	const target = contextForWorkbench(element, "dinah", silentHost, silentSpawner);
	assert.equal(target?.path, "C:/work/other");
});

test("contextForWorkbench answers undefined for an absent element", () => {
	// The palette hands a command no argument at all. Both new commands are
	// hidden from it, but a keybinding and another extension can still aim one
	// here, and reading a field off undefined throws before the handler's own
	// wrong-row branch could run.
	assert.equal(
		contextForWorkbench(undefined, "dinah", silentHost, silentSpawner),
		undefined,
	);
});

test("contextForWorkbench answers undefined for a dead end and for a row with no path", () => {
	const cases: [string, TreeElement][] = [
		["a dead end", { kind: "root", row: rootRow({ rowKind: "deadEnd" }) }],
		[
			"a row carrying neither data nor a candidate",
			{ kind: "root", row: rootRow({ data: undefined }) },
		],
	];
	for (const [what, element] of cases) {
		assert.equal(
			contextForWorkbench(element, "dinah", silentHost, silentSpawner),
			undefined,
			`${what} composed a context`,
		);
	}
});

test("contextForWorkbench answers undefined for every element kind that is not a root row", () => {
	const row = rootRow();
	const elements: TreeElement[] = [
		{
			kind: "column",
			row,
			node: { kind: "column", id: "spec", title: "Spec", count: 2 },
		},
		{
			kind: "group",
			row,
			node: { kind: "group", axis: "state", value: "ready", count: 2 },
		},
		{
			kind: "card",
			row,
			node: {
				kind: "card",
				id: "f00000000009",
				ref: "board-1",
				title: "A card",
				count: 0,
			},
		},
		{ kind: "note", owner: row, text: "a note", tooltip: "a note" },
	];
	for (const element of elements) {
		assert.equal(
			contextForWorkbench(element, "dinah", silentHost, silentSpawner),
			undefined,
			`a ${element.kind} row composed a context`,
		);
	}
});

// checkWorkbench
// ---------------------------------------------------------------------------

test("checkWorkbench asks for a plain check of the one workbench", async () => {
	// AC-4, from the command's side rather than runCheck's. A --migrate flag
	// reaching here would repair a workbench somebody only wanted to inspect.
	const r = recorder();
	await checkWorkbench(r.context);
	assert.deepEqual(r.calls[0], ["--json", "--workbench", BENCH, "check"]);
	assert.equal(
		r.calls[0].filter((arg) => arg.startsWith("--migrate") || arg === "--finish").length,
		0,
	);
});

test("a clean check says so and writes nothing to the channel", async () => {
	// AC-5. Silence would be the worst answer available: a reader who ran the
	// check and saw nothing cannot tell a clean workbench from a command that
	// did not fire.
	const r = recorder(CLEAN);
	await checkWorkbench(r.context);
	assert.deepEqual(r.infos, ["Work: check found no defects."]);
	assert.deepEqual(r.appended, []);
	assert.deepEqual(r.warnings, []);
	assert.equal(r.revealed, 0);
});

test("a clean check is read off the report's outcome, not off an empty findings array", async () => {
	// dinah-346 landed the outcome member so a client answers this with one
	// string comparison. An answer whose array is empty but whose outcome says
	// findings is a malformed report, and reading the array would quietly
	// call it clean.
	const r = recorder({
		code: EXIT_READ_FINDINGS,
		stdout: JSON.stringify({ outcome: "findings", findings: [] }),
		stderr: "",
	});
	await checkWorkbench(r.context);
	assert.deepEqual(r.infos, []);
	assert.equal(r.warnings.length, 1);
});

test("a check that found defects writes a header and one line for each of them", async () => {
	// AC-6. The count in the header is what a reader sees in the toast, so the
	// two are composed from the same number rather than from two counts.
	const r = recorder(DIRTY);
	r.answer = OPEN_OUTPUT;
	await checkWorkbench(r.context);
	assert.deepEqual(r.appended, [
		"Work: check found 2 defect(s):",
		"C:/work/board/cards/f00000000009/card.md: check.dangling-workstream (f00000000009)",
		"C:/work/board/workbench.md: check.missing-slug",
	]);
	assert.deepEqual(r.warnings, [
		"Work: check found 2 defect(s). See the Dinah output channel for details.",
	]);
	assert.deepEqual(r.infos, []);
});

test("the channel opens when the reader asks for it and stays shut when they do not", async () => {
	// AC-6's second half. A toast that revealed the channel either way would
	// take the window away from whatever the reader was doing.
	const picked = recorder(DIRTY);
	picked.answer = OPEN_OUTPUT;
	await checkWorkbench(picked.context);
	assert.equal(picked.revealed, 1);

	const dismissed = recorder(DIRTY);
	dismissed.answer = undefined;
	await checkWorkbench(dismissed.context);
	assert.equal(dismissed.revealed, 0);
	// The findings are in the channel either way, so dismissing the toast
	// loses nothing but the trip.
	assert.equal(dismissed.appended.length, 3);
});

test("a check that could not run at all reports itself rather than passing for clean", async () => {
	// AC-7. Every non-ok outcome takes this path, and the failure that matters
	// most is the refusal, which used to be indistinguishable from a dirty
	// workbench because both exited 2.
	const cases: [string, SpawnOutcome, string][] = [
		[
			"refused",
			{
				code: 2,
				stdout: JSON.stringify({
					outcome: "refused",
					refusal: "dinah.no-workbench",
					detail: "no workbench.md here",
				}),
				stderr: "",
			},
			"dinah.no-workbench: no workbench.md here",
		],
		["stale", { code: 3, stdout: "", stderr: "the cursor is old" }, "stale: the cursor is old"],
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
			{ code: 0, stdout: "everything is fine\n", stderr: "" },
			"not-json: dinah check exited 0 and wrote no JSON report",
		],
	];
	for (const [kind, outcome, sentence] of cases) {
		const r = recorder(outcome);
		r.answer = OPEN_OUTPUT;
		await checkWorkbench(r.context);
		assert.deepEqual(
			r.appended,
			[`Work: check could not run. ${sentence}`],
			`the ${kind} outcome wrote the wrong line`,
		);
		assert.deepEqual(
			r.warnings,
			["Work: check could not run. See the Dinah output channel for details."],
			`the ${kind} outcome told the reader the wrong thing`,
		);
		assert.deepEqual(r.infos, [], `the ${kind} outcome passed for a clean check`);
		assert.equal(r.revealed, 1, `the ${kind} outcome did not offer the channel`);
	}
});

// copyWorkbenchPath
// ---------------------------------------------------------------------------

test("copyWorkbenchPath copies the path and names what it copied", async () => {
	// AC-8. The path rather than the row's description, which is empty on the
	// ordinary single-workbench row and would put nothing on the clipboard
	// while reporting success (dinah-330 D-3).
	const r = recorder();
	await copyWorkbenchPath(r.context);
	assert.deepEqual(r.copied, [BENCH]);
	assert.deepEqual(r.infos, [`Copied ${BENCH}`]);
});

test("copyWorkbenchPath runs dinah not at all", async () => {
	// The clipboard already holds everything this act needs, and a spawn here
	// would make a copy cost a process against somebody's board.
	const r = recorder();
	await copyWorkbenchPath(r.context);
	assert.deepEqual(r.calls, []);
});

test("copyWorkbenchPath copies an unexpanded candidate's own path", async () => {
	// The end-to-end half of AC-8: the value the guard read off the row is the
	// value that reaches the clipboard, for a row that has never been opened.
	const element: TreeElement = {
		kind: "root",
		row: rootRow({
			rowKind: "workbenchCandidate",
			data: undefined,
			candidate: { path: "C:/work/other", title: "Other" },
		}),
	};
	const r = recorder();
	const target = contextForWorkbench(element, "dinah", r.host, silentSpawner);
	assert.notEqual(target, undefined);
	await copyWorkbenchPath(target as WorkbenchCommandContext);
	assert.deepEqual(r.copied, ["C:/work/other"]);
});
