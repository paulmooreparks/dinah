// What the sidebar draws, asserted on the data rather than on a screenshot.
//
// Every fixture below is a literal of a shape internal/verb publishes, so a
// rename on the Go side that this extension has not answered shows up here as
// a field nobody reads rather than as an empty sidebar somebody reports.
//
// The provider is driven through a stub spawner, which is also what counts
// spawns: two of the criteria below are about how many processes a fold or an
// expand costs, and a spy on the spawner is the only place that is visible.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { WorkbenchResolution } from "../../src/api";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import {
	CONTEXT_CARD_ACTIVE,
	CONTEXT_CARD_BLOCKED,
	CONTEXT_CARD_READY_CLAIM,
	CONTEXT_CARD_READY_NONE,
	CONTEXT_COLUMN,
	CONTEXT_STATE_GROUP,
	CONTEXT_WORKBENCH_CANDIDATE,
	CONTEXT_WORKBENCH_FOREST,
	CONTEXT_WORKBENCH_ROOT,
} from "../../src/identity";
import type { FolderInput, TreeElement } from "../../src/tree";
import {
	DinahTreeProvider,
	actionsFor,
	cardDescription,
	cardIcon,
	columnDescription,
	columnRef,
	relativeTo,
	treeItemFor,
	workWord,
} from "../../src/tree";
import type { CardView, ColumnView } from "../../src/wire";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/** A column, with the fields a fixture usually overrides spelled out. */
function column(over: Partial<ColumnView> & { id: string }): ColumnView {
	return {
		slug: over.id,
		title: over.id,
		kind: "work",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: true,
		capacity: 0,
		count: 0,
		...over,
	};
}

/** A card as `dinah --json ls` reports it. */
function card(over: Partial<CardView> & { id: string }): CardView {
	return { ref: over.id, title: over.id, state: "ready", ...over };
}

/** A card leaf as `dinah --json tree` reports it. */
function leaf(id: string, title: string): Record<string, unknown> {
	return { kind: "card", id, ref: id, title, count: 1 };
}

/** A state-axis group node holding the leaves given. */
function stateGroup(
	value: string,
	children: Record<string, unknown>[],
): Record<string, unknown> {
	return {
		kind: "group",
		axis: "state",
		value,
		count: children.length,
		children,
	};
}

/** A column-axis group node holding whatever children it was given. */
function columnGroup(
	value: string,
	children: Record<string, unknown>[],
	count?: number,
): Record<string, unknown> {
	return {
		kind: "group",
		axis: "column",
		value,
		count: count ?? children.length,
		children,
	};
}

/** A whole `dinah --json tree` answer around the column groups given. */
function treeAnswer(columns: Record<string, unknown>[]): Record<string, unknown> {
	return {
		producer: "grouped",
		subject: "card",
		group_by: ["column", "state"],
		depth: "cards",
		root: {
			kind: "workbench",
			title: "Trees",
			count: columns.length,
			children: columns,
		},
	};
}

/** An ok spawn outcome carrying the payload given. */
function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

/** A spawner answering by which verb the argv names, counting every call. */
function stubSpawner(answers: Record<string, unknown>): {
	spawner: Spawner;
	calls: string[][];
} {
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		for (const [verb, payload] of Object.entries(answers)) {
			if (argv.includes(verb)) {
				return ok(payload);
			}
		}
		return { code: 2, stdout: JSON.stringify({ refusal: "dinah.no-such-verb" }), stderr: "" };
	};
	return { spawner, calls };
}

const RESOLVED: WorkbenchResolution = {
	state: "ok",
	root: "C:\\work\\bench",
	title: "Trees",
	source: "search",
	profile: "dinah-core/0.7",
	insideWorkspace: true,
};

function provider(spawner: Spawner, logged: string[] = []): DinahTreeProvider {
	return new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: (line) => logged.push(line),
		caseInsensitive: true,
		deadEndSentence: (refusal) => `no workbench: ${refusal}`,
	});
}

function folder(over: Partial<FolderInput> & { folder: string }): FolderInput {
	return { name: over.folder, resolution: RESOLVED, ...over };
}

// ---------------------------------------------------------------------------
// AC-1: the hierarchy is dinah's, and the state groups are whatever it gave
// ---------------------------------------------------------------------------

