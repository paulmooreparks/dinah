// What the Problems panel is told, and what it stops being told.
//
// Every report below is the shape a real binary answered with. The two
// envelopes were reproduced against a build of cmd/dinah run over a scratch
// workbench: a clean workbench answers `{"outcome":"ok","findings":null}` on
// exit 0, and a workbench carrying an orphaned columns/<id> directory answers
// `outcome: "findings"` with one finding on exit 5, whose Path is that
// directory and whose Key and Detail are raw untranslated tokens. Nothing here
// invents a field name or a casing.
//
// The clearing half gets the same weight as the adding half, deliberately. A
// finding that has been fixed and a finding that has moved to another file
// both leave a row behind in a collection that is only ever written to, and a
// stale row is worse than a missing one because a reader acts on it.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { CliOutcome, SpawnOutcome, Spawner } from "../../src/cli";
import type {
	CheckDiagnosticsDeps,
	DiagnosticEntry,
	DiagnosticPlan,
	StatKind,
} from "../../src/diagnostics";
import { CheckDiagnostics, SOURCE, planFor } from "../../src/diagnostics";
import { ENGLISH } from "../../src/l10n";
import type { CheckAnswer, CheckFinding } from "../../src/wire";

const extensionRoot = join(__dirname, "..", "..", "..");

const ROOT = "C:/scratch/bench";
const OTHER_ROOT = "C:/scratch/other";
const DEFINITION = "C:/scratch/bench/workbench.md";
const CARD_FILE = "C:/scratch/bench/.dinah/abc/cards/0123456789ab/card.md";
const SECOND_FILE = "C:/scratch/bench/.dinah/abc/cards/ba9876543210/card.md";
const COLUMN_DIR = "C:/scratch/bench/.dinah/abc/columns/0123456789ab";

/** The clean report, exactly as the binary writes it: a nil slice, not an array. */
const CLEAN: CheckAnswer = { outcome: "ok", findings: null };

/** The finding a workbench carrying an orphaned column directory answers with. */
const ORPHANED: CheckFinding = {
	Path: COLUMN_DIR,
	Key: "check.orphaned-column-directory",
	Detail: "0123456789ab",
};

/** A finding whose Path is a card's own file, which the editor can open. */
function atFile(path: string): CheckFinding {
	return { Path: path, Key: "check.unknown-column", Detail: "" };
}

/** An ok CliOutcome carrying one report. */
function answered(report: CheckAnswer): CliOutcome {
	return { kind: "ok", json: report };
}

/**
 * A stand-in for the DiagnosticCollection, and a log of every call made to it.
 *
 * The collection half replaces one path's entries and touches no other path,
 * which is the behaviour that makes clearing a thing somebody has to do rather
 * than something that happens. The call log is what the clearing assertions
 * read, because a plan asserted on in isolation cannot show that the clearing
 * branch ever ran.
 */
function panel(): {
	readonly calls: DiagnosticPlan[];
	readonly collection: Map<string, readonly DiagnosticEntry[]>;
	readonly apply: (byPath: DiagnosticPlan) => void;
} {
	const calls: DiagnosticPlan[] = [];
	const collection = new Map<string, readonly DiagnosticEntry[]>();
	return {
		calls,
		collection,
		apply: (byPath) => {
			calls.push(new Map(byPath));
			for (const [path, entries] of byPath) {
				collection.set(path, entries);
			}
		},
	};
}

/** Every message currently standing at one path, which is empty when cleared. */
function messagesAt(
	collection: Map<string, readonly DiagnosticEntry[]>,
	path: string,
): string[] {
	return (collection.get(path) ?? []).map((entry) => entry.message);
}

/** Every message the panel is currently showing anywhere. */
function allMessages(
	collection: Map<string, readonly DiagnosticEntry[]>,
): string[] {
	return [...collection.values()].flatMap((entries) =>
		entries.map((entry) => entry.message),
	);
}

/** A statKind that answers from a table and calls anything unnamed missing. */
function stats(table: Readonly<Record<string, StatKind>>) {
	return async (path: string): Promise<StatKind> =>
		Promise.resolve(table[path] ?? "missing");
}

/** The deps a test does not care about, with the ones it does laid over. */
function deps(
	over: Partial<CheckDiagnosticsDeps> & Pick<CheckDiagnosticsDeps, "apply">,
): CheckDiagnosticsDeps {
	return {
		spawner: async () =>
			Promise.resolve({ code: 0, stdout: "{}", stderr: "" } as SpawnOutcome),
		exe: "dinah",
		log: () => {},
		t: ENGLISH,
		statKind: stats({}),
		resolveFallback: async () => Promise.resolve(DEFINITION),
		...over,
	};
}

// ---------------------------------------------------------------------------
// AC-1: both directions, in one file, on the reports the binary really writes
// ---------------------------------------------------------------------------

