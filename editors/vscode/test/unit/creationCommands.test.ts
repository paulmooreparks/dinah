// What the two creation commands send, and when they send nothing at all.
//
// The argv assertions carry the weight here for the same reason they do in
// cardCommands.test.ts: a verb handed the wrong positional argument fails
// against somebody's real workbench and nowhere else. Two of them are peculiar
// to this pair and are pinned by name below. `add` takes the title as a word
// and the column behind a flag, so a title that reached --column would file
// nothing and refuse nothing. `attach` reads two positional words with no
// shift-on-omission, so the workbench-level attach has to send an explicit
// empty first word rather than leaving it off.
//
// The cancellation cases carry the rest. Each prompt this module opens can be
// dismissed, and a dismissal has to reach the process as no spawn rather than
// as a spawn with an empty argument.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { CommandHost } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import {
	ATTACH_DIALOG_OPTIONS,
	attachFile,
	contextForAttach,
	contextForColumn,
	pickedFilePath,
} from "../../src/creationCommands";
import { newCard } from "../../src/creationCommands";
import type { RootRow, TreeElement } from "../../src/tree";
import type { CardView, ColumnView, TreeNode } from "../../src/wire";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/**
 * The workbench root, and the workspace folder the row was produced for.
 *
 * These are two different paths on purpose. A folder resolves to a workbench
 * at or above itself, so the two coincide on most benches and diverge on the
 * nested case dinah-241 named, and a context that carried one where it means
 * the other would be invisible in a fixture that gave them a single value.
 * The argv is pinned to ROOT and the checkpoint is asked for FOLDER, so a
 * swap of the two reddens both.
 */
const ROOT = "C:\\work\\bench";
const FOLDER = "C:\\work\\bench\\editors";

/**
 * The binary the call is asked to run, spelled apart from both paths above.
 *
 * A context carries three values the spawn is decided by, and the executable
 * is one of them. It is a third distinct string here so that handing the
 * workbench root, or the workspace folder, where the binary path belongs
 * reddens rather than reading as the same value twice.
 */
const EXE = "C:\\tools\\dinah.exe";

/**
 * The card's ref as each of the two joined answers carries it.
 *
 * The listing and the tree name the same card, so on a real read these hold
 * one value. They are spelled apart here because contextForAttach reads them
 * in an order, and a fixture that agreed with itself would let that order be
 * reversed with nothing to say so (dinah-331 AC-6).
 */
const LISTING_REF = "tr-4";
const TREE_REF = "tr-9";

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

function refused(refusal: string, detail?: string): SpawnOutcome {
	return { code: 2, stdout: JSON.stringify({ refusal, detail }), stderr: "" };
}

/**
 * One spawn, as the double saw it: every parameter the Spawner contract
 * carries rather than only the one the argv assertions happen to want.
 *
 * A double that drops a parameter cannot arm a check on it, however carefully
 * the fixture spells its values apart, because the blindness sits in the
 * observer rather than in the input. Both commands here decide all three, so
 * the double records all three and each is asserted at least once. The form
 * is workbench.test.ts's own spawner stub, which already records the working
 * directory beside the argv.
 */
interface Invocation {
	readonly exe: string;
	readonly argv: string[];
	readonly cwd?: string;
}

interface Recorder {
	readonly spawner: Spawner;
	readonly host: CommandHost;
	/** Every spawn the spawner was handed, in call order. */
	readonly calls: Invocation[];
	readonly errors: string[];
	readonly checkpoints: string[];
	/**
	 * Every host method neither command is supposed to touch, named as it was
	 * called.
	 *
	 * A double that answers a method with undefined and records nothing cannot
	 * notice the command starting to call it, which is the same blindness the
	 * spawner had: the observer drops the value rather than the fixture hiding
	 * it. Decision 5 is the reason it matters here. Neither command reveals,
	 * selects or opens what it created, and nothing else in this file would go
	 * red if one of them began to.
	 *
	 * log is left out on purpose. A diagnostic line is not something the
	 * operator sees, so it is not part of what Decision 5 rules on.
	 */
	readonly unused: string[];
	/** Every prompt the input box was opened with, in call order. */
	readonly prompts: string[];
	/** What each successive input box answers, consumed from the front. */
	typed: (string | undefined)[];
	/** What the next spawn answers, whatever the argv says. */
	answer: SpawnOutcome;
}

