// What each command sends, and what it says when dinah refuses.
//
// The argv assertions are the point. A verb that takes a card reference and
// gets a title, or takes a column identifier and gets the reference a person
// types, fails at run time against somebody's real board and nowhere else,
// and dinah-287 renamed exactly the field the move verb reads.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { ENGLISH } from "../../src/l10n";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import type { CommandContext, CommandHost, PickItem } from "../../src/cardCommands";
import {
	askBlockReason,
	askDeleteAttachmentConfirmation,
	askMoveDestination,
	blockCardWith,
	claimCard,
	contextFor,
	contextForAttachment,
	contextForAttachmentOpen,
	copiedRefMessage,
	copyCardRefs,
	deleteAttachmentAt,
	moveCardTo,
	movePick,
	openAttachment,
	openCard,
	openInstructions,
	orderLegalMoves,
	refusalMessage,
	releaseCard,
	unblockCard,
} from "../../src/cardCommands";
import type { RootRow, TreeElement } from "../../src/tree";
import type { LegalMove } from "../../src/wire";

interface Recorder {
	readonly context: CommandContext;
	readonly calls: string[][];
	readonly errors: string[];
	readonly checkpoints: string[];
	/** The messages the host was asked to show as plain information. */
	readonly infos: string[];
	/** The messages the host was asked to show as warnings, in order. */
	readonly warnings: string[];
	/** Every line the host was asked to write to the output channel. */
	readonly channelLines: string[];
	/** How many times the host was asked to reveal the output channel. */
	revealed: number;
	/** Every string the host was asked to put on the clipboard, in order. */
	readonly copied: string[];
	readonly opened: string[];
	/** The files handed to the host's file opener, in call order. */
	readonly files: string[];
	/** Every served-text tab the host was asked to open, in call order. */
	readonly served: { kind: string; root: string; ref: string; title: string }[];
	readonly logged: string[];
	/** The host the recorder watches, which the attachment handler reaches directly. */
	readonly host: CommandHost;
	/** What the next spawn answers, by which verb the argv names. */
	answers: Record<string, SpawnOutcome>;
	/** What the quick-pick returns, if it is opened. */
	picked?: PickItem;
	/** The items the quick-pick was offered. */
	offered: PickItem[];
	/** What the input box returns, if it is opened. */
	typed?: string;
	/** The two arguments every confirmDestructive call was made with, in order. */
	readonly confirmations: { message: string; label: string }[];
	/** What the confirmation answers, which every test that reaches one sets. */
	confirmed: boolean;
}

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

function refused(refusal: string, detail?: string): SpawnOutcome {
	return { code: 2, stdout: JSON.stringify({ refusal, detail }), stderr: "" };
}

function recorder(answers: Record<string, SpawnOutcome> = {}): Recorder {
	const calls: string[][] = [];
	const errors: string[] = [];
	const checkpoints: string[] = [];
	const infos: string[] = [];
	const warnings: string[] = [];
	const channelLines: string[] = [];
	const copied: string[] = [];
	const opened: string[] = [];
	const files: string[] = [];
	const served: { kind: string; root: string; ref: string; title: string }[] = [];
	const logged: string[] = [];
	const offered: PickItem[] = [];
	const confirmations: { message: string; label: string }[] = [];
	const state = {
		calls,
		errors,
		checkpoints,
		infos,
		warnings,
		channelLines,
		revealed: 0,
		copied,
		opened,
		files,
		served,
		logged,
		offered,
		confirmations,
		// A confirmation nobody set answers no, so a test that forgets to say
		// deletes nothing rather than deleting on a default.
		confirmed: false,
		answers,
	} as Recorder;

	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		for (const [verb, outcome] of Object.entries(state.answers)) {
			if (argv.includes(verb)) {
				return outcome;
			}
		}
		return ok({});
	};

	const host: CommandHost = {
		t: ENGLISH,
		showError: (message) => errors.push(message),
		showInfo: (message) => infos.push(message),
		showWarning: async (message) => {
			warnings.push(message);
			return undefined;
		},
		appendLines: (lines) => {
			channelLines.push(...lines);
		},
		revealOutput: () => {
			state.revealed += 1;
		},
		copyToClipboard: async (text) => {
			copied.push(text);
		},
		pick: async (items) => {
			offered.push(...items);
			return state.picked;
		},
		input: async () => state.typed,
		openDocument: async (path) => {
			opened.push(path);
		},
		openFile: async (path) => {
			files.push(path);
		},
		// No command in this file opens a picker, so this answers nothing.
		// dinah-331 put the field on the host; the creation commands drive it.
		pickFile: async () => undefined,
		openServedText: async (kind, root, ref, title) => {
			served.push({ kind, root, ref, title });
		},
		confirmDestructive: async (message, label) => {
			confirmations.push({ message, label });
			return state.confirmed;
		},
		checkpoint: async (folder) => {
			checkpoints.push(folder);
		},
		log: (line) => logged.push(line),
	};

	(state as { context: CommandContext }).context = {
		spawner,
		exe: "dinah",
		host,
		folder: "C:\\work\\bench",
		root: "C:\\work\\bench",
		ref: "tr-4",
	};
	// The host is built after the state it closes over, so it is attached here
	// rather than in the literal: the attachment handler takes the host alone,
	// with no CommandContext around it.
	(state as { host: CommandHost }).host = host;
	return state;
}

