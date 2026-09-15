// dinah-519: the tree's structure comes from the containment grammar and its
// detail from each kind's own view, and these are the tests that hold the two
// apart.
//
// A new file rather than more of tree.test.ts, which is already three thousand
// lines about the checkpoint and the kanban above a card. Everything here is
// about what happens at or below a card, through one call shape the tree did
// not make before, and the two populations share no fixture.
//
// Every test here stubs the spawner and asserts THE REQUEST as well as the
// answer. That is the discipline the contract mints: both halves run through
// the same getChildren over the same stub, so both are equally assertable, and
// a build that fetches the right structure and asks the wrong question for its
// detail draws a tree that looks correct on this workbench and is wrong
// everywhere below a card.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import type { Localizer } from "../../src/l10n";
import type { TreeElement } from "../../src/tree";
import {
	DinahTreeProvider,
	commentLabel,
	partitionByKind,
	treeItemFor,
} from "../../src/tree";
import type { CardView, CommentView, TreeNode } from "../../src/wire";
import { EXE, ROOT, columnView, ok, refused, rootRow } from "../support/rows";

// ---------------------------------------------------------------------------
// The harness
// ---------------------------------------------------------------------------

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(...args: string[]): string[] {
	return ["--json", "--workbench", ROOT, ...args];
}

/** A `contents` payload, which is a TreeAnswer whose root carries children. */
function contents(
	kind: string,
	ref: string,
	children: readonly TreeNode[],
): unknown {
	return {
		producer: "containment",
		subject: "entity",
		depth: "all",
		root: { kind, ref, count: children.length, children },
	};
}

/** One member node, as the containment walk emits one. */
function node(
	kind: string,
	ref: string,
	title: string,
	count = 0,
): TreeNode {
	return { kind, ref, title, count };
}

/**
 * The rank a reference sits at, counted from the workbench the way
 * contentsLimit counts it: a card is rank 1, a card's own members rank 2, and
 * a member of one of those rank 3.
 *
 * The stub needs this because the default depth is what a wrong build asks
 * for, and a stub that hands its payload back whatever depth was asked for
 * cannot tell the two apart.
 */
function rankOf(ref: string): number {
	// A collection reference such as `workbench/attachments` names no entity
	// and stands at the rank of the entity it holds, which for the workbench's
	// own collections is rank 1.
	const segments = ref.split("/");
	if (segments[0] === "workbench") {
		return 1;
	}
	// `<card>` is 1, `<card>/<collection>/<n>` is 2, and one more pair is 3.
	return 1 + Math.floor(segments.length / 2);
}

interface Stub {
	readonly spawner: Spawner;
	readonly calls: string[][];
}

/**
 * A recording spawner that answers by the verb and the reference it was asked
 * about, and that HONOURS THE DEPTH THE CALLER ASKED FOR.
 *
 * The depth half is what arms every member assertion below. contentsLimit
 * counts absolute rank from the workbench, so the default `entities` level
 * answers a card's collections in full and answers NOTHING below an item or a
 * comment. A stub with no such limit in it hands its payload back whatever was
 * asked, and a build spawning `contents <ref>` at the default depth then fails
 * on the argv alone while the empty member list the wrong depth produces never
 * appears at all.
 */