/**
 * A recorder whose input box answers a queue rather than one value.
 *
 * cardCommands.test.ts's own recorder answers every prompt with one stored
 * string, which is enough for a command that opens one box. attachFile opens a
 * box after the file pick, and newCard opens one before the spawn, so this one
 * takes the answers in the order the command will ask for them and hands back
 * undefined once the queue is empty.
 */
function recorder(typed: (string | undefined)[] = []): Recorder {
	const calls: Invocation[] = [];
	const errors: string[] = [];
	const checkpoints: string[] = [];
	const prompts: string[] = [];
	const unused: string[] = [];
	const state: Recorder = {
		calls,
		errors,
		checkpoints,
		prompts,
		unused,
		typed: [...typed],
		answer: ok({}),
		spawner: async (exe, argv, options) => {
			calls.push({ exe, argv: [...argv], cwd: options.cwd });
			return state.answer;
		},
		host: {
			showError: (message) => {
				errors.push(message);
			},
			// Neither creation command reports anything on success, copies
			// anything, or opens anything, so each of these answers nothing and
			// says it was reached. dinah-337 put the first two on the host for
			// Copy Reference, which is a card-row act.
			showInfo: () => {
				unused.push("showInfo");
			},
			copyToClipboard: async () => {
				unused.push("copyToClipboard");
			},
			pick: async () => {
				unused.push("pick");
				return undefined;
			},
			input: async (prompt) => {
				prompts.push(prompt);
				return state.typed.shift();
			},
			openDocument: async () => {
				unused.push("openDocument");
			},
			openFile: async () => {
				unused.push("openFile");
			},
			// attachFile takes its picker as a parameter rather than off the
			// host, so a reach for this field is a real change of shape and
			// this is what notices it.
			pickFile: async () => {
				unused.push("pickFile");
				return undefined;
			},
			checkpoint: async (folder) => {
				checkpoints.push(folder);
			},
			log: () => undefined,
		},
	};
	return state;
}

function rowFixture(over: Partial<RootRow> = {}): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: FOLDER,
		folderName: "bench",
		description: "",
		sole: false,
		data: {
			path: ROOT,
			title: "Trees",
			columns: new Map(),
			cards: new Map(),
		},
		...over,
	};
}

function columnView(over: Partial<ColumnView> = {}): ColumnView {
	return {
		id: "0a1b2c3d4e5f",
		slug: "intake",
		title: "Intake",
		kind: "work",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: true,
		count: 0,
		...over,
	};
}

/**
 * The tree's own name for the column, spelled apart from the listing's slug.
 *
 * The join keys a listing column by columnRef and a tree node names the same
 * column by that same value, so on a real read these two agree. They are
 * given different spellings here for the reason the two card refs are: the
 * context is composed from the listing's answer, and a fixture that let both
 * sources say "intake" would let the composition read the node instead with
 * nothing to notice.
 */
const NODE_COLUMN = "intake-from-the-tree";

function columnNode(): TreeNode {
	return { kind: "group", axis: "column", value: NODE_COLUMN, count: 0 };
}

function columnRow(over: Partial<ColumnView> | undefined, row = rowFixture()): TreeElement {
	return {
		kind: "column",
		row,
		node: columnNode(),
		view: over === undefined ? undefined : columnView(over),
	};
}

function cardRow(view: Partial<CardView> | undefined, node: Partial<TreeNode> = {}): TreeElement {
	return {
		kind: "card",
		row: rowFixture(),
		node: { kind: "card", id: "aaa", ref: TREE_REF, title: "Draw the guides", count: 1, ...node },
		view: view === undefined ? undefined : { id: "aaa", ref: LISTING_REF, ...view },
	};
}

/** The four elements that carry no column this act can be aimed at. */
function notAColumn(): [string, TreeElement | undefined][] {
	return [
		["no element at all, which is what the Command Palette passes", undefined],
		["a workbench root row", { kind: "root", row: rowFixture() }],
		["a card row", cardRow({})],
		["a column the status/tree join missed", columnRow(undefined)],
	];
}

// ---------------------------------------------------------------------------
// AC-4: which rows New Card can be aimed at
// ---------------------------------------------------------------------------

