// dinah-506: the checklist item as the extension draws it, reads it and acts
// on it.
//
// The twelve-cell hold table is the substance of this card and it gets a test
// per cell, because three of the twelve overlap two plausible rules and an
// implementation letting branch order settle them passes every check that
// drives fewer than all twelve. Both sweeps over the six tokens assert that
// each token was produced at least once, so a fixture that reaches four of
// them reddens rather than reporting success on all six.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { pinnedArgv, refusalMessage } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import { runDinah } from "../../src/cli";
import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import {
	COMMAND_CLAIM,
	COMMAND_COMMENT_ON_ITEM,
	COMMAND_FAIL_ITEM,
	COMMAND_FILE_ITEM,
	COMMAND_OPEN_ITEM,
	COMMAND_REOPEN_ITEM,
	COMMAND_RESOLVE_ITEM,
	COMMAND_VERIFY_ITEM,
} from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import type { Localizer } from "../../src/l10n";
import { columnPickItems, kindPickItems } from "../../src/itemCommands";
import type { ItemLabels } from "../../src/servedText";
import { cardRefOf, renderItemMarkdown } from "../../src/servedText";
import type {
	CardStanding,
	HoldDirection,
	TreeElement,
	WorkbenchData,
} from "../../src/tree";
import {
	DinahTreeProvider,
	HOLD_DIRECTIONS,
	actionsFor,
	holdDirection,
	itemContextValue,
	itemHoldDirection,
	itemLabel,
	treeItemFor,
} from "../../src/tree";
import type { CardView, ColumnView, ItemDetail, ItemView } from "../../src/wire";
import type { CheckResults, DraftLog, HostLog } from "../support/rows";
import {
	EXE,
	FOLDER,
	ROOT,
	catalogueWithKinds,
	checklistGroupRow,
	columnView,
	emptyDraftLog,
	emptyLog,
	itemRow,
	itemView,
	ok,
	refused,
	rootRow,
	spawnerLog,
	wiringFor,
} from "../support/rows";

/** What one driven run recorded. */
interface Run {
	readonly log: HostLog;
	readonly drafts: DraftLog;
	readonly calls: string[][];
}

/** Drives the entry the editor registers for this command id. */
async function invoke(
	id: string,
	elements: readonly TreeElement[],
	options: {
		readonly log?: HostLog;
		readonly drafts?: DraftLog;
		readonly answer?: (argv: readonly string[]) => SpawnOutcome;
		readonly catalogue?: ReturnType<typeof catalogueWithKinds>;
	} = {},
): Promise<Run> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === id);
	assert.notEqual(entry, undefined, `no table entry carries the id ${id}`);
	const log = options.log ?? emptyLog();
	const drafts = options.drafts ?? emptyDraftLog();
	const calls: string[][] = [];
	const answer = options.answer ?? ((): SpawnOutcome => ok());
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return answer(argv);
	};
	const results: CheckResults = { applied: [] };
	await (entry as (typeof ROW_COMMAND_TABLE)[number]).invoke(
		elements,
		wiringFor(log, spawner, results, drafts, options.catalogue ?? catalogueWithKinds()),
	);
	return { log, drafts, calls };
}

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(root: string, ...args: string[]): string[] {
	return ["--json", "--workbench", root, ...args];
}

// ---------------------------------------------------------------------------
// dinah-506/criteria/3: a card carrying items and no attachments expands
// ---------------------------------------------------------------------------