// ---------------------------------------------------------------------------
// dinah-337 AC-3: the reference reaches the clipboard, and nothing else runs
// ---------------------------------------------------------------------------

test("copying a card's reference puts the reference itself on the clipboard", async () => {
	// The reference rather than the title, because the reference is the argument
	// every dinah verb takes and the title is not accepted anywhere. A handler
	// reaching for the wrong field would still copy something, so the assertion
	// names the value rather than counting the calls.
	const r = recorder();
	await copyCardRefs([r.context], r.host);
	assert.deepEqual(r.copied, ["tr-4"]);
});

test("copying a card's reference tells the reader which reference it copied", async () => {
	// A clipboard write leaves no trace on screen, so the message is the only
	// confirmation. It names the reference so that a reader who clicked the
	// wrong row finds out before pasting.
	// The sentence now comes from the run's own summary rather than from the
	// per-row act, because the copy family reports once over a whole selection
	// (dinah-490 D-23). It is still the singular key over one card.
	const r = recorder();
	await copyCardRefs([r.context], r.host);
	const message = copiedRefMessage([r.context], ENGLISH);
	assert.equal(message, ENGLISH("dialog.card.copiedRef", { ref: "tr-4" }));
	assert.ok(
		message.includes("tr-4"),
		`the message did not name the reference: ${message}`,
	);
	assert.deepEqual(r.errors, []);
});

test("copying a card's reference spawns no dinah and runs no checkpoint", async () => {
	// dinah-337 D-2. A copy reads only what the row already holds, so a spawn
	// would cost a process for nothing and a checkpoint would repaint a tree
	// that cannot have moved. Both are asserted empty rather than one of them,
	// because runVerb would produce both together and either alone is a
	// half-finished mistake this catches.
	const r = recorder();
	await copyCardRefs([r.context], r.host);
	assert.deepEqual(r.calls, []);
	assert.deepEqual(r.checkpoints, []);
});

// ---------------------------------------------------------------------------
// AC-11: a refusal is surfaced, and a checkpoint follows it
// ---------------------------------------------------------------------------

test("a claim refused for awaiting-outside names the refusal and its detail", async () => {
	const r = recorder({
		claim: refused(
			"dinah.awaiting-outside",
			"this column waits on somebody outside the workbench",
		),
	});
	await claimCard(r.context);
	assert.equal(r.errors.length, 1);
	assert.ok(r.errors[0].includes("dinah.awaiting-outside"));
	assert.ok(r.errors[0].includes("waits on somebody outside"));
	// Exactly one off-cycle check follows, which is what shows the reader the
	// board as it now stands rather than as they believed it stood.
	assert.deepEqual(r.checkpoints, ["C:\\work\\bench"]);
});