function stub(answers: Record<string, unknown>): Stub {
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		const at = argv.indexOf("contents");
		if (at >= 0) {
			const ref = argv[at + 1] ?? "";
			const payload = answers[`contents ${ref}`];
			if (payload === undefined) {
				return refused("dinah.unknown-reference", ref);
			}
			const all = argv.includes("--depth") && argv[argv.indexOf("--depth") + 1] === "all";
			if (!all && rankOf(ref) >= 2) {
				// What contentsLimit does in production: the count is filled on
				// every node whatever the walk was cut at, and the children are
				// not returned.
				const root = (payload as { root: { count: number; kind: string; ref: string } }).root;
				return ok({
					producer: "containment",
					subject: "entity",
					depth: "entities",
					root: { kind: root.kind, ref: root.ref, count: root.count },
				});
			}
			return ok(payload);
		}
		for (const [key, payload] of Object.entries(answers)) {
			if (key.startsWith("contents ")) {
				continue;
			}
			const [verb, ref] = key.split(" ");
			if (argv.includes(verb) && (ref === undefined || argv.includes(ref))) {
				return ok(payload);
			}
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

/** A card element, with whatever CardView the case wants on it. */
function cardRow(ref: string, view: Partial<CardView> = {}): TreeElement {
	return {
		kind: "card",
		row: rootRow(),
		node: { kind: "card", ref, count: 1 },
		view: { id: "wb1", ref, ...view },
		column: columnView(),
	};
}

/** The collection rows one element yielded, refusing anything else. */
function collectionsOf(
	children: readonly TreeElement[],
): readonly Extract<TreeElement, { kind: "collection" }>[] {
	return children.map((child) => {
		if (child.kind !== "collection") {
			assert.fail(`the row drew a ${child.kind} row, wanted a collection`);
		}
		return child;
	});
}

/** The argv arrays recorded for one verb. */
function callsTo(calls: readonly string[][], verb: string): string[][] {
	return calls.filter((argv) => argv.includes(verb));
}

// ---------------------------------------------------------------------------
// dinah-519/criteria/1: a card's children come from the grammar, at the depth
// that answers below a card
// ---------------------------------------------------------------------------

test("a card's children come from the grammar and not from its published counts", async () => {
	// The CardView carries two counts that would each have drawn a row on
	// trunk, and the grammar answers one kind neither of them names. Exactly
	// one collection row is right; three is the hybrid that keeps the eager
	// counts and reaches for contents only on an unrecognised kind.
	const { spawner, calls } = stub({
		"contents wb-1": contents("card", "wb-1", [
			node("sketch", "wb-1/sketches/1", "A sketch", 0),
			node("sketch", "wb-1/sketches/2", "Another sketch", 2),
		]),
	});
	const view = providerOver(spawner);
	const children = await view.getChildren(
		cardRow("wb-1", { attachment_count: 2, checklist_count: 2, child_count: 4 }),
	);

	assert.equal(children.length, 1, "the card drew a number of collection rows other than one");
	const [group] = collectionsOf(children);
	assert.equal(group.memberKind, "sketch");
	// The label falls back to the kind token, because COLLECTION_LABELS states
	// no containment and an entry missing from it costs a translated noun
	// rather than a row.
	assert.equal(treeItemFor(group, ENGLISH).label, "sketch");
	assert.equal(treeItemFor(group, ENGLISH).description, "2");

	const members = await view.getChildren(group);
	assert.deepEqual(
		members.map((member) => member.kind),
		["entity", "entity"],
	);
	assert.deepEqual(
		members.map((member) => treeItemFor(member, ENGLISH).label),
		["A sketch", "Another sketch"],
	);
	assert.deepEqual(
		members.map((member) => treeItemFor(member, ENGLISH).description),
		[undefined, "2"],
	);

	// The request, and not only the answer. Exactly one argv, and it carries
	// the depth that answers below a card: a build asking at the default depth
	// is correct for a card row and silently empty below one.
	assert.equal(calls.length, 1, `the card's expansion spawned ${String(calls.length)} calls`);
	assert.deepEqual(calls[0], pinned("contents", "wb-1", "--depth", "all"));
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/2: a card whose only content is comments draws an arrow
// ---------------------------------------------------------------------------

test("a card's arrow comes from its published total, not from the per-collection counts", () => {
	// The population this serves on the operator's own workbench is four cards
	// out of 263: a card holding comments and no attachment and no checklist
	// item. On trunk such a card drew no arrow, so its getChildren was never
	// called and its comments were unreachable. VS Code asks for the tree item
	// first and decides from what it answers.
	const withComments = cardRow("wb-1", {
		attachment_count: 0,
		checklist_count: 0,
		child_count: 3,
	});
	assert.equal(treeItemFor(withComments, ENGLISH).collapsibleState, "collapsed");

	// A card holding anything else still draws one, because the total counts
	// every collection the grammar gives a card.
	const withItems = cardRow("wb-2", {
		attachment_count: 1,
		checklist_count: 2,
		child_count: 3,
	});
	assert.equal(treeItemFor(withItems, ENGLISH).collapsibleState, "collapsed");

	// The refusing case beside the accepting one, in both spellings the wire
	// can produce: child_count carries omitempty, so a zero arrives as absence.
	assert.equal(
		treeItemFor(cardRow("wb-3", { child_count: 0 }), ENGLISH).collapsibleState,
		"none",
	);
	assert.equal(treeItemFor(cardRow("wb-4"), ENGLISH).collapsibleState, "none");
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/3: the order and the membership both come from contents
// ---------------------------------------------------------------------------

test("a card's collection rows follow the contents order, not a mount order this extension holds", async () => {
	// The counts and the membership are made to disagree, so an implementation
	// reading a count for a row's membership or its description fails. And the
	// payload order is made to disagree with the grammar's own mount order for
	// a card, which is comments, checklist, attachments, so an implementation
	// holding a per-kind ordering table fails too: that table is a second
	// statement of the grammar and is the shape this card removes. In
	// production the two orders coincide, because contents walks Contains and
	// emits mounts in declared order.
	const { spawner } = stub({
		"contents wb-1": contents("card", "wb-1", [
			node("attachment", "wb-1/attachments/1", "spec.pdf"),
			node("comment", "wb-1/comments/1", "A thought"),
			node("item", "wb-1/questions/1", "First"),
			node("item", "wb-1/questions/2", "Second"),
		]),
		"attachments wb-1": { kind: "card", ref: "wb-1", attachments: [] },
		"show wb-1": { card: { ref: "wb-1" }, checklist: [], comments: [] },
	});
	const view = providerOver(spawner);
	const groups = collectionsOf(
		await view.getChildren(
			cardRow("wb-1", { attachment_count: 3, checklist_count: 5, child_count: 8 }),
		),
	);

	assert.deepEqual(
		groups.map((group) => group.memberKind),
		["attachment", "comment", "item"],
	);
	assert.deepEqual(
		groups.map((group) => treeItemFor(group, ENGLISH).description),
		["1", "1", "2"],
	);
	assert.deepEqual(
		groups.map((group) => group.members.length),
		[1, 1, 2],
	);
});

test("partitionByKind groups in first-seen order and counts every node", () => {
	const grouped = partitionByKind([
		node("attachment", "wb-1/attachments/1", "spec.pdf"),
		node("comment", "wb-1/comments/1", "A thought"),
		node("attachment", "wb-1/attachments/2", "shot.png"),
	]);
	assert.deepEqual(
		grouped.map(([kind, members]) => [kind, members.length]),
		[
			["attachment", 2],
			["comment", 1],
		],
	);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/4: what a comment row is called
// ---------------------------------------------------------------------------

test("commentLabel takes the opening words and strips exactly one construct", () => {
	assert.equal(
		commentLabel("## WHAT SHIPPED\n\nPull request: https://example.invalid/1"),
		"WHAT SHIPPED",
	);
	assert.equal(commentLabel("two   words\nand a third"), "two words");
	const long = "x".repeat(200);
	assert.equal(commentLabel(long).length, 120);
	assert.ok(commentLabel(long).endsWith("\u2026"));
	assert.equal(commentLabel("   ", "wb-1/comments/4"), "wb-1/comments/4");
	assert.equal(commentLabel("", "wb-1/comments/4"), "wb-1/comments/4");

	// The three non-strips, which are what keep the guard from widening past
	// its one example. A general Markdown stripper fails the first of them.
	assert.equal(commentLabel("**Tier: frontier.**"), "**Tier: frontier.**");
	assert.equal(commentLabel("#nospace"), "#nospace");
	assert.equal(commentLabel("plain text"), "plain text");
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/5: every comment row draws the same icon
// ---------------------------------------------------------------------------

test("every comment row draws the comment icon, whoever wrote it", () => {
	// The operator withdrew the icon distinction on 2026-09-15, and this is
	// what stops it coming back from an earlier draft of the contract. The
	// fixture is the one the withdrawn criterion used, with the workbench's
	// operator and this window's actor deliberately different, so a build
	// reading either of them to mark a row fails here.
	const row = rootRow();
	const data = { ...row.data!, operator: "paul", actor: "claude", isOperator: false };
	const owner = { ...row, data };
	const commentOf = (author: string, joined = true): TreeElement => ({
		kind: "comment",
		row: owner,
		root: ROOT,
		holder: "wb-1",
		node: node("comment", `wb-1/comments/${author}`, "A thought"),
		...(joined
			? {
					view: {
						id: "c1",
						ref: `wb-1/comments/${author}`,
						ts: "2026-09-15T09:00:00Z",
						author,
						body: "A thought",
					} satisfies CommentView,
				}
			: {}),
	});

	assert.deepEqual(treeItemFor(commentOf("paul"), ENGLISH).icon, { id: "comment" });
	assert.deepEqual(treeItemFor(commentOf("claude"), ENGLISH).icon, { id: "comment" });
	// The degraded row, where no view arrived at all. The icon is a constant,
	// so it cannot be lost by a call that did not answer.
	assert.deepEqual(treeItemFor(commentOf("paul", false), ENGLISH).icon, { id: "comment" });
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/6 and /7: the arrow comes from the grammar's own count
// ---------------------------------------------------------------------------

test("a comment row's arrow reads the contents node and never the view's attachments", () => {
	// The two sources are set the OTHER WAY ROUND from each other, which is the
	// only form of this assertion a test can hold: an implementation reading
	// the view rather than the node fails both cases rather than passing
	// silently on one.
	const commentOf = (count: number, attachments: number): TreeElement => ({
		kind: "comment",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		node: node("comment", "wb-1/comments/1", "A thought", count),
		view: {
			id: "c1",
			ref: "wb-1/comments/1",
			ts: "2026-09-15T09:00:00Z",
			author: "claude",
			body: "A thought",
			attachments: Array.from({ length: attachments }, (_unused, at) => ({
				id: `a${String(at)}`,
				ordinal: at + 1,
				ref: `wb-1/comments/1/attachments/${String(at + 1)}`,
				filename: "shot.png",
				provenance: "import",
			})),
		},
	});
	assert.equal(treeItemFor(commentOf(1, 0), ENGLISH).collapsibleState, "collapsed");
	assert.equal(treeItemFor(commentOf(0, 2), ENGLISH).collapsibleState, "none");
});

test("an item row's arrow reads the contents node and never comment_count", () => {
	// Same shape, same reason, and the fixture states comment_count on both
	// cases rather than leaving it unset: an absent count reads as zero, so a
	// fixture leaving it out would arm only one direction by accident.
	const itemOf = (count: number, comments: number): TreeElement => ({
		kind: "item",
		row: rootRow(),
		root: ROOT,
		card: "wb-1",
		node: node("item", "wb-1/questions/1", "First", count),
		view: {
			id: "b1",
			ordinal: 1,
			ref: "wb-1/questions/1",
			kind: "open_question",
			state: "pending",
			text: "First",
			comment_count: comments,
		},
		isOperator: false,
	});
	assert.equal(treeItemFor(itemOf(2, 0), ENGLISH).collapsibleState, "collapsed");
	assert.equal(treeItemFor(itemOf(0, 2), ENGLISH).collapsibleState, "none");
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/8: what a refused call draws
// ---------------------------------------------------------------------------

test("a refused comment detail draws every member row and no other row", async () => {
	// The rows come from the contents nodes. A row loses what the view
	// supplied, which is its description, and nothing else: the context value
	// and the icon are composed without reading a view and survive, and the
	// row knows its own reference, so its menu is honest rather than
	// decorative. The "and no other row" is asserted because a build that
	// draws the members AND appends a note about the refusal passes a per-row
	// assertion alone while giving the reader a row the design does not have.
	const members = [
		node("comment", "wb-1/comments/1", "First thought"),
		node("comment", "wb-1/comments/2", "Second thought"),
	];
	const logged: string[] = [];
	const view = providerOver(stub({}).spawner, logged);
	const children = await view.getChildren({
		kind: "collection",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		holderKind: "card",
		memberKind: "comment",
		members,
	});

	assert.deepEqual(
		children.map((child) => child.kind),
		["comment", "comment"],
	);
	for (const child of children) {
		const drawn = treeItemFor(child, ENGLISH);
		assert.equal(drawn.description, undefined);
		assert.equal(drawn.contextValue, "dinah.comment");
		assert.deepEqual(drawn.icon, { id: "comment" });
	}
	assert.deepEqual(
		children.map((child) => treeItemFor(child, ENGLISH).label),
		["First thought", "Second thought"],
	);
	assert.ok(
		logged.includes(ENGLISH("tree.comments.unreadable")),
		`the refusal never reached the channel: ${logged.join(" | ")}`,
	);
});

test("a refused contents call yields exactly one note row and no member row", async () => {
	const logged: string[] = [];
	const view = providerOver(stub({}).spawner, logged);
	const children = await view.getChildren(cardRow("wb-1", { child_count: 3 }));
	assert.equal(children.length, 1);
	assert.equal(children[0].kind, "note");
	assert.equal(
		treeItemFor(children[0], ENGLISH).label,
		ENGLISH("tree.contents.unreadable"),
	);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/17: the join is on the reference and never on position
// ---------------------------------------------------------------------------

test("a detail call that omits one member still draws that member's row", async () => {
	// THE FIXTURE CARRIES THREE MEMBERS AND THE OMITTED ONE IS NEITHER FIRST
	// NOR LAST, which is what arms this. An implementation joining the views to
	// the nodes BY POSITION passes whenever the omitted view is the last,
	// because every earlier row still lands on its own view. With the omission
	// in the middle a positional join shifts every later view one row along:
	// the omitted node draws a description it should not have, and the rows
	// after it draw somebody else's author.
	//
	// The divergence is real rather than hypothetical. The containment walk
	// keeps a member Show skips, an item whose anchor will not open, so a
	// SUCCESSFUL detail call can answer one view short. No measurement of live
	// data would catch a regression here.
	const members = [
		node("comment", "wb-1/comments/1", "First thought"),
		node("comment", "wb-1/comments/2", "Second thought"),
		node("comment", "wb-1/comments/3", "Third thought"),
	];
	const answered = (at: number): CommentView => ({
		id: `c${String(at)}`,
		ref: `wb-1/comments/${String(at)}`,
		ts: `2026-09-15T0${String(at)}:00:00Z`,
		author: `author-${String(at)}`,
		body: `Thought ${String(at)}`,
	});
	const { spawner } = stub({
		"show wb-1": { card: { ref: "wb-1" }, comments: [answered(1), answered(3)] },
	});
	const children = await providerOver(spawner).getChildren({
		kind: "collection",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		holderKind: "card",
		memberKind: "comment",
		members,
	});

	assert.equal(children.length, 3);
	const drawn = children.map((child) => treeItemFor(child, ENGLISH));
	// The two the call answered for carry their own author, each on its own row.
	assert.ok((drawn[0].description as string).includes("author-1"), drawn[0].description);
	assert.ok((drawn[2].description as string).includes("author-3"), drawn[2].description);
	// The omitted one draws from its contents node, with no description at all,
	// and keeps the context value and the icon every comment row carries.
	assert.equal(drawn[1].description, undefined);
	assert.equal(drawn[1].label, "Second thought");
	assert.equal(drawn[1].contextValue, "dinah.comment");
	assert.deepEqual(drawn[1].icon, { id: "comment" });
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/19: a column's own mounts, and the workbench root's
// ---------------------------------------------------------------------------

test("a column draws its own mounts from the grammar, after its card rows", async () => {
	const { spawner, calls } = stub({
		"contents doing": contents("column", "doing", [
			node("attachment", "doing/attachments/1", "policy.pdf"),
			node("sketch", "doing/sketches/1", "A sketch"),
		]),
		"attachments doing": { kind: "column", ref: "doing", attachments: [] },
	});
	const view = providerOver(spawner);
	const column: TreeElement = {
		kind: "column",
		row: rootRow(),
		node: {
			kind: "column",
			axis: "column",
			value: "doing",
			count: 1,
			children: [{ kind: "card", id: "wb1", ref: "wb-1", title: "A card", count: 0 }],
		},
		view: columnView({ id: "doing", slug: "doing" }),
	};
	const children = await view.getChildren(column);

	assert.deepEqual(
		children.map((child) => child.kind),
		["card", "collection", "collection"],
	);
	const groups = collectionsOf(children.slice(1));
	// In the order contents returned them, because mount order is the source
	// dinah-519/criteria/3 exists to refuse.
	assert.deepEqual(
		groups.map((group) => group.memberKind),
		["attachment", "sketch"],
	);
	assert.deepEqual(
		groups.map((group) => group.holderKind),
		["column", "column"],
	);
	// The unfamiliar kind draws its entity row, which is what makes a column's
	// comments reachable the day the grammar gives a column a comments mount.
	const unfamiliar = await view.getChildren(groups[1]);
	assert.deepEqual(
		unfamiliar.map((child) => child.kind),
		["entity"],
	);

	assert.deepEqual(callsTo(calls, "contents")[0], pinned("contents", "doing", "--depth", "all"));
});

test("the workbench root composes its collection row from the table and fetches its members", async () => {
	const { spawner, calls } = stub({
		"contents workbench/attachments": contents("collection", "workbench/attachments", [
			node("attachment", "workbench/attachments/1", "charter.pdf"),
		]),
		"attachments workbench": {
			kind: "workbench",
			ref: "workbench",
			attachments: [
				{
					id: "a1",
					ordinal: 1,
					ref: "workbench/attachments/1",
					filename: "charter.pdf",
					provenance: "import",
					path: "C:/work/bench/charter.pdf",
				},
			],
		},
	});
	const view = providerOver(spawner);
	const row = rootRow();
	const element: TreeElement = {
		kind: "root",
		row: { ...row, data: { ...row.data!, attachmentCount: 1 } },
	};
	const children = await view.getChildren(element);
	const last = children[children.length - 1];
	if (last.kind !== "collection") {
		assert.fail(`the root drew a ${last.kind} row last, wanted a collection`);
	}

	// The memberKind comes from the table and is SINGULAR, so the row's label
	// is the Attachments string and its contextValue is the same value the
	// attachments collection under a card carries. A row composed from the
	// plural directory name would read dinah.collection.attachments, would
	// miss the label table, and would be the one collection row in the tree
	// whose menu clauses do not match its siblings.
	assert.equal(last.memberKind, "attachment");
	assert.equal(treeItemFor(last, ENGLISH).label, ENGLISH("tree.attachments.label"));
	assert.equal(treeItemFor(last, ENGLISH).contextValue, "dinah.collection.attachment");

	const members = await view.getChildren(last);
	assert.deepEqual(
		members.map((member) => member.kind),
		["attachment"],
	);
	assert.deepEqual(
		callsTo(calls, "contents")[0],
		pinned("contents", "workbench/attachments", "--depth", "all"),
	);
});

test("a workbench whose checkpoint reports no attachments draws no row and spawns nothing", async () => {
	const { spawner, calls } = stub({});
	const view = providerOver(spawner);
	const children = await view.getChildren({ kind: "root", row: rootRow() });
	assert.equal(
		children.some((child) => child.kind === "collection"),
		false,
		"a row with no count to stand on was drawn",
	);
	assert.deepEqual(callsTo(calls, "contents"), []);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/20: the rows below a card ask at the depth that answers
// ---------------------------------------------------------------------------

test("an item, a comment and an entity row each ask contents at the depth that answers", async () => {
	// This is the site the default depth actually breaks. An item and a comment
	// sit at rank 2 and their children at rank 3, so the default answers
	// nothing below either, while a card sits at rank 1 and the default answers
	// its collections in full. The stub honours the depth it was asked for, so
	// a wrong-depth build fails twice here: on the argv, and on the empty
	// member list.
	const { spawner, calls } = stub({
		"contents wb-1/questions/1": contents("item", "wb-1/questions/1", [
			node("comment", "wb-1/questions/1/comments/1", "Round one"),
			node("comment", "wb-1/questions/1/comments/2", "Round two"),
		]),
		"contents wb-1/comments/1": contents("comment", "wb-1/comments/1", [
			node("attachment", "wb-1/comments/1/attachments/1", "shot.png"),
		]),
		"contents wb-1/sketches/1": contents("sketch", "wb-1/sketches/1", [
			node("scrap", "wb-1/sketches/1/scraps/1", "A scrap"),
		]),
		"show wb-1/questions/1": { ref: "wb-1/questions/1", text: "", comments: [] },
		"attachments wb-1/comments/1": {
			kind: "comment",
			ref: "wb-1/comments/1",
			attachments: [],
		},
	});
	const view = providerOver(spawner);

	const item: TreeElement = {
		kind: "item",
		row: rootRow(),
		root: ROOT,
		card: "wb-1",
		node: node("item", "wb-1/questions/1", "First", 2),
		isOperator: false,
	};
	const itemGroups = collectionsOf(await view.getChildren(item));
	assert.deepEqual(
		itemGroups.map((group) => group.memberKind),
		["comment"],
	);
	assert.equal(itemGroups[0].members.length, 2);
	assert.equal((await view.getChildren(itemGroups[0])).length, 2);
	assert.deepEqual(
		callsTo(calls, "contents")[0],
		pinned("contents", "wb-1/questions/1", "--depth", "all"),
	);

	const comment: TreeElement = {
		kind: "comment",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		node: node("comment", "wb-1/comments/1", "A thought", 1),
	};
	const commentGroups = collectionsOf(await view.getChildren(comment));
	assert.deepEqual(
		commentGroups.map((group) => group.memberKind),
		["attachment"],
	);
	assert.equal(commentGroups[0].members.length, 1);
	assert.deepEqual(
		callsTo(calls, "contents")[1],
		pinned("contents", "wb-1/comments/1", "--depth", "all"),
	);

	// The entity row, which is unreachable against today's grammar and
	// reachable from a stubbed contents answer. The assertion belongs here
	// rather than being left for the day the grammar grows a kind.
	const entity: TreeElement = {
		kind: "entity",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		node: node("sketch", "wb-1/sketches/1", "A sketch", 1),
	};
	assert.equal(treeItemFor(entity, ENGLISH).collapsibleState, "collapsed");
	const entityGroups = collectionsOf(await view.getChildren(entity));
	assert.equal(entityGroups.length, 1);
	assert.equal(entityGroups[0].members.length, 1);
	assert.deepEqual(
		callsTo(calls, "contents")[2],
		pinned("contents", "wb-1/sketches/1", "--depth", "all"),
	);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/21: a comment collection asks the way its holder is asked
// ---------------------------------------------------------------------------

/** A comment collection under the holder and holder kind given. */
function commentCollection(
	holder: string,
	holderKind: TreeElement["kind"],
	members: readonly TreeNode[],
): TreeElement {
	return {
		kind: "collection",
		row: rootRow(),
		root: ROOT,
		holder,
		holderKind,
		memberKind: "comment",
		members,
	};
}

test("a comment collection asks for its detail the way its holder is asked", async () => {
	// THE HOLDER'S REFERENCE IS MADE TO LIE ABOUT ITS KIND in both cases, which
	// is what refuses the one build these argv assertions would otherwise
	// admit. The form is chosen from the row kind the tree already holds, and
	// an implementation reading the holder's kind out of the REFERENCE'S SHAPE
	// instead, by looking for a /checklist/ segment or counting segments,
	// composes the same argv on every ordinarily shaped fixture and passes. So
	// the card case carries a holder reference with a /checklist/ segment in
	// it and the item case carries one shaped like a card's, and each still
	// asserts the argv its holderKind requires.
	const cardHolder = "wb-1/checklist/looks-like-an-item";
	const itemHolder = "wb-9";

	const cardStub = stub({ [`show ${cardHolder}`]: { comments: [] } });
	await providerOver(cardStub.spawner).getChildren(
		commentCollection(cardHolder, "card", [node("comment", `${cardHolder}/comments/1`, "One")]),
	);
	assert.deepEqual(callsTo(cardStub.calls, "show"), [
		pinned("show", cardHolder, "--fields", "comments"),
	]);

	// The item case is the ACCEPTING one, so it asserts what a successful join
	// produces as well as the request it made. It asserts nothing about the
	// icon, because every comment row carries the same one whether or not a
	// view arrived, so an icon assertion here could not fail.
	const joined: CommentView = {
		id: "c1",
		ref: `${itemHolder}/comments/1`,
		ts: "2026-09-15T09:00:00Z",
		author: "paul",
		body: "The ruling.",
	};
	const itemStub = stub({ [`show ${itemHolder}`]: { ref: itemHolder, comments: [joined] } });
	const drawn = await providerOver(itemStub.spawner).getChildren(
		commentCollection(itemHolder, "item", [node("comment", joined.ref, "The ruling.")]),
	);
	assert.deepEqual(callsTo(itemStub.calls, "show"), [pinned("show", itemHolder)]);
	const description = treeItemFor(drawn[0], ENGLISH).description as string;
	assert.ok(description.includes("1"), description);
	assert.ok(description.includes("paul"), description);
	assert.ok(description.includes("2026-09-15T09:00:00Z"), description);

	// Under a column no detail call is made at all, because there is no
	// structured reader for a column's comments, and the rows draw in the
	// degraded form.
	const columnStub = stub({});
	const degraded = await providerOver(columnStub.spawner).getChildren(
		commentCollection("doing", "column", [node("comment", "doing/comments/1", "A notice")]),
	);
	assert.deepEqual(columnStub.calls, []);
	assert.equal(degraded.length, 1);
	assert.equal(treeItemFor(degraded[0], ENGLISH).label, "A notice");
	assert.equal(treeItemFor(degraded[0], ENGLISH).description, undefined);
	assert.equal(treeItemFor(degraded[0], ENGLISH).contextValue, "dinah.comment");
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/22: an item collection and an attachment collection
// ---------------------------------------------------------------------------

test("an item collection asks its card, and the rows carry what the view supplied", async () => {
	const joined = {
		id: "b1",
		ordinal: 1,
		ref: "wb-1/questions/1",
		kind: "open_question",
		state: "pending",
		text: "Which vendor?",
		owner: "operator",
	};
	const { spawner, calls } = stub({
		"show wb-1": { card: { ref: "wb-1" }, checklist: [joined] },
	});
	const drawn = await providerOver(spawner).getChildren({
		kind: "collection",
		row: rootRow(),
		root: ROOT,
		holder: "wb-1",
		holderKind: "card",
		memberKind: "item",
		members: [node("item", "wb-1/questions/1", "Which vendor?")],
	});
	assert.deepEqual(callsTo(calls, "show"), [
		pinned("show", "wb-1", "--fields", "card,checklist"),
	]);
	const item = treeItemFor(drawn[0], ENGLISH);
	assert.ok((item.description as string).includes(ENGLISH("item.kind.question")), item.description);
	assert.ok((item.description as string).includes(ENGLISH("item.state.pending")), item.description);
	assert.ok((item.contextValue as string).startsWith("dinah.item."), item.contextValue);
});

test("an attachment collection asks against its own holder, at every holder the grammar gives one", async () => {
	// The wrong reference SUCCEEDS here, which is why this has to be asserted
	// on the argv rather than on the output: `attachments <card>` answers the
	// card's own attachments and exits ok, so a build composing the card's
	// reference under a comment joins nothing, draws every row with the view
	// absent, and produces output a test reading only the rows cannot tell
	// from a refusal.
	const holders: readonly {
		readonly holder: string;
		readonly holderKind: TreeElement["kind"];
		readonly member: string;
	}[] = [
		{ holder: "wb-1", holderKind: "card", member: "wb-1/attachments/1" },
		{
			holder: "wb-1/comments/1",
			holderKind: "comment",
			member: "wb-1/comments/1/attachments/1",
		},
		{ holder: "doing", holderKind: "column", member: "doing/attachments/1" },
		{ holder: "workbench", holderKind: "root", member: "workbench/attachments/1" },
	];

	for (const { holder, holderKind, member } of holders) {
		const { spawner, calls } = stub({
			[`attachments ${holder}`]: {
				kind: holderKind,
				ref: holder,
				attachments: [
					{
						id: "a1",
						ordinal: 1,
						ref: member,
						filename: "shot.png",
						provenance: "import",
						path: "C:/work/bench/shot.png",
					},
				],
			},
		});
		const drawn = await providerOver(spawner).getChildren({
			kind: "collection",
			row: rootRow(),
			root: ROOT,
			holder,
			holderKind,
			memberKind: "attachment",
			members: [node("attachment", member, "shot.png")],
		});
		assert.deepEqual(
			callsTo(calls, "attachments"),
			[pinned("attachments", holder)],
			`the collection under ${holderKind} ${holder} asked the wrong entity`,
		);
		assert.equal(drawn.length, 1);
		assert.equal(treeItemFor(drawn[0], ENGLISH).label, "shot.png");
	}
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/18: the wire mirror states CommentView.attachments
// ---------------------------------------------------------------------------

test("a CommentView payload decodes with its attachments, so the mirror cannot drop the member", () => {
	// The repository holds no automated wire-mirror guard and the convention is
	// carried by wire.ts's own header comment alone, so without this the Go
	// struct's member can stay unspelled here with nothing going red. The
	// member is read off the decoded value rather than only declared, so
	// dropping it from wire.ts fails this file to compile.
	const payload = JSON.parse(
		JSON.stringify({
			id: "c1",
			ref: "wb-1/comments/1",
			ts: "2026-09-15T09:00:00Z",
			author: "claude",
			body: "A thought",
			attachments: [
				{
					id: "a1",
					ordinal: 1,
					ref: "wb-1/comments/1/attachments/1",
					filename: "shot.png",
					provenance: "import",
				},
			],
		}),
	) as CommentView;
	const attachments = payload.attachments;
	assert.notEqual(attachments, undefined);
	assert.equal(attachments?.length, 1);
	assert.equal(attachments?.[0].filename, "shot.png");
	assert.equal(attachments?.[0].ref, "wb-1/comments/1/attachments/1");
});
