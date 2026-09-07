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

import type { BinaryState, WorkbenchResolution } from "../../src/api";
import type { CommandHost } from "../../src/cardCommands";
import type { CliOutcome, SpawnOutcome, Spawner } from "../../src/cli";
import { contextForPull } from "../../src/pullCommands";
import {
	COMMAND_OPEN_ATTACHMENT,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_CARD_BLOCKED,
	CONTEXT_CARD_READY_CLAIM,
	CONTEXT_CARD_READY_NONE,
	CONTEXT_COLUMN,
	CONTEXT_COLUMN_FULL,
	CONTEXT_COLUMN_FULL_PULL,
	CONTEXT_COLUMN_OPEN,
	CONTEXT_COLUMN_OPEN_PULL,
	CONTEXT_STATE_GROUP,
	CONTEXT_WORKBENCH_CANDIDATE,
	CONTEXT_WORKBENCH_FOREST,
	CONTEXT_WORKBENCH_ROOT,
} from "../../src/identity";
import type { FolderInput, TreeElement, TreeItemSpec, WorkbenchData } from "../../src/tree";
import {
	DinahTreeProvider,
	actionsFor,
	cardDescription,
	cardLabel,
	cardIcon,
	columnActionsFor,
	columnDescription,
	columnRef,
	columnTooltip,
	readWorkbench,
	relativeTo,
	treeItemFor,
} from "../../src/tree";
import {
	composeStatus,
	staleAfterMs,
	summarizeHolding,
} from "../../src/status";
import type { HoldingSummary, WorkbenchHoldingReport } from "../../src/status";
import { NO_CONFIGURED_WORKBENCH, parseRefusal } from "../../src/workbench";
import type {
	AttachmentListing,
	CardView,
	ColumnView,
	StatusAnswer,
} from "../../src/wire";

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

/**
 * The reports in a snapshot that carry a hand, narrowed to that arm.
 *
 * holdingSnapshot answers about every place the provider watches, including
 * the ones it could not read, so a test asking what a workbench reported has
 * to say which arm it means rather than indexing blindly into the array.
 */
function answered(
	snapshot: readonly WorkbenchHoldingReport[],
): Extract<WorkbenchHoldingReport, { state: "answered" }>[] {
	return snapshot.flatMap((entry) =>
		entry.state === "answered" ? [entry] : [],
	);
}

const RESOLVED: WorkbenchResolution = {
	state: "ok",
	root: "C:\\work\\bench",
	title: "Trees",
	source: "search",
	profile: "dinah-core/0.7",
	insideWorkspace: true,
};