test("a refusal is never swallowed, whichever verb raised it", async () => {
	for (const [verb, run] of [
		["release", releaseCard],
		["unblock", unblockCard],
	] as const) {
		const r = recorder({ [verb]: refused("dinah.not-held") });
		await run(r.context);
		assert.deepEqual(r.errors, ["dinah.not-held"]);
		assert.equal(r.checkpoints.length, 1);
	}
});

test("a successful call still checkpoints, and says nothing", async () => {
	const r = recorder({ claim: ok({ affordances: [] }) });
	await claimCard(r.context);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, ["C:\\work\\bench"]);
});

test("a refusal with no detail is named by itself", () => {
	assert.equal(
		refusalMessage({ kind: "refused", refusal: "dinah.not-held" }),
		"dinah.not-held",
	);
	assert.equal(
		refusalMessage({ kind: "spawn-failed", detail: "ENOENT" }),
		"spawn-failed: ENOENT",
	);
});

test("every verb is pinned to the workbench the card stands in", async () => {
	const r = recorder();
	await claimCard(r.context);
	assert.deepEqual(r.calls[0], [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"claim",
		"tr-4",
	]);
});

// ---------------------------------------------------------------------------
// AC-12: the card's own file, read off the answer rather than composed
// ---------------------------------------------------------------------------

test("opening a card uses the path show reported and not one built from the ref", async () => {
	const r = recorder({ show: ok({ path: "C:\\bench\\cards\\abc\\card.md" }) });
	await openCard(r.context);
	assert.deepEqual(r.opened, ["C:\\bench\\cards\\abc\\card.md"]);
	// The reference is what identifies the card to dinah; it is not a path,
	// and a path built from it would be a second spelling of a layout the
	// binary already owns.
	assert.ok(!r.opened[0].includes("tr-4"));
});

test("a show answer carrying no path opens nothing and says so to the channel", async () => {
	const r = recorder({ show: ok({ body: "" }) });
	await openCard(r.context);
	assert.deepEqual(r.opened, []);
	assert.equal(r.logged.length, 1);
});

test("a refused show reports the refusal rather than opening anything", async () => {
	const r = recorder({ show: refused("dinah.unknown-card") });
	await openCard(r.context);
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.errors, ["dinah.unknown-card"]);
});

// ---------------------------------------------------------------------------
// dinah-270: the served instruction chain, opened as a tab
// ---------------------------------------------------------------------------

test("opening the served instructions names the kind, the workbench and the card", async () => {
	// The handler spawns nothing itself: the content provider fetches when the
	// editor asks it to, so the whole of this command is composing a URI and
	// a title. A spawn here would mean the text was fetched twice.
	const r = recorder();
	await openInstructions(r.context);
	assert.deepEqual(r.served, [
		{
			kind: "instructions",
			root: "C:\\work\\bench",
			ref: "tr-4",
			title: "tr-4 (served instructions)",
		},
	]);
	assert.deepEqual(r.calls, []);
});

test("the tab's title comes from the catalogue and carries the card's own reference", async () => {
	// The title is the only label a reader gets, so it names the card rather
	// than the URI. It is resolved through the localizer here rather than
	// inside the tab machinery, because a later kind of served text resolves
	// its own key and calls the same host method.
	const r = recorder();
	await openInstructions(r.context);
	assert.ok(r.served[0].title.includes(r.context.ref));
	assert.equal(r.served[0].title, ENGLISH("servedText.title.instructions", { ref: "tr-4" }));
});

// ---------------------------------------------------------------------------
// AC-13: the move quick-pick, its order, and the field it passes
// ---------------------------------------------------------------------------

const MIXED_MOVES: LegalMove[] = [
	{ column: "col-back", ref: "intake", title: "Intake", direction: "backward" },
	{ column: "col-doing", ref: "doing", title: "Doing", direction: "forward" },
	{ column: "col-done", ref: "done", title: "Done", direction: "forward" },
];

test("forward destinations lead in their own array order, then the backward one", () => {
	assert.deepEqual(
		orderLegalMoves(MIXED_MOVES).map((move) => move.title),
		["Doing", "Done", "Intake"],
	);
});

