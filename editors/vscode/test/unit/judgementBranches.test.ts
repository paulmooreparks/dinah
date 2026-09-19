// dinah-536: a card's questions, criteria and decisions arrive as three
// published collection nodes, and these are the tests that hold the extension
// to drawing what it was given rather than to grouping the items itself.
//
// The discipline of contents.test.ts carries over: every case stubs the
// spawner and asserts the requests as well as the rows, because a build that
// draws the right branches and asks three times for the detail behind them is
// wrong in a way no row inspection finds.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import type { Localizer } from "../../src/l10n";
import type { TreeElement } from "../../src/tree";
import { DinahTreeProvider, collectionLabel, elementKey, treeItemFor } from "../../src/tree";
import type { CardView, ItemView, TreeNode } from "../../src/wire";
import { EXE, columnView, ok, refused, rootRow } from "../support/rows";

const CARD = "wb-1";

/** One published judgement branch, as the containment walk now emits one. */
function branch(
	word: string,
	narrow: string,
	members: readonly TreeNode[],
): TreeNode {
	return {
		kind: "collection",
		ref: `${CARD}/${word}`,
		member_kind: "item",
		narrow,
		member_count: members.length,
		count: members.length,
		children: members,
	};
}

function item(ref: string, title: string): TreeNode {
	return { kind: "item", ref, title, count: 0 };
}

function flat(kind: string, ref: string, title: string): TreeNode {
	return { kind, ref, title, count: 0 };
}

/**
 * The published sequence a card answers with: a comment, then the three
 * branches in the CLI's declaration order, then an attachment.
 */
const PUBLISHED: readonly TreeNode[] = [
	flat("comment", `${CARD}/comments/1`, "the first thought"),
	branch("questions", "open_question", [
		item(`${CARD}/questions/1`, "does the deadline move?"),
		item(`${CARD}/questions/2`, "which region?"),
	]),
	branch("criteria", "acceptance_criterion", [
		item(`${CARD}/criteria/1`, "the endpoint answers 404"),
	]),
	branch("decisions", "decision", [item(`${CARD}/decisions/1`, "the exporter is shared")]),
	flat("attachment", `${CARD}/attachments/1`, "notes.txt"),
];

/** The flat run an older binary answers with, which must still group. */
const LEGACY: readonly TreeNode[] = [
	flat("comment", `${CARD}/comments/1`, "the first thought"),
	item(`${CARD}/questions/1`, "does the deadline move?"),
	item(`${CARD}/criteria/1`, "the endpoint answers 404"),
	flat("attachment", `${CARD}/attachments/1`, "notes.txt"),
];

const CHECKLIST: readonly ItemView[] = [
	{ ref: `${CARD}/questions/1`, kind: "open_question", text: "does the deadline move?", state: "pending" },
	{ ref: `${CARD}/questions/2`, kind: "open_question", text: "which region?", state: "pending" },
	{ ref: `${CARD}/criteria/1`, kind: "acceptance_criterion", text: "the endpoint answers 404", state: "pending" },
	{ ref: `${CARD}/decisions/1`, kind: "decision", text: "the exporter is shared", state: "resolved" },
] as unknown as readonly ItemView[];

interface Stub {
	readonly spawner: Spawner;
	readonly calls: string[][];
}

/**
 * A spawner answering one card's structure and one card's checklist.
 *
 * `checklist` is a function rather than a payload so a case can count the
 * calls it makes and answer a different way on the second one, which is what
 * the cache cases need.
 */
function stub(
	children: readonly TreeNode[],
	checklist: () => { readonly ok: boolean; readonly views?: readonly ItemView[] },
): Stub {
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		if (argv.includes("list")) {
			return ok({
				producer: "containment",
				subject: "entity",
				depth: "all",
				root: { kind: "card", ref: CARD, count: children.length, children },
			});
		}
		if (argv.includes("show")) {
			const answer = checklist();
			if (!answer.ok) {
				return refused("dinah.unknown-reference", CARD);
			}
			return ok({ card: { id: "wb1", ref: CARD } as CardView, checklist: answer.views ?? [] });
		}
		return refused("dinah.no-such-verb");
	};
	return { spawner, calls };
}