/**
 * The three-column bench AC-1 asks for.
 *
 * Intake carries a ready group and an active group and no blocked group,
 * which is the two-group shape a column that takes work up produces once an
 * empty blocked group is dropped. Review is the queue-kind column: its tree
 * node carries no state groups at all and its cards hang directly off it.
 * Done carries a blocked group alone.
 */
const THREE_COLUMNS = treeAnswer([
	// A column-axis node's own Count is the cards at or below it, which is
	// three here and not the two state groups directly under it. The counts
	// are spelled explicitly for that reason.
	columnGroup(
		"intake",
		[
			stateGroup("ready", [leaf("aaa", "Draw the guides"), leaf("bbb", "Translate the headings")]),
			stateGroup("active", [leaf("ccc", "Retire the second map")]),
		],
		3,
	),
	// The queue-kind column, drawn as dinah tree draws it once dinah-322
	// lands: no state groups at all, the cards standing directly beneath.
	columnGroup(
		"review",
		[leaf("ddd", "Confirm the pricing page"), leaf("eee", "Confirm the launch date")],
		2,
	),
	columnGroup("done", [stateGroup("blocked", [leaf("fff", "Retire the atlas")])], 1),
]);

const THREE_STATUS = {
	workbench: "Trees",
	root: "C:\\work\\bench",
	columns: [
		column({ id: "intake", title: "Intake", takes_work_up: true, count: 3 }),
		column({
			id: "review",
			title: "Customer approval",
			kind: "work",
			takes_work_up: false,
			awaiting_outside: true,
			count: 2,
		}),
		column({ id: "done", title: "Done", kind: "done", takes_work_up: false, count: 1 }),
	],
};

const THREE_LISTING = {
	cards: [
		card({ id: "aaa", ref: "tr-1", title: "Draw the guides", severity: "major", priority: "now" }),
		card({ id: "bbb", ref: "tr-2", title: "Translate the headings" }),
		card({ id: "ccc", ref: "tr-3", title: "Retire the second map", state: "active", holder: "alka" }),
		card({ id: "ddd", ref: "tr-4", title: "Confirm the pricing page" }),
		card({ id: "eee", ref: "tr-5", title: "Confirm the launch date" }),
		card({
			id: "fff",
			ref: "tr-6",
			title: "Retire the atlas",
			state: "blocked",
			block_kind: "external",
			block_reason: "waiting on the printer",
		}),
	],
};

async function loadedBench(): Promise<DinahTreeProvider> {
	const { spawner } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	return view;
}

test("the root is one workbench whose children are the declared columns in order", async () => {
	const view = await loadedBench();
	const roots = await view.getChildren();
	assert.equal(roots.length, 1);
	assert.equal(treeItemFor(roots[0]).label, "Trees");

	const columns = await view.getChildren(roots[0]);
	assert.deepEqual(
		columns.map((element) => treeItemFor(element).label),
		["Intake", "Customer approval", "Done"],
	);
	for (const element of columns) {
		assert.equal(treeItemFor(element).contextValue, CONTEXT_COLUMN);
	}
});

test("a column draws exactly the state groups the tree returned, two and not three", async () => {
	// The defect this pins is a provider that enumerates ready, active and
	// blocked for every column. Intake's own answer carries two groups, so a
	// third drawn here would be a group dinah did not return.
	const view = await loadedBench();
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const groups = await view.getChildren(intake);
	assert.deepEqual(
		groups.map((element) => treeItemFor(element).label),
		["Ready", "Active"],
	);
	for (const element of groups) {
		assert.equal(treeItemFor(element).contextValue, CONTEXT_STATE_GROUP);
	}
});

test("a column whose answer carries no state groups draws its cards with no group level", async () => {
	// The queue-kind shape, which dinah-322 is what makes dinah tree actually
	// produce. The provider reaches it through the same code path as the
	// two-group case above: it reads the children it was given and asks
	// nothing about the column's kind or its awaiting_outside flag.
	const view = await loadedBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const children = await view.getChildren(review);
	assert.deepEqual(
		children.map((element) => element.kind),
		["card", "card"],
	);
	assert.deepEqual(
		children.map((element) => treeItemFor(element).label),
		["Confirm the pricing page", "Confirm the launch date"],
	);
});