test("a card with checklist items and no attachments draws an arrow and yields one checklist group", async () => {
	// Two objects rather than one. VS Code asks for the tree item before it
	// asks for children, so an arrow decided from attachment_count alone means
	// getChildren is never called at all, and either half alone leaves the
	// other half of that defect live.
	const view: CardView = { id: "wb1", ref: "wb-1", checklist_count: 2 };
	const element: TreeElement = {
		kind: "card",
		row: rootRow(),
		node: { kind: "card", ref: "wb-1", count: 1 },
		view,
		column: columnView(),
	};
	assert.equal(view.attachment_count, undefined);
	assert.equal(treeItemFor(element, ENGLISH).collapsibleState, "collapsed");

	const provider = new DinahTreeProvider({
		exe: EXE,
		spawner: async () => ok(),
		log: () => undefined,
		t: ENGLISH,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
	const children = await provider.getChildren(element);
	assert.equal(children.length, 1);
	assert.equal(children[0].kind, "checklistGroup");
	assert.equal(
		(children[0] as { readonly count: number }).count,
		2,
		"the group row carries a count other than the one the card published",
	);
});

test("a card carrying neither attachments nor items still draws no arrow", async () => {
	// The accepting case beside the refusing one. A collapsibleState that
	// answered collapsed unconditionally would satisfy the test above.
	const element: TreeElement = {
		kind: "card",
		row: rootRow(),
		node: { kind: "card", ref: "wb-2", count: 1 },
		view: { id: "wb2", ref: "wb-2" },
		column: columnView(),
	};
	assert.equal(treeItemFor(element, ENGLISH).collapsibleState, "none");
	const provider = new DinahTreeProvider({
		exe: EXE,
		spawner: async () => ok(),
		log: () => undefined,
		t: ENGLISH,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
	assert.deepEqual(await provider.getChildren(element), []);
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/4 to 7: the item document
// ---------------------------------------------------------------------------

/** Labels whose six hold sentences are distinguishable from each other. */
function labels(overrides: Partial<ItemLabels> = {}): ItemLabels {
	return {
		title: "dinah-506/questions/1 (Open question)",
		textHeading: "WHAT-IT-SAYS",
		statusHeading: "STATUS",
		noteHeading: "NOTE",
		commentsHeading: "COMMENTS",
		state: "STATE-PENDING",
		hold: {
			entryAhead: "HOLD-ENTRY-AHEAD",
			entryPassed: "HOLD-ENTRY-PASSED",
			exitHere: "HOLD-EXIT-HERE",
			exitAhead: "HOLD-EXIT-AHEAD",
			exitPassed: "HOLD-EXIT-PASSED",
			nothing: "HOLD-NOTHING",
		},
		owner: "OWNER",
		commentsEmpty: "THREAD-EMPTY",
		textUnavailable: "TEXT-UNAVAILABLE",
		commentHeading: (ordinal, author, ts) => `${String(ordinal)}|${author}|${ts}`,
		...overrides,
	};
}

/** One comment, as `show <item>` reports it. */
function comment(ordinal: number, body: string) {
	return {
		id: `c${String(ordinal)}`,
		ref: `dinah-506/questions/1/comments/${String(ordinal)}`,
		ts: `2026-09-15T0${String(ordinal)}:00:00Z`,
		author: `author-${String(ordinal)}`,
		body,
	};
}

test("the item document puts the text, the note and the whole thread on one page", () => {
	const view = itemView({
		ref: "dinah-506/questions/1",
		text: "Is the one-line comment prompt enough to ship?",
		note: "Settled at Implement on the operator's ruling.",
	});
	const detail: ItemDetail = {
		ref: view.ref,
		text: "---\nkind: open_question\n---\nthe anchor body",
		comments: [comment(1, "The alternative is an untitled tab."), comment(2, "A file loses nothing.")],
	};
	const rendered = renderItemMarkdown(view, detail, labels(), "exitHere");
	for (const wanted of [
		view.text,
		view.note as string,
		"The alternative is an untitled tab.",
		"A file loses nothing.",
		"1|author-1|2026-09-15T01:00:00Z",
		"2|author-2|2026-09-15T02:00:00Z",
	]) {
		assert.ok(rendered.includes(wanted), `the document omits ${wanted}`);
	}
});

test("a pending item's note renders, because the condition is the note and never the state", () => {
	// Exactly one of the pending items on the workbench this was written
	// against carries a note, and nothing will convert those items, so the
	// case is permanent rather than transitional. A renderer suppressing the
	// note while the state reads pending shows that reader nothing.
	const note = "The reasoning this item was raised with, before item comments existed.";
	const rendered = renderItemMarkdown(
		itemView({ state: "pending", note }),
		{ ref: "tr-1/questions/1", text: "" },
		labels(),
		"nothing",
	);
	assert.ok(rendered.includes(note), "a pending item's note was suppressed");
	assert.ok(rendered.includes("NOTE"), "the note heading was suppressed");
});

test("the renderer never emits an anchor's frontmatter", () => {
	// The fixture is a verbatim copy of a real payload, frontmatter delimiters
	// and all, because ItemDetail.text is the anchor file exactly as
	// bench.ReadText returns it.
	const anchor = [
		"---",
		"kind: open_question",
		"state: resolved",
		"column: 5729d4578008",
		"owner: operator",
		"ts: 2026-09-14T07:30:15Z",
		"ordinal: 23",
		"---",
		"Is a write naming an undeclared key refused, or accepted and preserved?",
	].join("\n");
	const rendered = renderItemMarkdown(
		itemView({ text: "the item's own text" }),
		{ ref: "tr-1/questions/1", text: anchor, comments: [comment(1, "a body")] },
		labels(),
		"nothing",
	);
	assert.ok(
		!rendered.split("\n").some((line) => line === "---"),
		"the document carries a frontmatter delimiter on a line of its own",
	);
	assert.ok(!rendered.includes("kind:"), "the document carries a frontmatter key");
});

test("the document falls back without parsing the anchor", () => {
	const body = "Is a write naming an undeclared key refused, or accepted and preserved?";
	const rendered = renderItemMarkdown(
		undefined,
		{
			ref: "dinah-506/questions/1",
			text: `---\nkind: open_question\n---\n${body}`,
			comments: [comment(1, "first body"), comment(2, "second body")],
		},
		labels(),
		"nothing",
	);
	// All three assertions are needed: the first two alone pass on a renderer
	// that also emits the anchor.
	assert.ok(rendered.includes("dinah-506/questions/1"), "the reference is missing");
	assert.ok(rendered.includes("first body") && rendered.includes("second body"));
	assert.ok(!rendered.includes(body), "the document fell back to the anchor's own text");
	assert.ok(rendered.includes("TEXT-UNAVAILABLE"));
});

test("a thread carrying nothing draws its heading and says so", () => {
	const rendered = renderItemMarkdown(
		itemView(),
		{ ref: "tr-1/questions/1", text: "" },
		labels(),
		"nothing",
	);
	assert.ok(rendered.includes("COMMENTS"), "the comments heading was dropped");
	assert.ok(rendered.includes("THREAD-EMPTY"));
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/8: the two calls the item resolver makes
// ---------------------------------------------------------------------------

/**
 * What extension.ts's KIND_ITEM entry does, with its dependencies injected.
 *
 * Composed here rather than imported, because extension.ts loads vscode as a
 * value and no unit test can import it. servedText.test.ts's resolveHistory
 * is the precedent and the shape is the same one: the body is recomposed out
 * of the importable pieces the shipped entry composes it from, the
 * recomposition is driven against the recording spawner the support module
 * already carries, and a source-shape test beside it holds the shipped entry
 * to the same two calls, so the recomposition cannot drift away from the code
 * it stands for without one of the two reddening.
 */
async function resolveItemDocument(
	spawner: Spawner,
	root: string,
	ref: string,
	data: WorkbenchData | undefined,
): Promise<string> {
	const detailOutcome = await runDinah(spawner, EXE, pinnedArgv(root, ["show", ref]), {
		cwd: root,
	});
	if (detailOutcome.kind !== "ok") {
		throw new Error(refusalMessage(detailOutcome));
	}
	const detail = detailOutcome.json as ItemDetail;
	const cardOutcome = await runDinah(
		spawner,
		EXE,
		pinnedArgv(root, ["show", cardRefOf(ref), "--fields", "card,checklist"]),
		{ cwd: root },
	);
	const view =
		cardOutcome.kind === "ok"
			? (cardOutcome.json as { checklist?: readonly ItemView[] }).checklist?.find(
					(candidate) => candidate.ref === ref,
				)
			: undefined;
	const direction =
		view === undefined ? "nothing" : itemHoldDirection(data, cardRefOf(ref), view);
	return renderItemMarkdown(view, detail, labels(), direction);
}

test("the item document resolver makes exactly the two calls the contract names", async () => {
	const ref = "wb-1/questions/1";
	const view = itemView({ ref, column: "col-b", text: "the item's own prose" });
	const log = spawnerLog();
	log.queue.push(
		ok({
			ref,
			text: `---\nkind: open_question\n---\nthe anchor`,
			comments: [],
		}),
		ok({ card: { ref: "wb-1" }, checklist: [view] }),
	);
	const rendered = await resolveItemDocument(log.spawner, ROOT, ref, flowData());
	// The two recorded argv arrays, whole. --json is composed by cli.ts and
	// refused to a caller who spells it, so what the resolver hands over
	// carries the workbench pin and the verb alone.
	assert.equal(log.calls.length, 2, "the resolver made a number of calls other than two");
	assert.deepEqual(log.calls[0], ["--json", "--workbench", ROOT, "show", ref]);
	assert.deepEqual(log.calls[1], [
		"--json",
		"--workbench",
		ROOT,
		"show",
		"wb-1",
		"--fields",
		"card,checklist",
	]);
	assert.equal(log.options[0].cwd, ROOT);
	assert.equal(log.options[1].cwd, ROOT);
	// The second call is the one carrying the item's own prose, which is the
	// whole reason it exists: the anchor's frontmatter is never split here.
	assert.ok(rendered.includes("the item's own prose"), "the view's text never reached the page");
	assert.ok(!rendered.includes("kind: open_question"), "the anchor's frontmatter reached the page");
});

test("a refused item detail throws what the existing refusal path already renders", async () => {
	const log = spawnerLog();
	log.queue.push(refused("unknown-item", "wb-1/questions/9"));
	await assert.rejects(
		() => resolveItemDocument(log.spawner, ROOT, "wb-1/questions/9", undefined),
		/^Error: unknown-item: wb-1\/questions\/9$/,
	);
	// The second call is never made, so a refusal costs one spawn and not two.
	assert.equal(log.calls.length, 1);
});

test("the shipped KIND_ITEM entry composes the same two calls the driven resolver does", () => {
	// The tie between the recomposition above and the code it stands for,
	// read as source on the terms servedText.test.ts already reads the
	// history entry.
	const source = readFileSync(
		join(__dirname, "..", "..", "..", "src", "extension.ts"),
		"utf8",
	);
	const at = source.indexOf("[KIND_ITEM]:");
	assert.notEqual(at, -1, "extension.ts declares no KIND_ITEM resolver");
	const entry = source.slice(at, source.indexOf("\n\t};\n", at));
	assert.equal(
		(entry.match(/runDinah\(/g) ?? []).length,
		2,
		"the item resolver makes a number of calls other than two",
	);
	assert.match(
		entry,
		/pinnedArgv\(root, \["show", ref\]\)/,
		"the first call is not show <item> through pinnedArgv",
	);
	assert.match(
		entry,
		/pinnedArgv\(root, \["show", cardRefOf\(ref\), "--fields", "card,checklist"\]\)/,
		"the second call is not show <card> --fields card,checklist through pinnedArgv",
	);
	assert.doesNotMatch(entry, /--json/, "the item resolver spells --json by hand");
	// The anchor is never split, which is the whole reason the second call
	// exists.
	assert.doesNotMatch(entry, /detail\.text/, "the item resolver reads the anchor's text");
});

test("cardRefOf takes the reference up to its first slash, and leaves a bare card alone", () => {
	assert.equal(cardRefOf("dinah-506/questions/1"), "dinah-506");
	assert.equal(cardRefOf("dinah-506"), "dinah-506");
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/10, 11: the selection policies, driven
// ---------------------------------------------------------------------------

test("Comment, Resolve, Verify and Fail each refuse a multi-row selection and spawn nothing", async () => {
	// Zero spawns is the assertion rather than "not two", because a command
	// that spawned once and reported an error would pass a weaker check.
	let driven = 0;
	for (const id of [
		COMMAND_COMMENT_ON_ITEM,
		COMMAND_RESOLVE_ITEM,
		COMMAND_VERIFY_ITEM,
		COMMAND_FAIL_ITEM,
	]) {
		const log = emptyLog();
		log.typed = "a note";
		const drafts = emptyDraftLog();
		const run = await invoke(
			id,
			[
				itemRow({ ref: "tr-1/questions/1" }),
				itemRow({ ref: "tr-1/questions/2", id: "b00000000002", ordinal: 2 }),
			],
			{ log, drafts },
		);
		assert.deepEqual(run.calls, [], `${id} spawned over a two-row selection`);
		assert.equal(run.log.errors.length, 1, `${id} recorded ${String(run.log.errors.length)} errors`);
		assert.equal(run.log.errors[0], ENGLISH("dialog.bulk.oneRowOnly"));
		assert.deepEqual(run.drafts.written, [], `${id} wrote a draft over a two-row selection`);
		driven += 1;
	}
	assert.equal(driven, 4, "the sweep drove a number of commands other than four");
});

test("Reopen asks once and acts on every selected row", async () => {
	// Both halves, because asking once and acting once is the wrong outcome a
	// single-half check admits.
	const log = emptyLog();
	log.typed = "the reviewer found it was closed wrongly";
	const run = await invoke(
		COMMAND_REOPEN_ITEM,
		[
			itemRow({ ref: "tr-1/questions/1", state: "resolved" }),
			itemRow({ ref: "tr-1/questions/2", state: "resolved", id: "b2", ordinal: 2 }),
			itemRow({ ref: "tr-1/criteria/1", state: "verified", id: "b3", ordinal: 3 }),
		],
		{ log },
	);
	assert.equal(run.log.prompts.length, 1, "Reopen asked more than once");
	assert.deepEqual(run.calls, [
		pinned(ROOT, "reopen", "tr-1/questions/1", log.typed),
		pinned(ROOT, "reopen", "tr-1/questions/2", log.typed),
		pinned(ROOT, "reopen", "tr-1/criteria/1", log.typed),
	]);
});

test("Resolve on one row sends the verb, the reference and the note", async () => {
	const log = emptyLog();
	log.typed = "  the operator ruled on 2026-09-15  ";
	const run = await invoke(COMMAND_RESOLVE_ITEM, [itemRow({ ref: "tr-1/questions/1" })], {
		log,
	});
	assert.deepEqual(run.calls, [
		pinned(ROOT, "resolve", "tr-1/questions/1", "the operator ruled on 2026-09-15"),
	]);
	assert.deepEqual(run.log.checkpoints, [FOLDER]);
});

test("Open Item opens the item's own document under the item kind", async () => {
	const run = await invoke(COMMAND_OPEN_ITEM, [itemRow({ ref: "tr-1/questions/1" })]);
	assert.deepEqual(run.log.served, ["item:tr-1/questions/1"]);
	assert.deepEqual(run.calls, [], "opening a document spawned dinah");
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/12, 33: the twelve cells, and every one of the six tokens
// ---------------------------------------------------------------------------

/** The table in section 7 step 3 of the specification, one cell per input. */
const CELLS: readonly {
	readonly hold: string | undefined;
	readonly ahead: HoldDirection;
	readonly here: HoldDirection;
	readonly behind: HoldDirection;
}[] = [
	{ hold: undefined, ahead: "nothing", here: "nothing", behind: "nothing" },
	{ hold: "on", ahead: "entryAhead", here: "entryPassed", behind: "entryPassed" },
	{ hold: "out", ahead: "exitAhead", here: "exitHere", behind: "exitPassed" },
	{ hold: "both", ahead: "entryAhead", here: "exitHere", behind: "exitPassed" },
];

test("holdDirection answers each of the twelve cells the table gives", () => {
	// Twelve assertions for twelve cells, so the sweep's size is asserted by
	// the shape of the table rather than by a count, and the three cells of
	// the both row are pinned to the calls the specification makes rather than
	// to whatever branch order the implementation chose.
	const produced = new Set<HoldDirection>();
	for (const cell of CELLS) {
		assert.equal(holdDirection(cell.hold, 5, 3), cell.ahead, `${String(cell.hold)} ahead`);
		assert.equal(holdDirection(cell.hold, 3, 3), cell.here, `${String(cell.hold)} here`);
		assert.equal(holdDirection(cell.hold, 1, 3), cell.behind, `${String(cell.hold)} behind`);
		produced.add(cell.ahead);
		produced.add(cell.here);
		produced.add(cell.behind);
	}
	assert.deepEqual(
		[...produced].sort(),
		Object.keys(HOLD_DIRECTIONS).sort(),
		"the twelve cells do not between them produce every one of the six tokens",
	);
});

test("a hold no healthy server sends answers the way no hold at all does", () => {
	// The wire declares hold as a string, so the function has to answer for a
	// value outside the three typed words. It answers the table's first row
	// rather than inventing a policy of its own.
	for (const stored of ["true", "off", "false", "yes", ""]) {
		assert.equal(holdDirection(stored, 5, 3), "nothing", stored);
	}
});

test("renderItemMarkdown carries the hold sentence belonging to the token it was given", () => {
	const tokens = Object.keys(HOLD_DIRECTIONS) as HoldDirection[];
	assert.equal(tokens.length, 6, "the token set is not the six the table gives");
	const sentences = labels().hold;
	for (const token of tokens) {
		const rendered = renderItemMarkdown(
			itemView(),
			{ ref: "tr-1/questions/1", text: "" },
			labels(),
			token,
		);
		assert.ok(
			rendered.includes(sentences[token]),
			`the document carries no sentence for ${token}`,
		);
		for (const other of tokens) {
			if (other !== token) {
				assert.ok(
					!rendered.includes(sentences[other]),
					`the document carries ${other}'s sentence under ${token}`,
				);
			}
		}
	}
});

/**
 * A workbench standing a card in a column that itself declares a hold.
 *
 * The card stands fourth of seven, and each of the four hold values is
 * declared both ahead of it and behind it. The card's own column declares
 * `both`, which is what the exitHere row needs and what columns ahead and
 * behind alone never supply: a fixture described only in terms of ahead and
 * behind reaches at most five of the six, and one standing the card at the
 * end of the flow reaches four.
 */
function flowData(): WorkbenchData {
	const declared: readonly { readonly ref: string; readonly hold?: string }[] = [
		{ ref: "behind-none" },
		{ ref: "behind-on", hold: "on" },
		{ ref: "behind-out", hold: "out" },
		{ ref: "behind-both", hold: "both" },
		{ ref: "here", hold: "both" },
		{ ref: "ahead-none" },
		{ ref: "ahead-on", hold: "on" },
		{ ref: "ahead-out", hold: "out" },
		{ ref: "ahead-both", hold: "both" },
	];
	const columns = new Map<string, ColumnView>();
	for (const entry of declared) {
		columns.set(
			entry.ref,
			columnView({
				id: `c-${entry.ref}`,
				slug: entry.ref,
				title: entry.ref,
				hold: entry.hold,
			}),
		);
	}
	return {
		path: ROOT,
		title: "Bench",
		columns,
		cards: new Map([["wb1", { id: "wb1", ref: "wb-1", column: "c-here" }]]),
		root: {
			kind: "workbench",
			count: 1,
			children: declared.map((entry) => ({
				kind: "group",
				axis: "column",
				value: entry.ref,
				count: 0,
			})),
		},
		holding: [],
	};
}

test("the column pick states the direction for every entry, and offers no empty value", () => {
	const items = columnPickItems(flowData(), "wb-1", ENGLISH);
	assert.equal(items.length, 9, "the pick offered a number of columns other than nine");

	const wanted: Readonly<Record<string, HoldDirection>> = {
		"behind-none": "nothing",
		"behind-on": "entryPassed",
		"behind-out": "exitPassed",
		"behind-both": "exitPassed",
		here: "exitHere",
		"ahead-none": "nothing",
		"ahead-on": "entryAhead",
		"ahead-out": "exitAhead",
		"ahead-both": "entryAhead",
	};
	const produced = new Set<string>();
	for (const item of items) {
		const token = wanted[item.label];
		assert.notEqual(token, undefined, `the pick offered an unexpected column ${item.label}`);
		assert.equal(
			item.detail,
			ENGLISH(`form.file.column.detail.${token}`),
			`${item.label} states the wrong direction`,
		);
		assert.notEqual(item.value, "", `${item.label} offers an empty value`);
		produced.add(token);
	}
	// The floor this workbench asks of every sweep. A fixture reaching four of
	// the six reports success on all six without it, and the two it cannot
	// reach are the two telling the operator his item will actually stop the
	// card.
	assert.deepEqual(
		[...produced].sort(),
		Object.keys(HOLD_DIRECTIONS).sort(),
		"the pick did not produce every one of the six sentences",
	);
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/13, 14: the filing form
// ---------------------------------------------------------------------------

test("the filing form sends every flag, in the order the verb takes them", async () => {
	const log = emptyLog();
	log.typed = "The column pick states the direction it will hold";
	// One scripted answer per pick, in the order the form asks: kind, column,
	// owner.
	const answers = [
		{ label: "Open question", value: "open_question" },
		{ label: "here", value: "here" },
		{ label: "The operator", value: "operator" },
	];
	let at = 0;
	const picking: HostLog = {
		...log,
		get picked() {
			return answers[Math.min(at++, answers.length - 1)];
		},
	} as HostLog;
	const run = await invoke(
		COMMAND_FILE_ITEM,
		[
			{
				kind: "card",
				row: { ...rootRow(), data: flowData() },
				node: { kind: "card", ref: "wb-1", count: 1 },
				view: { id: "wb1", ref: "wb-1" },
				column: columnView(),
			},
		],
		{ log: picking },
	);
	assert.deepEqual(run.calls, [
		[
			"--json",
			"--workbench",
			ROOT,
			"file",
			"wb-1",
			"open_question",
			"The column pick states the direction it will hold",
			"--column",
			"here",
			"--owner",
			"operator",
		],
	]);
});

test("the form's kind choices come from the tool schema and not from a list here", () => {
	// The fabricated catalogue carries a fourth kind, so an extension holding
	// its own list of three cannot pass.
	const build = catalogueWithKinds([
		"acceptance_criterion",
		"open_question",
		"decision",
		"constraint",
	]);
	assert.equal(build.kind, "ok");
	const items = kindPickItems(build as { kind: "ok" } & typeof build extends never ? never : Extract<typeof build, { kind: "ok" }>, ENGLISH);
	assert.equal(items.length, 4, "the pick offered a number of kinds other than four");
	assert.deepEqual(
		items.map((item) => item.value),
		["acceptance_criterion", "open_question", "decision", "constraint"],
	);
	// A token this extension has no word for renders as the token rather than
	// being skipped, because a kind the form refuses to offer is a kind nobody
	// can file.
	assert.equal(items[3].label, "constraint");
	assert.equal(items[1].label, ENGLISH("item.kind.question"));
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/15, 16: the contextValue, and the card row left alone
// ---------------------------------------------------------------------------

test("an operator-owned pending item withholds the terminal verbs from a non-operator", () => {
	const owned = itemView({ owner: "operator", state: "pending" });
	const locked = itemContextValue(owned, false);
	assert.ok(locked.endsWith(".locked"), `the value is ${locked}`);
	assert.equal(itemContextValue(owned, true), "dinah.item.question.pending");

	// The manifest's own clause, read off the shipped package.json rather than
	// off a copy of the regex in this file.
	const manifest = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "package.json"), "utf8"),
	) as {
		contributes: { menus: Record<string, { command: string; when?: string }[]> };
	};
	const clause = manifest.contributes.menus["view/item/context"].find(
		(entry) => entry.command === "dinah.tree.resolveItem",
	)?.when;
	assert.notEqual(clause, undefined, "the manifest offers Resolve on no row");
	const pattern = /viewItem =~ \/(.+)\//.exec(clause as string);
	assert.notEqual(pattern, null, `the clause is not a regex match: ${String(clause)}`);
	assert.ok(
		!new RegExp((pattern as RegExpExecArray)[1]).test(locked),
		"the shipped resolveItem clause matches a locked contextValue",
	);
	assert.ok(
		new RegExp((pattern as RegExpExecArray)[1]).test("dinah.item.question.pending"),
		"the shipped resolveItem clause matches no unlocked question either, so this proves nothing",
	);
});

test("a closed item of any kind reads as closed, and so does a state the format does not declare", () => {
	for (const state of ["resolved", "verified", "failed", "wrenched"]) {
		assert.equal(
			itemContextValue(itemView({ state, owner: "operator" }), false),
			"dinah.item.question.closed",
			state,
		);
	}
});

test("no card row's actions changed", () => {
	// The four standings, each driven twice: once bare and once with the new
	// count on the view, and each answer compared against the contextValue
	// spelled out here rather than against another call of actionsFor. An
	// earlier form of this test asserted actionsFor(x) === actionsFor({...x}),
	// which is true of any deterministic function and stayed green while a
	// planted swap of the two contextValues reddened seven other tests. The
	// expected strings are literals rather than the identity constants, so a
	// rename that moved a constant's value would redden here too.
	const standings: { standing: CardStanding; want: string }[] = [
		{
			standing: { state: "ready", column: columnView({ takes_work_up: true }) },
			want: "dinah.card.ready.claim",
		},
		{
			standing: { state: "ready", column: columnView({ takes_work_up: false }) },
			want: "dinah.card.ready.none",
		},
		{ standing: { state: "active", column: columnView() }, want: "dinah.card.active" },
		{ standing: { state: "blocked", column: columnView() }, want: "dinah.card.blocked" },
	];
	assert.equal(standings.length, 4);
	// The four wanted values are distinct, so a function collapsing two
	// branches onto one answer cannot satisfy all four.
	assert.equal(new Set(standings.map((each) => each.want)).size, 4);
	for (const { standing, want } of standings) {
		assert.equal(actionsFor(standing), want, standing.state);
		// checklist_count is what this card adds to CardView. It is not a
		// member of CardStanding, so it cannot reach actionsFor at all, and
		// driving it through anyway is what says so out loud. The extra
		// member is declared in the variable's own type rather than asserted
		// past the compiler, so nothing here claims a shape it does not have.
		const counted: CardStanding & { readonly checklist_count: number } = {
			...standing,
			checklist_count: 7,
		};
		assert.equal(actionsFor(counted), want, standing.state);
	}
	const manifest = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "package.json"), "utf8"),
	) as {
		contributes: { menus: Record<string, { command: string; when?: string }[]> };
	};
	const claim = manifest.contributes.menus["view/item/context"].find(
		(entry) => entry.command === COMMAND_CLAIM,
	);
	assert.equal(
		claim?.when,
		"view == dinah.workbenchView && viewItem == dinah.card.ready.claim",
		"dinah.tree.claim's when clause moved",
	);
});

// ---------------------------------------------------------------------------
// The row itself
// ---------------------------------------------------------------------------

test("an item row draws a bounded one-line label and says its kind, state and thread", () => {
	assert.equal(itemLabel("  two   words\nand a third  "), "two words and a third");
	const long = "x".repeat(200);
	assert.equal(itemLabel(long).length, 120);
	assert.ok(itemLabel(long).endsWith("…"));

	const element = itemRow({ comment_count: 3 }, false, ROOT, flowData());
	const item = treeItemFor(element, ENGLISH);
	assert.equal(item.collapsibleState, "none");
	assert.equal(item.command?.command, COMMAND_OPEN_ITEM);
	assert.ok((item.description as string).includes(ENGLISH("item.kind.question")));
	assert.ok((item.description as string).includes(ENGLISH("item.state.pending")));
	assert.ok((item.description as string).includes(ENGLISH("item.comments", { count: "3" })));
});

test("a locked row's tooltip says why its menu is short, and an unlocked one's does not", () => {
	const owned: Partial<ItemView> = { owner: "operator", state: "pending" };
	const locked = treeItemFor(itemRow(owned, false, ROOT, flowData()), ENGLISH);
	const unlocked = treeItemFor(itemRow(owned, true, ROOT, flowData()), ENGLISH);
	assert.ok((locked.tooltip as string).includes(ENGLISH("item.locked")));
	assert.ok(!(unlocked.tooltip as string).includes(ENGLISH("item.locked")));
});

test("a checklist group row carries the eager count and expands", () => {
	const item = treeItemFor(checklistGroupRow(4), ENGLISH);
	assert.equal(item.label, ENGLISH("tree.checklistGroup.label"));
	assert.equal(item.description, "4");
	assert.equal(item.collapsibleState, "collapsed");
});

test("a checklist group whose listing was refused yields one localized note", async () => {
	const provider = new DinahTreeProvider({
		exe: EXE,
		spawner: async () => refused("dinah.unknown-card"),
		log: () => undefined,
		t: ENGLISH as Localizer,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
	const children = await provider.getChildren(checklistGroupRow());
	assert.equal(children.length, 1);
	assert.equal(children[0].kind, "note");
	assert.equal(
		(children[0] as { readonly text: string }).text,
		ENGLISH("tree.checklist.unreadable"),
	);
});