function providerOver(spawner: Spawner, logged: string[] = []): DinahTreeProvider {
	return new DinahTreeProvider({
		exe: EXE,
		spawner,
		log: (line: string) => logged.push(line),
		t: ENGLISH as Localizer,
	} as unknown as ConstructorParameters<typeof DinahTreeProvider>[0]);
}

function cardRow(): TreeElement {
	return {
		kind: "card",
		row: rootRow(),
		node: { kind: "card", ref: CARD, count: 1 },
		view: { id: "wb1", ref: CARD } as CardView,
		column: columnView(),
	} as TreeElement;
}

/** How many checklist-detail calls a stub recorded. */
function detailCalls(calls: readonly string[][]): string[][] {
	return calls.filter((argv) => argv.includes("show"));
}

// ---------------------------------------------------------------------------
// criteria/7: the published sequence is the visible sequence
// ---------------------------------------------------------------------------

test("a card draws its published branches in payload order, between its comments and its attachments", async () => {
	const { spawner } = stub(PUBLISHED, () => ({ ok: true, views: CHECKLIST }));
	const rows = await providerOver(spawner).getChildren(cardRow());

	assert.deepEqual(
		rows.map((row) => treeItemFor(row, ENGLISH as Localizer).label),
		["Comments", "Questions", "Criteria", "Decisions", "Attachments"],
	);
	assert.deepEqual(
		rows.map((row) => treeItemFor(row, ENGLISH as Localizer).description),
		["1", "2", "1", "1", "1"],
	);
});

test("the extension takes its branch order from the payload and holds none of its own", async () => {
	// The same three branches, sent in an order the CLI's declaration does not
	// put them in. An extension ordering them from a table of its own draws
	// Questions, Criteria, Decisions here and fails.
	const reordered: readonly TreeNode[] = [
		branch("decisions", "decision", [item(`${CARD}/decisions/1`, "the exporter is shared")]),
		branch("criteria", "acceptance_criterion", [item(`${CARD}/criteria/1`, "the endpoint answers 404")]),
		branch("questions", "open_question", [item(`${CARD}/questions/1`, "does the deadline move?")]),
	];
	const { spawner } = stub(reordered, () => ({ ok: true, views: CHECKLIST }));
	const rows = await providerOver(spawner).getChildren(cardRow());

	assert.deepEqual(
		rows.map((row) => treeItemFor(row, ENGLISH as Localizer).label),
		["Decisions", "Criteria", "Questions"],
	);
});

test("a branch draws the members it was published with and nothing else", async () => {
	const { spawner } = stub(PUBLISHED, () => ({ ok: true, views: CHECKLIST }));
	const view = providerOver(spawner);
	const rows = await view.getChildren(cardRow());
	const questions = rows[1];
	const members = await view.getChildren(questions);

	assert.deepEqual(
		members.map((row) => (row as { node: TreeNode }).node.ref),
		[`${CARD}/questions/1`, `${CARD}/questions/2`],
	);
	// Joined to the detail answer by reference, so each row carries its own
	// view rather than the one standing at its position in the whole
	// checklist.
	assert.deepEqual(
		members.map((row) => (row as { view?: ItemView }).view?.ref),
		[`${CARD}/questions/1`, `${CARD}/questions/2`],
	);
});

test("a detail answer missing one view leaves that row in its own branch", async () => {
	const missing = CHECKLIST.filter((view) => view.ref !== `${CARD}/questions/1`);
	const { spawner } = stub(PUBLISHED, () => ({ ok: true, views: missing }));
	const view = providerOver(spawner);
	const rows = await view.getChildren(cardRow());
	const members = await view.getChildren(rows[1]);

	assert.deepEqual(
		members.map((row) => (row as { node: TreeNode }).node.ref),
		[`${CARD}/questions/1`, `${CARD}/questions/2`],
	);
	assert.equal((members[0] as { view?: ItemView }).view, undefined);
	assert.equal((members[1] as { view?: ItemView }).view?.ref, `${CARD}/questions/2`);
});

// ---------------------------------------------------------------------------
// criteria/8: three branches holding one member kind are three rows
// ---------------------------------------------------------------------------