test("invoking the second forward entry moves to that entry's own column", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	// The second forward entry is Done, whose column identifier is col-done.
	r.picked = movePick(MIXED_MOVES[2]);
	const column = await askMoveDestination([r.context], r.host);
	assert.equal(typeof column, "string");
	await moveCardTo(r.context, column ?? "");
	const move = r.calls.find((argv) => argv.includes("move"));
	assert.deepEqual(move, [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"move",
		"tr-4",
		"col-done",
	]);
});

test("the destination passed is the Column field and not the Ref or the Title", () => {
	// dinah-287 renamed this field from State, and LegalMove carries three
	// strings any of which looks plausible in a quick-pick handler.
	const pick = movePick(MIXED_MOVES[2]);
	assert.equal(pick.value, "col-done");
	assert.notEqual(pick.value, "done");
	assert.notEqual(pick.value, "Done");
	assert.equal(pick.label, "Done");
});

test("the quick-pick is offered the destinations in the order the ordering gives", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	await askMoveDestination([r.context], r.host);
	assert.deepEqual(
		r.offered.map((item) => item.label),
		["Doing", "Done", "Intake"],
	);
	// A backward destination says so, because moving a card back is a
	// different act from moving it on and the list does not otherwise show it.
	assert.equal(r.offered[2].detail, "backward");
	assert.equal(r.offered[0].detail, undefined);
});

test("dismissing the quick-pick moves nothing", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	r.picked = undefined;
	assert.equal(await askMoveDestination([r.context], r.host), undefined);
	assert.equal(
		r.calls.find((argv) => argv.includes("move")),
		undefined,
	);
	assert.deepEqual(r.checkpoints, []);
});

test("a card with no legal moves says so rather than opening an empty picker", async () => {
	const r = recorder({ instructions: ok({ legal_moves: [] }) });
	await askMoveDestination([r.context], r.host);
	assert.deepEqual(r.offered, []);
	assert.equal(r.errors.length, 1);
	assert.ok(r.errors[0].includes("tr-4"));
});

// ---------------------------------------------------------------------------
// Block, whose reason the verb requires
// ---------------------------------------------------------------------------

test("blocking sends the reason that was typed", async () => {
	const r = recorder();
	r.typed = "  waiting on the printer  ";
	const reason = await askBlockReason([r.context], r.host);
	assert.equal(reason, "waiting on the printer");
	await blockCardWith(r.context, reason ?? "");
	assert.deepEqual(r.calls[0].slice(3), ["block", "tr-4", "waiting on the printer"]);
});

test("an empty reason blocks nothing", async () => {
	// dinah block takes the reason as an argument, and a blank one would
	// record a block nobody can act on.
	for (const typed of [undefined, "", "   "]) {
		const r = recorder();
		r.typed = typed;
		assert.equal(await askBlockReason([r.context], r.host), undefined);
		assert.deepEqual(r.calls, []);
		assert.deepEqual(r.checkpoints, []);
	}
});

// ---------------------------------------------------------------------------
// dinah-335 AC-8: an attachment opens its own file, through openFile and
// nothing else on the host
// ---------------------------------------------------------------------------

/** The smallest row an element can carry: resolved, and carrying no data. */
function rowFixture(): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: "C:\\work\\bench",
		folderName: "bench",
		description: "",
		sole: false,
	};
}

/**
 * An attachment element over that row, with the payload path given
 * explicitly so both spellings of a missing one reach the handler.
 */
function attachmentRow(path: string | undefined): TreeElement {
	return {
		kind: "attachment",
		row: rowFixture(),
		root: "C:\\work\\bench",
		owner: "tr-4",
		view: {
			id: "9a1b2c3d4e5f",
			ordinal: 1,
			ref: "tr-4/attachments/1",
			filename: "screenshot.png",
			provenance: "copy",
			path,
		},
	};
}