test("New Card resolves a context from a column row and from nothing else", () => {
	// dinah-331 AC-4. The undefined element is the one this list exists for:
	// dinah-342 was filed because every tree command threw on the argument the
	// Command Palette does not pass, and a context builder that reads a field
	// off it before checking brings that back.
	for (const [name, element] of notAColumn()) {
		const r = recorder();
		assert.equal(
			contextForColumn(element, EXE, r.host, r.spawner),
			undefined,
			name,
		);
	}
});

test("a column row whose workbench path is missing names no context either", () => {
	// The path pins the call to a workbench. Without one the spawn would run
	// against whatever directory the editor happened to be standing in, which
	// on this product is a different operator's live board.
	const r = recorder();
	const row = rowFixture({ data: undefined });
	assert.equal(
		contextForColumn(columnRow({}, row), EXE, r.host, r.spawner),
		undefined,
	);
});

test("a column row carries its own ref and title into the context", () => {
	// dinah-331 AC-4's positive half. The ref is the slug where a slug was
	// published, which is what columnRef composes and what `--column` takes.
	const r = recorder();
	const context = contextForColumn(columnRow({}), EXE, r.host, r.spawner);
	assert.notEqual(context, undefined);
	assert.equal(context?.column, "intake");
	assert.equal(context?.label, "Intake");
	assert.equal(context?.root, ROOT);
	assert.equal(context?.folder, FOLDER);
});

test("a column published with no slug is named by its id", () => {
	const r = recorder();
	const context = contextForColumn(
		columnRow({ slug: undefined }),
		EXE,
		r.host,
		r.spawner,
	);
	assert.equal(context?.column, "0a1b2c3d4e5f");
});

// ---------------------------------------------------------------------------
// AC-5: what New Card sends, and when it sends nothing
// ---------------------------------------------------------------------------

/** The context every newCard test drives, over the recorder given. */
function columnContext(r: Recorder) {
	const context = contextForColumn(columnRow({}), EXE, r.host, r.spawner);
	assert.notEqual(context, undefined);
	return context as NonNullable<typeof context>;
}

test("a dismissed title prompt files nothing and says nothing", async () => {
	// dinah-331 AC-5. Escape is a cancellation and not an empty title, so
	// nothing is spawned, nothing is reported, and no checkpoint runs: the
	// board did not move, so there is nothing to repaint.
	const r = recorder([undefined]);
	assert.equal(await newCard(columnContext(r)), undefined);
	assert.deepEqual(r.calls, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, []);
});

test("a title of nothing but spaces is treated as a dismissal", async () => {
	// `add` refuses an empty title itself, so sending one would trade a quiet
	// no-op for a refusal notification the operator did not ask for.
	const r = recorder(["   "]);
	assert.equal(await newCard(columnContext(r)), undefined);
	assert.deepEqual(r.calls, []);
	assert.deepEqual(r.checkpoints, []);
});

test("a title files one card, trimmed, with the column behind its flag", async () => {
	// dinah-331 AC-5's argv. The title is a positional word and the column
	// rides behind --column; the two swapped would file a card titled after
	// the column into a column named after the title, and dinah would refuse
	// the unknown column rather than say what happened.
	const r = recorder(["  Fix the thing  "]);
	await newCard(columnContext(r));
	assert.equal(r.calls.length, 1);
	assert.deepEqual(r.calls[0].argv, [
		// runDinah composes --json itself, so the machine surface is pinned
		// here too; the spec's own argv list names the tail pinnedArgv builds.
		"--json",
		"--workbench",
		ROOT,
		"add",
		"Fix the thing",
		"--column",
		"intake",
	]);
	assert.deepEqual(r.checkpoints, [FOLDER]);
	assert.deepEqual(r.errors, []);
});

test("the filing runs the binary it was handed, in the workbench it is pinned to", async () => {
	// dinah-331 AC-5. The argv is one of three things this call decides, and
	// the executable and the working directory are the other two. They are
	// asserted here because the recorder now sees them: a double that took
	// only the argv left both free to be swapped for any other string the
	// context carries, and EXE, ROOT and FOLDER are three distinct values so
	// that any such swap reddens this row.
	const r = recorder(["Fix the thing"]);
	await newCard(columnContext(r));
	assert.equal(r.calls.length, 1);
	assert.equal(r.calls[0].exe, EXE);
	assert.equal(r.calls[0].cwd, ROOT);
});