test("a clean workbench shows nothing and a damaged one shows what it found", async () => {
	const clean = panel();
	const cleanRun = new CheckDiagnostics(deps({ apply: clean.apply }));
	await cleanRun.applyResult(ROOT, "Bench", answered(CLEAN));
	assert.deepEqual(
		allMessages(clean.collection),
		[],
		"a confirmed-clean run put something in the panel",
	);

	const damaged = panel();
	const damagedRun = new CheckDiagnostics(
		deps({
			apply: damaged.apply,
			statKind: stats({ [COLUMN_DIR]: "directory", [DEFINITION]: "file" }),
		}),
	);
	await damagedRun.applyResult(
		ROOT,
		"Bench",
		answered({ outcome: "findings", findings: [ORPHANED] }),
	);
	const shown = allMessages(damaged.collection);
	assert.equal(shown.length, 1);
	assert.ok(
		shown[0].includes(ORPHANED.Key),
		`wanted the finding's key in "${shown[0]}"`,
	);
});

// ---------------------------------------------------------------------------
// AC-2: what was fixed, and what moved, both stop being shown
// ---------------------------------------------------------------------------

test("a finding that is fixed is cleared from the file it was on", async () => {
	const shown = panel();
	const run = new CheckDiagnostics(
		deps({ apply: shown.apply, statKind: stats({ [CARD_FILE]: "file" }) }),
	);
	await run.applyResult(
		ROOT,
		"Bench",
		answered({ outcome: "findings", findings: [atFile(CARD_FILE)] }),
	);
	await run.applyResult(ROOT, "Bench", answered(CLEAN));

	// Read off the calls rather than off the second plan alone. A second call
	// that simply wrote an empty plan would leave the first call's row standing
	// in a real collection, and only the argument the host was invoked with
	// shows whether the clearing branch ran at all.
	assert.equal(shown.calls.length, 2);
	assert.deepEqual(
		[...(shown.calls[0].get(CARD_FILE) ?? [])].map((entry) => entry.message),
		["check.unknown-column"],
	);
	assert.deepEqual(
		shown.calls[1].get(CARD_FILE),
		[],
		"the second call did not clear the file the first one wrote to",
	);
	assert.deepEqual(messagesAt(shown.collection, CARD_FILE), []);
});

test("a finding that moves to another file leaves nothing behind on the old one", async () => {
	const shown = panel();
	const run = new CheckDiagnostics(
		deps({
			apply: shown.apply,
			statKind: stats({ [CARD_FILE]: "file", [SECOND_FILE]: "file" }),
		}),
	);
	await run.applyResult(
		ROOT,
		"Bench",
		answered({ outcome: "findings", findings: [atFile(CARD_FILE)] }),
	);
	await run.applyResult(
		ROOT,
		"Bench",
		answered({ outcome: "findings", findings: [atFile(SECOND_FILE)] }),
	);

	assert.deepEqual(
		shown.calls[1].get(CARD_FILE),
		[],
		"the file the finding moved off was not cleared",
	);
	assert.deepEqual(messagesAt(shown.collection, CARD_FILE), []);
	assert.deepEqual(messagesAt(shown.collection, SECOND_FILE), [
		"check.unknown-column",
	]);
});

// ---------------------------------------------------------------------------
// AC-3: where a finding lands, decided at run time rather than from a list
// ---------------------------------------------------------------------------

test("a finding lands on its own file, on the definition file, or in the log", async () => {
	const onItsOwnFile = await planFor(
		{ outcome: "findings", findings: [atFile(CARD_FILE)] },
		DEFINITION,
		stats({ [CARD_FILE]: "file" }),
		() => {},
	);
	assert.deepEqual([...onItsOwnFile.keys()], [CARD_FILE]);

	const onTheDefinition = await planFor(
		{ outcome: "findings", findings: [ORPHANED] },
		DEFINITION,
		stats({ [COLUMN_DIR]: "directory" }),
		() => {},
	);
	assert.deepEqual([...onTheDefinition.keys()], [DEFINITION]);
	const moved = (onTheDefinition.get(DEFINITION) ?? [])[0];
	assert.ok(
		moved.message.includes(ORPHANED.Path),
		`the real path was lost when the diagnostic moved: "${moved.message}"`,
	);
	assert.ok(moved.message.includes(ORPHANED.Key));

	const lines: string[] = [];
	const nowhere = await planFor(
		{ outcome: "findings", findings: [ORPHANED] },
		undefined,
		stats({}),
		(line) => lines.push(line),
	);
	assert.equal(nowhere.size, 0, "a finding was attached with nowhere to put it");
	assert.equal(lines.length, 1);
	assert.ok(lines[0].includes(ORPHANED.Key));
	assert.ok(lines[0].includes(ORPHANED.Path));
});

// ---------------------------------------------------------------------------
// AC-4: an empty panel never means both "clean" and "never asked"
// ---------------------------------------------------------------------------

