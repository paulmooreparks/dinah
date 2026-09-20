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

import type { SpawnOutcome, Spawner } from "../../src/cli";
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
	itemLabel,
	treeItemFor,
} from "../../src/tree";
import type { CardView, ColumnView, ItemView } from "../../src/wire";
import type { CheckResults, CommentLog, HostLog } from "../support/rows";
import {
	EXE,
	FOLDER,
	ROOT,
	catalogueWithKinds,
	collectionRow,
	columnView,
	emptyCommentLog,
	emptyLog,
	itemRow,
	itemView,
	ok,
	refused,
	rootRow,
	wiringFor,
} from "../support/rows";

/** What one driven run recorded. */
interface Run {
	readonly log: HostLog;
	readonly comments: CommentLog;
	readonly calls: string[][];
}

/** Drives the entry the editor registers for this command id. */
async function invoke(
	id: string,
	elements: readonly TreeElement[],
	options: {
		readonly log?: HostLog;
		readonly comments?: CommentLog;
		readonly answer?: (argv: readonly string[]) => SpawnOutcome;
		readonly catalogue?: ReturnType<typeof catalogueWithKinds>;
	} = {},
): Promise<Run> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === id);
	assert.notEqual(entry, undefined, `no table entry carries the id ${id}`);
	const log = options.log ?? emptyLog();
	const comments = options.comments ?? emptyCommentLog();
	const calls: string[][] = [];
	const answer = options.answer ?? ((): SpawnOutcome => ok());
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return answer(argv);
	};
	const results: CheckResults = { applied: [] };
	await (entry as (typeof ROW_COMMAND_TABLE)[number]).invoke(
		elements,
		wiringFor(log, spawner, results, comments, options.catalogue ?? catalogueWithKinds()),
	);
	return { log, comments, calls };
}

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(root: string, ...args: string[]): string[] {
	return ["--json", "--workbench", root, ...args];
}

// ---------------------------------------------------------------------------
// dinah-506/criteria/3: a card carrying items and no attachments expands
// ---------------------------------------------------------------------------

test("a card with checklist items and no attachments draws an arrow and yields one Checklist row", async () => {
	// VS Code asks for the tree item before it asks for children, so an arrow
	// the checkpoint withholds means getChildren is never called at all. One
	// published total decides it now, summed across every collection the
	// grammar gives a card, and the rows beneath come from the grammar.
	const view: CardView = {
		id: "wb1",
		ref: "wb-1",
		checklist_count: 2,
		child_count: 2,
	};
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
		spawner: async () =>
			ok({
				root: {
					kind: "card",
					ref: "wb-1",
					count: 2,
					children: [
						{ kind: "item", ref: "wb-1/questions/1", title: "First", count: 0 },
						{ kind: "item", ref: "wb-1/questions/2", title: "Second", count: 0 },
					],
				},
			}),
		log: () => undefined,
		t: ENGLISH,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
	const children = await provider.getChildren(element);
	assert.equal(children.length, 1);
	const group = children[0];
	if (group.kind !== "collection") {
		assert.fail(`the card drew a ${group.kind} row, wanted a collection`);
	}
	assert.equal(group.memberKind, "item");
	assert.equal(
		group.members.length,
		2,
		"the collection row carries a member count other than the one the grammar answered",
	);
});

test("a card carrying nothing at all still draws no arrow", async () => {
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
});

// ---------------------------------------------------------------------------
// dinah-506/criteria/10, 11: the selection policies, driven
// ---------------------------------------------------------------------------

