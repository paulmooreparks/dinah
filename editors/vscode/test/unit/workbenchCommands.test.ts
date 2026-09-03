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
import { COMMAND_EDIT_WORKBENCH_DEFINITION } from "../../src/identity";
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
	editWorkbenchDefinition,
} from "../../src/workbenchCommands";

const BENCH = "C:/work/board";

/** How a resolved binary describes itself, as describeVersion composes it. */
const TOOL = "dinah 0.1.0, dinah-core/0.7, format 1";

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
	/** Every path handed to the editor, in call order. */
	readonly opened: string[];
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
		opened: [],
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
			openDocument: async (path) => {
				r.opened.push(path);
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
		toolDescription: TOOL,
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
	openDocument: async () => undefined,
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
		const target = contextForWorkbench(element, "dinah", TOOL, silentHost, silentSpawner);
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
	const target = contextForWorkbench(element, "dinah", TOOL, silentHost, silentSpawner);
	assert.equal(target?.path, "C:/work/other");
});

test("contextForWorkbench answers undefined for an absent element", () => {
	// The palette hands a command no argument at all. Both new commands are
	// hidden from it, but a keybinding and another extension can still aim one
	// here, and reading a field off undefined throws before the handler's own
	// wrong-row branch could run.
	assert.equal(
		contextForWorkbench(undefined, "dinah", TOOL, silentHost, silentSpawner),
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
			contextForWorkbench(element, "dinah", TOOL, silentHost, silentSpawner),
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
			contextForWorkbench(element, "dinah", TOOL, silentHost, silentSpawner),
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

test("a check that produced no report reports itself rather than passing for clean", async () => {
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
			[
				`Work: check produced no report the extension could read. ${sentence}`,
				`The binary that answered: dinah (${TOOL}).`,
			],
			`the ${kind} outcome wrote the wrong line`,
		);
		assert.deepEqual(
			r.warnings,
			[
				"Work: check produced no report the extension could read. See the Dinah output channel for details.",
			],
			`the ${kind} outcome told the reader the wrong thing`,
		);
		assert.deepEqual(r.infos, [], `the ${kind} outcome passed for a clean check`);
		assert.equal(r.revealed, 1, `the ${kind} outcome did not offer the channel`);
	}
});

test("the answer a binary predating dinah-346 gives names that binary and no false clause", async () => {
	// The incident this card is named for, driven through the whole path a
	// reader meets rather than through readRefusal alone. A binary built before
	// dinah-346 forces exit 2 for a workbench carrying defects and writes its
	// old report body, which has no refusal field in it.
	//
	// Two properties are asserted here and neither held before. The leading
	// clause no longer says the check could not run, which on this arm is
	// false: the check ran and found the defect. And the line beneath names
	// the executable and what it reported itself to be, so a reader who has
	// been told the answer could not be read learns whose answer it was.
	const r = recorder({
		code: 2,
		stdout: JSON.stringify({
			findings: [
				{ Path: "C:/work/board/workbench.md", Key: "check.missing-slug", Detail: "" },
			],
		}),
		stderr: "",
	});
	r.answer = OPEN_OUTPUT;
	await checkWorkbench(r.context);
	assert.deepEqual(r.appended, [
		'Work: check produced no report the extension could read. not-json: dinah exited 2 (refused), but its JSON carried no string "refusal" field, which every refusal envelope carries. Top-level keys: findings.',
		`The binary that answered: dinah (${TOOL}).`,
	]);
	assert.equal(
		r.appended[0].includes("could not run"),
		false,
		"the line still says the check could not run",
	);
});

test("the binary line falls back to the path alone, and says so when nothing was resolved", async () => {
	// Both degradations are representable: a context whose binary answered but
	// never described itself, and a context composed while no binary was
	// resolved at all, which is what extension.ts passes when resolution
	// failed. Neither may print an empty parenthesis or a bare "at".
	const described = recorder({ code: 3, stdout: "", stderr: "the cursor is old" });
	await checkWorkbench({ ...described.context, toolDescription: "" });
	assert.equal(described.appended[1], "The binary that answered: dinah.");

	const unresolved = recorder({ code: 3, stdout: "", stderr: "the cursor is old" });
	await checkWorkbench({ ...unresolved.context, exe: "", toolDescription: "" });
	assert.equal(
		unresolved.appended[1],
		"No dinah binary was resolved, so nothing ran.",
	);
});

test("contextForWorkbench carries the binary's description to the command", async () => {
	// The field is display-only, so the guard that composes a context is the
	// only place it can arrive from. A context composed without it would leave
	// every message about an unreadable answer naming a path and nothing else.
	const element: TreeElement = { kind: "root", row: rootRow() };
	const target = contextForWorkbench(
		element,
		"C:/tools/dinah.exe",
		TOOL,
		silentHost,
		silentSpawner,
	);
	assert.equal(target?.toolDescription, TOOL);
	assert.equal(target?.exe, "C:/tools/dinah.exe");
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
	const target = contextForWorkbench(element, "dinah", TOOL, r.host, silentSpawner);
	assert.notEqual(target, undefined);
	await copyWorkbenchPath(target as WorkbenchCommandContext);
	assert.deepEqual(r.copied, ["C:/work/other"]);
});

// editWorkbenchDefinition
// ---------------------------------------------------------------------------

/** What `path workbench` answers once dinah-272 lands, as PathAnswer marshals it. */
const RESOLVED: SpawnOutcome = {
	code: 0,
	stdout: JSON.stringify({
		path: `${BENCH}/workbench.md`,
		workbench_source: "search",
	}),
	stderr: "",
};

test("editWorkbenchDefinition asks path for the workbench and nothing else", async () => {
	// dinah-332 AC-5. The argv is the contract between this command and the
	// binary, and the bare `workbench` reference is the one dinah-272's own
	// fast path answers from discovery alone. A --migrate flag here would turn
	// an act that reads into an act that writes.
	const r = recorder(RESOLVED);
	await editWorkbenchDefinition(r.context);
	assert.deepEqual(r.calls, [
		["--json", "--workbench", BENCH, "path", "workbench"],
	]);
});

test("editWorkbenchDefinition hands the resolved path to the editor and says nothing", async () => {
	// dinah-332 AC-6. Opening the file is the whole answer, so a toast would
	// tell a reader something the editor has already shown them.
	const r = recorder(RESOLVED);
	await editWorkbenchDefinition(r.context);
	assert.deepEqual(r.opened, [`${BENCH}/workbench.md`]);
	assert.deepEqual(r.appended, []);
	assert.deepEqual(r.infos, []);
	assert.deepEqual(r.warnings, []);
	assert.equal(r.revealed, 0);
});

test("editWorkbenchDefinition opens nothing when the answer carries no path", async () => {
	// dinah-332 AC-6's other half. An ok answer with no path is a binary this
	// build and dinah disagree about, not a file to open, and openDocument on
	// an empty string would ask the editor for the current directory.
	for (const [what, stdout] of [
		["an absent path", JSON.stringify({ workbench_source: "search" })],
		["an empty path", JSON.stringify({ path: "", workbench_source: "search" })],
	] as const) {
		const r = recorder({ code: 0, stdout, stderr: "" });
		await editWorkbenchDefinition(r.context);
		assert.deepEqual(r.opened, [], `${what} still reached the editor`);
		assert.deepEqual(
			r.logged,
			[`${COMMAND_EDIT_WORKBENCH_DEFINITION} answered with no path`],
			`${what} logged the wrong line`,
		);
		assert.deepEqual(r.warnings, [], `${what} raised a toast`);
	}
});

test("editWorkbenchDefinition reports every refusal and opens nothing", async () => {
	// dinah-332 AC-7. Each non-ok kind reaches the same arm, and each has to
	// say why: a definition file that will not open is exactly the moment a
	// reader needs the reason rather than silence.
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
			{ code: 0, stdout: "C:/work/board/workbench.md\n", stderr: "" },
			"not-json: this binary is not dinah, or is too old to answer `--json version`",
		],
	];
	for (const [kind, outcome, sentence] of cases) {
		const r = recorder(outcome);
		r.answer = OPEN_OUTPUT;
		await editWorkbenchDefinition(r.context);
		assert.deepEqual(
			r.appended,
			[`Work: could not open the workbench definition file. ${sentence}`],
			`the ${kind} outcome wrote the wrong line`,
		);
		assert.deepEqual(
			r.warnings,
			[
				"Work: could not open the workbench definition file. See the Dinah output channel for details.",
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
	await editWorkbenchDefinition(r.context);
	assert.equal(r.revealed, 0);
	assert.equal(r.appended.length, 1);
});