/** A binary that resolved, so the bar's own trailer never moves in a test. */
const GOOD_BINARY: BinaryState = {
	state: "ok",
	path: "/usr/local/bin/dinah",
	source: "path",
	version: { tool: "v0.1.0-dev.42", profile: "dinah-core/0.4", format: 1 },
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
	// All three declare no capacity, so all three take another card and carry
	// the open spelling. This assertion read the bare CONTEXT_COLUMN until
	// dinah-331 gave a column row two acts of its own; the bare value now
	// means only that the status/tree join missed the column, which the
	// join-miss test below still pins.
	//
	// Customer approval is the one row that differs, and dinah-375 is why: it
	// is a queue with Done standing after it, so it carries the pull suffix.
	// Intake takes work up, and Done is a queue with nothing after it, so
	// neither does.
	assert.deepEqual(
		columns.map((element) => treeItemFor(element).contextValue),
		[CONTEXT_COLUMN_OPEN, CONTEXT_COLUMN_OPEN_PULL, CONTEXT_COLUMN_OPEN],
	);
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
		["tr-4: Confirm the pricing page", "tr-5: Confirm the launch date"],
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
		["tr-1: Draw the guides", "tr-2: Translate the headings"],
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

// ---------------------------------------------------------------------------
// dinah-337 AC-1: the reference leads the label
// ---------------------------------------------------------------------------

test("a card row's label is its reference, then its title", () => {
	// The order is the whole point. VS Code truncates a label from its end, so
	// the reference has to come first to survive a narrow sidebar, and the
	// operator ruled on that placement (dinah-337 OQ-1). Asserting the string
	// whole is what catches a separator or an order somebody swapped.
	assert.equal(
		cardLabel("dinah-337", "A card row does not say what to type"),
		"dinah-337: A card row does not say what to type",
	);
});

test("a card with no title renders its reference alone, and never a bare separator", () => {
	// The ls join can miss a card, and a tree node can carry a reference with
	// no title beside it. A naive template would draw "dinah-337: " with a
	// separator hanging off the end of it.
	assert.equal(cardLabel("dinah-337", undefined), "dinah-337");
	assert.equal(cardLabel("dinah-337", ""), "dinah-337");
});

test("a card with no reference renders its title alone, which is what the row drew before", () => {
	// This is the fallback the label already had, kept rather than replaced: a
	// node publishing no reference still names its card to a reader. The empty
	// case is the row that has neither, which draws nothing at all rather than
	// a separator.
	assert.equal(cardLabel("", "Draw the guides"), "Draw the guides");
	assert.equal(cardLabel("", undefined), "");
	assert.equal(cardLabel("", ""), "");
});

test("the card branch draws the new label and leaves description and tooltip alone", async () => {
	// AC-1's other half. The label changed and the two fields beside it did
	// not, so both are asserted here: a change that moved the reference into
	// the description strip instead would pass the helper tests above.
	const view = await loadedBench();
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const [ready] = await view.getChildren(intake);
	const cards = await view.getChildren(ready);
	const item = treeItemFor(cards[0]);
	assert.equal(item.label, "tr-1: Draw the guides");
	assert.equal(item.description, "major · now");
	// The tooltip already led with the reference before this card, and it still
	// does; the row is not where the reference was added twice.
	assert.equal(item.tooltip?.split("\n")[0], "tr-1");
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
	// The reference still leads the label, because tree publishes it on the
	// node and the join that missed only carries the levels (dinah-337 AC-1).
	assert.equal(item.label, "aaa: Draw the guides");
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

test("the column row's description is the bare count, plus waiting when the column declares it", () => {
	// dinah-373. takes_work_up no longer reaches the description at all: the
	// tree's own shape separates a column that takes work up from one that
	// holds it, so the two rows below read alike. Every case pins the whole
	// string, because a row that rendered nothing would satisfy a check that
	// only asked for the old words to be gone.
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	const cases: [string, ColumnView, string][] = [
		["work is taken up here", column({ id: "a", takes_work_up: true, count: 4 }), "4"],
		["work is only held here", column({ id: "b", takes_work_up: false, count: 4 }), "4"],
		[
			"awaiting somebody outside",
			column({ id: "c", takes_work_up: false, awaiting_outside: true, count: 2 }),
			"2, waiting",
		],
		[
			"awaiting outside while also taking work up",
			column({ id: "d", takes_work_up: true, awaiting_outside: true, count: 4 }),
			"4, waiting",
		],
	];
	for (const [name, view, expected] of cases) {
		assert.equal(columnDescription(view, node), expected, name);
	}
});

test("a declared capacity shows the count against it, and no capacity shows the count alone", () => {
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	assert.equal(
		columnDescription(column({ id: "a", capacity: 5, count: 3 }), node),
		"3/5",
	);
	assert.equal(
		columnDescription(column({ id: "a", capacity: 0, count: 3 }), node),
		"3",
	);
	// dinah-373 D-1. A limit is a fact about the column whether or not
	// anything currently sits against it, and the CLI's renderColumns already
	// reads 0/3 for the same column, so an empty limited column does not
	// collapse to a bare count that reads like a column with no limit.
	assert.equal(
		columnDescription(column({ id: "a", capacity: 3, count: 0 }), node),
		"0/3",
	);
});

test("the column row's tooltip says what the row's shape does not", () => {
	// dinah-373. The claim-or-pull-through fact the description used to spend
	// a word on lives here now, so this is its only home.
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	const cases: [string, ColumnView | undefined, string][] = [
		[
			"work is taken up here, and an agent moves a card out",
			column({ id: "a", title: "Intake", takes_work_up: true }),
			"Intake\nCards are claimed here.\nAn agent moves a card out.",
		],
		[
			"a queue column the operator owns",
			column({
				id: "b",
				title: "Customer approval",
				takes_work_up: false,
				operator_owned: true,
			}),
			"Customer approval\nA card here waits to be pulled onward.\nOnly the operator moves a card out.",
		],
		[
			"a column awaiting somebody outside says so on its own line",
			column({
				id: "c",
				title: "Printing",
				takes_work_up: false,
				awaiting_outside: true,
			}),
			"Printing\nA card here waits to be pulled onward.\nThis column is waiting on somebody outside the workbench.\nAn agent moves a card out.",
		],
		["the join missed the column, so nothing is described", undefined, "x"],
	];
	for (const [name, view, expected] of cases) {
		assert.equal(columnTooltip(view, node), expected, name);
	}
});

test("a queue with a column after it names that column, resolving the title where it can", () => {
	// dinah-375 AC-10. The next column arrives as a ref and, where the
	// status/tree join answered for it, as its view. The ref alone is what a
	// reader would have to look up themselves, so the title is shown when
	// there is one and the raw reference when there is not: a reader who can
	// see "spec" can find the column and a reader shown a blank cannot.
	const node = { kind: "group", axis: "column", value: "x", count: 0 };
	const queue = column({ id: "b", title: "Design Queue", takes_work_up: false });
	const spec = column({ id: "spec", title: "Spec" });
	assert.equal(
		columnTooltip(queue, node, spec, "spec"),
		"Design Queue\nA card here waits to be pulled onward.\nRight-click to pull the next ready card into Spec.\nAn agent moves a card out.",
		"the resolved view names the destination's title",
	);
	assert.equal(
		columnTooltip(queue, node, undefined, "spec"),
		"Design Queue\nA card here waits to be pulled onward.\nRight-click to pull the next ready card into spec.\nAn agent moves a card out.",
		"a ref the join has not resolved falls back to the raw reference",
	);
	// The line appears for a queue with a column after it and for nothing
	// else, so a work column's tooltip is byte-identical to what dinah-373
	// shipped even when a column stands after it.
	assert.equal(
		columnTooltip(column({ id: "a", title: "Intake", takes_work_up: true }), node, spec, "spec"),
		"Intake\nCards are claimed here.\nAn agent moves a card out.",
		"a work column is described as it was before",
	);
	assert.equal(
		columnTooltip(column({ id: "c", title: "Held", takes_work_up: false }), node),
		"Held\nA card here waits to be pulled onward.\nAn agent moves a card out.",
		"a queue standing last is described as it was before",
	);
});

// ---------------------------------------------------------------------------
// OQ-3: the column-level join miss, mirroring AC-2's card-level coverage
// ---------------------------------------------------------------------------

test("a column the status join missed decorates conservatively and says so", async () => {
	// A column deleted between the status and tree calls of one checkpoint.
	// The row falls back to the group's own value for its label and tooltip
	// and to the tree's own count for its description, and the miss is
	// reported to the output channel.
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
	assert.equal(item.description, "3");
	// The tooltip falls back to the same node value the label does, and says
	// nothing else: a column that published no facts is not described from
	// defaults.
	assert.equal(item.tooltip, "intake");
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
	answered: true,
	candidates: [
		{ title: "First", slug: "one", path: "C:\\multi\\second\\one" },
		{ title: "Second", slug: "two", path: "C:\\multi\\second\\two" },
	],
};

const NOTHING: WorkbenchResolution = {
	state: "refused",
	refusal: "dinah.no-workbench-found",
	answered: true,
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
	const items = roots.map((element) => treeItemFor(element));
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

test("only a queue row with a column after it names a pull", async () => {
	// This stood as "no row this provider composes ever names a pull", the
	// tree-side twin of the manifest guard dinah-375 D-5 retired, and its
	// stated reason was that dinah could not aim the act. dinah-280 published
	// the destination and dinah-375 gave the row the act, so the reason is
	// discharged and the guard would now refuse the feature it was written
	// before.
	//
	// What is worth keeping is the other half of the claim: no row of any
	// other kind names a pull. A card row, a state group, an attachment and
	// the workbench root are all still wrong places for it, and D-2 keeps a
	// work column out too.
	const view = await loadedBench();
	const seen: [string, string][] = [];
	const walk = async (element?: TreeElement): Promise<void> => {
		for (const child of await view.getChildren(element)) {
			const item = treeItemFor(child);
			if (item.contextValue !== undefined) {
				seen.push([child.kind, item.contextValue]);
			}
			await walk(child);
		}
	};
	await walk();
	assert.ok(seen.length > 0, "the walk visited nothing, so it proved nothing");
	const pulls = seen.filter(([, value]) => value.includes("pull"));
	assert.deepEqual(
		pulls,
		[["column", CONTEXT_COLUMN_OPEN_PULL]],
		"exactly one row of this bench is a queue with a column after it",
	);
});

// ---------------------------------------------------------------------------
// dinah-335: attachments at every level, drawn from the counts the reads
// already carry
// ---------------------------------------------------------------------------

/**
 * A `dinah --json attachments` answer, field-for-field with
 * verb.AttachmentListing and its views, so a field the Go side publishes that
 * this file has not mirrored is a compile error here rather than a silent
 * miss. The second view carries no `path`, which is how the wire spells a
 * payload that will not read: the attachment stays present and unopenable
 * rather than dropping off the row.
 */
const TWO_ATTACHMENTS: AttachmentListing = {
	kind: "card",
	ref: "tr-4",
	attachments: [
		{
			id: "9a1b2c3d4e5f",
			ordinal: 1,
			ref: "tr-4/attachments/1",
			filename: "screenshot.png",
			description: "the sidebar as the operator left it",
			provenance: "copy",
			path: "C:\\bench\\cards\\tr-4\\attachments\\screenshot.png",
		},
		{
			id: "0b2c3d4e5f61",
			ordinal: 2,
			ref: "tr-4/attachments/2",
			filename: "spec.pdf",
			provenance: "import",
		},
	],
};

/**
 * The three-column bench with attachments reported at the workbench, at
 * Intake (the grouped column) and at Customer approval (the queue shape), so
 * one fixture serves the column test and the root test at once.
 */
const ATTACHING_STATUS = {
	...THREE_STATUS,
	attachment_count: 5,
	columns: THREE_STATUS.columns.map((view) => {
		if (view.id === "intake") {
			return { ...view, attachment_count: 2 };
		}
		if (view.id === "review") {
			return { ...view, attachment_count: 4 };
		}
		return view;
	}),
};

/** A bench whose status reports attachments at the workbench and two columns. */
async function attachingBench(): Promise<{
	view: DinahTreeProvider;
	calls: string[][];
}> {
	const { spawner, calls } = stubSpawner({
		status: ATTACHING_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
		attachments: TWO_ATTACHMENTS,
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	return { view, calls };
}

/** The same card element with the attachment count replaced. */
function carryingAttachments(
	element: TreeElement,
	count: number | undefined,
): TreeElement {
	if (element.kind !== "card" || element.view === undefined) {
		throw new Error("the fixture element is not a card the ls join found");
	}
	return { ...element, view: { ...element.view, attachment_count: count } };
}

/** The same attachment element with the payload path replaced. */
function withPath(element: TreeElement, path: string | undefined): TreeElement {
	if (element.kind !== "attachment") {
		throw new Error("the fixture element is not an attachment");
	}
	return { ...element, view: { ...element.view, path } };
}

/** The WorkbenchData one resolved root element carries. */
function dataOf(element: TreeElement): WorkbenchData {
	if (element.kind !== "root" || element.row.data === undefined) {
		throw new Error("the fixture element is not a resolved root row");
	}
	return element.row.data;
}

/**
 * The row spec with the clicked element replaced by its kind, so two specs
 * built from two different elements compare as wholes. A spec's command
 * carries the element it was built from, and comparing whole specs without
 * this would fail on that recursion rather than on anything the row shows.
 */
function specOf(element: TreeElement): TreeItemSpec {
	const item = treeItemFor(element);
	return item.command === undefined
		? item
		: {
				...item,
				command: {
					...item.command,
					args: item.command.args.map((arg) => (arg as TreeElement).kind),
				},
			};
}

test("a card carrying attachments grows an expand arrow and changes nothing else about its row", async () => {
	// The plain card is the control: a view carrying no count at all, which
	// is what every card rendered as before attachments existed.
	const view = await loadedBench();
	const [intake] = await view.getChildren((await view.getChildren())[0]);
	const [ready] = await view.getChildren(intake);
	const [aaa] = await view.getChildren(ready);
	const plain = treeItemFor(aaa);
	assert.equal(plain.collapsibleState, "none");

	// A positive count grows the arrow. Every other field of the row is held
	// against the plain one, so a count that moved a label, a tooltip or a
	// menu answer fails here rather than shipping.
	const carrying = carryingAttachments(aaa, 2);
	const arrow = treeItemFor(carrying);
	assert.equal(arrow.collapsibleState, "collapsed");
	assert.deepEqual(specOf(carrying), {
		...specOf(aaa),
		collapsibleState: "collapsed",
	});
	// The click still opens the card the row stands for, element and all.
	assert.deepEqual(arrow.command?.args, [carrying]);

	// A zero and an explicit undefined read as absence: no arrow, and the
	// row a reader already knew.
	assert.deepEqual(specOf(carryingAttachments(aaa, 0)), specOf(aaa));
	assert.deepEqual(specOf(carryingAttachments(aaa, undefined)), specOf(aaa));
});

test("a card with attachments holds one Attachments row beneath it, drawn at no spawn", async () => {
	const { view, calls } = await attachingBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const [ddd] = await view.getChildren(review);
	// The fixture's card view carries no count, so the card draws nothing
	// beneath it and costs nothing.
	const before = calls.length;
	assert.deepEqual(await view.getChildren(ddd), []);

	const carrying = carryingAttachments(ddd, 2);
	const children = await view.getChildren(carrying);
	assert.equal(children.length, 1);
	const group = children[0];
	if (group.kind !== "attachmentsGroup") {
		assert.fail(`the card drew a ${group.kind} row, wanted an attachmentsGroup`);
	}
	// The group carries the card's own ref, which is what its expansion asks
	// about, and the eager count status already reported.
	assert.equal(group.ref, "tr-4");
	assert.equal(group.count, 2);
	assert.equal(group.root, "C:\\work\\bench");
	assert.equal(calls.length, before, "drawing the Attachments row spawned a call");
});

test("expanding a card's Attachments row asks once, named by the card's own ref", async () => {
	const { view, calls } = await attachingBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const [ddd] = await view.getChildren(review);
	const [group] = await view.getChildren(carryingAttachments(ddd, 2));
	const before = calls.length;
	await view.getChildren(group);
	assert.equal(calls.length - before, 1, "expanding the row did not cost exactly one call");
	assert.deepEqual(calls[calls.length - 1], [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"attachments",
		"tr-4",
	]);
});

test("expanding the workbench's own Attachments row asks with no ref at all", async () => {
	const { view, calls } = await attachingBench();
	const [root] = await view.getChildren();
	const children = await view.getChildren(root);
	const last = children[children.length - 1];
	if (last.kind !== "attachmentsGroup") {
		assert.fail(`the root drew a ${last.kind} row last, wanted an attachmentsGroup`);
	}
	// The workbench is asked about by omitting the argument, which is the
	// spelling the resolver owns; composing "workbench" here would be a second
	// spelling of a reference the binary already holds.
	assert.equal(last.ref, "");
	const before = calls.length;
	await view.getChildren(last);
	assert.equal(calls.length - before, 1, "expanding the row did not cost exactly one call");
	assert.deepEqual(calls[calls.length - 1], [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"attachments",
	]);
});

test("an expanded Attachments row draws the listing's own order, one row per attachment", async () => {
	const { view } = await attachingBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const [ddd] = await view.getChildren(review);
	const [group] = await view.getChildren(carryingAttachments(ddd, 2));
	const attachments = await view.getChildren(group);
	assert.deepEqual(
		attachments.map((element) => element.kind),
		["attachment", "attachment"],
	);
	assert.deepEqual(
		attachments.map((element) => treeItemFor(element).label),
		["screenshot.png", "spec.pdf"],
	);
});

test("an Attachments row that could not read draws one note row and names the failure", async () => {
	// No `attachments` entry in the answers, so the stub answers the call
	// with a refusal, which is what a checkpoint whose call failed looks
	// like here.
	const { spawner } = stubSpawner({
		status: ATTACHING_STATUS,
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const logged: string[] = [];
	const view = provider(spawner, logged);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const [ddd] = await view.getChildren(review);
	const [group] = await view.getChildren(carryingAttachments(ddd, 2));
	const children = await view.getChildren(group);
	assert.equal(children.length, 1);
	assert.equal(children[0].kind, "note");
	const item = treeItemFor(children[0]);
	assert.ok(
		item.label.includes("could not read the attachments"),
		`the note said: ${item.label}`,
	);
	// The channel line names both the ref and the root, which is what lets a
	// reader tell which entity and which workbench failed.
	assert.ok(
		logged.some((line) => line.includes("tr-4") && line.includes("C:\\work\\bench")),
		`the failure was not named with its ref and root: ${logged.join(" | ")}`,
	);
});

test("an attachment with a file opens on a plain click, and one without a file offers no click at all", async () => {
	const { view } = await attachingBench();
	const [, review] = await view.getChildren((await view.getChildren())[0]);
	const [ddd] = await view.getChildren(review);
	const [group] = await view.getChildren(carryingAttachments(ddd, 2));
	const [shot, spec] = await view.getChildren(group);

	// The openable half, which is also the control that proves the key can
	// be there at all: without it, the absence below could pass for a
	// command set to undefined.
	const openable = treeItemFor(shot);
	assert.deepEqual(openable.icon, { id: "file" });
	assert.equal(openable.command?.command, COMMAND_OPEN_ATTACHMENT);
	assert.deepEqual(openable.command?.args, [shot]);

	// The fixture's second attachment carries no path, which is how the
	// wire spells a payload that will not read. It stays on the row, and the
	// row offers no click.
	const missing = treeItemFor(spec);
	assert.deepEqual(missing.icon, { id: "warning" });
	assert.equal("command" in missing, false);
	assert.ok(missing.tooltip?.includes("no local file"));
	// The empty-string spelling reads the same as absence.
	const blanked = treeItemFor(withPath(spec, ""));
	assert.deepEqual(blanked.icon, { id: "warning" });
	assert.equal("command" in blanked, false);
});

test("a column with attachments carries its own Attachments row after every card", async () => {
	const { view } = await attachingBench();
	const columns = await view.getChildren((await view.getChildren())[0]);

	// The queue shape: two card leaves directly beneath the column, then the
	// column's own Attachments row last, in the tree's own order.
	const review = columns[1];
	const children = await view.getChildren(review);
	assert.deepEqual(
		children.map((element) => element.kind),
		["card", "card", "attachmentsGroup"],
	);
	const group = children[children.length - 1];
	if (group.kind !== "attachmentsGroup") {
		assert.fail("the column's own Attachments row was not drawn last");
	}
	assert.equal(group.count, 4);
	assert.equal(group.ref, "review");
	assert.equal(group.root, "C:\\work\\bench");

	// The grouped shape draws its state groups, then the same row last.
	const intake = columns[0];
	const groups = await view.getChildren(intake);
	assert.deepEqual(
		groups.map((element) => element.kind),
		["group", "group", "attachmentsGroup"],
	);

	// Under a state group the row is absent, because a state group is a
	// heading over cards and the attachments belong to the station.
	const [ready] = groups;
	const underReady = await view.getChildren(ready);
	assert.deepEqual(
		underReady.map((element) => element.kind),
		["card", "card"],
	);
});

test("a workbench with attachments draws its own Attachments row after every column", async () => {
	const { view } = await attachingBench();
	const [root] = await view.getChildren();
	const children = await view.getChildren(root);
	assert.deepEqual(
		children.map((element) => treeItemFor(element).label),
		["Intake", "Customer approval", "Done", "Attachments"],
	);
	const last = children[children.length - 1];
	if (last.kind !== "attachmentsGroup") {
		assert.fail("the workbench's own Attachments row was not drawn last");
	}
	assert.equal(last.count, 5);
	assert.equal(last.ref, "");
	assert.equal(treeItemFor(last).description, "5");
});

test("a workbench whose status reports no attachments draws no Attachments row", async () => {
	// The plain bench is the control: its status carries no count at all,
	// and no row of the new kind appears.
	const view = await loadedBench();
	const [root] = await view.getChildren();
	const children = await view.getChildren(root);
	assert.ok(children.length > 0, "the bench drew no rows to check");
	assert.equal(
		children.some((element) => element.kind === "attachmentsGroup"),
		false,
		"a row with no count to stand on was drawn",
	);
	// An explicit zero reads the same, which is also what the wire always
	// says: attachment_count is omitempty, so a zero count arrives as
	// absence and the two spellings must not draw different trees.
	const { spawner } = stubSpawner({
		status: { ...THREE_STATUS, attachment_count: 0 },
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const zero = provider(spawner);
	await zero.load([folder({ folder: "C:\\work\\bench" })]);
	const [zeroRoot] = await zero.getChildren();
	const zeroChildren = await zero.getChildren(zeroRoot);
	assert.equal(zeroChildren.some((element) => element.kind === "attachmentsGroup"), false);
});

test("a checkpoint whose reads fail keeps the attachment count the last good one carried", async () => {
	// Single-workbench mode. The first checkpoint answers with a count of
	// five, the second fails, and the count survives the failure on the same
	// terms the columns do: a passing failure must not blank the row's
	// attachments either.
	let declining = false;
	const spawner: Spawner = async (_exe, argv) => {
		if (declining) {
			return {
				code: 2,
				stdout: JSON.stringify({ refusal: "dinah.unreadable-workbench" }),
				stderr: "",
			};
		}
		for (const [verb, payload] of Object.entries({
			status: ATTACHING_STATUS,
			tree: THREE_COLUMNS,
			ls: THREE_LISTING,
		})) {
			if (argv.includes(verb)) {
				return ok(payload);
			}
		}
		return { code: 2, stdout: JSON.stringify({ refusal: "dinah.no-such-verb" }), stderr: "" };
	};
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	const [root] = await view.getChildren();
	assert.equal(dataOf(root).attachmentCount, 5);

	declining = true;
	await view.refresh("C:\\work\\bench");
	const [again] = await view.getChildren();
	assert.equal(dataOf(again).attachmentCount, 5);
	assert.equal(dataOf(again).columns.size, 3);
});

test("a forest member that declined a read keeps the attachment count it carried", async () => {
	// The forest twin of the test above, on the fixture the existing
	// declining-member test uses, so the held branch is exercised on both
	// reads that carry a workbench.
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
						? { status: ATTACHING_STATUS }
						: { listing: THREE_LISTING }),
		};
		return ok({ root: "C:\\customers", workbenches: [member] });
	};
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	const [first] = await view.getChildren();
	assert.equal(dataOf(first).attachmentCount, 5);

	declining = true;
	await view.refresh("C:\\customers");
	const [second] = await view.getChildren();
	assert.equal(dataOf(second).attachmentCount, 5);
});

// ---------------------------------------------------------------------------
// dinah-331 AC-2 and AC-3: which columns offer New Card, and how the row says so
// ---------------------------------------------------------------------------

test("a column row says whether it will take another card, from the two fields it publishes", () => {
	// dinah-331 AC-2. The table is the whole contract: a declared capacity the
	// count has reached is the one condition that closes the row, an undeclared
	// capacity never closes it however many cards stand there, and a column the
	// status/tree join missed answers neither way.
	//
	// The capacity 0 rows matter most. Zero is what a column that declares no
	// capacity reports, so reading it as a limit would close every such column
	// against a count of nothing, which is every column on a young workbench.
	const cases: [string, ColumnView | undefined, string][] = [
		["no capacity declared, and nothing in it", column({ id: "a" }), CONTEXT_COLUMN_OPEN],
		[
			"no capacity declared, and plenty in it",
			column({ id: "a", capacity: 0, count: 9 }),
			CONTEXT_COLUMN_OPEN,
		],
		[
			"capacity absent from the answer entirely",
			column({ id: "a", capacity: undefined, count: 9 }),
			CONTEXT_COLUMN_OPEN,
		],
		["room for one more", column({ id: "a", capacity: 3, count: 2 }), CONTEXT_COLUMN_OPEN],
		["exactly full", column({ id: "a", capacity: 3, count: 3 }), CONTEXT_COLUMN_FULL],
		["over its own limit", column({ id: "a", capacity: 3, count: 4 }), CONTEXT_COLUMN_FULL],
		["the join missed the column", undefined, CONTEXT_COLUMN],
	];
	for (const [name, view, expected] of cases) {
		assert.equal(columnActionsFor(view), expected, name);
	}
});

test("the pull suffix is decided by the column standing next, independently of capacity", () => {
	// dinah-375 AC-1. Two axes, eight rows: {takes_work_up} x {a column
	// stands next} x {capacity reached}. Capacity never gates a pull, because
	// a pull takes a card out of the queue rather than putting one in, so the
	// suffix has to appear on the full spelling as well as the open one.
	//
	// The work-column rows are the ones this table exists for. A work column
	// has a column standing after it as readily as a queue does, so a function
	// that read the next column alone would offer the act there, and D-2 says
	// it must not.
	const cases: [string, ColumnView | undefined, string | undefined, string][] = [
		[
			"a queue with room and a column after it",
			column({ id: "a", takes_work_up: false }),
			"spec",
			CONTEXT_COLUMN_OPEN_PULL,
		],
		[
			"a queue at capacity with a column after it",
			column({ id: "a", takes_work_up: false, capacity: 3, count: 3 }),
			"spec",
			CONTEXT_COLUMN_FULL_PULL,
		],
		[
			"a queue standing last in the flow",
			column({ id: "a", takes_work_up: false }),
			undefined,
			CONTEXT_COLUMN_OPEN,
		],
		[
			"a queue at capacity standing last in the flow",
			column({ id: "a", takes_work_up: false, capacity: 3, count: 3 }),
			undefined,
			CONTEXT_COLUMN_FULL,
		],
		[
			"a work column with a column after it all the same",
			column({ id: "a", takes_work_up: true }),
			"spec",
			CONTEXT_COLUMN_OPEN,
		],
		[
			"a work column at capacity with a column after it",
			column({ id: "a", takes_work_up: true, capacity: 3, count: 3 }),
			"spec",
			CONTEXT_COLUMN_FULL,
		],
		[
			"a work column standing last in the flow",
			column({ id: "a", takes_work_up: true }),
			undefined,
			CONTEXT_COLUMN_OPEN,
		],
		[
			"a column the status/tree join missed, whatever stands after it",
			undefined,
			"spec",
			CONTEXT_COLUMN,
		],
	];
	for (const [name, view, next, expected] of cases) {
		assert.equal(columnActionsFor(view, next), expected, name);
	}
});

test("the row's own tooltip names the column standing after it, resolved to its title", async () => {
	// dinah-375 AC-11. The table above drives columnTooltip directly with the
	// two facts handed to it. This drives the whole join, so a treeItemFor
	// that stopped passing element.nextColumn would show the raw reference
	// here and go red, rather than passing on the strength of a unit test of
	// an argument nothing supplies.
	const { spawner } = stubSpawner({
		status: {
			workbench: "Trees",
			root: "C:\\work\\bench",
			columns: [
				column({ id: "intake", title: "Intake", count: 3 }),
				column({
					id: "review",
					title: "Design Queue",
					takes_work_up: false,
					count: 2,
				}),
				column({ id: "done", title: "Specification", count: 1 }),
			],
		},
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const treeView = provider(spawner);
	await treeView.load([folder({ folder: "C:\\work\\bench" })]);
	const columns = await treeView.getChildren((await treeView.getChildren())[0]);
	const item = treeItemFor(columns[1]);
	assert.equal(
		item.tooltip,
		"Design Queue\nA card here waits to be pulled onward.\nRight-click to pull the next ready card into Specification.\nAn agent moves a card out.",
	);
	assert.equal(item.contextValue, CONTEXT_COLUMN_OPEN_PULL);
});

test("two queues standing in a row each pull into their own next column", async () => {
	// dinah-375 AC-12, the criterion the operator's OQ-1 ruling was filed
	// against. Intake and Waiting are both queues, so before the ruling both
	// rows published the far end of the chain and clicking Intake advanced a
	// card that was standing in Waiting, out of a row the reader was not
	// looking at.
	//
	// The fixture is the tree fixture the rest of this file already uses, with
	// a status that makes its first two columns queues. Both destinations are
	// read off the real provider rather than assembled by hand, so a columnsOf
	// that walked past a queue would answer "doing" for both rows here.
	const { spawner } = stubSpawner({
		status: {
			workbench: "Trees",
			root: "C:\\work\\bench",
			columns: [
				column({ id: "intake", title: "Intake", takes_work_up: false, count: 3 }),
				column({ id: "review", title: "Waiting", takes_work_up: false, count: 2 }),
				column({ id: "done", title: "Doing", takes_work_up: true, count: 1 }),
			],
		},
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const treeView = provider(spawner);
	await treeView.load([folder({ folder: "C:\\work\\bench" })]);
	const columns = await treeView.getChildren((await treeView.getChildren())[0]);
	const [intake, waiting] = columns;

	assert.equal(intake.kind === "column" ? intake.nextColumnRef : undefined, "review");
	assert.equal(waiting.kind === "column" ? waiting.nextColumnRef : undefined, "done");

	// Both rows offer the act, and each names its own neighbour. The
	// inequality is the whole point: one destination shared between the two
	// rows is the defect this criterion pins.
	assert.equal(treeItemFor(intake).contextValue, CONTEXT_COLUMN_OPEN_PULL);
	assert.equal(treeItemFor(waiting).contextValue, CONTEXT_COLUMN_OPEN_PULL);

	const host = {} as CommandHost;
	const first = contextForPull(intake, "dinah", host, spawner);
	const second = contextForPull(waiting, "dinah", host, spawner);
	assert.equal(first?.destination, "review", "Intake pulls into Waiting");
	assert.equal(second?.destination, "done", "Waiting pulls into Doing");
	assert.notEqual(
		first?.destination,
		second?.destination,
		"neither queue's destination is computed by walking past the other",
	);
	assert.equal(first?.label, "Intake");
	assert.equal(second?.label, "Waiting");
});

test("the column row's other four fields are what they were before the contextValue moved", async () => {
	// dinah-331 AC-3. Changing one field of a TreeItemSpec is the kind of edit
	// that quietly takes a neighbouring field with it, so the label, the
	// description, the tooltip and the collapsible state are pinned here
	// against the same fixture the AC-1 tests draw.
	const view = await loadedBench();
	const roots = await view.getChildren();
	const columns = await view.getChildren(roots[0]);
	const item = treeItemFor(columns[0]);
	assert.equal(item.label, "Intake");
	assert.equal(item.description, "3");
	assert.equal(
		item.tooltip,
		"Intake\nCards are claimed here.\nAn agent moves a card out.",
	);
	assert.equal(item.collapsibleState, "expanded");
	assert.equal(item.contextValue, CONTEXT_COLUMN_OPEN);
});

test("a column standing at its declared capacity draws the full suffix through the provider", async () => {
	// The table above drives columnActionsFor directly. This drives the whole
	// join, so a treeItemFor that stopped calling columnActionsFor at all would
	// be caught here rather than passing on the strength of a unit test of a
	// function nothing calls.
	const { spawner } = stubSpawner({
		status: {
			workbench: "Trees",
			root: "C:\\work\\bench",
			columns: [
				column({ id: "intake", title: "Intake", capacity: 3, count: 3 }),
				column({ id: "review", title: "Customer approval", capacity: 9, count: 2 }),
				column({ id: "done", title: "Done", count: 1 }),
			],
		},
		tree: THREE_COLUMNS,
		ls: THREE_LISTING,
	});
	const treeView = provider(spawner);
	await treeView.load([folder({ folder: "C:\\work\\bench" })]);
	const columns = await treeView.getChildren((await treeView.getChildren())[0]);
	assert.deepEqual(
		columns.map((element) => treeItemFor(element).contextValue),
		[CONTEXT_COLUMN_FULL, CONTEXT_COLUMN_OPEN, CONTEXT_COLUMN_OPEN],
	);
});

// ---------------------------------------------------------------------------
// dinah-366: the column a malformed read named is carried through and marked
// ---------------------------------------------------------------------------

/** The three columns the marker fixtures below draw, keyed by slug as ever. */
const DAMAGED_STATUS = {
	workbench: "Trees",
	root: "C:\\work\\bench",
	columns: [
		column({ id: "b00000000001", slug: "backlog", title: "Backlog", count: 2 }),
		column({ id: "b00000000002", slug: "doing", title: "Doing", count: 1 }),
		column({ id: "b00000000003", slug: "done", title: "Done", count: 0 }),
	],
};

const DAMAGED_TREE = treeAnswer([
	columnGroup("backlog", [stateGroup("ready", [leaf("aaa", "One"), leaf("bbb", "Two")])], 2),
	columnGroup("doing", [stateGroup("active", [leaf("ccc", "Three")])], 1),
	columnGroup("done", [], 0),
]);

const DAMAGED_LISTING = {
	cards: [
		card({ id: "aaa", ref: "tr-1", title: "One" }),
		card({ id: "bbb", ref: "tr-2", title: "Two" }),
		card({ id: "ccc", ref: "tr-3", title: "Three", state: "active", holder: "alka" }),
	],
};

/** The refusal envelope a malformed column raises, exit code and all. */
function malformedColumn(context: Record<string, string>): SpawnOutcome {
	return {
		code: 2,
		stdout: JSON.stringify({
			outcome: "refused",
			refusal: "malformed",
			detail: "column b00000000002",
			context,
		}),
		stderr: "",
	};
}

/**
 * A spawner whose `tree` call refuses over a malformed column while status
 * and ls answer normally, which is exactly the shape a hand-edited column
 * file produces: the workbench opens far enough to be listed and the read
 * that carries the hierarchy is the one that gives up.
 *
 * `answering` lets one provider load a good checkpoint first and then decline
 * the next, so the cached columns the marker attaches to are real rather than
 * hand-placed.
 */
function malformedTreeSpawner(context: Record<string, string>): {
	spawner: Spawner;
	answering: { value: boolean };
} {
	const answering = { value: true };
	const spawner: Spawner = async (_exe, argv) => {
		if (argv.includes("tree")) {
			return answering.value ? ok(DAMAGED_TREE) : malformedColumn(context);
		}
		if (argv.includes("status")) {
			return ok(DAMAGED_STATUS);
		}
		if (argv.includes("ls")) {
			return ok(DAMAGED_LISTING);
		}
		return { code: 2, stdout: JSON.stringify({ refusal: "dinah.no-such-verb" }), stderr: "" };
	};
	return { spawner, answering };
}

const DAMAGED_CONTEXT = {
	path: "C:\\work\\bench\\columns\\b00000000002\\column.md",
	workbench: "C:\\work\\bench",
	column: "b00000000002",
};

test("a refused tree read carries its detail and the column it named into the row's data", async () => {
	const { spawner, answering } = malformedTreeSpawner(DAMAGED_CONTEXT);
	answering.value = false;
	const data = await readWorkbench(spawner, "dinah", "C:\\work\\bench", () => {}, undefined);
	assert.equal(data.unanswered, "malformed");
	assert.equal(data.unansweredDetail, "column b00000000002");
	// Read from the declared context field and from nothing else: the detail
	// beside it is composed English and is never parsed for an identifier.
	assert.equal(data.unansweredColumn, "b00000000002");
});

test("a refusal whose context names no column leaves the column undefined and the rest intact", async () => {
	// What an older dinah on the caller's PATH produces, and equally what any
	// malformed refusal about something other than a column produces.
	const { spawner, answering } = malformedTreeSpawner({
		path: "C:\\work\\bench\\workbench.md",
		workbench: "C:\\work\\bench",
	});
	answering.value = false;
	const data = await readWorkbench(spawner, "dinah", "C:\\work\\bench", () => {}, undefined);
	assert.equal(data.unanswered, "malformed");
	assert.equal(data.unansweredDetail, "column b00000000002");
	assert.equal(data.unansweredColumn, undefined);
});

test("the marker lands on the column the refusal named and on neither of its neighbours", async () => {
	// A check that some row shows a warning would pass on a provider that
	// marked every column, so all three rows are asserted and the two healthy
	// ones are asserted to be untouched.
	const { spawner, answering } = malformedTreeSpawner(DAMAGED_CONTEXT);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	answering.value = false;
	await view.refresh("C:\\work\\bench");

	const [root] = await view.getChildren();
	// The workbench row's own note row stands ahead of the columns when a read
	// declined, so the column rows are selected rather than sliced off by index.
	const columns = (await view.getChildren(root)).filter(
		(element) => element.kind === "column",
	);
	const items = columns.map((element) => treeItemFor(element));
	assert.deepEqual(
		items.map((item) => item.label),
		["Backlog", "Doing", "Done"],
	);
	assert.deepEqual(
		items.map((item) => item.description),
		["2", "damaged", "0"],
	);
	assert.equal(items[0].icon, undefined);
	assert.equal(items[2].icon, undefined);
	assert.deepEqual(items[1].icon, { id: "warning" });
	// Named, not merely flagged: the hover carries the column's own cached
	// title, which is what tells a reader which file to open.
	assert.ok(items[1].tooltip?.includes("Doing"));
	assert.ok(items[1].tooltip?.includes("malformed: column b00000000002"));
});

test("a column the last good read never cached leaves the marker off and says so on the workbench row", async () => {
	// A column added since the last successful read cannot be matched, so no
	// row is marked at all. The reader is told something is broken and not
	// where, which is the honest answer rather than a marker on the wrong row.
	const { spawner, answering } = malformedTreeSpawner({
		...DAMAGED_CONTEXT,
		column: "b00000000009",
	});
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\work\\bench" })]);
	answering.value = false;
	await view.refresh("C:\\work\\bench");

	const [root] = await view.getChildren();
	const columns = (await view.getChildren(root)).filter(
		(element) => element.kind === "column",
	);
	// The labels are pinned before the loop runs, because a loop over an empty
	// list passes for the wrong reason and a fixture change could empty it.
	const items = columns.map((element) => treeItemFor(element));
	assert.deepEqual(
		items.map((item) => item.label),
		["Backlog", "Doing", "Done"],
	);
	for (const item of items) {
		assert.notEqual(item.description, "damaged");
		assert.equal(item.icon, undefined);
	}
	const rootItem = treeItemFor(root);
	assert.equal(rootItem.description, "did not answer");
	assert.ok(rootItem.tooltip?.includes("malformed: column b00000000002"));
});

// ---------------------------------------------------------------------------
// dinah-419: what status says the reader is holding
// ---------------------------------------------------------------------------

/**
 * A `dinah --json status` answer as the binary actually emits one, written
 * out as text rather than as an object literal.
 *
 * The text is what a mirror can be wrong about. An object literal is checked
 * against the interface by the compiler and so agrees with it whatever either
 * one says, while a parse of real output fails when the mirror has drifted
 * from the field names the Go tags publish.
 */
const STATUS_JSON = `{
	"workbench": "Trees",
	"root": "C:\\\\work\\\\bench",
	"actor": "alka",
	"is_operator": false,
	"operator": "paul",
	"profile": "dinah-core/0.7",
	"columns": [],
	"holding": [
		{
			"id": "aaa",
			"ref": "tr-3",
			"title": "Retire the second map",
			"column": "doing",
			"state": "active",
			"holder": "alka",
			"claim_since": "2026-09-07T09:00:00Z",
			"expires": "2026-09-07T13:30:00Z",
			"revision": "r1",
			"blocking_items": 2
		}
	],
	"blocked": [],
	"workbench_source": "search",
	"attachment_count": 1
}`;

test("the status mirror carries the actor, the holding list and each claim's own stamps", async () => {
	// AC-1's first half. Every field this card added to the mirror is read off
	// a parse of real output and asserted with its type, because a field
	// spelled wrongly here reads as undefined rather than as an error.
	const answer = JSON.parse(STATUS_JSON) as StatusAnswer;
	assert.equal(answer.actor, "alka");
	assert.equal(answer.is_operator, false);
	assert.equal(typeof answer.is_operator, "boolean");
	assert.equal(answer.operator, "paul");
	assert.equal(answer.workbench_source, "search");
	assert.ok(Array.isArray(answer.holding));
	assert.ok(Array.isArray(answer.blocked));
	assert.deepEqual(answer.blocked, []);
	assert.equal(answer.holding.length, 1);
	const first = answer.holding[0];
	assert.equal(first.ref, "tr-3");
	assert.equal(first.claim_since, "2026-09-07T09:00:00Z");
	assert.equal(first.expires, "2026-09-07T13:30:00Z");
	assert.equal(first.blocking_items, 2);
	assert.equal(typeof first.blocking_items, "number");
});

test("a status answer carrying no holding list leaves an empty hand rather than throwing", async () => {
	// AC-1's second half. The three fields carry no omitempty on the Go side,
	// so a current binary always writes them, but a binary older than the
	// field would not, and a reader that trusts the mirror's promise would
	// throw on the first read rather than degrade.
	const { spawner } = stubSpawner({
		status: { workbench: "Trees", root: "C:\\work\\bench", columns: [] },
		tree: treeAnswer([]),
		ls: { cards: [] },
	});
	const data = await readWorkbench(spawner, "dinah", "C:\\work\\bench", () => {});
	assert.deepEqual([...data.holding], []);
	assert.equal(data.actor, undefined);
});

test("what the reader holds is read off the status call the tree already makes", async () => {
	const held = {
		id: "aaa",
		ref: "tr-3",
		title: "Retire the second map",
		state: "active",
		holder: "alka",
		claim_since: "2026-09-07T09:00:00Z",
		expires: "2026-09-07T13:30:00Z",
	};
	const { spawner, calls } = stubSpawner({
		status: {
			workbench: "Trees",
			root: "C:\\work\\bench",
			actor: "alka",
			is_operator: false,
			columns: [],
			holding: [held],
			blocked: [],
		},
		tree: treeAnswer([]),
		ls: { cards: [] },
	});
	const data = await readWorkbench(
		spawner,
		"dinah",
		"C:\\work\\bench",
		() => {},
		undefined,
		() => 7_000,
	);
	assert.equal(data.actor, "alka");
	assert.deepEqual([...data.holding], [held]);
	assert.equal(data.fetchedAt, 7_000);
	// Three calls and no fourth: the holding list rode the status call the
	// tree was making anyway.
	assert.equal(calls.length, 3);
	assert.deepEqual(
		calls.map((argv) => argv[argv.length - 1]).sort(),
		["ls", "status", "tree"],
	);
});

test("two folders resolving to one workbench report one held hand between them", async () => {
	// AC-9. Folders A and B resolve to the same root spelled two ways, and a
	// third folder resolves to a workbench of its own whose status call fails
	// after an initial success.
	const SHARED = "C:\\work\\bench";
	const OTHER = "C:\\work\\other";
	const failing = { value: false };
	let reading = 1_000;

	const held = {
		id: "aaa",
		ref: "tr-3",
		state: "active",
		holder: "alka",
		expires: "2026-09-07T13:30:00Z",
	};
	const statusFor = (root: string): Record<string, unknown> => ({
		workbench: root === SHARED ? "Trees" : "Maps",
		root,
		actor: "alka",
		is_operator: false,
		columns: [],
		holding: root === SHARED ? [held] : [],
		blocked: [],
	});
	const spawner: Spawner = async (_exe, argv) => {
		const root = argv[argv.indexOf("--workbench") + 1] ?? "";
		const verb = argv[argv.length - 1];
		if (verb === "status") {
			if (failing.value && root.toLowerCase() === OTHER.toLowerCase()) {
				return {
					code: 2,
					stdout: JSON.stringify({ refusal: "dinah.unreachable" }),
					stderr: "",
				};
			}
			return ok(statusFor(root));
		}
		if (verb === "tree") {
			return ok(treeAnswer([]));
		}
		return ok({ cards: [] });
	};

	const view = new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: () => {},
		caseInsensitive: true,
		deadEndSentence: (refusal) => refusal,
		now: () => reading,
	});
	await view.load([
		folder({ folder: "C:\\ws\\a", resolution: { ...RESOLVED, root: SHARED } }),
		// The same workbench, reached through a second folder and spelled with
		// different case and separators, which is what the deduplication key
		// has to see through.
		folder({
			folder: "C:\\ws\\b",
			resolution: { ...RESOLVED, root: "c:/work/BENCH" },
		}),
		folder({
			folder: "C:\\ws\\c",
			resolution: { ...RESOLVED, root: OTHER, title: "Maps" },
		}),
	]);

	const first = answered(view.holdingSnapshot());
	assert.deepEqual(
		first.map((entry) => entry.source),
		[SHARED, OTHER],
		"the shared workbench appears once and the other appears beside it",
	);
	assert.deepEqual(
		first.map((entry) => entry.holding.length),
		[1, 0],
	);
	assert.deepEqual(
		first.map((entry) => entry.fetchedAt),
		[1_000, 1_000],
	);

	// The third folder's status call now fails. Its last good answer stands,
	// and its fetchedAt does not advance, so the failure reads as staleness
	// rather than as a fresh empty hand.
	failing.value = true;
	reading = 9_000;
	await view.refresh("C:\\ws\\c");
	const second = answered(view.holdingSnapshot());
	const other = second.find((entry) => entry.source === OTHER);
	assert.notEqual(other, undefined);
	assert.equal(other?.fetchedAt, 1_000);

	// The shared workbench's own entry is untouched by that failure, and one
	// entry is still all it gets.
	assert.deepEqual(
		second.map((entry) => entry.source),
		[SHARED, OTHER],
	);
	assert.equal(second[0].fetchedAt, 1_000);

	// A successful read does advance it, which is what makes the assertion
	// above a real one rather than a stamp that never moves at all.
	failing.value = false;
	reading = 12_000;
	await view.refresh("C:\\ws\\c");
	assert.equal(
		answered(view.holdingSnapshot()).find((entry) => entry.source === OTHER)
			?.fetchedAt,
		12_000,
	);
});

test("a forest member that declined a read keeps the hand it was last confirmed to hold", async () => {
	// AC-9's other read. The root-scoped walk answers for every workbench
	// beneath a folder in one call each, so the same rule about what a failed
	// read may and may not change has to hold on that path too, and a test
	// covering only the single-workbench read would leave it unguarded.
	let declining = false;
	let statusRefusing = false;
	let reading = 1_000;
	const carried = {
		id: "aaa",
		ref: "tr-3",
		state: "active",
		holder: "alka",
		expires: "2026-09-07T13:30:00Z",
	};
	const spawner: Spawner = async (_exe, argv) => {
		if (statusRefusing && argv.includes("status")) {
			return {
				code: 2,
				stdout: JSON.stringify({ refusal: "dinah.unreachable" }),
				stderr: "",
			};
		}
		const member = {
			title: "Carter LLP",
			slug: "carter",
			path: "C:\\customers\\carter\\board",
			...(declining
				? { unanswered: "dinah.unknown-column" }
				: argv.includes("tree")
					? { tree: THREE_COLUMNS }
					: argv.includes("status")
						? {
								status: {
									...THREE_STATUS,
									actor: "alka",
									is_operator: false,
									holding: [carried],
									blocked: [],
								},
							}
						: { listing: THREE_LISTING }),
		};
		return ok({ root: "C:\\customers", workbenches: [member] });
	};
	const view = new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: () => {},
		caseInsensitive: true,
		deadEndSentence: (refusal) => refusal,
		now: () => reading,
	});
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);

	const [confirmed] = answered(view.holdingSnapshot());
	assert.deepEqual([...confirmed.holding], [carried]);
	assert.equal(confirmed.fetchedAt, 1_000);

	// The member declines this checkpoint. What it was last confirmed to hold
	// stands, and the stamp does not move, so the display ages out rather
	// than reading as freshly empty.
	declining = true;
	reading = 9_000;
	await view.refresh("C:\\customers");
	const [stale] = answered(view.holdingSnapshot());
	assert.deepEqual([...stale.holding], [carried]);
	assert.equal(stale.fetchedAt, 1_000);

	// A read that answers does move it, which is what keeps the assertion
	// above from passing against a stamp that never moves at all.
	declining = false;
	reading = 12_000;
	await view.refresh("C:\\customers");
	assert.equal(answered(view.holdingSnapshot())[0].fetchedAt, 12_000);

	// The other way a status answer goes missing: the member itself reads
	// fine and the root-scoped status call is the one that refused, so no
	// member declined anything and there is simply nothing to read a hand
	// off. That is a different branch from the declining one above, and it
	// has to leave the stamp alone for the same reason.
	statusRefusing = true;
	reading = 20_000;
	await view.refresh("C:\\customers");
	const [unheard] = answered(view.holdingSnapshot());
	assert.deepEqual([...unheard.holding], [carried]);
	assert.equal(unheard.fetchedAt, 12_000);
});

test("a folder whose walk did not answer keeps its rows rather than emptying the hand", async () => {
	// The failure that deletes the record the uncertainty would be written
	// on. When the root-scoped `tree` call declines, no member list comes
	// back, so a walk that returned nothing would replace the folder's rows
	// with nothing and take every workbench under it out of the snapshot.
	// The bar would then compose from an empty snapshot, find no stale entry
	// to warn about, and render the idle text: a reader holding a card would
	// be told they hold none. Both halves are driven here, the walk failing
	// alone and then nothing answering at all.
	let walkFailing = false;
	let statusFailing = false;
	let reading = 1_000;
	const carried = {
		id: "aaa",
		ref: "tr-3",
		state: "active",
		holder: "alka",
		expires: "2026-09-07T13:30:00Z",
	};
	const memberStatus = {
		...THREE_STATUS,
		actor: "alka",
		is_operator: false,
		holding: [carried],
		blocked: [],
	};
	const refusal: SpawnOutcome = {
		code: 2,
		stdout: JSON.stringify({ refusal: "dinah.unreachable" }),
		stderr: "",
	};
	const spawner: Spawner = async (_exe, argv) => {
		if (argv.includes("tree") && walkFailing) {
			return refusal;
		}
		if (argv.includes("status") && statusFailing) {
			return refusal;
		}
		const member = {
			title: "Carter LLP",
			slug: "carter",
			path: "C:\\customers\\carter\\board",
			...(argv.includes("tree")
				? { tree: THREE_COLUMNS }
				: argv.includes("status")
					? { status: memberStatus }
					: { listing: THREE_LISTING }),
		};
		return ok({ root: "C:\\customers", workbenches: [member] });
	};
	const view = new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: () => {},
		caseInsensitive: true,
		deadEndSentence: (name) => name,
		now: () => reading,
	});
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	assert.deepEqual(
		[...answered(view.holdingSnapshot())[0].holding],
		[carried],
	);

	// The walk declines and the root-scoped status call answers. The member
	// is still there, and its hand is this checkpoint's own answer rather
	// than the last one's, so the stamp moves with it.
	const window = staleAfterMs(10);
	walkFailing = true;
	reading = 9_000;
	await view.refresh("C:\\customers");
	const [walked] = answered(view.holdingSnapshot());
	assert.equal(walked.source, "C:\\customers\\carter\\board");
	assert.deepEqual([...walked.holding], [carried]);
	assert.equal(walked.fetchedAt, 9_000);
	assert.deepEqual(summarizeHolding(view.holdingSnapshot(), 9_000, window), {
		cards: [
			{
				ref: "tr-3",
				workbenchTitle: "Carter LLP",
				expiresAt: carried.expires,
			},
		],
		uncertain: false,
	});

	// Now nothing under the folder answers. The row survives, its stamp
	// stops advancing, and the summary the bar composes from says outright
	// that this window cannot tell, which is the whole point of the row
	// surviving.
	statusFailing = true;
	reading = 60_000;
	await view.refresh("C:\\customers");
	const [silent] = answered(view.holdingSnapshot());
	assert.equal(silent.source, "C:\\customers\\carter\\board");
	assert.equal(silent.fetchedAt, 9_000);
	assert.deepEqual(summarizeHolding(view.holdingSnapshot(), 60_000, window), {
		cards: [],
		uncertain: true,
	});
});

test("a folder whose read threw contributes a doubt rather than nothing", async () => {
	// The other way a folder ends up with no rows at all. runDinah does not
	// catch a spawner that rejects, so a rejection travels out through
	// readWorkbench and load and leaves the folder standing with the empty
	// row list blankState gave it. Nothing later fills it in, and before
	// holdingSnapshot answered for the folder itself that folder simply left
	// the snapshot, which is the same lie by a third road.
	const view = provider(async () => {
		throw new Error("spawn refused by the operating system");
	});
	await assert.rejects(() =>
		view.load([folder({ folder: "C:\\ws\\thrown", resolution: RESOLVED })]),
	);
	assert.deepEqual(view.holdingSnapshot(), [
		{ state: "unheard", source: "C:\\ws\\thrown" },
	]);
	assert.deepEqual(
		summarizeHolding(view.holdingSnapshot(), 5_000, staleAfterMs(10)),
		{ cards: [], uncertain: true },
	);
});

test("an unopened candidate is a hand this window has not read", async () => {
	// A folder holding several workbenches resolves to candidate rows, and
	// this window opens none of them until a reader expands one. What is held
	// inside them is therefore unread rather than empty, and the bar says so.
	// Before this, an ambiguous folder contributed nothing to the snapshot, so
	// a reader holding a card in one of two sibling workbenches was told they
	// held nothing.
	const view = provider(async () => {
		throw new Error("no call is made for an unopened candidate");
	});
	await view.load([folder({ folder: "C:\\multi\\second", resolution: AMBIGUOUS })]);
	assert.deepEqual(
		view.holdingSnapshot().map((entry) => entry.state),
		["unheard", "unheard"],
	);
	assert.deepEqual(
		summarizeHolding(view.holdingSnapshot(), 5_000, staleAfterMs(10)),
		{ cards: [], uncertain: true },
	);
});

test("a member that declined the walk still reports the hand the status call answered with", async () => {
	// The one branch of readForest that had no fixture. A member can decline
	// the walk while the root-scoped status call answers for it perfectly
	// well, and the hand that answer carries is this checkpoint's own. The
	// root-scoped read used to drop it and report the last checkpoint's hand
	// instead, where the single-workbench read kept it, so the two paths gave
	// different answers to one question. They share heldHand now, and this is
	// what holds them to it: reverting the branch to read the hand off the
	// held data alone reports the first card and the first stamp, and both
	// assertions below go red.
	let declining = false;
	let reading = 1_000;
	const first = {
		id: "aaa",
		ref: "ca-1",
		state: "active",
		holder: "alka",
		expires: "2026-09-07T13:30:00Z",
	};
	const second = { ...first, id: "bbb", ref: "ca-2" };
	const spawner: Spawner = async (_exe, argv) => {
		const member = {
			title: "Carter LLP",
			slug: "carter",
			path: "C:\\customers\\carter\\board",
			...(argv.includes("tree")
				? declining
					? { unanswered: "dinah.unanswered" }
					: { tree: THREE_COLUMNS }
				: argv.includes("status")
					? {
							status: {
								...THREE_STATUS,
								actor: "alka",
								is_operator: false,
								holding: [declining ? second : first],
								blocked: [],
							},
						}
					: { listing: THREE_LISTING }),
		};
		return ok({ root: "C:\\customers", workbenches: [member] });
	};
	const view = new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: () => {},
		caseInsensitive: true,
		deadEndSentence: (name) => name,
		now: () => reading,
	});
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	assert.deepEqual(
		[...answered(view.holdingSnapshot())[0].holding],
		[first],
	);

	// The member declines the walk. The status call answered for it in the
	// same checkpoint, so its hand is that answer and its stamp moves with
	// it, exactly as the single-workbench read has always done.
	declining = true;
	reading = 9_000;
	await view.refresh("C:\\customers");
	const [reported] = answered(view.holdingSnapshot());
	assert.deepEqual([...reported.holding], [second]);
	assert.equal(reported.fetchedAt, 9_000);
});

test("a folder never read and a workbench never read give the reader one answer", async () => {
	// The blocker, driven the way the review drove it: the same failure put
	// to the window twice, once through a folder whose walk has never
	// answered and once through a single workbench whose reads have never
	// answered, with the two bars laid beside each other. Round two closed
	// this on the folder path only where an earlier good walk had left rows
	// behind, so a folder that had never been read still handed the bar an
	// empty snapshot and the bar reported an empty hand. Asserting on the
	// code would not have caught that, because the code looked repaired.
	const refusal: SpawnOutcome = {
		code: 2,
		stdout: JSON.stringify({ refusal: "dinah.unreachable" }),
		stderr: "",
	};
	const window = staleAfterMs(10);
	const refusing: Spawner = async () => refusal;
	const QUIET = "C:\\ws\\quiet";

	// One quiet folder, read through whichever resolution it is given, then
	// asked again. The bar is composed for both from the same binary and the
	// same resolved workbench, so the only thing that can move the text is
	// what this window believes about the reader's hand.
	const quiet = async (resolution: WorkbenchResolution) => {
		const view = provider(refusing);
		await view.load([folder({ folder: QUIET, resolution })]);
		const first = summarizeHolding(view.holdingSnapshot(), 5_000, window);
		// Every later checkpoint asks again and hears nothing again, which is
		// where the folder path used to go on reporting an empty hand instead
		// of converging on the answer the other path gave.
		await view.refresh(QUIET);
		const later = summarizeHolding(view.holdingSnapshot(), 50_000, window);
		const view50 = composeStatus(
			GOOD_BINARY,
			RESOLVED,
			"1.0.0",
			later,
			50_000,
		);
		return {
			first,
			later,
			text: view50.text,
			lead: view50.tooltip.split("\n")[0],
		};
	};

	const asFolder = await quiet(NOTHING);
	const asWorkbench = await quiet(RESOLVED);

	// Neither can say what is held, and each says so on the first checkpoint
	// as well as on the fourth, rather than only once a good read has left
	// something behind to go stale.
	assert.deepEqual(asFolder.first, { cards: [], uncertain: true });
	assert.deepEqual(asFolder.later, { cards: [], uncertain: true });
	assert.deepEqual(
		asFolder,
		asWorkbench,
		"the same failure put to the window twice reads the same way twice",
	);
	assert.ok(
		asFolder.text.endsWith("$(warning)"),
		`the bar warns rather than claiming an empty hand: ${asFolder.text}`,
	);
	assert.equal(
		asFolder.lead,
		"Dinah could not confirm whether you are holding anything right now",
	);

	// The control, without which the assertions above would also pass for a
	// window that warned whatever it heard. A walk that answers and names no
	// workbenches is dinah saying there is nothing beneath the folder, so
	// the reader is told they hold nothing and is told it plainly.
	const empty = provider(async (_exe, argv) =>
		argv.includes("tree")
			? ok({ root: QUIET, workbenches: [] })
			: ok({ workbenches: [] }),
	);
	await empty.load([folder({ folder: QUIET, resolution: NOTHING })]);
	const confirmed = summarizeHolding(empty.holdingSnapshot(), 5_000, window);
	assert.deepEqual(confirmed, { cards: [], uncertain: false });
	// Composed from what this provider answered rather than from the module's
	// own empty constant, which is a value no provider produced and which
	// would have rendered the same line however the control had gone.
	assert.equal(
		composeStatus(GOOD_BINARY, RESOLVED, "1.0.0", confirmed, 5_000).text
			.endsWith("$(warning)"),
		false,
	);
});

test("a forest member answering with no path of its own contributes no hand", async () => {
	// holdingSnapshot keys every entry on the workbench root, so a member
	// that came back without one has nothing to key on and would collide
	// with any other such member on the empty string. One negative row pins
	// the disjunct that drops it.
	const { spawner } = forestSpawner([
		{ title: "Ghost", slug: "ghost", path: "" },
		{ title: "Acme Co", slug: "acme", path: "C:\\customers\\acme\\board" },
	]);
	const view = provider(spawner);
	await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
	assert.deepEqual(
		answered(view.holdingSnapshot()).map((entry) => entry.source),
		["C:\\customers\\acme\\board"],
	);
});

// ---------------------------------------------------------------------------
// AC-6, the whole class: every way of failing to learn about a place
// ---------------------------------------------------------------------------

/** The tooltip line a window that cannot see the reader's hand leads with. */
const UNCERTAIN_LEAD =
	"Dinah could not confirm whether you are holding anything right now";

/** How long an answer is trusted for in the tests below, in milliseconds. */
const TRUST_WINDOW = staleAfterMs(10);

/** A moment far enough past any stamp below that nothing is still fresh. */
const LONG_AFTER = 9_000_000;

/**
 * Composes the bar from what a provider is actually holding.
 *
 * The binary and the resolution are fixed, so the only thing that can move
 * the text or the lead line is what this window believes about the reader's
 * hand. Both come back beside the summary, because a summary that reads
 * uncertain and a bar that draws the warning are two claims and the card
 * promises both.
 */
function barFrom(
	view: DinahTreeProvider,
	at: number,
): { summary: HoldingSummary; text: string; lead: string } {
	const summary = summarizeHolding(view.holdingSnapshot(), at, TRUST_WINDOW);
	const composed = composeStatus(GOOD_BINARY, RESOLVED, "1.0.0", summary, at);
	return {
		summary,
		text: composed.text,
		lead: composed.tooltip.split("\n")[0] ?? "",
	};
}

/** A provider whose clock a test names, and whose resolver it may name too. */
function clocked(
	spawner: Spawner,
	now: () => number,
	resolve?: (folder: string) => Promise<WorkbenchResolution>,
): DinahTreeProvider {
	return new DinahTreeProvider({
		spawner,
		exe: "dinah",
		log: () => {},
		caseInsensitive: true,
		deadEndSentence: (refusal) => `no workbench: ${refusal}`,
		now,
		resolve,
	});
}

/** A spawner that refuses everything it is asked. */
const REFUSING: Spawner = async () => ({
	code: 2,
	stdout: JSON.stringify({ refusal: "dinah.unreachable" }),
	stderr: "",
});

/** A spawner whose walk answers and names no workbench beneath the folder. */
const EMPTY_WALK: Spawner = async (_exe, argv) =>
	argv.includes("tree")
		? ok({ root: "C:\\ws\\quiet", workbenches: [] })
		: ok({ workbenches: [] });

/**
 * One way this window can fail to learn what is held somewhere.
 *
 * `at` is the moment the bar is composed, which matters for the routes where
 * the failure is an answer nobody renewed rather than an answer nobody gave.
 */
interface FailureRoute {
	readonly name: string;
	readonly reach: () => Promise<DinahTreeProvider>;
	readonly at: number;
}

/**
 * A workspace folder that resolved to a dead end, through a real refusal.
 *
 * The outcome goes through parseRefusal rather than being written out as a
 * resolution literal, because parseRefusal is where a transport failure and
 * dinah's own envelope are flattened into one string, and a test writing the
 * resolution by hand would be asserting on its own opinion of that flattening
 * instead of driving it.
 */
async function deadEnd(
	outcome: CliOutcome,
	now: () => number,
	resolve?: (folder: string) => Promise<WorkbenchResolution>,
): Promise<DinahTreeProvider> {
	const view = clocked(REFUSING, now, resolve);
	await view.load([
		folder({ folder: "C:\\ws\\quiet", resolution: parseRefusal(outcome) }),
	]);
	return view;
}

/** The refusal envelope dinah answers with where a folder configures none. */
const NO_CONFIGURED: CliOutcome = {
	kind: "refused",
	refusal: NO_CONFIGURED_WORKBENCH,
	detail: "no workbench is configured for this directory",
};

/**
 * Every route by which this window can end up not knowing what is held.
 *
 * The card's rule is one sentence: a reader who holds nothing and a reader
 * whose window could not find out are two different people. Three rounds of
 * review each found that rule broken on a route the round before had not
 * looked at, and each repair was proven on the routes the reviewer named. So
 * the proof here is the enumeration rather than the instance. Every entry
 * reaches its state by driving the provider into it, and the assertion below
 * is made once, over all of them.
 */
const FAILURE_ROUTES: readonly FailureRoute[] = [
	{
		name: "a workbench whose every read refuses",
		at: 5_000,
		reach: async () => {
			const view = clocked(REFUSING, () => 1_000);
			await view.load([folder({ folder: "C:\\work\\bench" })]);
			return view;
		},
	},
	{
		name: "a folder whose walk has never answered",
		at: 5_000,
		reach: async () => {
			const view = clocked(REFUSING, () => 1_000);
			await view.load([
				folder({ folder: "C:\\ws\\quiet", resolution: NOTHING }),
			]);
			return view;
		},
	},
	{
		name: "a folder whose walk answered once and then stopped",
		at: LONG_AFTER,
		reach: async () => {
			const { spawner } = forestSpawner(FOREST_MEMBERS);
			let walking = true;
			const view = clocked(
				async (exe, argv, options) =>
					walking
						? spawner(exe, argv, options)
						: REFUSING(exe, argv, options),
				() => 1_000,
			);
			await view.load([
				folder({ folder: "C:\\customers", resolution: NOTHING }),
			]);
			walking = false;
			await view.refresh("C:\\customers");
			return view;
		},
	},
	{
		name: "a folder whose walk answered that nothing is there and then stopped",
		at: LONG_AFTER,
		reach: async () => {
			let walking = true;
			const view = clocked(
				async (exe, argv, options) =>
					walking
						? EMPTY_WALK(exe, argv, options)
						: REFUSING(exe, argv, options),
				() => 1_000,
			);
			await view.load([
				folder({ folder: "C:\\ws\\quiet", resolution: NOTHING }),
			]);
			walking = false;
			await view.refresh("C:\\ws\\quiet");
			return view;
		},
	},
	{
		name: "a folder dinah confirmed empty that nobody has asked again",
		at: LONG_AFTER,
		reach: () => deadEnd(NO_CONFIGURED, () => 1_000),
	},
	{
		name: "a folder whose resolution could not launch the binary",
		at: 5_000,
		reach: () =>
			deadEnd(
				{ kind: "spawn-failed", errno: "ENOENT", detail: "no dinah" },
				() => 1_000,
			),
	},
	{
		name: "a folder whose resolution never came back",
		at: 5_000,
		reach: () =>
			deadEnd({ kind: "unreachable", detail: "timed out" }, () => 1_000),
	},
	{
		name: "a folder resolved by a binary too old to answer",
		at: 5_000,
		reach: () => deadEnd({ kind: "stale", detail: "format 0" }, () => 1_000),
	},
	{
		name: "a folder whose resolution came back garbled",
		at: 5_000,
		reach: () =>
			deadEnd({ kind: "not-json", detail: "syntax error" }, () => 1_000),
	},
	{
		name: "a folder holding several workbenches, none of them opened",
		at: 5_000,
		reach: async () => {
			const view = clocked(
				async () => {
					throw new Error("no call is made for an unopened candidate");
				},
				() => 1_000,
			);
			await view.load([
				folder({ folder: "C:\\multi\\second", resolution: AMBIGUOUS }),
			]);
			return view;
		},
	},
	{
		name: "a folder whose spawn threw rather than answering",
		at: 5_000,
		reach: async () => {
			const view = clocked(
				async () => {
					throw new Error("spawn ENOENT");
				},
				() => 1_000,
			);
			// runDinah does not catch a spawner that rejects, so the
			// rejection travels out through load and leaves the folder
			// standing with the empty row list blankState gave it. The
			// window is left running, which is exactly why the bar has to
			// speak for the folder afterwards.
			await assert.rejects(() =>
				view.load([
					folder({ folder: "C:\\ws\\thrown", resolution: NOTHING }),
				]),
			);
			return view;
		},
	},
	{
		name: "a workbench whose last answer has gone stale",
		at: LONG_AFTER,
		reach: async () => {
			const { spawner } = stubSpawner({
				status: THREE_STATUS,
				tree: THREE_COLUMNS,
				ls: THREE_LISTING,
			});
			const view = clocked(spawner, () => 1_000);
			await view.load([folder({ folder: "C:\\work\\bench" })]);
			return view;
		},
	},
];

test("every way of failing to learn about a place makes the bar warn", async () => {
	// The proof the class asked for, rather than the two instances the third
	// review reported. Each round of this card repaired the route it was
	// shown and left the same lie one path over, so what is driven here is
	// every route there is: a read that refuses, a walk that never came back,
	// a walk that came back once and stopped, a vacancy nobody renewed, each
	// of the four ways a resolution can fail without dinah having answered, a
	// folder whose workbenches were never opened, a spawn that threw, and an
	// answer that simply aged. Every one of them ends with a window that does
	// not know what the reader is holding, and the card's promise is that all
	// of them look the same to the reader.
	for (const route of FAILURE_ROUTES) {
		const bar = barFrom(await route.reach(), route.at);
		assert.deepEqual(
			bar.summary,
			{ cards: [], uncertain: true },
			`${route.name}: the window has nothing confirmed and knows it`,
		);
		assert.ok(
			bar.text.endsWith("$(warning)"),
			`${route.name}: the bar warns rather than drawing an empty hand: ${bar.text}`,
		);
		assert.equal(bar.lead, UNCERTAIN_LEAD, route.name);
	}
});

test("a confident empty hand takes an answer, and takes a recent one", async () => {
	// The control the enumeration needs, and the second half of what the
	// third review asked for. A window made to warn at everything would pass
	// that enumeration, so these three are the cases that must not warn, and
	// each one is a place that answered rather than a place that went quiet.
	//
	// The renewed dead end is the one worth reading twice. A folder dinah has
	// confirmed holds no workbench is the only kind this provider never
	// re-read, so its vacancy used to stand for the life of the window. It
	// now expires like every other answer, which means somebody has to renew
	// it, and the checkpoint's own re-resolution is what does.
	const answering = async () => parseRefusal(NO_CONFIGURED);

	const walked = clocked(EMPTY_WALK, () => 1_000);
	await walked.load([folder({ folder: "C:\\ws\\quiet", resolution: NOTHING })]);
	assert.deepEqual(
		barFrom(walked, 5_000).summary,
		{ cards: [], uncertain: false },
		"a walk that answered and named nothing is dinah saying the folder is empty",
	);
	assert.equal(barFrom(walked, 5_000).text.endsWith("$(warning)"), false);

	const held = clocked(
		stubSpawner({
			status: THREE_STATUS,
			tree: THREE_COLUMNS,
			ls: THREE_LISTING,
		}).spawner,
		() => 1_000,
	);
	await held.load([folder({ folder: "C:\\work\\bench" })]);
	assert.equal(barFrom(held, 5_000).summary.uncertain, false);
	assert.equal(barFrom(held, 5_000).text.endsWith("$(warning)"), false);

	// The same dead end the enumeration above lets expire, renewed instead.
	// The clock is read on every call, so the refresh restamps the folder at
	// a moment the later composition still trusts.
	let clock = 1_000;
	const renewed = await deadEnd(NO_CONFIGURED, () => clock, answering);
	assert.deepEqual(barFrom(renewed, LONG_AFTER).summary, {
		cards: [],
		uncertain: true,
	});
	clock = LONG_AFTER;
	await renewed.refresh("C:\\ws\\quiet");
	assert.deepEqual(
		barFrom(renewed, LONG_AFTER).summary,
		{ cards: [], uncertain: false },
		"dinah said again that nothing is here, so the hand is empty and known",
	);
	assert.equal(barFrom(renewed, LONG_AFTER).text.endsWith("$(warning)"), false);
});

test("a vacancy cannot be reported without the moment it was answered", async () => {
	// The type is what closes the class, and this is what shows it closing.
	// Every vacancy in a snapshot carries a stamp, so a producer with nothing
	// to stamp cannot name the reassuring arm at all, and the compiler is
	// what enforces that rather than a reviewer reading each producer one at
	// a time. The row here asserts the stamp's value rather than only its
	// presence, because a stamp read off the wrong clock would age wrongly.
	const view = clocked(EMPTY_WALK, () => 4_242);
	await view.load([folder({ folder: "C:\\ws\\quiet", resolution: NOTHING })]);
	assert.deepEqual(view.holdingSnapshot(), [
		{ state: "vacant", source: "C:\\ws\\quiet", answeredAt: 4_242 },
	]);
});