test("Comment, Resolve, Verify, Fail and Raise each refuse a multi-row selection and spawn nothing", async () => {
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
		const comments = emptyCommentLog();
		const run = await invoke(
			id,
			[
				itemRow({ ref: "tr-1/questions/1" }),
				itemRow({ ref: "tr-1/questions/2", id: "b00000000002", ordinal: 2 }),
			],
			{ log, comments },
		);
		assert.deepEqual(run.calls, [], `${id} spawned over a two-row selection`);
		assert.equal(run.log.errors.length, 1, `${id} recorded ${String(run.log.errors.length)} errors`);
		assert.equal(run.log.errors[0], ENGLISH("dialog.bulk.oneRowOnly"));
		assert.deepEqual(run.comments.opened, [], `${id} opened a comment over a two-row selection`);
		driven += 1;
	}
	assert.equal(driven, 4, "the sweep drove a number of commands other than four");

	// Raise is the fifth rowOnly command and it is driven beside the loop
	// rather than inside it, because it acts on card rows and the four above
	// act on item rows. SELECTION_POLICIES declares it rowOnly, but the guard
	// that reads that table drives only the commands declared fanOut, so a
	// command declaring rowOnly and then fanning out is caught here or
	// nowhere.
	const raiseRow = (ref: string): TreeElement => ({
		kind: "card",
		row: { ...rootRow(), data: flowData() },
		node: { kind: "card", ref, count: 1 },
		view: { id: ref.replace("-", ""), ref },
		column: columnView(),
	});
	// The form is driven with every answer it would need, so that zero spawns
	// is a refusal rather than a cancellation. Left unscripted, the first
	// quick pick declines on its own and the form answers undefined whether
	// the guard is present or not, which is the reading a guard that
	// complained and then filed anyway would also pass.
	const raiseLog = emptyLog();
	raiseLog.typed = "Which column settles this?";
	const raiseAnswers = [
		{ label: "Open question", value: "open_question" },
		{ label: "here", value: "here" },
		{ label: "The operator", value: "operator" },
	];
	let raiseAt = 0;
	const raisePicking: HostLog = {
		...raiseLog,
		get picked() {
			return raiseAnswers[Math.min(raiseAt++, raiseAnswers.length - 1)];
		},
	} as HostLog;
	const raise = await invoke(COMMAND_FILE_ITEM, [raiseRow("wb-1"), raiseRow("wb-2")], {
		log: raisePicking,
	});
	assert.deepEqual(raise.calls, [], "Raise spawned over a two-row selection");
	assert.equal(
		raise.log.errors.length,
		1,
		`Raise recorded ${String(raise.log.errors.length)} errors`,
	);
	assert.equal(raise.log.errors[0], ENGLISH("dialog.bulk.oneRowOnly"));
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

test("Open Item asks path for the item's own reference and opens what it answered", async () => {
	// dinah-519/criteria/9. The composed document is gone: opening an item
	// opens `item.md`, through one call and with no holder composed out of the
	// reference. THE ITEM'S REFERENCE IS MADE TO LIE ABOUT ITS SHAPE, carrying
	// no /checklist/ segment and no segment the deleted resolver would have
	// cut, so a build that still composes a holder records a different argv
	// rather than passing by accident.
	const run = await invoke(COMMAND_OPEN_ITEM, [itemRow({ ref: "tr-1/questions/1" })], {
		answer: () =>
			ok({ path: "C:\\work\\bench\\cards\\aa\\checklist\\bb\\item.md" }),
	});
	assert.deepEqual(run.calls, [pinned(ROOT, "path", "tr-1/questions/1")]);
	assert.deepEqual(run.log.opened, [
		"C:\\work\\bench\\cards\\aa\\checklist\\bb\\item.md",
	]);
	assert.deepEqual(run.log.served, [], "opening an anchor file served a composed page");
});

test("a refused path shows the refusal and opens nothing", async () => {
	const run = await invoke(COMMAND_OPEN_ITEM, [itemRow({ ref: "tr-1/questions/1" })], {
		answer: () => refused("dinah.unknown-item", "tr-1/questions/1"),
	});
	assert.deepEqual(run.calls, [pinned(ROOT, "path", "tr-1/questions/1")]);
	assert.deepEqual(run.log.opened, []);
	assert.deepEqual(run.log.errors, ["dinah.unknown-item: tr-1/questions/1"]);
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

	// An item is no longer a leaf: three comments below it are three rows, and
	// the arrow comes from the grammar's own count. dinah-519/criteria/7 is
	// where the two sources are made to disagree.
	const element = itemRow({ comment_count: 3 }, false, ROOT, flowData());
	const item = treeItemFor(element, ENGLISH);
	assert.equal(item.collapsibleState, "collapsed");
	assert.equal(item.command?.command, COMMAND_OPEN_ITEM);
	assert.ok((item.description as string).includes(ENGLISH("item.kind.question")));
	assert.ok((item.description as string).includes(ENGLISH("item.state.pending")));
	assert.ok((item.description as string).includes(ENGLISH("item.comments", { count: "3" })));
});

test("a row's tooltip states its column's hold, and a locked row's says why its menu is short", () => {
	const owned: Partial<ItemView> = { owner: "operator", state: "pending" };
	const locked = treeItemFor(itemRow(owned, false, ROOT, flowData()), ENGLISH);
	const unlocked = treeItemFor(itemRow(owned, true, ROOT, flowData()), ENGLISH);
	assert.ok((locked.tooltip as string).includes(ENGLISH("item.locked")));
	assert.ok(!(unlocked.tooltip as string).includes(ENGLISH("item.locked")));

	// The hold sentence is the line of the tooltip a reader can get nowhere
	// else, so it is read off a row driven through the same treeItemFor the
	// tree calls rather than off itemTooltip alone. The row names a column
	// ahead of the card that holds on the way in, which the column pick's own
	// table above fixes at entryAhead, and the five sentences that do not
	// belong to that token are asserted absent, because a tooltip carrying all
	// six would satisfy a check that only looked for the right one.
	const built = itemRow({ ...owned, column: "ahead-on" }, false, ROOT, flowData());
	assert.equal(built.kind, "item", "the helper stopped building an item row");
	const held = treeItemFor(
		{ ...(built as Extract<TreeElement, { kind: "item" }>), card: "wb-1" },
		ENGLISH,
	);
	const tooltip = held.tooltip as string;
	const tokens = Object.keys(HOLD_DIRECTIONS) as HoldDirection[];
	assert.equal(tokens.length, 6, "the token set is not the six the table gives");
	for (const token of tokens) {
		assert.equal(
			tooltip.includes(ENGLISH(`item.hold.${token}`, { 0: "ahead-on" })),
			token === "entryAhead",
			`the tooltip's hold sentence for ${token}`,
		);
	}
});

test("a Checklist collection row counts the members the grammar answered and expands", () => {
	const members = [1, 2, 3, 4].map((at) => ({
		kind: "item",
		ref: `tr-1/questions/${String(at)}`,
		title: `Item ${String(at)}`,
		count: 0,
	}));
	const item = treeItemFor(collectionRow("item", members), ENGLISH);
	assert.equal(item.label, ENGLISH("tree.checklistGroup.label"));
	assert.equal(item.description, "4");
	assert.equal(item.collapsibleState, "collapsed");
	assert.equal(item.contextValue, "dinah.collection.item");
});

test("a Checklist collection whose listing was refused still draws every member row", async () => {
	// dinah-519/criteria/8's other half, on the item arm. The grammar answered
	// and it is the detail that did not, so the rows are drawn from the
	// contents nodes and nothing is appended to say so.
	const members = [1, 2].map((at) => ({
		kind: "item",
		ref: `tr-1/questions/${String(at)}`,
		title: `Item ${String(at)}`,
		count: 0,
	}));
	const logged: string[] = [];
	const provider = new DinahTreeProvider({
		exe: EXE,
		spawner: async () => refused("dinah.unknown-card"),
		log: (line: string) => logged.push(line),
		t: ENGLISH as Localizer,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
	const children = await provider.getChildren(collectionRow("item", members));
	assert.deepEqual(
		children.map((element) => element.kind),
		["item", "item"],
	);
	for (const child of children) {
		const drawn = treeItemFor(child, ENGLISH);
		assert.equal(drawn.description, undefined);
		assert.equal(drawn.contextValue, undefined);
		assert.equal(drawn.icon, undefined);
	}
	assert.deepEqual(
		children.map((element) => treeItemFor(element, ENGLISH).label),
		["Item 1", "Item 2"],
	);
	assert.ok(
		logged.includes(ENGLISH("tree.checklist.unreadable")),
		`the refusal never reached the channel: ${logged.join(" | ")}`,
	);
});