test("a queue column that does carry a state group draws it, and its cards still offer no Claim", async () => {
	// This is what trunk actually produces for an OCCUPIED queue column after
	// dinah-322: the column declares no state, so nothing is drawn from the
	// declared list, but the cards standing there carry `ready` and the
	// grouped producer draws a group for a value a card carries whatever the
	// workbench declares. Verified against a bench built from the binary at
	// 7438f8c: `column=intake count=2 -> children: group:ready`.
	//
	// The empty queue column is the other half and produces no groups at all
	// (`column=done count=0 -> children: (none)`), which is the shape the
	// fixture above draws. Both travel this same code path, which is the point:
	// the provider reads what it was given either way and decides nothing.
	const carried = treeAnswer([
		columnGroup(
			"review",
			[stateGroup("ready", [leaf("ddd", "Confirm the pricing page")])],
			1,
		),
	]);
	const { spawner } = stubSpawner({
		status: THREE_STATUS,
		tree: carried,
		ls: THREE_LISTING,
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	const [review] = await view.getChildren((await view.getChildren())[0]);
	const groups = await view.getChildren(review);
	assert.deepEqual(
		groups.map((element) => treeItemFor(element).label),
		["Ready"],
	);
	// The heading above the card says nothing about the card's own menu. The
	// column takes no work up, so no Claim is offered under it. The count is
	// asserted first, because a group that drew no cards would satisfy the
	// loop below without the loop having read a single contextValue.
	const carriedCards = await view.getChildren(groups[0]);
	assert.equal(carriedCards.length, 1, "the ready group drew no cards to check");
	for (const element of carriedCards) {
		assert.equal(treeItemFor(element).contextValue, CONTEXT_CARD_READY_NONE);
	}
});

test("cards inside a group come back in the fixture's own arrival order", async () => {
	const view = await loadedBench();
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const [ready] = await view.getChildren(intake);
	const cards = await view.getChildren(ready);
	assert.deepEqual(
		cards.map((element) => treeItemFor(element).label),
		["Draw the guides", "Translate the headings"],
	);
});

// ---------------------------------------------------------------------------
// AC-2: the ls join, and what a miss costs
// ---------------------------------------------------------------------------

test("a card's description is its two levels joined, or nothing", async () => {
	const view = await loadedBench();
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const [ready] = await view.getChildren(intake);
	const cards = await view.getChildren(ready);
	assert.equal(treeItemFor(cards[0]).description, "major · now");
	assert.equal(treeItemFor(cards[1]).description, "");
});

test("only one level set shows that level alone", () => {
	assert.equal(cardDescription({ id: "a", severity: "major" }), "major");
	assert.equal(cardDescription({ id: "a", priority: "now" }), "now");
	assert.equal(cardDescription(undefined), "");
});

test("a card the ls join missed renders undecorated rather than throwing", async () => {
	// The ordinary race: a card filed between the tree call and the ls call of
	// one checkpoint. The row still draws, with nothing but what tree gave it,
	// and the gap closes on the next checkpoint.
	const { spawner } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: { cards: [] },
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const [ready] = await view.getChildren(intake);
	const cards = await view.getChildren(ready);
	const item = treeItemFor(cards[0]);
	assert.equal(item.description, "");
	assert.equal(item.label, "Draw the guides");
});

// ---------------------------------------------------------------------------
// AC-3: which contextValue a ready card composes
// ---------------------------------------------------------------------------

test("a ready card at a column that takes work up offers Claim", () => {
	const takes = column({ id: "intake", takes_work_up: true, awaiting_outside: false });
	assert.equal(actionsFor({ state: "ready", column: takes }), CONTEXT_CARD_READY_CLAIM);
});

test("a ready card at an awaiting_outside column offers no Claim", () => {
	// Nobody with access to the workbench claims work here, so the take-up act
	// is absent rather than present and refused.
	const waiting = column({
		id: "approval",
		takes_work_up: false,
		awaiting_outside: true,
	});
	assert.equal(actionsFor({ state: "ready", column: waiting }), CONTEXT_CARD_READY_NONE);
});

test("a ready card at a buffer column offers no Claim either", () => {
	const buffer = column({
		id: "buffer",
		kind: "dinah.buffer",
		takes_work_up: false,
		awaiting_outside: false,
	});
	assert.equal(actionsFor({ state: "ready", column: buffer }), CONTEXT_CARD_READY_NONE);
});

test("a ready card whose column the join missed offers no Claim", () => {
	// The conservative default: say nothing is offered rather than offer an
	// act that may be refused.
	assert.equal(actionsFor({ state: "ready" }), CONTEXT_CARD_READY_NONE);
});

test("the ready cards of the queue column carry the no-Claim contextValue", async () => {
	// The assertion is about the card's own contextValue and holds whether or
	// not a Ready group heading was drawn above it: these cards stand directly
	// beneath their column row and still answer the same way.
	const view = await loadedBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const cards = await view.getChildren(review);
	assert.equal(cards.length, 2, "the queue column drew no cards to check");
	for (const element of cards) {
		assert.equal(treeItemFor(element).contextValue, CONTEXT_CARD_READY_NONE);
	}
});

// ---------------------------------------------------------------------------
// AC-4 and AC-5: the column row's description
// ---------------------------------------------------------------------------

test("the column row leads with the Work word and follows with the count", () => {
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	assert.equal(
		columnDescription(column({ id: "a", takes_work_up: true, count: 4 }), node),
		"taken, 4",
	);
	assert.equal(
		columnDescription(
			column({ id: "b", takes_work_up: false, awaiting_outside: true, count: 2 }),
			node,
		),
		"waiting, 2",
	);
	assert.equal(
		columnDescription(
			column({ id: "c", takes_work_up: false, awaiting_outside: false, count: 7 }),
			node,
		),
		"none, 7",
	);
});

test("awaiting_outside wins over takes_work_up, as the CLI's own renderer has it", () => {
	// renderColumns runs the three values most specific first, so a column
	// carrying both flags reads waiting. Mirrored rather than re-decided.
	assert.equal(
		workWord(column({ id: "a", takes_work_up: true, awaiting_outside: true })),
		"waiting",
	);
});

test("a declared capacity shows the count against it, and no capacity shows the count alone", () => {
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	assert.equal(
		columnDescription(column({ id: "a", capacity: 5, count: 3 }), node),
		"taken, 3/5",
	);
	assert.equal(
		columnDescription(column({ id: "a", capacity: 0, count: 3 }), node),
		"taken, 3",
	);
});

// ---------------------------------------------------------------------------
// OQ-3: the column-level join miss, mirroring AC-2's card-level coverage
// ---------------------------------------------------------------------------

test("a column the status join missed decorates conservatively and says so", async () => {
	// A column deleted between the status and tree calls of one checkpoint.
	// The row falls back to the group's own value for its label and to the
	// tree's own count, reads `none` because nothing published otherwise, and
	// the miss is reported to the output channel.
	const logged: string[] = [];
	const { spawner } = stubSpawner({
		status: { workbench: "Trees", columns: [] },
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const view = provider(spawner, logged);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	const columns = await view.getChildren((await view.getChildren())[0]);
	const item = treeItemFor(columns[0]);
	assert.equal(item.label, "intake");
	assert.equal(item.description, "none, 3");
	assert.equal(item.contextValue, CONTEXT_COLUMN);
	assert.ok(
		logged.some((line) => line.includes("intake")),
		`the join miss was not reported: ${logged.join(" | ")}`,
	);
});

test("the status and tree sides of the column join agree on the key", () => {
	// A column-axis group node carries no id, so the ref is the only key both
	// sides publish. bench.Column.Ref falls back from slug to identifier, and
	// this is that fallback computed on the other side of the wire.
	assert.equal(columnRef(column({ id: "abc", slug: "intake" })), "intake");
	assert.equal(columnRef({ ...column({ id: "abc" }), slug: undefined }), "abc");
	assert.equal(columnRef({ ...column({ id: "abc" }), slug: "" }), "abc");
});

// ---------------------------------------------------------------------------
// AC-6: active and blocked cards, wherever they stand
// ---------------------------------------------------------------------------

test("an active card's contextValue and icon do not depend on the column it stands at", () => {
	const here = column({ id: "intake", takes_work_up: true });
	const there = column({ id: "approval", takes_work_up: false, awaiting_outside: true });
	assert.equal(actionsFor({ state: "active", column: here }), CONTEXT_CARD_ACTIVE);
	assert.equal(actionsFor({ state: "active", column: there }), CONTEXT_CARD_ACTIVE);
	assert.deepEqual(cardIcon("active"), { id: "circle-filled", color: "charts.blue" });
});

test("a blocked card's contextValue and icon do not depend on the column it stands at", () => {
	const here = column({ id: "intake", takes_work_up: true });
	const there = column({ id: "done", kind: "done", takes_work_up: false });
	assert.equal(actionsFor({ state: "blocked", column: here }), CONTEXT_CARD_BLOCKED);
	assert.equal(actionsFor({ state: "blocked", column: there }), CONTEXT_CARD_BLOCKED);
	assert.deepEqual(cardIcon("blocked"), { id: "circle-slash", color: "charts.red" });
});

test("a ready card's icon carries no colour", () => {
	assert.deepEqual(cardIcon("ready"), { id: "circle-outline" });
});

test("an active card's tooltip names its holder and a blocked card's names its obstacle", async () => {
	const view = await loadedBench();
	const [intake, , done] = await view.getChildren((await view.getChildren())[0]);
	const [, active] = await view.getChildren(intake);
	const [activeCard] = await view.getChildren(active);
	assert.ok(treeItemFor(activeCard).tooltip?.includes("held by alka"));

	const [blocked] = await view.getChildren(done);
	const [blockedCard] = await view.getChildren(blocked);
	assert.ok(treeItemFor(blockedCard).tooltip?.includes("waiting on the printer"));
});

// ---------------------------------------------------------------------------
// AC-14: the root list in a multi-root window
// ---------------------------------------------------------------------------

const AMBIGUOUS: WorkbenchResolution = {
	state: "refused",
	refusal: "dinah.ambiguous-workbench",
	candidates: [
		{ title: "First", slug: "one", path: "C:\\multi\\second\\one" },
		{ title: "Second", slug: "two", path: "C:\\multi\\second\\two" },
	],
};

const NOTHING: WorkbenchResolution = {
	state: "refused",
	refusal: "dinah.no-workbench-found",
};

test("three folders produce one resolved row, two candidate rows and one dead end", async () => {
	const { spawner } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	// The third folder's own walk finds nothing, which is the case the single
	// informational row survives for.
	const walking: Spawner = async (exe, argv, options) => {
		if (argv.includes("--root")) {
			return ok({ root: "C:\\multi\\third", workbenches: [] });
		}
		return spawner(exe, argv, options);
	};
	const view = provider(walking);
	await view.load([
		folder({ folder: "C:\\work\\bench" }),
		folder({ folder: "C:\\multi\\second", resolution: AMBIGUOUS }),
		folder({ folder: "C:\\multi\\third", resolution: NOTHING }),
	]);

	const roots = await view.getChildren();
	const items = roots.map(treeItemFor);
	assert.deepEqual(
		items.map((item) => item.contextValue),
		[
			CONTEXT_WORKBENCH_ROOT,
			CONTEXT_WORKBENCH_CANDIDATE,
			CONTEXT_WORKBENCH_CANDIDATE,
			undefined,
		],
	);
	assert.deepEqual(items.map((item) => item.label).slice(1, 3), ["First", "Second"]);
	// A candidate row is never expanded by default: resolving it costs a spawn
	// and a window opening onto several would spend one on each.
	assert.equal(items[1].collapsibleState, "collapsed");
	assert.equal(items[2].collapsibleState, "collapsed");
	// The empty-forest folder is one flat row and nothing else.
	assert.equal(items[3].collapsibleState, "none");
	assert.equal(items[3].tooltip, "no workbench: dinah.no-workbench-found");
	assert.equal(await (await view.getChildren(roots[3])).length, 0);
});

test("a window holding exactly one row opens it, and a window holding several does not", async () => {
	const view = await loadedBench();
	assert.equal(treeItemFor((await view.getChildren())[0]).collapsibleState, "expanded");

	const { spawner } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const several = provider(spawner);
	await several.load([
		folder({ folder: "C:\\work\\bench" }),
		folder({ folder: "C:\\work\\other" }),
	]);
	const rows = await several.getChildren();
	assert.equal(rows.length, 2, "the two folders drew no root rows to check");
	for (const element of rows) {
		assert.equal(treeItemFor(element).collapsibleState, "collapsed");
	}
});

// ---------------------------------------------------------------------------
// AC-18: the forest walk answers for every workbench in one call
// ---------------------------------------------------------------------------

const FOREST_MEMBERS = [
	{ title: "Acme Co", slug: "acme", path: "C:\\customers\\acme\\board" },
	{ title: "Bell Industries", slug: "bell", path: "C:\\customers\\bell\\tracker" },
];

/** A spawner answering the three root-scoped calls with a two-member forest. */
function forestSpawner(
	members: readonly { title: string; slug?: string; path: string; refused?: string; unanswered?: string }[],
): { spawner: Spawner; calls: string[][] } {
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		const answering = members.map((member) => ({
			...member,
			...(member.refused === undefined && member.unanswered === undefined
				? argv.includes("tree")
					? { tree: THREE_COLUMNS }
					: argv.includes("status")
						? { status: THREE_STATUS }
						: { listing: THREE_LISTING }
				: {}),
		}));
		return ok({ root: "C:\\customers", workbenches: answering });
	};
	return { spawner, calls };
}

test("a folder holding a nested forest draws one row per member from three calls", async () => {
	const { spawner, calls } = forestSpawner(FOREST_MEMBERS);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);

	const roots = await view.getChildren();
	assert.deepEqual(
		roots.map((element) => treeItemFor(element).label),
		["Acme Co", "Bell Industries"],
	);
	for (const element of roots) {
		assert.equal(treeItemFor(element).contextValue, CONTEXT_WORKBENCH_FOREST);
	}
	// Three spawns for the folder, not three per member. A client walking the
	// tree itself would have paid six here and more on a real customer list.
	assert.equal(calls.length, 3);
	assert.deepEqual(
		calls.map((argv) => argv[1]).sort(),
		["ls", "status", "tree"],
	);
	for (const argv of calls) {
		assert.ok(argv.includes("--root"));
	}
});

test("a forest member's subtree is what that workbench alone would have drawn", async () => {
	const { spawner, calls } = forestSpawner(FOREST_MEMBERS);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	const before = calls.length;

	const [acme] = await view.getChildren();
	const columns = await view.getChildren(acme);
	assert.deepEqual(
		columns.map((element) => treeItemFor(element).label),
		["Intake", "Customer approval", "Done"],
	);
	// Expanding costs nothing: the subtree arrived with the walk.
	assert.equal(calls.length, before);
});

// ---------------------------------------------------------------------------
// AC-19: the three ways a row can fail to answer
// ---------------------------------------------------------------------------

test("a member the walk could not read at all is a flat row of path and refusal", async () => {
	const { spawner } = forestSpawner([
		{ title: "", path: "C:\\customers\\acme\\scratch", refused: "dinah.unreadable-workbench" },
	]);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	const [row] = await view.getChildren();
	const item = treeItemFor(row);
	assert.equal(item.label, "C:\\customers\\acme\\scratch");
	assert.equal(item.description, "dinah.unreadable-workbench");
	assert.equal(item.collapsibleState, "none");
	// No identity means no menu: there is nothing here to act on.
	assert.equal(item.contextValue, undefined);
	assert.deepEqual(await view.getChildren(row), []);
});

test("a member that gave up an identity and then would not open keeps that identity", async () => {
	const { spawner } = forestSpawner([
		{
			title: "Bell Industries",
			slug: "bell",
			path: "C:\\customers\\bell\\tracker",
			refused: "dinah.unreadable-workbench",
		},
	]);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	const [row] = await view.getChildren();
	const item = treeItemFor(row);
	// Throwing away an identity the row is already holding would leave a
	// reader unable to say which customer failed.
	assert.equal(item.label, "Bell Industries");
	assert.equal(item.description, "would not open");
	assert.equal(item.collapsibleState, "expanded");
	const children = await view.getChildren(row);
	assert.equal(children.length, 1);
	assert.ok(treeItemFor(children[0]).label.includes("would not open"));
});

test("a member that opened and declined this read is told apart from one that would not open", async () => {
	const { spawner } = forestSpawner([
		{
			title: "Carter LLP",
			slug: "carter",
			path: "C:\\customers\\carter\\board",
			unanswered: "dinah.unknown-column",
		},
	]);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	const [row] = await view.getChildren();
	const item = treeItemFor(row);
	assert.equal(item.label, "Carter LLP");
	// The three cases render three different ways, which is the whole reason
	// the wire carries refused and unanswered as two separate fields.
	assert.equal(item.description, "did not answer");
	assert.notEqual(item.description, "would not open");
	const children = await view.getChildren(row);
	assert.ok(treeItemFor(children[0]).label.includes("did not answer"));
});

test("a member that declined a read keeps its last-known subtree rather than blanking", async () => {
	// A passing hiccup must not empty a customer's tree. The first checkpoint
	// answers, the second declines, and the columns are still there.
	let declining = false;
	const spawner: Spawner = async (_exe, argv) => {
		const member = {
			title: "Carter LLP",
			slug: "carter",
			path: "C:\\customers\\carter\\board",
			...(declining
				? { unanswered: "dinah.unknown-column" }
				: argv.includes("tree")
					? { tree: THREE_COLUMNS }
					: argv.includes("status")
						? { status: THREE_STATUS }
						: { listing: THREE_LISTING }),
		};
		return ok({ root: "C:\\customers", workbenches: [member] });
	};
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	declining = true;
	await view.refresh("C:\\customers");

	const [row] = await view.getChildren();
	const children = await view.getChildren(row);
	assert.equal(treeItemFor(children[0]).label.includes("did not answer"), true);
	assert.deepEqual(
		children.slice(1).map((element) => treeItemFor(element).label),
		["Intake", "Customer approval", "Done"],
	);
});

// ---------------------------------------------------------------------------
// AC-20: a candidate resolves once and no more
// ---------------------------------------------------------------------------

test("expanding a candidate joins once, scoped to that candidate's own path", async () => {
	const { spawner, calls } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\multi\\second", resolution: AMBIGUOUS })]);
	assert.equal(calls.length, 0, "a candidate row is a stub until it is expanded");

	const [first] = await view.getChildren();
	const columns = await view.getChildren(first);
	assert.equal(calls.length, 3);
	for (const argv of calls) {
		// Scoped to the candidate, not to the workspace folder that holds it.
		assert.deepEqual(argv.slice(0, 3), ["--json", "--workbench", "C:\\multi\\second\\one"]);
	}
	assert.equal(columns.length, 3);

	// The second expand reads what the first resolved. A provider that
	// re-joined here would spawn three more processes every time a reader
	// folded a customer away and opened it again.
	await view.getChildren(first);
	assert.equal(calls.length, 3);
	await view.getChildren(first);
	assert.equal(calls.length, 3);
});

test("two expands racing each other still join once", async () => {
	const { spawner, calls } = stubSpawner({
		status: THREE_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\multi\\second", resolution: AMBIGUOUS })]);
	const [first] = await view.getChildren();
	await Promise.all([view.getChildren(first), view.getChildren(first)]);
	assert.equal(calls.length, 3);
});

// ---------------------------------------------------------------------------
// AC-21: telling two same-titled workbenches apart
// ---------------------------------------------------------------------------

test("a forest row carries its path and a folder-rooted row carries none", async () => {
	const { spawner } = forestSpawner([
		{ title: "Board", slug: "board", path: "C:\\customers\\acme\\board" },
	]);
	const nested = provider(spawner);
	await nested.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	assert.equal(treeItemFor((await nested.getChildren())[0]).description, "acme/board");

	const { spawner: plain } = stubSpawner({
		status: { workbench: "Board", root: "C:\\work\\bench", columns: [] },
		tree: treeAnswer([]),
		ls: { cards: [] },
	});
	const flat = provider(plain);
	await flat.load([folder({ folder: "C:\\work\\bench" })]);
	assert.equal(treeItemFor((await flat.getChildren())[0]).description, "");
});

test("a relative path is spelled POSIX-style and is measured segment by segment", () => {
	assert.equal(relativeTo("C:\\a\\b\\c", "C:\\a", true), "b/c");
	assert.equal(relativeTo("C:\\A\\B", "C:\\a", true), "B");
	// A sibling whose name merely begins with the folder's own is outside it,
	// so it is spelled absolutely rather than climbing out with dot-dots.
	assert.equal(relativeTo("C:\\ab\\c", "C:\\a", true), "C:/ab/c");
});

// ---------------------------------------------------------------------------
// AC-17: nothing anywhere offers a Pull
// ---------------------------------------------------------------------------

test("no row this provider composes ever names a pull", async () => {
	const view = await loadedBench();
	const seen: string[] = [];
	const walk = async (element?: TreeElement): Promise<void> => {
		for (const child of await view.getChildren(element)) {
			const item = treeItemFor(child);
			if (item.contextValue !== undefined) {
				seen.push(item.contextValue);
			}
			await walk(child);
		}
	};
	await walk();
	assert.ok(seen.length > 0, "the walk visited nothing, so it proved nothing");
	for (const value of seen) {
		assert.ok(
			!value.includes("pull"),
			`a row composed the contextValue ${value}, which names an act dinah cannot aim`,
		);
	}
});