test("an unchecked workbench says so, and a later failure does not bring that back", async () => {
	const shown = panel();
	const run = new CheckDiagnostics(
		deps({ apply: shown.apply, statKind: stats({ [DEFINITION]: "file" }) }),
	);
	const uncertain = ENGLISH("diagnostics.check.uncertain", {
		workbench: "Bench",
	});

	await run.markPending(ROOT, "Bench");
	assert.deepEqual(messagesAt(shown.collection, DEFINITION), [uncertain]);
	assert.deepEqual(
		(shown.collection.get(DEFINITION) ?? []).map((entry) => entry.source),
		[SOURCE],
	);

	await run.applyResult(ROOT, "Bench", answered(CLEAN));
	assert.deepEqual(
		allMessages(shown.collection),
		[],
		"the placeholder outlived the run that answered",
	);

	const callsBefore = shown.calls.length;
	await run.applyResult(ROOT, "Bench", {
		kind: "refused",
		refusal: "workbench.not-found",
	});
	assert.equal(
		shown.calls.length,
		callsBefore,
		"a failed re-check wrote to the panel",
	);
	assert.deepEqual(
		allMessages(shown.collection),
		[],
		"a failed re-check reintroduced the placeholder",
	);

	// The same guarantee from the other side: nothing asks the panel to say
	// "unknown" again once the workbench has been confirmed once.
	await run.markPending(ROOT, "Bench");
	assert.deepEqual(allMessages(shown.collection), []);
});

// ---------------------------------------------------------------------------
// AC-5: one sweep at a time per workbench, and one rerun behind it
// ---------------------------------------------------------------------------

/** A spawner whose every call waits until the test hands it its answer. */
function heldSpawner(): {
	readonly spawner: Spawner;
	readonly argvs: string[][];
	release: () => void;
} {
	const argvs: string[][] = [];
	const waiting: ((outcome: SpawnOutcome) => void)[] = [];
	return {
		argvs,
		spawner: async (_exe, argv) => {
			argvs.push([...argv]);
			return new Promise<SpawnOutcome>((resolve) => {
				waiting.push(resolve);
			});
		},
		release: () => {
			const pending = waiting.splice(0, waiting.length);
			for (const resolve of pending) {
				resolve({ code: 0, stdout: JSON.stringify(CLEAN), stderr: "" });
			}
		},
	};
}

/** Lets every already-resolved promise run before the assertion reads state. */
async function settle(): Promise<void> {
	for (let turn = 0; turn < 8; turn += 1) {
		await Promise.resolve();
	}
}

test("triggers arriving during a sweep collapse into one rerun, per workbench", async () => {
	const held = heldSpawner();
	const shown = panel();
	const run = new CheckDiagnostics(
		deps({ apply: shown.apply, spawner: held.spawner }),
	);

	void run.runFor(ROOT, "Bench");
	void run.runFor(ROOT, "Bench");
	void run.runFor(ROOT, "Bench");
	await settle();
	// The spawner's own calls are what is counted. A count of apply calls
	// would move for reasons of its own, since a coalesced run can write the
	// panel more or fewer times than it spawned.
	assert.equal(held.argvs.length, 1, "three triggers spawned more than one sweep");

	// A different workbench is not held up by the first one's run.
	void run.runFor(OTHER_ROOT, "Other");
	await settle();
	assert.equal(held.argvs.length, 2);
	assert.ok(held.argvs[1].includes(OTHER_ROOT));

	held.release();
	await settle();
	assert.equal(
		held.argvs.length,
		3,
		"the triggers that arrived during the sweep earned no rerun",
	);
	assert.ok(held.argvs[2].includes(ROOT));

	held.release();
	await settle();
	assert.equal(
		held.argvs.length,
		3,
		"the rerun earned a rerun of its own with nothing to answer",
	);
});

// ---------------------------------------------------------------------------
// AC-6: the check runs on a checkpoint, never on a keystroke
// ---------------------------------------------------------------------------

test("nothing wires the check to a keystroke or to editor focus", () => {
	const listeners = [
		"onDidChangeTextDocument",
		"onDidChangeActiveTextEditor",
		"onDidChangeTextEditorSelection",
	];
	for (const file of ["diagnostics.ts", "extension.ts"]) {
		const body = readFileSync(join(extensionRoot, "src", file), "utf8");
		for (const listener of listeners) {
			assert.ok(
				!body.includes(listener),
				`src/${file} listens on ${listener}, so the check runs on typing`,
			);
		}
	}
});

test("the check is reached from the two call sites the design names", () => {
	// The structural claim is that runFor is reached from activation and from
	// the checkpoint loop's refresh callback and from nowhere else, and a
	// reader settles that by reading the two sites. What this pins is the
	// count, so a third site cannot appear without somebody deciding to change
	// this number and saying why.
	const body = readFileSync(join(extensionRoot, "src", "extension.ts"), "utf8");
	const occurrences = (needle: string): number =>
		body.split(needle).length - 1;
	assert.equal(occurrences("diagnostics.runFor("), 2);
	assert.equal(occurrences("diagnostics.markPending("), 1);
});