test("an attachment with a file opens it through openFile and touches nothing else on the host", async () => {
	const r = recorder();
	const lines: string[] = [];
	const context = contextForAttachmentOpen(
		attachmentRow("C:\\bench\\cards\\tr-4\\attachments\\screenshot.png"),
	);
	assert.notEqual(context, undefined);
	await openAttachment(context ?? { path: "" }, r.host);
	assert.deepEqual(r.files, ["C:\\bench\\cards\\tr-4\\attachments\\screenshot.png"]);
	// No other call the host offers was made: no document forced open, no
	// checkpoint spent, no error surface, no picker, and no channel line
	// through the host, which is why the handler's own sayings go through a
	// callback the host does not hold.
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.checkpoints, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.offered, []);
	assert.deepEqual(r.logged, []);
	assert.deepEqual(lines, []);
});

test("an attachment with no path resolves to no context, so nothing opens", async () => {
	// The channel line moved with dinah-490: a row that yields no context is
	// recorded as a skipped row of the run and reaches the channel from there,
	// with the run's own skip reason, so the resolution is what this asserts
	// and the line is asserted where the run writes it.
	for (const path of [undefined, ""]) {
		const r = recorder();
		assert.equal(contextForAttachmentOpen(attachmentRow(path)), undefined);
		assert.deepEqual(r.files, []);
		assert.deepEqual(r.opened, []);
		assert.deepEqual(r.checkpoints, []);
		assert.deepEqual(r.errors, []);
		assert.deepEqual(r.offered, []);
		assert.deepEqual(r.logged, []);
	}
});

// ---------------------------------------------------------------------------
// contextFor, and the argument the Command Palette does not pass
// ---------------------------------------------------------------------------

/** A host whose calls are recorded nowhere, since contextFor makes none. */
const silentHost: CommandHost = {
	t: ENGLISH,
	showError: () => undefined,
	showInfo: () => undefined,
	showWarning: async () => undefined,
	appendLines: () => undefined,
	revealOutput: () => undefined,
	copyToClipboard: async () => undefined,
	pick: async () => undefined,
	input: async () => undefined,
	openDocument: async () => undefined,
	openFile: async () => undefined,
	pickFile: async () => undefined,
	openServedText: async () => undefined,
	confirmDestructive: async () => false,
	checkpoint: async () => undefined,
	log: () => undefined,
};

function rowFor(path: string | undefined): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: "C:/work",
		folderName: "work",
		description: "",
		sole: true,
		data:
			path === undefined
				? undefined
				: {
						path,
						title: "Work",
						columns: new Map(),
						cards: new Map(),
						holding: [],
					},
	};
}

/** A card row of the shape the tree hands a context-menu invocation. */
function cardElement(): TreeElement {
	return {
		kind: "card",
		row: rowFor("C:/work/board"),
		node: { kind: "card", ref: "dinah-1", title: "A card", count: 1 },
	};
}

test("contextFor answers undefined for the argument the Command Palette passes", () => {
	// The palette invokes a command with no argument at all, and every flow
	// command shares one handler that called this function first. Reading a
	// field off the absent element threw a TypeError before the handler's own
	// missing-row branch could run, so the six commands each reported "Running
	// the contributed command failed" and said nothing a reader could act on.
	assert.equal(contextFor(undefined, "dinah", silentHost), undefined);
});

test("contextFor answers undefined for a row that is not a card", () => {
	// A workbench root, a column and a state group are all rows a keybinding or
	// another extension can aim a card command at, and none of them names a
	// card. The handler logs that and returns, which needs this answer rather
	// than a throw.
	const rows: TreeElement[] = [
		{ kind: "root", row: rowFor("C:/work/board") },
		{
			kind: "column",
			row: rowFor("C:/work/board"),
			node: { kind: "column", id: "spec", title: "Spec", count: 2 },
		},
		{
			kind: "group",
			row: rowFor("C:/work/board"),
			node: { kind: "group", axis: "state", value: "ready", count: 2 },
		},
	];
	for (const row of rows) {
		assert.equal(
			contextFor(row, "dinah", silentHost),
			undefined,
			`a ${row.kind} row composed a context`,
		);
	}
});