test("the title prompt names the column the card is being filed into", async () => {
	const r = recorder(["Fix the thing"]);
	await newCard(columnContext(r));
	assert.equal(r.prompts.length, 1);
	assert.ok(
		r.prompts[0].includes("Intake"),
		`the prompt does not name the column: ${r.prompts[0]}`,
	);
});

test("a refused filing is reported and the row is repainted anyway", async () => {
	// dinah-331 AC-5's refusal half, and Decision 2's self-heal. The menu
	// gates on a capacity read from the last checkpoint, so a column that
	// filled in the meantime refuses here; the checkpoint that follows is what
	// stops the row going on claiming it is open.
	//
	// The refusal token and its detail are spelled as `dinah --json add`
	// actually emits them against a column at its wip_limit, which is
	// "at-capacity" carrying no layer prefix and the column's own ref as the
	// detail. refusalMessage joins the two with a colon, so what reaches the
	// notification is "at-capacity: doing".
	const r = recorder(["Fix the thing"]);
	r.answer = refused("at-capacity", "doing");
	await newCard(columnContext(r));
	assert.equal(r.errors.length, 1);
	assert.equal(r.errors[0], "at-capacity: doing");
	assert.ok(r.errors[0].includes("at-capacity"));
	assert.ok(r.errors[0].includes("doing"));
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

// ---------------------------------------------------------------------------
// AC-6: which rows Attach File can be aimed at
// ---------------------------------------------------------------------------

test("Attach File names no context for a row that receives no attachment", () => {
	// dinah-331 AC-6. A dead-end row resolved to no workbench at all, and the
	// other three are rows whose own identifier never arrived.
	const cases: [string, TreeElement | undefined][] = [
		["no element at all, which is what the Command Palette passes", undefined],
		[
			"a dead-end row, which resolved to no workbench",
			// The candidate path is set so that the path guard on the next two
			// lines of contextForAttach cannot be what refuses this row. A real
			// dead-end row carries neither a data path nor a candidate one, so
			// this fixture is impossible on purpose: it leaves the rowKind check
			// as the only thing that can answer, which is the check this row is
			// named for.
			{
				kind: "root",
				row: rowFixture({
					rowKind: "deadEnd",
					data: undefined,
					candidate: { title: "Other", path: "C:\\work\\other" },
				}),
			},
		],
		[
			"a root row carrying neither a resolved path nor a candidate one",
			{ kind: "root", row: rowFixture({ data: undefined }) },
		],
		["a column the status/tree join missed", columnRow(undefined)],
		["a card row whose ref never arrived", cardRow(undefined, { ref: undefined })],
	];
	const r = recorder();
	for (const [name, element] of cases) {
		assert.equal(contextForAttach(element, EXE, r.host, r.spawner), undefined, name);
	}
});

test("Attach File aims at each of the three levels the format carries one on", () => {
	// dinah-331 AC-6's positive half. The workbench's own ref is the empty
	// string, which is the convention `attach` reads as the workbench itself.
	const r = recorder();
	const cases: [string, TreeElement, string][] = [
		["the workbench root", { kind: "root", row: rowFixture() }, ""],
		["a column", columnRow({}), "intake"],
		["a card", cardRow({}), LISTING_REF],
	];
	for (const [name, element, ref] of cases) {
		const context = contextForAttach(element, EXE, r.host, r.spawner);
		assert.notEqual(context, undefined, name);
		assert.equal(context?.ref, ref, name);
		assert.equal(context?.root, ROOT, name);
	}
});

test("a card row falls back to the tree's own ref when the listing missed the card", () => {
	// The two answers are joined per checkpoint and either can miss, so the
	// card's ref is read from the view first and from the tree node behind it.
	// This row drives the fallback: no view at all, so only the node can answer.
	const r = recorder();
	const context = contextForAttach(cardRow(undefined), EXE, r.host, r.spawner);
	assert.equal(context?.ref, TREE_REF);
});

test("a card row prefers the listing's ref to the tree's own when the two disagree", () => {
	// The precedence itself, which the fallback row above cannot ask: it is
	// silent on which source wins because it offers only one. Here both
	// sources answer and they answer differently, so reversing the two
	// operands in contextForAttach reddens this row and nothing else.
	const r = recorder();
	const element = cardRow({ ref: LISTING_REF }, { ref: TREE_REF });
	assert.notEqual(LISTING_REF, TREE_REF, "the fixture has to disagree with itself here");
	const context = contextForAttach(element, EXE, r.host, r.spawner);
	assert.equal(context?.ref, LISTING_REF);
});

test("an expanded candidate attaches to the workbench it resolved to, not to the path it was offered under", () => {
	// A candidate that has been expanded carries both fields (RootRow.data is
	// "set on a resolved row and on a candidate that has been expanded"), and
	// the two name different directories whenever discovery climbed. The
	// resolved path is the workbench, so it is the one the call is pinned to.
	const r = recorder();
	const row = rowFixture({
		rowKind: "workbenchCandidate",
		candidate: { path: "C:\\work", title: "Work" },
	});
	const context = contextForAttach({ kind: "root", row }, EXE, r.host, r.spawner);
	assert.equal(context?.root, ROOT);
});

test("a candidate row that has not been expanded attaches to its candidate path", () => {
	const r = recorder();
	const row = rowFixture({
		rowKind: "workbenchCandidate",
		data: undefined,
		candidate: { path: "C:\\work\\other", title: "Other" },
	});
	const context = contextForAttach({ kind: "root", row }, EXE, r.host, r.spawner);
	assert.equal(context?.root, "C:\\work\\other");
	assert.equal(context?.ref, "");
});

// ---------------------------------------------------------------------------
// AC-7 and AC-8: what Attach File sends, and when it sends nothing
// ---------------------------------------------------------------------------

/** The context every attachFile test drives, over the recorder and row given. */
function attachContext(r: Recorder, element: TreeElement = cardRow({})) {
	const context = contextForAttach(element, EXE, r.host, r.spawner);
	assert.notEqual(context, undefined);
	return context as NonNullable<typeof context>;
}

test("a dismissed file picker attaches nothing and never asks for a description", async () => {
	// dinah-331 AC-7. File first and description second (Decision 4) is what
	// makes this possible: an operator who cancels the pick is never left
	// having typed a description for an attachment that will not happen.
	const r = recorder(["a screenshot"]);
	assert.equal(await attachFile(attachContext(r), async () => undefined), undefined);
	assert.deepEqual(r.calls, []);
	assert.deepEqual(r.prompts, []);
	assert.deepEqual(r.checkpoints, []);
	assert.deepEqual(r.errors, []);
});

test("a dismissed description prompt attaches nothing", async () => {
	// Escape at the second step cancels the whole act, which is the one thing
	// that separates it from the empty answer the next test pins.
	const r = recorder([undefined]);
	assert.equal(
		await attachFile(attachContext(r), async () => "/tmp/x.png"),
		undefined,
	);
	assert.deepEqual(r.calls, []);
	assert.equal(r.prompts.length, 1);
	assert.deepEqual(r.checkpoints, []);
});

test("an empty description is an answer, and the attach goes through without the flag", async () => {
	// dinah-331 AC-7. Empty and dismissed arrive as "" and undefined, and
	// reading them as the same thing would make "no description" impossible to
	// say. The flag is absent rather than empty, because `--description=` would
	// be a description of nothing rather than none.
	const r = recorder([""]);
	await attachFile(attachContext(r), async () => "/tmp/x.png");
	assert.equal(r.calls.length, 1);
	assert.deepEqual(r.calls[0].argv, [
		"--json",
		"--workbench",
		ROOT,
		"attach",
		"tr-4",
		"/tmp/x.png",
	]);
	assert.ok(
		!r.calls[0].argv.some((word) => word.includes("--description")),
		`a description flag was sent for an empty description: ${r.calls[0].argv.join(" ")}`,
	);
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

test("a typed description rides in one --description word, trimmed", async () => {
	// dinah-331 AC-7. One word rather than two, because a description holding
	// a space would otherwise arrive as a description and a stray positional.
	const r = recorder(["  a screenshot  "]);
	await attachFile(attachContext(r), async () => "/tmp/x.png");
	assert.deepEqual(r.calls[0].argv, [
		"--json",
		"--workbench",
		ROOT,
		"attach",
		"tr-4",
		"/tmp/x.png",
		"--description=a screenshot",
	]);
	assert.equal(r.calls[0].argv[r.calls[0].argv.length - 1], "--description=a screenshot");
});

test("the attach runs the binary it was handed, in the workbench it is pinned to", async () => {
	// dinah-331 AC-7, on the same terms as the filing above. Both commands
	// compose these two values from the context rather than from the argv, so
	// each needs its own row; one assertion covering only newCard would leave
	// attachFile's pair unexercised.
	const r = recorder([""]);
	await attachFile(attachContext(r), async () => "/tmp/x.png");
	assert.equal(r.calls.length, 1);
	assert.equal(r.calls[0].exe, EXE);
	assert.equal(r.calls[0].cwd, ROOT);
});

test("the workbench's own attach sends the empty ref as a word rather than omitting it", async () => {
	// dinah-331 AC-8, and the one argv on this card that a reasonable reading
	// gets wrong. `attach` reads its ref and its file positionally with no
	// shift-on-omission, so dropping the empty ref would hand the file path to
	// the ref and leave the file unnamed. The spawner takes an array with no
	// shell in between, so the empty element survives as a real argument.
	const r = recorder([""]);
	const context = attachContext(r, { kind: "root", row: rowFixture() });
	assert.equal(context.ref, "");
	await attachFile(context, async () => "/tmp/x.png");
	assert.deepEqual(r.calls[0].argv, ["--json", "--workbench", ROOT, "attach", "", "/tmp/x.png"]);
	// Spelled again as a position, because the deep-equal above would go on
	// passing if the empty word vanished and the file path shifted into it
	// only on some other fixture.
	const verb = r.calls[0].argv.indexOf("attach");
	assert.equal(r.calls[0].argv[verb + 1], "");
	assert.equal(r.calls[0].argv[verb + 2], "/tmp/x.png");
	// The root branch composes its folder and its root separately, as the
	// column and card branches do, and this is the only test that drives it.
	// Until dinah-331's fourth round the test read the argv alone, so the
	// branch could hand the workbench root where the workspace folder belongs
	// and repaint the wrong row with nothing to say so.
	assert.equal(r.calls[0].cwd, ROOT);
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

test("a refused attach is reported and the row is repainted anyway", async () => {
	// Decision 3's race. Nothing the sidebar can read says another actor holds
	// the entity's lock, so the menu offers the act and this is what the
	// operator sees when it loses.
	const r = recorder([""]);
	r.answer = refused("dinah.locked", "bob");
	await attachFile(attachContext(r), async () => "/tmp/x.png");
	assert.equal(r.errors.length, 1);
	assert.ok(r.errors[0].includes("dinah.locked"));
	assert.ok(r.errors[0].includes("bob"));
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

// ---------------------------------------------------------------------------
// Decision 5: what neither command does once it has succeeded
// ---------------------------------------------------------------------------

test("neither creation command reveals, selects or opens what it made", async () => {
	// Decision 5. Both acts end at the checkpoint and let the next tree read
	// repaint the row, on the same terms Claim, Move, Release, Block and
	// Unblock already follow. An operator who wants to see the new card clicks
	// it.
	//
	// This is the one row that can fail if that changes. Every other test here
	// asserts what the commands do send; the host double used to answer the
	// rest of its methods with undefined and record nothing, so a command that
	// began opening its own creation would have passed the whole file.
	const filing = recorder(["Fix the thing"]);
	await newCard(columnContext(filing));
	assert.deepEqual(filing.checkpoints, [FOLDER]);
	assert.deepEqual(filing.unused, []);
	assert.deepEqual(filing.errors, []);

	const attaching = recorder([""]);
	await attachFile(attachContext(attaching), async () => "/tmp/x.png");
	assert.deepEqual(attaching.checkpoints, [FOLDER]);
	// The picker rides in as a parameter, so the host's own field stays
	// untouched; a command reading it instead would name it here.
	assert.deepEqual(attaching.unused, []);
	assert.deepEqual(attaching.errors, []);
});

// ---------------------------------------------------------------------------
// AC-12: the dialog's own settings, and the read of what it answered
// ---------------------------------------------------------------------------

/**
 * The two parts of the file picker that carry a decision are driven here.
 *
 * `pickFile` binds a prompt to the real VS Code window, and nothing in this
 * layer stubs vscode.window, so the call itself is out of reach. The options
 * the call is given and the read of what it answered are not: both live in
 * creationCommands.ts, which imports no vscode, so both are ordinary values a
 * test drives. What remains unprovable anywhere in this repository is whether
 * VS Code honours those options, which is a promise of the editor rather than
 * a property of this code.
 */
const EXTENSION_SOURCE = readFileSync(
	join(__dirname, "..", "..", "..", "src", "extension.ts"),
	"utf8",
).replace(/\s+/g, " ");

test("the attach dialog asks for one file and never for a folder", () => {
	// dinah-331 AC-12. Each option is asserted on its own rather than as one
	// object, so a failure names which setting moved.
	assert.equal(ATTACH_DIALOG_OPTIONS.canSelectMany, false);
	assert.equal(ATTACH_DIALOG_OPTIONS.canSelectFiles, true);
	assert.equal(ATTACH_DIALOG_OPTIONS.canSelectFolders, false);
	assert.equal(ATTACH_DIALOG_OPTIONS.openLabel, "Attach");
});

test("the selection read answers a path only when something was chosen", () => {
	// dinah-331 AC-12's other half, and all three answers the dialog can give.
	// The empty array is the one the showOpenDialog documentation does not
	// promise either way, which is why the read defends against it and why
	// this row exists.
	assert.equal(pickedFilePath(undefined), undefined, "a cancelled dialog");
	assert.equal(pickedFilePath([]), undefined, "an empty answer");
	assert.equal(
		pickedFilePath([{ fsPath: "C:\\work\\shot.png" }]),
		"C:\\work\\shot.png",
		"one file",
	);
});

test("the selection read takes the first file when the dialog answers several", () => {
	// canSelectMany is false, so this cannot arise from the dialog above. It
	// is pinned anyway because the read is exported and the option could be
	// flipped without anyone revisiting the read.
	assert.equal(
		pickedFilePath([{ fsPath: "C:\\work\\first.png" }, { fsPath: "C:\\work\\second.png" }]),
		"C:\\work\\first.png",
	);
});

test("extension.ts opens the dialog through that seam rather than around it", () => {
	// The one thing left that no value can prove: that the field the editor
	// actually calls is wired to the two names above. This reads source text,
	// which is weak, so it reads only the two identifiers rather than the
	// shape of the code around them, and a reformat cannot break it.
	// One spelling per name, because only one of them can exist. The options
	// are the dialog's argument and the read takes its answer, so the pair of
	// alternatives this loop used to accept included two shapes that could
	// never compile: calling the options object, and passing the read where
	// options are expected.
	const seams: [string, RegExp][] = [
		["ATTACH_DIALOG_OPTIONS", /showOpenDialog\(\s*ATTACH_DIALOG_OPTIONS\s*\)/],
		["pickedFilePath", /pickedFilePath\(\s*await vscode\.window\.showOpenDialog/],
	];
	for (const [name, pattern] of seams) {
		assert.match(
			EXTENSION_SOURCE,
			pattern,
			`extension.ts no longer reaches the file picker through ${name}`,
		);
	}
	assert.ok(
		!EXTENSION_SOURCE.includes("canSelectFolders"),
		"extension.ts spells the dialog options itself again, so the tested copy is not the one in use",
	);
});

test("both creation commands are registered against the identifiers identity.ts names", () => {
	// The registration is the other half extension.ts owns. A command declared
	// in the manifest and registered nowhere shows up as "command not found"
	// on the first click and in no test, which is what this notices.
	//
	// dinah-369 routed every registration through activate()'s own register()
	// helper, so the spelling this reads is that helper's rather than
	// vscode.commands.registerCommand's. The same card also made the whole
	// roster fail activation when a registration is dropped, which is a
	// stronger guard than this one and covers all fifteen commands rather than
	// these two.
	for (const constant of ["COMMAND_NEW_CARD", "COMMAND_ATTACH_FILE"]) {
		assert.ok(
			EXTENSION_SOURCE.includes(`register(${constant},`),
			`${constant} is declared in the manifest and registered nowhere`,
		);
	}
	// Each handler resolves a context before it acts, which is what keeps a
	// palette invocation carrying no row from throwing (dinah-342). The column
	// resolver is named here under the alias extension.ts imports it as, because
	// dinah-332's columnCommands exports a contextForColumn of its own and the
	// two cannot both be called by their module's name in one file.
	assert.ok(EXTENSION_SOURCE.includes("contextForNewCard("));
	assert.ok(EXTENSION_SOURCE.includes("contextForAttach("));
});