test("questions, criteria and decisions compose three distinct element keys", async () => {
	const { spawner } = stub(PUBLISHED, () => ({ ok: true, views: CHECKLIST }));
	const rows = await providerOver(spawner).getChildren(cardRow());
	const keys = rows.slice(1, 4).map(elementKey);

	assert.equal(new Set(keys).size, 3, `three branches share a key: ${keys.join(" | ")}`);
	// Item keys are the canonical item references, and no row sets an id.
	const view = providerOver(spawner);
	const members = await view.getChildren((await view.getChildren(cardRow()))[1]);
	assert.ok(elementKey(members[0]).includes(`${CARD}/questions/1`));
	// TreeItemSpec carries no id member at all, which is this card's promise
	// about it: nothing here sets one, so there is none to assert a value for.
	assert.equal(
		Object.prototype.hasOwnProperty.call(treeItemFor(members[0], ENGLISH as Localizer), "id"),
		false,
	);
});

// ---------------------------------------------------------------------------
// criteria/9: one checklist read per card per checkpoint
// ---------------------------------------------------------------------------

test("expanding all three branches reads the checklist once", async () => {
	let reads = 0;
	const { spawner, calls } = stub(PUBLISHED, () => {
		reads += 1;
		return { ok: true, views: CHECKLIST };
	});
	const view = providerOver(spawner);
	const rows = await view.getChildren(cardRow());
	await view.getChildren(rows[1]);
	await view.getChildren(rows[2]);
	await view.getChildren(rows[3]);

	assert.equal(reads, 1, "the checklist was read more than once for one card");
	assert.equal(detailCalls(calls).length, 1);
});

test("two branches opened at once join one read rather than racing two", async () => {
	let reads = 0;
	const { spawner, calls } = stub(PUBLISHED, () => {
		reads += 1;
		return { ok: true, views: CHECKLIST };
	});
	const view = providerOver(spawner);
	const rows = await view.getChildren(cardRow());
	await Promise.all([view.getChildren(rows[1]), view.getChildren(rows[2])]);

	assert.equal(reads, 1, "two concurrent expansions made two reads");
	assert.equal(detailCalls(calls).length, 1);
});

test("an empty answer and a refusal are both remembered until the next checkpoint", async () => {
	for (const answer of [
		{ ok: true, views: [] as readonly ItemView[] },
		{ ok: false },
	]) {
		let reads = 0;
		const { spawner, calls } = stub(PUBLISHED, () => {
			reads += 1;
			return answer;
		});
		const view = providerOver(spawner);
		const rows = await view.getChildren(cardRow());
		await view.getChildren(rows[1]);
		await view.getChildren(rows[2]);
		assert.equal(reads, 1, "a non-answer was asked for twice in one checkpoint");
		assert.equal(detailCalls(calls).length, 1);
	}
});

// ---------------------------------------------------------------------------
// The legacy path: an older binary still draws its checklist
// ---------------------------------------------------------------------------

test("a flat item run from an older binary still groups into one Checklist row", async () => {
	const { spawner } = stub(LEGACY, () => ({ ok: true, views: CHECKLIST }));
	const rows = await providerOver(spawner).getChildren(cardRow());

	assert.deepEqual(
		rows.map((row) => treeItemFor(row, ENGLISH as Localizer).label),
		["Comments", "Checklist", "Attachments"],
	);
});

// ---------------------------------------------------------------------------
// The label lookup decides text and nothing else
// ---------------------------------------------------------------------------

test("a branch is labelled from its narrow token, and an unknown one falls back", () => {
	assert.equal(collectionLabel("item", ENGLISH as Localizer, "open_question"), "Questions");
	assert.equal(collectionLabel("item", ENGLISH as Localizer, "acceptance_criterion"), "Criteria");
	assert.equal(collectionLabel("item", ENGLISH as Localizer, "decision"), "Decisions");
	// A kind the CLI gains before this extension knows its name draws as the
	// member kind's own noun rather than as a raw token.
	assert.equal(collectionLabel("item", ENGLISH as Localizer, "risk"), "Checklist");
	assert.equal(collectionLabel("item", ENGLISH as Localizer), "Checklist");
});
