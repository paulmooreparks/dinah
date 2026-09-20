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
	COMMAND_ADD_CRITERION,
	COMMAND_RAISE_QUESTION,
	COMMAND_RECORD_DECISION,
	COMMAND_OPEN_ITEM,
	COMMAND_REOPEN_ITEM,
	COMMAND_RESOLVE_ITEM,
	COMMAND_VERIFY_ITEM,
} from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import type { Localizer } from "../../src/l10n";
import { columnPickItems, publishesKind } from "../../src/itemCommands";
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
	collectionContextValue,
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

	// The three filing commands are the other rowOnly commands and they are
	// driven beside the loop rather than inside it, because they act on card
	// rows and the four above act on item rows. SELECTION_POLICIES declares
	// each of them rowOnly, but the guard that reads that table drives only
	// the commands declared fanOut, so a command declaring rowOnly and then
	// fanning out is caught here or nowhere.
	const raiseRow = (ref: string): TreeElement => ({
		kind: "card",
		row: { ...rootRow(), data: flowData() },
		node: { kind: "card", ref, count: 1 },
		view: { id: ref.replace("-", ""), ref },
		column: columnView(),
	});
	let filing = 0;
	for (const id of [
		COMMAND_RAISE_QUESTION,
		COMMAND_RECORD_DECISION,
		COMMAND_ADD_CRITERION,
	]) {
		// The form is driven with every answer it would need, so that zero
		// spawns is a refusal rather than a cancellation. Left unscripted, the
		// first quick pick declines on its own and the form answers undefined
		// whether the guard is present or not, which is the reading a guard
		// that complained and then filed anyway would also pass.
		const raiseLog = emptyLog();
		raiseLog.typed = "Which column settles this?";
		const raiseAnswers = [
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
		const raise = await invoke(id, [raiseRow("wb-1"), raiseRow("wb-2")], {
			log: raisePicking,
		});
		assert.deepEqual(raise.calls, [], `${id} spawned over a two-row selection`);
		assert.equal(
			raise.log.errors.length,
			1,
			`${id} recorded ${String(raise.log.errors.length)} errors`,
		);
		assert.equal(raise.log.errors[0], ENGLISH("dialog.bulk.oneRowOnly"));
		filing += 1;
	}
	assert.equal(filing, 3, "the sweep drove a number of filing commands other than three");
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
// dinah-506/criteria/13, 14 and dinah-517/criteria/2, 8, 11: the filing forms
// ---------------------------------------------------------------------------

/** One card row, with a flow behind it, for a filing command to be aimed at. */
function filingCardRow(ref = "wb-1"): TreeElement {
	return {
		kind: "card",
		row: { ...rootRow(), data: flowData() },
		node: { kind: "card", ref, count: 1 },
		view: { id: ref.replace("-", ""), ref },
		column: columnView(),
	};
}

/** A log answering the column pick and then the owner pick, in that order. */
function filingLog(): HostLog {
	const log = emptyLog();
	log.typed = "The column pick states the direction it will hold";
	const answers = [
		{ label: "here", value: "here" },
		{ label: "The operator", value: "operator" },
	];
	let at = 0;
	return {
		...log,
		get picked() {
			return answers[Math.min(at++, answers.length - 1)];
		},
	} as HostLog;
}

test("each filing command sends its own kind, and asks no kind question", async () => {
	// dinah-517/criteria/2. The three commands are driven over one card row
	// each, and the argv is compared whole, so a command sending another
	// command's kind fails rather than passing on a shared prefix.
	const wanted: readonly { id: string; kind: string }[] = [
		{ id: COMMAND_RAISE_QUESTION, kind: "open_question" },
		{ id: COMMAND_RECORD_DECISION, kind: "decision" },
		{ id: COMMAND_ADD_CRITERION, kind: "acceptance_criterion" },
	];
	assert.equal(new Set(wanted.map((each) => each.kind)).size, 3);
	let driven = 0;
	for (const { id, kind } of wanted) {
		const log = filingLog();
		const run = await invoke(id, [filingCardRow()], { log });
		assert.deepEqual(run.calls, [
			[
				"--json",
				"--workbench",
				ROOT,
				"file",
				"wb-1",
				kind,
				"The column pick states the direction it will hold",
				"--column",
				"here",
				"--owner",
				"operator",
			],
		]);
		// Two quick picks and no more: the column and the owner. A third
		// would be the kind question the command answers by being itself.
		assert.deepEqual(run.log.placeholders, [
			ENGLISH("form.file.column.placeholder"),
			ENGLISH("form.file.owner.placeholder"),
		]);
		driven += 1;
	}
	assert.equal(driven, 3, "the sweep drove a number of commands other than three");
});

test("a filing command aimed at its own judgement branch files against the card", async () => {
	// dinah-517/criteria/3's other half. The menus offer Raise an Open
	// Question on a Questions branch, so invoking it there has to reach the
	// card the branch hangs from rather than skipping the row.
	const branch: TreeElement = {
		kind: "collection",
		row: { ...rootRow(), data: flowData() },
		root: ROOT,
		holder: "wb-1",
		holderKind: "card",
		memberKind: "item",
		narrow: "open_question",
		members: [],
	};
	const run = await invoke(COMMAND_RAISE_QUESTION, [branch], { log: filingLog() });
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

test("publishesKind reads the tool schema rather than a list held here", () => {
	// The fabricated catalogue carries a fourth kind and omits one of the
	// three, so an extension holding its own list of three cannot pass either
	// half.
	const build = catalogueWithKinds(["open_question", "decision", "constraint"]);
	assert.equal(build.kind, "ok");
	const ok = build as Extract<typeof build, { kind: "ok" }>;
	assert.equal(publishesKind(ok, "open_question"), true);
	assert.equal(publishesKind(ok, "decision"), true);
	assert.equal(publishesKind(ok, "constraint"), true);
	assert.equal(publishesKind(ok, "acceptance_criterion"), false);
});

test("a filing command refuses a kind the schema does not publish, and files one it does", async () => {
	// dinah-517/criteria/8 and criteria/11, both halves. An implementation
	// that drops the check passes the filing half, and one that refuses every
	// kind passes the refusing half, so neither half proves anything alone.
	const refusing = filingLog();
	const refused = await invoke(COMMAND_RECORD_DECISION, [filingCardRow()], {
		log: refusing,
		catalogue: catalogueWithKinds(["open_question", "acceptance_criterion"]),
	});
	assert.deepEqual(refused.calls, [], "Record a Decision spawned on a catalogue omitting decision");
	assert.deepEqual(refused.log.errors, [ENGLISH("dialog.runVerb.enumerationFailed.toast")]);

	const accepting = filingLog();
	const filed = await invoke(COMMAND_RECORD_DECISION, [filingCardRow()], {
		log: accepting,
		catalogue: catalogueWithKinds(["open_question", "acceptance_criterion", "decision"]),
	});
	assert.equal(filed.calls.length, 1, "Record a Decision spawned nothing on a catalogue publishing decision");
	assert.deepEqual(filed.log.errors, []);
	assert.equal(filed.calls[0][5], "decision");
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

test("an item row draws a bounded one-line label and says its kind and state", () => {
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
	// dinah-517/criteria/4. The description is the two words and the
	// separator between them, whole, so a build leaving the comment segment
	// on it fails here rather than on a substring that would still be found.
	assert.equal(
		item.description,
		`${ENGLISH("item.kind.question")} \u00b7 ${ENGLISH("item.state.pending")}`,
	);
	assert.ok(
		!(item.description as string).includes(ENGLISH("item.comments", { count: "3" })),
	);
});

test("an item row draws its comment count at the head of its label, at any width", () => {
	// dinah-517/criteria/4. Two counts, one of them two digits, so a build
	// that fixes the width or caps at nine fails rather than passing on the
	// easy case.
	const text = "Which vendor do we cite for the SLA numbers?";
	for (const count of [3, 12]) {
		const item = treeItemFor(
			itemRow({ comment_count: count, text }, false, ROOT, flowData()),
			ENGLISH,
		);
		assert.equal(item.label, `[${String(count)}] ${text}`);
	}

	// The prefix sits outside the label cap. The item's own text is capped to
	// exactly what itemLabel answers for it, and the prefix is prepended to
	// that, so a long item loses no further text to make room for its count.
	const long = "x".repeat(200);
	const capped = treeItemFor(
		itemRow({ comment_count: 12, text: long }, false, ROOT, flowData()),
		ENGLISH,
	);
	assert.equal(capped.label, `[12] ${itemLabel(long)}`);
	assert.equal(itemLabel(long).length, 120);
});

test("an item row carrying no comments draws the label it drew before", () => {
	// dinah-517/criteria/5. Both absences, because a build drawing an empty
	// bracket on one of them and not the other would pass a single case.
	const text = "Which vendor do we cite for the SLA numbers?";
	for (const comment_count of [undefined, 0]) {
		const item = treeItemFor(
			itemRow({ comment_count, text }, false, ROOT, flowData()),
			ENGLISH,
		);
		assert.equal(item.label, itemLabel(text));
		assert.ok(!(item.label as string).includes("["), String(item.label));
	}
});

test("an item row's tooltip names its state, in every state an item can be in", () => {
	// dinah-517/criteria/6. The state rides the row's description, which the
	// count has just left, and the row's icon carries the kind but nothing
	// carries the state, so the tooltip does. A resolved item is one carrying
	// a resolution, which is what dinah-525 left in the retired note's place.
	const states: readonly { state: string; word: string }[] = [
		{ state: "pending", word: ENGLISH("item.state.pending") },
		{ state: "resolved", word: ENGLISH("item.state.resolved") },
		{ state: "verified", word: ENGLISH("item.state.verified") },
		{ state: "failed", word: ENGLISH("item.state.failed") },
	];
	assert.equal(new Set(states.map((each) => each.word)).size, 4);
	let checked = 0;
	for (const { state, word } of states) {
		const resolution = state === "pending" ? undefined : "tr-1/comments/1";
		const item = treeItemFor(
			itemRow({ state, resolution }, false, ROOT, flowData()),
			ENGLISH,
		);
		const lines = (item.tooltip as string).split("\n");
		assert.ok(
			lines.includes(`${ENGLISH("item.state.label")} ${word}`),
			`${state}: ${String(item.tooltip)}`,
		);
		checked += 1;
	}
	assert.equal(checked, 4, "the sweep read a number of states other than four");
});

test("a judgement branch row carries the short kind token, and an unknown narrow does not", () => {
	// dinah-517/criteria/3. The three values are the short tokens the item
	// rows already use, not the stored spellings that arrive in `narrow`, and
	// the three are asserted separately so a build composing one of them from
	// the other spelling fails on that one.
	const wanted: readonly { narrow: string; value: string }[] = [
		{ narrow: "open_question", value: "dinah.collection.item.question" },
		{ narrow: "decision", value: "dinah.collection.item.decision" },
		{ narrow: "acceptance_criterion", value: "dinah.collection.item.criterion" },
	];
	assert.equal(wanted.length, 3);
	for (const { narrow, value } of wanted) {
		assert.equal(collectionContextValue("item", narrow), value);
	}
	assert.notEqual(
		collectionContextValue("item", "open_question"),
		"dinah.collection.item.open_question",
	);
	assert.notEqual(
		collectionContextValue("item", "acceptance_criterion"),
		"dinah.collection.item.acceptance_criterion",
	);
	// A kind this extension has no token for falls back to the unnarrowed
	// value rather than to contextKindOf's own default, which would offer Add
	// an Acceptance Criterion on a row that said something else.
	assert.equal(collectionContextValue("item", "risk"), "dinah.collection.item");
	assert.equal(collectionContextValue("item"), "dinah.collection.item");
});

test("each filing command is offered on the card row, on the legacy row and on its own branch", () => {
	// dinah-517/criteria/3. The five row shapes are driven against the clause
	// the shipped manifest carries, read off package.json rather than off a
	// copy of the regex written here, and each command is required to match
	// three of them and to refuse the other two.
	const manifest = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "package.json"), "utf8"),
	) as {
		contributes: { menus: Record<string, { command: string; when?: string }[]> };
	};
	const rows: readonly { shape: string; value: string }[] = [
		{ shape: "card", value: "dinah.card.ready.claim" },
		{ shape: "legacy", value: "dinah.collection.item" },
		{ shape: "question", value: "dinah.collection.item.question" },
		{ shape: "decision", value: "dinah.collection.item.decision" },
		{ shape: "criterion", value: "dinah.collection.item.criterion" },
	];
	assert.equal(rows.length, 5);
	const commands: readonly { id: string; shape: string }[] = [
		{ id: COMMAND_RAISE_QUESTION, shape: "question" },
		{ id: COMMAND_RECORD_DECISION, shape: "decision" },
		{ id: COMMAND_ADD_CRITERION, shape: "criterion" },
	];
	let matched = 0;
	for (const { id, shape } of commands) {
		const clause = manifest.contributes.menus["view/item/context"].find(
			(entry) => entry.command === id,
		)?.when;
		assert.notEqual(clause, undefined, `the manifest offers ${id} on no row`);
		const pattern = /viewItem =~ \/(.+)\//.exec(clause as string);
		assert.notEqual(pattern, null, `${id}'s clause is not a regex match: ${String(clause)}`);
		const regex = new RegExp((pattern as RegExpExecArray)[1]);
		for (const row of rows) {
			const offered =
				row.shape === "card" || row.shape === "legacy" || row.shape === shape;
			assert.equal(
				regex.test(row.value),
				offered,
				`${id} on a ${row.shape} row carrying ${row.value}`,
			);
			matched += 1;
		}
	}
	assert.equal(matched, 15, "the sweep read a number of pairs other than fifteen");
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