test("a row that names no attachment at all resolves to no context", async () => {
	const r = recorder();
	// A note row is a row of the wrong kind, and the same guard has to fire
	// for every kind the tree composes.
	const wrong: TreeElement = {
		kind: "note",
		owner: rowFixture(),
		text: "nothing to open here",
		tooltip: "nothing to open here",
	};
	assert.equal(contextForAttachmentOpen(wrong), undefined);
	assert.deepEqual(r.files, []);
});

test("openAttachment survives the argument the Command Palette does not pass", async () => {
	// The palette invokes a command with no argument at all, and this handler
	// read element.kind before it had established that an element arrived, so
	// the invocation threw a TypeError and the reader saw "Running the
	// contributed command failed" instead of a sentence naming the cause. Two
	// reviewers flagged the shape on this card while dinah-342 was in flight,
	// and the repair belongs here because that card's branch never touched
	// this file. The row command is also hidden from the palette now, which
	// closes the route rather than the hole; both are wanted, because a
	// keybinding and another extension reach the handler past the manifest.
	const r = recorder();
	assert.equal(contextForAttachmentOpen(undefined), undefined);
	assert.deepEqual(r.files, []);
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.checkpoints, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.offered, []);
	assert.deepEqual(r.logged, []);
});

test("contextFor still composes the context a card row names", () => {
	// The two refusals above are satisfied by a function that refuses
	// everything, so this is what keeps them honest: the ordinary invocation
	// from a card's context menu goes on producing the same pinned call.
	const target = contextFor(cardElement(), "dinah", silentHost);
	assert.ok(target, "a card row composed no context");
	assert.equal(target.ref, "dinah-1");
	assert.equal(target.root, "C:/work/board");
	assert.equal(target.folder, "C:/work");
	assert.equal(target.exe, "dinah");
	assert.equal(target.host, silentHost);
});

test("contextFor answers undefined for a card row whose workbench never resolved", () => {
	// A candidate row that has not been expanded carries no data, so there is
	// no path to pin the call to. This is the branch that predates dinah-342
	// and it is asserted here so the reordering above did not remove it.
	const element: TreeElement = {
		kind: "card",
		row: rowFor(undefined),
		node: { kind: "card", ref: "dinah-1", count: 1 },
	};
	assert.equal(contextFor(element, "dinah", silentHost), undefined);
});

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionSource = readFileSync(
	join(__dirname, "..", "..", "..", "src", "extension.ts"),
	"utf8",
);

test("the command handler hands its argument straight on, reading no field", () => {
	// Testing the decision and not the wiring is how the defect survived. The
	// handler reads nothing off the element itself: dinah-490 moved the
	// resolution out of extension.ts entirely, so the one registration loop
	// passes its two parameters to targetsFor and the entry's own invoke calls
	// the contextFor the three tests above hold to the missing-element
	// contract. A regression that read a field first, which is exactly what
	// shipped, would leave those three green, so this reads the one module the
	// unit layer cannot import the way layers.ts and spawn-sites.ts already
	// read src for a single-site invariant.
	assert.ok(
		extensionSource.includes("targetsFor("),
		"extension.ts no longer calls targetsFor, so this check proved nothing",
	);
	const dereferences = extensionSource
		.split(/\r?\n/)
		.filter((line) => /\belement[.?]/.test(line));
	assert.deepEqual(
		dereferences,
		[],
		"extension.ts reads a field off an element binding, which throws when the Command Palette passes none",
	);
});

test("the handler's parameter admits the argument the palette does not pass", () => {
	// The type is half the guard. A handler declared to take a TreeElement
	// tells every later reader that an element always arrives, and the compiler
	// then agrees that reading a field off it is safe. dinah-490 moved the
	// declaration onto the one register helper every command goes through, so
	// the spelling this reads for is the helper's parameter rather than one
	// handler's.
	assert.ok(
		extensionSource.includes("element: TreeElement | undefined,"),
		"the register helper no longer declares its element as possibly absent",
	);
	// The selection is the second half, and it is optional for the same
	// reason: a palette invocation, a keybinding and another extension all
	// reach a command with neither argument.
	assert.ok(
		extensionSource.includes("selection?: readonly TreeElement[],"),
		"the register helper no longer declares the selection as possibly absent",
	);
});

const cardCommandsSource = readFileSync(
	join(__dirname, "..", "..", "..", "src", "cardCommands.ts"),
	"utf8",
);

const DOC_OPEN = "/" + "**";
const DOC_CLOSE = "*" + "/";

/**
 * The doc comment sitting immediately above `export function <name>`, if one
 * is there.
 *
 * The text between the declaration and the comment has to be blank, so a
 * comment separated from the declaration by another comment and a whole
 * function, which is what dinah-393 found, is not that declaration's comment
 * and is not returned.
 */
function docCommentAbove(source: string, name: string): string | undefined {
	const declaration = source.indexOf(`export function ${name}`);
	if (declaration < 0) {
		return undefined;
	}
	const preceding = source.slice(0, declaration).trimEnd();
	if (!preceding.endsWith(DOC_CLOSE)) {
		return undefined;
	}
	const opener = preceding.lastIndexOf(DOC_OPEN);
	if (opener < 0) {
		return undefined;
	}
	return preceding.slice(opener + DOC_OPEN.length, preceding.length - DOC_CLOSE.length);
}

test("each function's doc comment says what that function does", () => {
	// A check that the file has no two comments stacked on one another passes
	// on a file with no comments at all, so this pins what each comment says
	// instead. dinah-335 moved isRow in between contextFor and the comment
	// describing it, leaving that comment claiming the check happens "by isRow
	// above" while sitting above isRow, and nothing red went red.
	const isRowDoc = docCommentAbove(cardCommandsSource, "isRow");
	const contextForDoc = docCommentAbove(cardCommandsSource, "contextFor");
	assert.ok(
		isRowDoc?.includes("The row a command was aimed at"),
		"isRow no longer carries the comment describing the row guard",
	);
	assert.ok(
		contextForDoc?.includes("The workbench a card's row stands in"),
		"contextFor no longer carries the comment describing the workbench pin",
	);
	assert.ok(
		!isRowDoc?.includes("The workbench a card's row stands in"),
		"contextFor's comment has drifted above isRow again",
	);
});

test("the comment that says isRow is above it is in fact below isRow", () => {
	// The self-contradiction is what made the original defect findable, and a
	// claim about position is only true at one position, so the guard checks
	// the position rather than the wording.
	const isRowAt = cardCommandsSource.indexOf("export function isRow");
	const claimAt = /by isRow\r?\n \* above/.exec(cardCommandsSource)?.index ?? -1;
	assert.ok(isRowAt >= 0, "isRow is gone, so this check proved nothing");
	assert.ok(claimAt >= 0, "the claim this guards is gone, so it proved nothing");
	assert.ok(
		claimAt > isRowAt,
		"a comment claims the check happens by isRow above while sitting above isRow",
	);
});

// ---------------------------------------------------------------------------
// dinah-451: deleting an attachment, and the confirmation in front of it
// ---------------------------------------------------------------------------

/**
 * The attachment row the delete tests are aimed at.
 *
 * The second entry of the fixture tree.test.ts uses, chosen because its
 * identifier and its position differ: the reference the row was drawn with
 * ends in 2 and the identifier is 0b2c3d4e5f61, so an argv composed from the
 * wrong one is a visibly different string rather than a coincidence.
 */
function deletableRow(): TreeElement {
	return {
		kind: "attachment",
		row: rowFixture(),
		root: "C:\\work\\bench",
		owner: "tr-4",
		view: {
			id: "0b2c3d4e5f61",
			ordinal: 2,
			ref: "tr-4/attachments/2",
			filename: "spec.pdf",
			provenance: "import",
		},
	};
}

/**
 * Resolves the deletable row, asks once, and deletes if the answer was yes.
 *
 * dinah-490 split the confirmation from the verb, because one confirmation
 * now stands over a whole selection while the delete stays a function of one
 * attachment. This helper is the two halves in the order runBulk runs them,
 * so the assertions below say what they said before the split.
 */
async function deleteConfirmedAttachment(r: Recorder): Promise<unknown> {
	const context = contextForAttachment(
		deletableRow(),
		"dinah",
		r.host,
		r.context.spawner,
	);
	assert.notEqual(context, undefined);
	if (context === undefined) {
		return undefined;
	}
	const confirmed = await askDeleteAttachmentConfirmation([context], r.host);
	if (confirmed === undefined) {
		return undefined;
	}
	return deleteAttachmentAt(context);
}

test("deleting an attachment addresses it by its identifier and not by its position", async () => {
	// dinah-451 AC-5. view.ref ends in the attachment's position within its
	// collection, and a position shifts the moment somebody deletes an earlier
	// attachment, so a row left sitting in a sidebar would address a different
	// file with nothing refusing. The identifier names one attachment for as
	// long as it exists, and --yes is what the tool requires of a caller that
	// has already asked.
	const r = recorder();
	r.confirmed = true;
	await deleteConfirmedAttachment(r);
	assert.equal(r.calls.length, 1);
	assert.deepEqual(r.calls[0], [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"delete",
		"tr-4/attachments/0b2c3d4e5f61",
		"--yes",
	]);
});

test("a declined confirmation deletes nothing and asks dinah nothing", async () => {
	// dinah-451 AC-6. This is what stops the modal from being decoration: a
	// dialog shown after the act, or one whose answer is never read, looks
	// identical in a screenshot and identical in review.
	const r = recorder();
	r.confirmed = false;
	assert.equal(await deleteConfirmedAttachment(r), undefined);
	assert.equal(r.calls.length, 0);
	assert.equal(r.checkpoints.length, 0);
	assert.equal(r.errors.length, 0);
	// The dismissal path answers the same way. The bound host reads an
	// undefined answer from showWarningMessage as declined, which is what an
	// Escape or a click on the editor's own Cancel produces, so a recorder
	// whose confirmation answers false stands for both.
	const dismissed = recorder();
	dismissed.confirmed = false;
	assert.equal(await deleteConfirmedAttachment(dismissed), undefined);
	assert.equal(dismissed.calls.length, 0);
	assert.equal(dismissed.checkpoints.length, 0);
});

test("the confirmation names the file and the address the row was drawn from", async () => {
	// dinah-451 AC-7. Equality rather than containment, because a containment
	// check goes on passing while the two placeholders are filled from each
	// other's fields, and the row's label is the filename alone, so a reader
	// with two attachments of the same name needs the address as well.
	const r = recorder();
	r.confirmed = true;
	await deleteConfirmedAttachment(r);
	assert.equal(r.confirmations.length, 1);
	assert.equal(
		r.confirmations[0].message,
		ENGLISH("dialog.attachment.delete.confirm", {
			filename: "spec.pdf",
			ref: "tr-4/attachments/2",
		}),
	);
	assert.equal(r.confirmations[0].label, ENGLISH("dialog.attachment.delete.action"));
});

test("a delete checkpoints the folder the row stands in, whichever way dinah answered", async () => {
	// dinah-451 AC-8. The checkpoint is the whole of the extension's refresh
	// obligation here: it re-reads the workbench, which is what redraws the
	// card's own count and the group's contents. It runs on a refusal too,
	// because a refusal usually means the board moved under the reader.
	const accepted = recorder();
	accepted.confirmed = true;
	await deleteConfirmedAttachment(accepted);
	assert.deepEqual(accepted.checkpoints, ["C:\\work\\bench"]);
	assert.equal(accepted.errors.length, 0);

	const denied = recorder({
		delete: refused("dinah.unknown-path", "tr-4/attachments/0b2c3d4e5f61"),
	});
	denied.confirmed = true;
	await deleteConfirmedAttachment(denied);
	assert.deepEqual(denied.checkpoints, ["C:\\work\\bench"]);
	assert.equal(denied.errors.length, 1);
	assert.equal(
		denied.errors[0],
		"dinah.unknown-path: tr-4/attachments/0b2c3d4e5f61",
	);
});
