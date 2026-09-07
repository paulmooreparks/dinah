// The sidebar tree: what it shows, and where every field of every row comes
// from.
//
// Nothing in this module imports vscode at run time. The provider composes a
// plain TreeItemSpec for each row and extension.ts turns that into a real
// vscode.TreeItem, which is what lets the unit layer drive every row shape,
// every join miss and every refusal without a VS Code host. The same reason
// keeps cli.ts's spawner a parameter rather than an import.
//
// Two rules are load-bearing and both are about not re-deriving an answer the
// binary already gave.
//
// The hierarchy is dinah's. `dinah tree` decides which column a card stands
// at, which state group it stands in, and which state groups a column carries
// at all. This module reads whatever came back and draws it. It never
// enumerates the states a column can hold, and it never branches on a
// column's kind or on AwaitingOutside to decide whether a group is drawn. A
// queue column that carries no state groups therefore renders its cards
// directly beneath the column row with no further change here (dinah-322 owns
// what `dinah tree` publishes for such a column; this module owns only what
// the sidebar draws from it).
//
// A card's menu is one function's answer. actionsFor reads exactly the facts
// verb.Library's own affordances() reads, and no other code here consults a
// card's state or a column's TakesWorkUp to decide a menu.

import type { Candidate, WorkbenchResolution } from "./api";
import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import {
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
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_CARD,
} from "./identity";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type {
	AttachmentListing,
	AttachmentView,
	CardView,
	ColumnView,
	ForestAnswer,
	ListingAnswer,
	RootListingAnswer,
	RootStatusAnswer,
	StatusAnswer,
	TreeAnswer,
	TreeNode,
} from "./wire";
import {
	AXIS_COLUMN,
	NODE_CARD,
	NODE_GROUP,
	STATE_ACTIVE,
	STATE_BLOCKED,
	STATE_READY,
} from "./wire";
import type { WorkbenchHoldingReport } from "./status";
import {
	AMBIGUOUS_WORKBENCH,
	FOUND_BENEATH,
	NO_CONFIGURED_WORKBENCH,
	NO_WORKBENCH,
	NO_WORKBENCH_FOUND,
	isInside,
} from "./workbench";

/** The three ways VS Code can draw a row's expand arrow. */
export type CollapsibleState = "none" | "collapsed" | "expanded";

/** A ThemeIcon by id, with an optional ThemeColor id. */
export interface IconSpec {
	readonly id: string;
	readonly color?: string;
}

/** A command a row runs when it is clicked. */
export interface CommandSpec {
	readonly command: string;
	readonly title: string;
	readonly args: readonly unknown[];
}

/**
 * Everything a row shows, as data.
 *
 * This is the whole of what a unit test has to assert on, and the whole of
 * what extension.ts has to translate. A field absent here is a field VS Code
 * is never told about, which is how an absent context menu stays absent
 * rather than becoming a disabled item.
 */
export interface TreeItemSpec {
	readonly label: string;
	readonly description?: string;
	readonly tooltip?: string;
	readonly contextValue?: string;
	readonly collapsibleState: CollapsibleState;
	readonly icon?: IconSpec;
	readonly command?: CommandSpec;
}

/**
 * One workbench's three joined answers, plus how it failed if it did.
 *
 * `refused` and `unanswered` are kept apart because they are two different
 * facts and the sketch draws them as two different rows: a workbench that
 * would not read at all has no identity to show, and a workbench that read
 * perfectly well and declined this one question keeps its identity and its
 * last-known subtree. Reading the two off one field would force a client to
 * tell them apart by how a refusal name is spelled.
 */
export interface WorkbenchData {
	readonly path: string;
	readonly title: string;
	readonly slug?: string;
	readonly refused?: string;
	readonly unanswered?: string;
	/**
	 * The refusal's own detail text, when it carried one, so a reader is
	 * shown the sentence the CLI composed rather than only the bare name.
	 */
	readonly unansweredDetail?: string;
	/**
	 * The column this workbench's own read named, when the refusal was about
	 * one column. It is read from the refusal's declared `column` context
	 * field and from nothing else, so it is absent on every refusal naming no
	 * single column and on a CLI predating that field.
	 */
	readonly unansweredColumn?: string;
	readonly columns: ReadonlyMap<string, ColumnView>;
	readonly cards: ReadonlyMap<string, CardView>;
	/** The tree's own workbench root node, absent when no tree answered. */
	readonly root?: TreeNode;
	/**
	 * How many attachments hang from the workbench itself, one count beside
	 * the counts each column and each card carry. Held from the last good
	 * checkpoint when this one's status did not answer, exactly as `root`
	 * is, and read by rootChildren to decide whether the root draws an
	 * Attachments row at all.
	 */
	readonly attachmentCount?: number;
	/**
	 * The owner this window's invocations act as here, as `status` reports
	 * it. Held from the last good checkpoint when this one's status did not
	 * answer, for the reason `root` is.
	 */
	readonly actor?: string;
	/**
	 * The cards that actor holds in this workbench right now, as `status`
	 * reports them. Empty is a real answer meaning the actor holds nothing,
	 * so a caller telling "holds nothing" apart from "we do not know" reads
	 * `fetchedAt` rather than this array's length.
	 */
	readonly holding: readonly CardView[];
	/**
	 * When the last `status` call that answered ok came back, in milliseconds.
	 *
	 * It advances on an ok answer and on nothing else, so a run of refusals
	 * reads as increasing staleness rather than as freshness. It is absent
	 * until a status call has answered ok at least once, which is how a
	 * workbench nobody has heard from yet is told apart from one that
	 * answered a moment ago.
	 */
	readonly fetchedAt?: number;
}

/** How one workspace folder resolved, and therefore what rows it contributes. */
export type FolderMode = "single" | "forest" | "candidates" | "dead-end";

/** One root-level row of the tree. */
export interface RootRow {
	readonly rowKind:
		| "workbenchRoot"
		| "workbenchCandidate"
		| "workbenchForest"
		| "deadEnd";
	/** The workspace folder this row was produced for. */
	readonly folder: string;
	/** The label VS Code shows for that folder in its own multi-root UI. */
	readonly folderName: string;
	/** Set on a resolved row and on a candidate that has been expanded. */
	data?: WorkbenchData;
	/**
	 * The disambiguating path this row shows, empty where it needs none.
	 *
	 * A forest row always carries one, because a folder holding several
	 * customers will often hold several same-titled workbenches. A row for a
	 * folder that resolved to exactly one workbench carries one only when the
	 * resolved root is not the folder itself, which is dinah-241's case.
	 */
	readonly description: string;
	/** Set on a candidate row before it is expanded. */
	readonly candidate?: Candidate;
	/** The refusal a candidate's own expansion raised, if it raised one. */
	failure?: string;
	/** The sentence a dead-end row shows. */
	readonly sentence?: string;
	/** The refusal a dead-end row names. */
	readonly refusal?: string;
	/** True when this is the only row in the whole tree. */
	sole: boolean;
	/** The in-flight expansion of a candidate row, so two expands join it. */
	pending?: Promise<void>;
}

/** Every element getChildren can return. */
export type TreeElement =
	| { readonly kind: "root"; readonly row: RootRow }
	| {
			readonly kind: "note";
			readonly owner: RootRow;
			readonly text: string;
			readonly tooltip: string;
	  }
	| {
			readonly kind: "column";
			readonly row: RootRow;
			readonly node: TreeNode;
			readonly view?: ColumnView;
			/**
			 * The ref of the AXIS_COLUMN node standing immediately after this
			 * one in the workbench's own declared flow order, absent when this
			 * column is last. This is downstreamOf's answer
			 * (internal/verb/pull.go), read off the tree's own ordering rather
			 * than recomputed, since that ordering is already the fact
			 * columnsOf draws the rows in (dinah-375 OQ-1).
			 */
			readonly nextColumnRef?: string;
			/**
			 * nextColumnRef resolved to its ColumnView through the same
			 * status/tree join columnsOf already runs, absent when the join
			 * missed it. A reader holding the ref alone still has enough to
			 * name the destination in the CLI argv; only the display text
			 * needs the view, and it falls back to the raw ref.
			 */
			readonly nextColumn?: ColumnView;
	  }
	| {
			readonly kind: "group";
			readonly row: RootRow;
			readonly node: TreeNode;
			readonly column?: ColumnView;
	  }
	| {
			readonly kind: "card";
			readonly row: RootRow;
			readonly node: TreeNode;
			readonly view?: CardView;
			readonly column?: ColumnView;
			/** The state group this card stands under, absent where none was drawn. */
			readonly groupValue?: string;
	  }
	| {
			readonly kind: "attachmentsGroup";
			readonly row: RootRow;
			/** The workbench root this group's own fetch is pinned to. */
			readonly root: string;
			/**
			 * What `attachments` is called with: "" for the workbench itself,
			 * because an omitted argument is how the binary is asked about the
			 * workbench, and the entity's own ref for a column or a card.
			 */
			readonly ref: string;
			/** The eager count status or ls already reported, shown beside the label. */
			readonly count: number;
	  }
	| {
			readonly kind: "attachment";
			readonly row: RootRow;
			readonly view: AttachmentView;
	  };

// ---------------------------------------------------------------------------
// The joins
// ---------------------------------------------------------------------------

/**
 * The key a column publishes on both sides of the status/tree join.
 *
 * A column-axis group node carries no ID, because TreeNode.ID is absent on
 * every node but a card leaf. What it does carry is the column's own ref, and
 * bench.Column.Ref() is the slug when there is one and the identifier
 * otherwise, so the same fallback computed here matches the value tree.go
 * filled the group's Value with.
 */
export function columnRef(view: ColumnView): string {
	return view.slug !== undefined && view.slug !== "" ? view.slug : view.id;
}

/** Indexes a status answer's columns by the ref a tree node names them with. */
export function joinColumns(
	status: StatusAnswer | undefined,
): Map<string, ColumnView> {
	const byRef = new Map<string, ColumnView>();
	for (const view of status?.columns ?? []) {
		byRef.set(columnRef(view), view);
	}
	return byRef;
}

/** Indexes a listing answer's cards by the 12-hex id a card leaf names. */
export function joinCards(
	listing: ListingAnswer | undefined,
): Map<string, CardView> {
	const byId = new Map<string, CardView>();
	for (const card of listing?.cards ?? []) {
		byId.set(card.id, card);
	}
	return byId;
}

// ---------------------------------------------------------------------------
// Composing what a row shows
// ---------------------------------------------------------------------------

/**
 * A column row's description: the occupancy, and one word where it is earned.
 *
 * The count leads and mostly stands alone, because the tree's own shape
 * already separates a column that takes work up from one that only holds it:
 * the first heads Ready and Active groups, the second draws its cards inline.
 * A column awaiting somebody outside gets the one word the shape cannot
 * carry, which tells a reader scanning the rows not to go there.
 *
 * A declared capacity always shows itself, so an empty limited column reads
 * `0/3` rather than a bare `0` indistinguishable from a column with no limit.
 * The CLI's renderColumns composes the same two values the same way.
 */
export function columnDescription(
	view: ColumnView | undefined,
	node: TreeNode,
	t: Localizer = ENGLISH,
): string {
	const count = view?.count ?? node.count;
	const capacity = view?.capacity ?? 0;
	const occupancy =
		capacity > 0 ? `${String(count)}/${String(capacity)}` : String(count);
	return view?.awaiting_outside === true
		? t("tree.column.occupancyWaiting", { occupancy })
		: occupancy;
}

/**
 * A card row's label: the reference the operator types, then the title.
 *
 * The reference leads because VS Code truncates a label from its end, so a
 * narrow sidebar cuts the title and keeps the handle a reader acts on. The
 * operator ruled on that placement after seeing both renderings (dinah-337
 * OQ-1). Either half alone still reads, which is what the two fallbacks are
 * for: a card the ls join missed carries no title, and a node with no
 * reference at all leaves the title standing on its own.
 */
export function cardLabel(ref: string, title: string | undefined): string {
	if (title === undefined || title === "") {
		return ref;
	}
	return ref === "" ? title : `${ref}: ${title}`;
}

/** A card row's description: its two levels, joined, or nothing at all. */
export function cardDescription(view: CardView | undefined): string {
	const parts = [view?.severity, view?.priority].filter(
		(part): part is string => part !== undefined && part !== "",
	);
	return parts.join(" · ");
}

/**
 * Which state a card stands in.
 *
 * `ls` is the surface that publishes a card's state, so it is read first. The
 * state group the card renders beneath is the fallback for a card the ls join
 * missed, and ready is the fallback for a card standing under no group at
 * all, which is the shape a queue column produces.
 *
 * That last fallback carries more than it used to. A queue column now inlines
 * its ready and its active cards rather than heading either, so an active card
 * standing at one has no group value for the second fallback to read and reads
 * as ready when the ls join misses it. Only a hand edit puts a card in active
 * at such a column, and the ls join is the path every card reached by the tool
 * takes, so this is recorded here rather than worked around.
 */
export function cardState(
	view: CardView | undefined,
	groupValue: string | undefined,
): string {
	if (view?.state !== undefined && view.state !== "") {
		return view.state;
	}
	if (groupValue !== undefined && groupValue !== "") {
		return groupValue;
	}
	return STATE_READY;
}

/** The icon a card carries for the state it stands in. */
export function cardIcon(state: string): IconSpec {
	switch (state) {
		case STATE_ACTIVE:
			return { id: "circle-filled", color: "charts.blue" };
		case STATE_BLOCKED:
			return { id: "circle-slash", color: "charts.red" };
		default:
			return { id: "circle-outline" };
	}
}

/** A card, and the column it stands at, as actionsFor reads them. */
export interface CardStanding {
	readonly state: string;
	readonly column?: ColumnView;
}

/**
 * The one function that decides a card's menu.
 *
 * It reads exactly what verb.Library's own affordances() reads: the card's
 * state, and for a ready card, whether its column takes work up. Move and
 * Block never vary within a state and Release and Unblock never vary at all,
 * so they are folded into the state's own answer rather than recomputed.
 *
 * There is no pull answer here, and there will not be one. dinah's pull verb
 * takes a destination column and chooses its own card from that column's
 * upstream, so no card-scoped Pull could be aimed at the row that was clicked.
 * dinah-375 put the act on the column row, where the verb's own scope is, and
 * columnActionsFor below is what decides it.
 */
export function actionsFor(card: CardStanding): string {
	switch (card.state) {
		case STATE_ACTIVE:
			return CONTEXT_CARD_ACTIVE;
		case STATE_BLOCKED:
			return CONTEXT_CARD_BLOCKED;
		default:
			return card.column?.takes_work_up === true
				? CONTEXT_CARD_READY_CLAIM
				: CONTEXT_CARD_READY_NONE;
	}
}

/**
 * The contextValue a column row carries, which decides whether New Card and
 * Pull are offered on it.
 *
 * Capacity is the only column fact this reads, because it is the only one
 * ColumnView publishes. Add can also refuse Locked when the destination column
 * is mid-retirement, and that is not a fact this tree holds, so the menu
 * accepts that race rather than gating on a field that does not exist
 * (dinah-331 Decision 2). The two fields read here are the same two
 * columnDescription already reads for the row's own description text.
 *
 * A column the status/tree join missed carries no ColumnView at all, and it
 * gets the bare CONTEXT_COLUMN, which offers neither New Card nor Attach File
 * until the next checkpoint's tree and status answers agree again. That is the
 * same self-heal columnsOf already logs for the same miss.
 *
 * dinah-375 added a second, independent axis. Capacity still decides the
 * open/full half, and a queue column with a column standing after it in the
 * flow takes the .pull suffix on top of it. The test is the column's own
 * takes_work_up, the same field actionsFor reads to tell a claimable ready
 * card from one that is only pulled through, so a work column never carries
 * the suffix whatever stands downstream of it (D-2): a work column's cards
 * are individually actionable already, and a column-level pull there would
 * step around the per-card Claim rather than adding anything.
 *
 * The other half of the test is nextColumnRef, the column immediately after
 * this one in the flow's declared order, and the operator's OQ-1 ruling is
 * why. A queue offers a pull into its own next column, so a click moves the
 * card standing in the row that was clicked. The retired reading walked past
 * any number of intervening queues, which made every queue in a chain publish
 * one destination and moved a card the reader could not see.
 */
export function columnActionsFor(
	view: ColumnView | undefined,
	nextColumnRef?: string,
): string {
	if (view === undefined) {
		return CONTEXT_COLUMN;
	}
	const capacity = view.capacity ?? 0;
	const full = capacity > 0 && view.count >= capacity;
	const pullable = !view.takes_work_up && nextColumnRef !== undefined;
	if (full) {
		return pullable ? CONTEXT_COLUMN_FULL_PULL : CONTEXT_COLUMN_FULL;
	}
	return pullable ? CONTEXT_COLUMN_OPEN_PULL : CONTEXT_COLUMN_OPEN;
}

/** The label a state group carries, title-cased from the axis value. */
export function groupLabel(
	value: string | undefined,
	t: Localizer = ENGLISH,
): string {
	switch (value) {
		case STATE_READY:
			return t("tree.group.ready");
		case STATE_ACTIVE:
			return t("tree.group.active");
		case STATE_BLOCKED:
			return t("tree.group.blocked");
		default:
			return value === undefined || value === ""
				? t("tree.group.none")
				: value;
	}
}

/** A card row's hover text, one fact per line and no empty lines. */
export function cardTooltip(
	node: TreeNode,
	view: CardView | undefined,
	column: ColumnView | undefined,
	state: string,
	t: Localizer = ENGLISH,
): string {
	const lines: string[] = [];
	const ref = view?.ref ?? node.ref;
	if (ref !== undefined && ref !== "") {
		lines.push(ref);
	}
	const columnTitle = column?.title ?? view?.column_title;
	lines.push(
		columnTitle === undefined || columnTitle === ""
			? state
			: `${columnTitle} · ${state}`,
	);
	if (state === STATE_ACTIVE && view?.holder !== undefined && view.holder !== "") {
		lines.push(t("tree.card.heldBy", { holder: view.holder }));
	}
	if (state === STATE_BLOCKED) {
		const blocked = [view?.block_kind, view?.block_reason].filter(
			(part): part is string => part !== undefined && part !== "",
		);
		if (blocked.length > 0) {
			lines.push(blocked.join(": "));
		}
	}
	if (view?.workstreams !== undefined && view.workstreams.length > 0) {
		lines.push(view.workstreams.join(", "));
	}
	return lines.join("\n");
}

/**
 * A column row's hover text: the title, then what the row's shape doesn't say.
 *
 * The description carries the count and nothing else, so the facts a reader
 * used to read off a leading word live here instead: whether a card is
 * claimed at this column or pulled through it, whether it waits on somebody
 * outside, and who may move it out.
 *
 * A column the status/tree join missed publishes none of those facts, so it
 * keeps the title alone rather than being described from defaults. That is
 * the same conservative miss columnActionsFor takes, and it self-heals on the
 * next checkpoint.
 *
 * A queue with a column after it in the flow names that column, because
 * dinah-375 gave the row an act and a reader who cannot see where the card
 * would land has no way to judge it. Both facts arrive resolved from
 * columnsOf, which does the status/tree join once for the whole row rather
 * than leaving the tooltip to repeat it.
 *
 * nextColumnRef stays a parameter of its own rather than being folded into
 * nextColumn, because the two can disagree: a column can stand next in the
 * flow while the join has not resolved its view, the same race columnsOf's
 * own view lookup already tolerates. Dropping the line in that case would say
 * nothing is pullable when something is, so it falls back to the raw
 * reference, which the next checkpoint repairs.
 */
export function columnTooltip(
	view: ColumnView | undefined,
	node: TreeNode,
	nextColumn?: ColumnView,
	nextColumnRef?: string,
	t: Localizer = ENGLISH,
): string {
	const title = view?.title ?? node.value ?? "";
	if (view === undefined) {
		return title;
	}
	const lines: string[] = [title];
	lines.push(
		view.takes_work_up
			? t("tree.column.claimedHere")
			: t("tree.column.pulledOnward"),
	);
	if (!view.takes_work_up && nextColumnRef !== undefined) {
		const destination = nextColumn?.title ?? nextColumnRef;
		lines.push(t("tree.column.pullHint", { destination }));
	}
	if (view.awaiting_outside) {
		lines.push(t("tree.column.awaitingOutside"));
	}
	lines.push(
		view.operator_owned
			? t("tree.column.operatorOwned")
			: t("tree.column.agentMoves"),
	);
	return lines.join("\n");
}

/**
 * A column row's hover text when this is the column the last read named as
 * broken: the column's own title, what went wrong in one line, then the
 * refusal's name and detail in the same "name: detail" form every other
 * refusal this extension shows already uses.
 *
 * The two lines this function writes itself, the column's title aside, are
 * localised through l10n.ts and reach a reader in the language the editor is
 * displaying (dinah-379). The refusal's own name and detail are not: they come
 * off the CLI's `--json` wire, which spells them in English whatever
 * DINAH_LANG says, so the sentence they compose is relayed exactly as
 * cardCommands.ts's refusalMessage relays its own.
 */
export function columnBrokenTooltip(
	view: ColumnView | undefined,
	data: WorkbenchData | undefined,
	t: Localizer = ENGLISH,
): string {
	const name = data?.unanswered ?? "";
	const detail = data?.unansweredDetail ?? "";
	const sentence =
		name === "" ? "" : detail === "" ? name : `${name}: ${detail}`;
	const lines = [
		view?.title ?? "",
		t("tree.column.brokenTooltip.unreadable"),
		sentence,
	];
	return lines.filter((line) => line !== "").join("\n");
}

/**
 * The path of a workbench relative to the workspace folder that produced its
 * row, spelled POSIX-style so two same-titled customers read alike on every
 * platform.
 *
 * A root that is not inside the folder is spelled absolutely, because a
 * relative path climbing out of the folder would say less than the path
 * itself. isInside is workbench.ts's own segment-wise containment test,
 * reused rather than restated.
 */
export function relativeTo(
	root: string,
	folder: string,
	caseInsensitive: boolean,
): string {
	const posix = (value: string): string => value.replace(/\\/g, "/");
	if (!isInside(root, folder, caseInsensitive)) {
		return posix(root);
	}
	const outer = posix(folder)
		.split("/")
		.filter((segment) => segment !== "");
	const inner = posix(root)
		.split("/")
		.filter((segment) => segment !== "");
	return inner.slice(outer.length).join("/");
}

// ---------------------------------------------------------------------------
// The row a TreeElement draws
// ---------------------------------------------------------------------------

/** The icon a resolved workbench row carries. */
const WORKBENCH_ICON: IconSpec = { id: "book" };

/** The icon a row that could not answer carries. */
const WARNING_ICON: IconSpec = { id: "warning" };

/** The label a workbench with no title of its own falls back to. */
export const UNTITLED_WORKBENCH = "Dinah";

/** Whether a root row is drawn expanded, collapsed, or with no arrow at all. */
function rootCollapsibleState(row: RootRow): CollapsibleState {
	if (row.rowKind === "deadEnd") {
		return "none";
	}
	// A walked row that would not read at all has nothing beneath it and no
	// identity to head it, so it is drawn flat, exactly as a dead end is.
	if (unreadableWithoutIdentity(row.data)) {
		return "none";
	}
	if (row.rowKind === "workbenchCandidate") {
		return "collapsed";
	}
	return row.sole ? "expanded" : "collapsed";
}

/** The first of the three failure shapes: no identity to draw the row with. */
function unreadableWithoutIdentity(data: WorkbenchData | undefined): boolean {
	if (data?.refused === undefined || data.refused === "") {
		return false;
	}
	return data.title === "" && (data.slug === undefined || data.slug === "");
}

/** The label a resolved or walked workbench row carries. */
function workbenchLabel(data: WorkbenchData | undefined, row: RootRow): string {
	if (data === undefined) {
		const candidate = row.candidate;
		const title = candidate?.title ?? "";
		return title === "" ? UNTITLED_WORKBENCH : title;
	}
	if (unreadableWithoutIdentity(data)) {
		return data.path;
	}
	return data.title === "" ? UNTITLED_WORKBENCH : data.title;
}

/** Composes the row for one element. */
export function treeItemFor(
	element: TreeElement,
	t: Localizer = ENGLISH,
): TreeItemSpec {
	switch (element.kind) {
		case "root":
			return rootItem(element.row, t);
		case "note":
			return {
				label: element.text,
				tooltip: element.tooltip,
				collapsibleState: "none",
				icon: WARNING_ICON,
			};
		case "column": {
			const view = element.view;
			const named = element.row.data?.unansweredColumn;
			const broken = named !== undefined && view?.id === named;
			return {
				label: view?.title ?? element.node.value ?? "",
				description: broken
					? t("tree.column.damaged")
					: columnDescription(view, element.node, t),
				tooltip: broken
					? columnBrokenTooltip(view, element.row.data, t)
					: columnTooltip(
							view,
							element.node,
							element.nextColumn,
							element.nextColumnRef,
							t,
						),
				contextValue: columnActionsFor(view, element.nextColumnRef),
				collapsibleState: "expanded",
				icon: broken ? WARNING_ICON : undefined,
			};
		}
		case "group":
			return {
				label: groupLabel(element.node.value, t),
				description: String(element.node.count),
				contextValue: CONTEXT_STATE_GROUP,
				collapsibleState: element.node.count === 0 ? "collapsed" : "expanded",
			};
		case "card": {
			const state = cardState(element.view, element.groupValue);
			const ref = element.view?.ref ?? element.node.ref ?? "";
			return {
				label: cardLabel(ref, element.node.title ?? element.view?.title),
				description: cardDescription(element.view),
				tooltip: cardTooltip(
					element.node,
					element.view,
					element.column,
					state,
					t,
				),
				contextValue: actionsFor({ state, column: element.column }),
				// An arrow only when the count says something is there to expand.
				// A card the ls join missed reads no count, and a card carrying
				// none renders exactly as it did before attachments existed.
				collapsibleState:
					element.view?.attachment_count !== undefined &&
					element.view.attachment_count > 0
						? "collapsed"
						: "none",
				icon: cardIcon(state),
				command: {
					command: COMMAND_OPEN_CARD,
					title: "Open Card",
					args: [element],
				},
			};
		}
		case "attachmentsGroup":
			return {
				label: t("tree.attachments.label"),
				description: String(element.count),
				collapsibleState: "collapsed",
			};
		case "attachment": {
			const view = element.view;
			const openable = view.path !== undefined && view.path !== "";
			const tooltip = [
				view.filename,
				...(
					view.description !== undefined && view.description !== ""
						? [view.description]
						: []
				),
				openable ? (view.path as string) : t("tree.attachment.noLocalFile"),
			];
			return {
				label: view.filename,
				description: view.description,
				tooltip: tooltip.join("\n"),
				icon: openable ? { id: "file" } : WARNING_ICON,
				collapsibleState: "none",
				// A command only when the file can be opened. The key is
				// absent rather than set to undefined so a row carrying no
				// payload is a row VS Code will not offer as clickable, which
				// is the same treatment toTreeItem gives an absent contextValue.
				...(openable
					? {
							command: {
								command: COMMAND_OPEN_ATTACHMENT,
								title: "Open Attachment",
								args: [element],
							},
						}
					: {}),
			};
		}
	}
}

/** Composes a root-level row. */
function rootItem(row: RootRow, t: Localizer): TreeItemSpec {
	if (row.rowKind === "deadEnd") {
		return {
			label: t("tree.root.deadEnd.label", {
				folder: row.folderName,
				refusal: row.refusal ?? t("tree.root.noWorkbench"),
			}),
			tooltip: row.sentence ?? "",
			collapsibleState: "none",
			icon: WARNING_ICON,
		};
	}

	const data = row.data;
	const collapsibleState = rootCollapsibleState(row);
	const label = workbenchLabel(data, row);

	if (unreadableWithoutIdentity(data)) {
		// No title, no slug, no context menu: the walk never read far enough
		// to give this row an identity, so its path is all it can be named by.
		return {
			label,
			description: data?.refused,
			tooltip: t("tree.root.unreadable.tooltip", {
				refusal: data?.refused ?? "",
			}),
			collapsibleState,
			icon: WARNING_ICON,
		};
	}

	const contextValue =
		row.rowKind === "workbenchCandidate"
			? CONTEXT_WORKBENCH_CANDIDATE
			: row.rowKind === "workbenchForest"
				? CONTEXT_WORKBENCH_FOREST
				: CONTEXT_WORKBENCH_ROOT;

	const description =
		data?.refused !== undefined && data.refused !== ""
			? t("tree.root.wouldNotOpen")
			: data?.unanswered !== undefined && data.unanswered !== ""
				? t("tree.root.didNotAnswer")
				: row.description;

	const path = data?.path ?? row.candidate?.path ?? "";
	// A reader who gets no column-level marker, because the refusal named no
	// column or because the column it named was never cached, still needs
	// more here than "did not answer".
	const detail =
		data?.unanswered !== undefined &&
		data.unanswered !== "" &&
		data.unansweredDetail !== undefined &&
		data.unansweredDetail !== ""
			? `${data.unanswered}: ${data.unansweredDetail}`
			: "";
	return {
		label,
		description,
		tooltip: detail === "" ? path : `${path}\n${detail}`,
		contextValue,
		collapsibleState,
		icon: WORKBENCH_ICON,
	};
}

// ---------------------------------------------------------------------------
// Reading a workbench's three answers
// ---------------------------------------------------------------------------

/** How a workbench is addressed on a single-workbench call. */
function pinned(root: string, args: readonly string[]): string[] {
	return ["--workbench", root, ...args];
}

/**
 * The three read-only calls that make one workbench's subtree, against a
 * single resolved root.
 *
 * The root is pinned with --workbench rather than left to a cwd, so a
 * session's tree calls stay on the workbench this session resolved once
 * instead of repeating the discovery walk, and its dinah-241 hazard, on every
 * refresh.
 */
export async function readWorkbench(
	spawner: Spawner,
	exe: string,
	root: string,
	log: (line: string) => void,
	held?: WorkbenchData,
	now: () => number = Date.now,
): Promise<WorkbenchData> {
	const [status, tree, listing] = await Promise.all([
		runDinah(spawner, exe, pinned(root, ["status"]), { cwd: root }),
		runDinah(spawner, exe, pinned(root, ["tree"]), { cwd: root }),
		runDinah(spawner, exe, pinned(root, ["ls"]), { cwd: root }),
	]);

	for (const [name, outcome] of [
		["status", status],
		["tree", tree],
		["ls", listing],
	] as const) {
		if (outcome.kind !== "ok") {
			log(`dinah ${name} at ${root}: ${outcome.kind}`);
		}
	}

	const statusJson =
		status.kind === "ok" ? (status.json as StatusAnswer) : undefined;
	const treeJson = tree.kind === "ok" ? (tree.json as TreeAnswer) : undefined;
	const listingJson =
		listing.kind === "ok" ? (listing.json as ListingAnswer) : undefined;

	if (treeJson === undefined) {
		// The read that carries the hierarchy did not answer. The row keeps
		// its identity and whatever subtree the last good checkpoint left,
		// because the next tick tries again and a passing failure must not
		// blank the tree in the meantime.
		return {
			path: root,
			title: statusJson?.workbench ?? held?.title ?? "",
			unanswered: refusalNameOf(tree),
			unansweredDetail: tree.kind === "refused" ? tree.detail : undefined,
			unansweredColumn:
				tree.kind === "refused" &&
				typeof tree.context?.column === "string" &&
				tree.context.column !== ""
					? tree.context.column
					: undefined,
			columns: held?.columns ?? new Map(),
			cards: held?.cards ?? new Map(),
			root: held?.root,
			attachmentCount: held?.attachmentCount,
			// The tree is what failed here. Status may well have answered,
			// and where it did its holding list is this checkpoint's own
			// answer rather than the last one's.
			...heldHand(statusJson, held, now),
		};
	}

	return {
		path: root,
		title: statusJson?.workbench ?? "",
		columns: joinColumns(statusJson),
		cards: joinCards(listingJson),
		root: treeJson.root,
		attachmentCount: statusJson?.attachment_count,
		...heldHand(statusJson, held, now),
	};
}

/**
 * What a row says about the reader's hand, given this checkpoint's status
 * answer for it and whatever the last good checkpoint left behind.
 *
 * Both read paths ask this question, and before this helper existed they
 * answered it differently: the single-workbench read preferred a status
 * answer that arrived even when its sibling calls failed, while the
 * root-scoped read dropped one. A status answer that came back this
 * checkpoint is this checkpoint's own answer wherever it arrives, so the two
 * paths now decide it in one place. `fetchedAt` moves only on an answer,
 * which is what makes a run of failures read as increasing staleness.
 */
function heldHand(
	status: StatusAnswer | undefined,
	held: WorkbenchData | undefined,
	now: () => number,
): Pick<WorkbenchData, "actor" | "holding" | "fetchedAt"> {
	if (status === undefined) {
		return {
			actor: held?.actor,
			holding: held?.holding ?? [],
			fetchedAt: held?.fetchedAt,
		};
	}
	return {
		actor: status.actor ?? held?.actor,
		holding: status.holding ?? held?.holding ?? [],
		fetchedAt: now(),
	};
}

/** The refusal name a non-ok outcome carries, or its own arm's name. */
function refusalNameOf(outcome: { kind: string; refusal?: string }): string {
	return outcome.refusal !== undefined && outcome.refusal !== ""
		? outcome.refusal
		: outcome.kind;
}

/** Indexes a forest answer's members by the path the walk keys them on. */
function byPath<T extends { path: string }>(
	members: readonly T[] | undefined,
): Map<string, T> {
	const found = new Map<string, T>();
	for (const member of members ?? []) {
		found.set(member.path, member);
	}
	return found;
}

/**
 * What a root-scoped walk came back with, and whether it came back at all.
 *
 * `walked` is a separate question from how many members arrived. A walk that
 * answered and named no workbenches, and a walk that never answered while the
 * folder had nothing held from before, both produce an empty `members`, and
 * the caller has to tell a folder holding nothing apart from a folder it
 * knows nothing about. Returning the members alone left that undecidable, and
 * the provider guessed the reassuring answer.
 */
export interface ForestReading {
	readonly walked: boolean;
	readonly members: readonly WorkbenchData[];
}

/**
 * The three root-scoped calls that answer for every workbench beneath a
 * folder, in one process each rather than one process per workbench.
 *
 * The members of the three answers are joined by path, which is the handle
 * the walk keys its own rows on, exactly as a single-workbench answer's cards
 * are joined by id and its columns by ref.
 */
export async function readForest(
	spawner: Spawner,
	exe: string,
	folder: string,
	previous: ReadonlyMap<string, WorkbenchData>,
	log: (line: string) => void,
	now: () => number = Date.now,
): Promise<ForestReading> {
	const [status, tree, listing] = await Promise.all([
		runDinah(spawner, exe, ["status", "--root", folder], { cwd: folder }),
		runDinah(spawner, exe, ["tree", "--root", folder], { cwd: folder }),
		runDinah(spawner, exe, ["ls", "--root", folder], { cwd: folder }),
	]);

	for (const [name, outcome] of [
		["status --root", status],
		["tree --root", tree],
		["ls --root", listing],
	] as const) {
		if (outcome.kind !== "ok") {
			log(`dinah ${name} at ${folder}: ${outcome.kind}`);
		}
	}

	const forest = tree.kind === "ok" ? (tree.json as ForestAnswer) : undefined;
	const statuses = byPath(
		status.kind === "ok"
			? (status.json as RootStatusAnswer).workbenches
			: undefined,
	);
	const listings = byPath(
		listing.kind === "ok"
			? (listing.json as RootListingAnswer).workbenches
			: undefined,
	);

	if (forest === undefined) {
		// The walk itself did not answer, so this checkpoint learned nothing
		// about which workbenches lie under the folder. Returning no members
		// would delete the rows the caller replaces its own with, and the
		// rows are what every later reader records a failed read against, so
		// a reader holding a card would watch it vanish from the bar with
		// nothing left to say the window had stopped hearing. The members
		// the last good walk found are kept and marked unanswered, which is
		// the decision the per-member branch below already makes for a
		// member that read fine and declined this one question.
		const name = refusalNameOf(tree);
		log(`the walk at ${folder} did not answer: ${name}`);
		const kept = [...previous.values()].map(
			(held): WorkbenchData => ({
				path: held.path,
				title: held.title,
				slug: held.slug,
				unanswered: name,
				columns: held.columns,
				cards: held.cards,
				root: held.root,
				attachmentCount: held.attachmentCount,
				...heldHand(statuses.get(held.path)?.status, held, now),
			}),
		);
		return { walked: false, members: kept };
	}

	// The walk's own order is path-sorted and deliberate, so that two heads
	// walking one tree report it identically. It is preserved rather than
	// re-sorted here.
	const members = (forest.workbenches ?? []).map((member): WorkbenchData => {
		const held = previous.get(member.path);
		if (member.refused !== undefined && member.refused !== "") {
			// The workbench itself would not read. A row in this condition
			// keeps whatever identity its anchor gave up before it failed,
			// and carries no subtree, because it never opened.
			return {
				path: member.path,
				title: member.title,
				slug: member.slug,
				refused: member.refused,
				columns: new Map(),
				cards: new Map(),
				holding: [],
			};
		}
		const statusMember = statuses.get(member.path);
		const listingMember = listings.get(member.path);
		const unanswered =
			firstSet(
				member.unanswered,
				statusMember?.unanswered,
				listingMember?.unanswered,
			) ?? (member.tree === undefined ? "dinah.unanswered" : undefined);
		if (unanswered !== undefined) {
			// The workbench opened and declined this checkpoint's read. Its
			// last-known subtree is left standing rather than cleared, because
			// a passing race must not blank a customer's tree.
			log(`workbench at ${member.path} did not answer: ${unanswered}`);
			return {
				path: member.path,
				title: member.title,
				slug: member.slug,
				unanswered,
				columns: held?.columns ?? new Map(),
				cards: held?.cards ?? new Map(),
				root: held?.root,
				attachmentCount: held?.attachmentCount,
				...heldHand(statusMember?.status, held, now),
			};
		}
		return {
			path: member.path,
			title: member.title,
			slug: member.slug,
			columns: joinColumns(statusMember?.status),
			cards: joinCards(listingMember?.listing),
			root: member.tree?.root,
			attachmentCount: statusMember?.status?.attachment_count,
			...heldHand(statusMember?.status, held, now),
		};
	});
	return { walked: true, members };
}

/** The first of its arguments that is set and non-empty. */
function firstSet(...values: (string | undefined)[]): string | undefined {
	for (const value of values) {
		if (value !== undefined && value !== "") {
			return value;
		}
	}
	return undefined;
}

// ---------------------------------------------------------------------------
// The one read an expanded Attachments row makes
// ---------------------------------------------------------------------------

/**
 * One entity's attachments, fetched when a reader asks for them.
 *
 * An empty `ref` asks about the workbench itself, because an omitted argument
 * is how the binary expects that question and composing "workbench" here would
 * be a second spelling of a reference the resolver already owns. The answer is
 * never cached (dinah-335's Decision 2): the call runs on every expansion of
 * the row and the count on the row itself still comes from the checkpoint, so
 * an attachment added or renamed since the last expansion is shown as it now
 * stands, and the only cost of that freshness is one call a row somebody
 * opened once more.
 */
export async function readAttachments(
	spawner: Spawner,
	exe: string,
	root: string,
	ref: string,
	log: (line: string) => void,
): Promise<AttachmentListing | undefined> {
	const args = ref === "" ? ["attachments"] : ["attachments", ref];
	const outcome = await runDinah(spawner, exe, pinned(root, args), { cwd: root });
	if (outcome.kind !== "ok") {
		log(`dinah attachments ${ref} at ${root}: ${outcome.kind}`);
		return undefined;
	}
	return outcome.json as AttachmentListing;
}

// ---------------------------------------------------------------------------
// The provider
// ---------------------------------------------------------------------------

/** One workspace folder, as the provider tracks it. */
export interface FolderState {
	readonly folder: string;
	readonly folderName: string;
	readonly resolution: WorkbenchResolution;
	mode: FolderMode;
	rows: RootRow[];
	/**
	 * When an answer last established which workbenches this folder holds,
	 * in milliseconds, or undefined while none ever has.
	 *
	 * A folder with no rows is two different things, and this is what tells
	 * them apart: dinah said there is nothing here, or nobody has managed to
	 * ask. It carries the moment rather than a yes, because the report it
	 * feeds is a vacancy and a vacancy expires. Every checkpoint that gets an
	 * answer restamps it, so a folder that goes quiet stops being counted as
	 * a confident nothing once the answer behind it is older than the window
	 * is willing to trust. `holdingSnapshot` is its only reader.
	 */
	membershipAnsweredAt: number | undefined;
	/** The single-workbench root, on a folder that resolved to exactly one. */
	readonly root?: string;
}

/** What the provider needs in order to read and to report. */
export interface TreeDeps {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly log: (line: string) => void;
	readonly caseInsensitive: boolean;
	/** The sentence a folder with no workbench beneath it shows. */
	readonly deadEndSentence: (refusal: string) => string;
	/**
	 * Renders one row's text in the language the editor is displaying.
	 *
	 * Optional so that the several hundred existing unit-layer call sites go on
	 * driving the provider without each naming a localizer; an absent one reads
	 * English. extension.ts always passes one.
	 */
	readonly t?: Localizer;
	/**
	 * Reads the wall clock, so the unit layer stamps a fetch without waiting.
	 *
	 * Optional for the reason `t` is: the existing call sites construct a
	 * provider without one, and an absent reader is the real clock.
	 */
	readonly now?: () => number;
	/**
	 * Resolves one workspace folder's workbench again, the way activation
	 * first resolved it.
	 *
	 * A folder that resolved to no workbench at all is the one kind this
	 * provider never re-read, because it has no rows to re-read and nothing
	 * beneath it to walk. That left its vacancy standing on a single answer
	 * for the life of the window, and a vacancy that expires needs somebody
	 * to renew it. The checkpoint calls this and the answer restamps the
	 * folder, so the claim that nothing is there is re-earned at the same
	 * cadence every other claim on the bar is.
	 *
	 * Optional for the reason `t` and `now` are. Where it is absent such a
	 * folder simply stops being renewed, which the bar reports as a doubt.
	 */
	readonly resolve?: (folder: string) => Promise<WorkbenchResolution>;
}

/**
 * The refusals that say a folder holds no workbench.
 *
 * `no-workbench` is a path the caller named, through the folder's pinned
 * `dinah.workbench` setting, that carries no `workbench.md`.
 * `no-configured-workbench` is the stored setting naming such a path once the
 * search has also found nothing local. Both report the same thing about the
 * disk, which is that there is nothing there to hold a card, so both earn a
 * vacancy.
 *
 * Membership is what the set states, and everything outside it is a doubt.
 * The rule was written the other way round until round five, excluding
 * `no-workbench-found` and `ambiguous-workbench` and admitting the rest, and
 * that shape cannot be safe here: `internal/contract/contract.go` publishes
 * over a hundred refusal names and this extension owns none of them, so an
 * exclusion list admits every name it has not been taught about. Three
 * published names say the opposite of emptiness and were being admitted.
 * `dinah.unreadable-workbench` is a `workbench.md` the walk found and could
 * not open, `dinah.damaged-workbench` is one whose anchor will not parse, and
 * `dinah.needs-container-migration` is a workbench that is simply in the
 * layout the format used to have. Each of those is a place that may be
 * holding the reader's card.
 */
const VACANCY_REFUSALS: readonly string[] = [
	NO_WORKBENCH,
	NO_CONFIGURED_WORKBENCH,
];

/**
 * Whether a resolution settles that a folder holds no workbench at all.
 *
 * Three questions have to be yes together, and each one has been the reason a
 * vacancy was claimed without grounds. The resolution has to be a refusal,
 * because an ok resolution names a workbench whose hand is a separate
 * question. Dinah has to have produced that refusal, because a spawn that
 * never ran and an answer that would not parse arrive wearing the same shape
 * and say nothing about what is on disk. And the refusal has to be one of the
 * names above.
 *
 * A refusal minted after this was written falls to the doubt, which is the
 * direction that costs a warning rather than a false reassurance. The cost of
 * that direction is real and nothing here catches it: a future refusal that
 * genuinely does mean emptiness has to be added to the set by hand, and until
 * somebody does the bar warns at a folder it could speak confidently about.
 *
 * Membership in the set is necessary and it is not sufficient, because one of
 * the names in it covers two situations. `dinah.no-workbench` carries the
 * `found` key when the pinned path holds no workbench of its own but its own
 * container holds one a level down, and that path names a place dinah refused
 * to read rather than a place confirmed empty. The workbench beneath it may
 * be holding the reader's own claimed card, so the honest answer for it is
 * the doubt. Nothing here opens that workbench; the folder becomes unheard
 * through the existing report path and the bar warns, exactly as it already
 * does for `dinah.no-workbench-found` and `dinah.ambiguous-workbench`.
 * `dinah.no-configured-workbench` is untouched by that test, since
 * `internal/bench` raises it through `contract.Refuse`, which carries no
 * context at all.
 *
 * One predicate rather than the same three conditions at the two producers,
 * so that what earns a vacancy is decided once.
 */
export function vacancyAnsweredBy(resolution: WorkbenchResolution): boolean {
	if (resolution.state !== "refused" || !resolution.answered) {
		return false;
	}
	if (
		resolution.refusal === NO_WORKBENCH &&
		resolution.context?.[FOUND_BENEATH] !== undefined
	) {
		return false;
	}
	return VACANCY_REFUSALS.includes(resolution.refusal);
}

/**
 * The report a place with no rows of its own gets.
 *
 * One function rather than the same conditional at each of the two sites that
 * needs it, because those two sites are exactly where the reassuring answer
 * was reachable without evidence and a single spelling is what keeps a later
 * edit from restoring that. A stamp produces the vacancy and its absence
 * produces the doubt, so nothing here can name a vacancy it cannot date.
 */
function vacancyOrDoubt(
	source: string,
	answeredAt: number | undefined,
): WorkbenchHoldingReport {
	if (answeredAt === undefined) {
		return { state: "unheard", source };
	}
	return { state: "vacant", source, answeredAt };
}

/**
 * What one row says about the reader's hand there.
 *
 * A row with no data at all, and a row whose data carries no path to key it
 * on, are both unheard rather than empty. Dropping either used to leave the
 * bar with nothing to warn about, which is how a window that could not read
 * a workbench came to tell its reader they were holding nothing.
 */
function holdingReportOf(
	row: RootRow,
	membershipAnsweredAt: number | undefined,
): WorkbenchHoldingReport {
	const data = row.data;
	if (row.rowKind === "deadEnd") {
		// A dead-end row is drawn several ways and only some of them are
		// dinah saying the folder holds no workbench. markSole draws the same
		// row for a forest folder that came back with no members at all,
		// blankState draws it for a folder this window could not reach dinah
		// about, and reading either as a vacancy is how this bar has twice
		// claimed an empty hand it could not see. The stamp the folder is
		// carrying settles it, and a folder nobody has answered for carries
		// none.
		return vacancyOrDoubt(row.folder, membershipAnsweredAt);
	}
	if (data === undefined) {
		// An unexpanded candidate is this case too. The resolution named
		// several workbenches here and this window has opened none of them,
		// so what is held in them is genuinely unread.
		return { state: "unheard", source: row.candidate?.path ?? row.folder };
	}
	if (data.fetchedAt === undefined || data.path === "") {
		// No status call has answered for this row, or one has and gave no
		// path to key the row on. The second is kept out of the count
		// because two such rows would fold together on the empty string, and
		// a hand that cannot be counted is a hand this window cannot report.
		return { state: "unheard", source: data.path === "" ? row.folder : data.path };
	}
	return {
		state: "answered",
		source: data.path,
		title: data.title === "" ? UNTITLED_WORKBENCH : data.title,
		holding: data.holding,
		fetchedAt: data.fetchedAt,
	};
}

/**
 * What one folder's rows say about the reader's hand, one report per row.
 *
 * A folder holding no rows still produces a report, because a place that drops
 * out of the answer leaves a summary with nothing to be uncertain about, which
 * is the defect holdingSnapshot's own comment records. Both callers below read
 * this rather than walking the rows themselves.
 */
function reportsForFolder(
	state: FolderState,
): readonly WorkbenchHoldingReport[] {
	if (state.rows.length === 0) {
		return [vacancyOrDoubt(state.folder, state.membershipAnsweredAt)];
	}
	return state.rows.map((row) =>
		holdingReportOf(row, state.membershipAnsweredAt),
	);
}

/** One workspace folder as the provider is told about it. */
export interface FolderInput {
	readonly folder: string;
	readonly name: string;
	readonly resolution: WorkbenchResolution;
}

/**
 * The sidebar's data source.
 *
 * getChildren is synchronous over already-loaded state for every level but
 * two. A candidate row's first expansion resolves it once, and an
 * Attachments row's expansion fetches that entity's own list, which is
 * never cached and so runs on every expansion. Everything else was fetched
 * by load() or by the checkpoint loop.
 */
export class DinahTreeProvider {
	private readonly folders = new Map<string, FolderState>();

	constructor(private readonly deps: TreeDeps) {}

	/** Every folder this provider is tracking, in the order it was given them. */
	get states(): readonly FolderState[] {
		return [...this.folders.values()];
	}

	/** Reads every folder, replacing whatever was held for it. */
	async load(inputs: readonly FolderInput[]): Promise<void> {
		this.folders.clear();
		for (const input of inputs) {
			this.folders.set(input.folder, this.blankState(input));
		}
		for (const input of inputs) {
			await this.refresh(input.folder);
		}
		this.markSole();
	}

	/** Re-reads one folder's rows, keeping its last-known subtrees on a race. */
	async refresh(folder: string): Promise<void> {
		const state = this.folders.get(folder);
		if (state === undefined) {
			return;
		}
		switch (state.mode) {
			case "single": {
				const data = await readWorkbench(
					this.deps.spawner,
					this.deps.exe,
					state.root ?? folder,
					this.deps.log,
					state.rows[0]?.data,
					this.deps.now,
				);
				state.rows = [
					{
						rowKind: "workbenchRoot",
						folder,
						folderName: state.folderName,
						data,
						description: this.rootDescription(data.path, folder),
						sole: false,
					},
				];
				// readWorkbench returns a row whatever happened, so this
				// folder's membership is settled from here on and the row
				// itself carries whether its reads answered. The stamp is
				// never read for a folder in this mode, which always has its
				// one row, and it is written anyway so that no mode leaves
				// the field saying something untrue about the folder.
				state.membershipAnsweredAt = this.clock();
				break;
			}
			case "forest": {
				const previous = new Map<string, WorkbenchData>();
				for (const row of state.rows) {
					if (row.data !== undefined) {
						previous.set(row.data.path, row.data);
					}
				}
				const reading = await readForest(
					this.deps.spawner,
					this.deps.exe,
					folder,
					previous,
					this.deps.log,
					this.deps.now,
				);
				// A walk that answered settles what this folder holds, and
				// restamps it. A walk that fails keeps the members the last
				// good one found, which are still an answer somebody gave,
				// and keeps the stamp it had, which is what lets the folder
				// age out of trust while the walks go on failing.
				if (reading.walked) {
					state.membershipAnsweredAt = this.clock();
				}
				state.rows = reading.members.map((data) => ({
					rowKind: "workbenchForest" as const,
					folder,
					folderName: state.folderName,
					data,
					// A forest row always carries its path, because a folder
					// holding several customers will often hold several
					// same-titled workbenches and the path is what tells them
					// apart.
					description: relativeTo(
						data.path,
						folder,
						this.deps.caseInsensitive,
					),
					sole: false,
				}));
				break;
			}
			case "dead-end":
				// A dead end has no rows to re-read and nothing beneath it to
				// walk, so what gets re-asked is the resolution itself, which
				// is the answer this folder's vacancy rests on. Renewing it
				// is what keeps the vacancy honest: dinah says again that
				// there is no workbench here, or it stops saying so and the
				// bar reports a doubt instead of a confident empty hand.
				await this.reresolve(state);
				break;
			case "candidates":
				// A candidate row is a stub until it is expanded, and the
				// resolution already named which workbenches this folder
				// holds, so there is nothing to re-ask at the folder level.
				// Each candidate row reports its own hand as unread until a
				// reader opens it.
				break;
		}
		this.markSole();
	}

	/**
	 * What every place this provider watches last said the actor holds.
	 *
	 * The answer is total over what the provider knows. Every row produces a
	 * report, and a folder holding no rows produces one of its own, so no
	 * failure can take a place out of the snapshot and leave the bar with
	 * nothing to be uncertain about. That was the defect twice over: a folder
	 * whose walk never answered had no rows to record the silence on, and the
	 * summary read the missing entry as a confirmed empty hand.
	 *
	 * Whether the folder's own membership is known is what its report turns
	 * on. A folder dinah has answered for contributes `vacant` when it has no
	 * rows, because there is nothing beneath it to hold a card. A folder
	 * nobody has heard from contributes `unheard`, which is what makes the
	 * bar warn on the very first failed walk instead of only after a good one
	 * has left rows behind.
	 *
	 * Two workspace folders resolving to the same workbench collapse to one
	 * answered report, because a card held there is one card and counting it
	 * twice would tell a reader they hold two. The first report for a root
	 * wins, so the answer follows workspace folder order the way rootRows()
	 * does. An unheard report is never folded away, since folding one would
	 * be dropping a doubt.
	 */
	holdingSnapshot(): readonly WorkbenchHoldingReport[] {
		const reports: WorkbenchHoldingReport[] = [];
		const seen = new Set<string>();
		for (const state of this.folders.values()) {
			for (const report of reportsForFolder(state)) {
				if (report.state === "answered") {
					const key = this.rootKey(report.source);
					if (seen.has(key)) {
						continue;
					}
					seen.add(key);
				}
				reports.push(report);
			}
		}
		return reports;
	}

	/**
	 * Every workbench one folder has answered for, named and titled.
	 *
	 * This is holdingSnapshot narrowed to one folder and to the arm that
	 * carries a root, which is what a caller wanting to run something against
	 * each workbench under a folder needs. The two share reportsForFolder
	 * rather than each walking the rows, so a change to what a row reports
	 * reaches both.
	 *
	 * No cross-folder deduplication happens here, because there is one folder.
	 * A folder this provider was never told about answers with nothing, which
	 * is the same answer it would give for a folder holding no workbench.
	 */
	rootsFor(
		folder: string,
	): readonly { readonly root: string; readonly title: string }[] {
		const state = this.folders.get(folder);
		if (state === undefined) {
			return [];
		}
		const found: { root: string; title: string }[] = [];
		for (const report of reportsForFolder(state)) {
			if (report.state === "answered") {
				found.push({ root: report.source, title: report.title });
			}
		}
		return found;
	}

	/**
	 * The key two roots are compared on.
	 *
	 * Separators are folded because one directory reached two ways is still
	 * one directory, and case is folded on the platforms this provider is
	 * told to fold it on, which is the same flag every other path comparison
	 * in this module reads.
	 */
	private rootKey(root: string): string {
		const posix = root.replace(/\\/g, "/");
		return this.deps.caseInsensitive ? posix.toLowerCase() : posix;
	}

	/** The rows every folder contributes, in workspace folder order. */
	rootRows(): RootRow[] {
		const rows: RootRow[] = [];
		for (const state of this.folders.values()) {
			rows.push(...state.rows);
		}
		return rows;
	}

	getTreeItem(element: TreeElement): TreeItemSpec {
		return treeItemFor(element, this.deps.t);
	}

	async getChildren(element?: TreeElement): Promise<TreeElement[]> {
		if (element === undefined) {
			return this.rootRows().map((row) => ({ kind: "root", row }));
		}
		switch (element.kind) {
			case "root":
				return this.rootChildren(element.row);
			case "note":
				return [];
			case "column":
			case "group":
				return childElements(
					element.row,
					element.node,
					element,
					this.deps.log,
				);
			case "card": {
				// The eager count decides here. The list itself is one call
				// this row's own expansion makes, never one the checkpoint
				// makes, so a tree of two hundred cards still costs no
				// attachments call to draw.
				const count = element.view?.attachment_count ?? 0;
				if (count === 0) {
					return [];
				}
				const root = element.row.data?.path;
				const ref = element.view?.ref ?? element.node.ref;
				if (root === undefined || ref === undefined || ref === "") {
					return [];
				}
				return [{ kind: "attachmentsGroup", row: element.row, root, ref, count }];
			}
			case "attachmentsGroup": {
				const listing = await readAttachments(
					this.deps.spawner,
					this.deps.exe,
					element.root,
					element.ref,
					this.deps.log,
				);
				if (listing === undefined) {
					return [
						{
							kind: "note",
							owner: element.row,
							text: "This checkpoint could not read the attachments here.",
							tooltip:
								"dinah attachments did not answer; see the Dinah output channel.",
						},
					];
				}
				return listing.attachments.map((view) => ({
					kind: "attachment" as const,
					row: element.row,
					view,
				}));
			}
			case "attachment":
				return [];
		}
	}

	/** Resolves a candidate row on its first expansion and no time after. */
	private async rootChildren(row: RootRow): Promise<TreeElement[]> {
		if (row.rowKind === "deadEnd") {
			return [];
		}
		if (row.rowKind === "workbenchCandidate" && row.data === undefined) {
			row.pending ??= this.resolveCandidate(row);
			await row.pending;
		}
		const data = row.data;
		if (data === undefined) {
			return row.failure === undefined
				? []
				: [
						{
							kind: "note",
							owner: row,
							text: `This workbench would not resolve (${row.failure}).`,
							tooltip: row.failure,
						},
					];
		}
		if (unreadableWithoutIdentity(data)) {
			return [];
		}

		const notes: TreeElement[] = [];
		if (data.refused !== undefined && data.refused !== "") {
			notes.push({
				kind: "note",
				owner: row,
				text: `This workbench's definition would not open (${data.refused}).`,
				tooltip: data.refused,
			});
			// A workbench that would not open has no subtree to draw under
			// the note, so the note is the whole of its children.
			return notes;
		}
		if (data.unanswered !== undefined && data.unanswered !== "") {
			notes.push({
				kind: "note",
				owner: row,
				text: `This checkpoint's read did not answer (${data.unanswered}).`,
				tooltip: data.unanswered,
			});
		}
		// The workbench's own attachments stand after the columns, as one row,
		// carrying the count status already reported. The ref is empty because
		// an omitted argument is how the binary is asked about the workbench
		// itself, and the list is this row's own expansion to fetch.
		const attachmentEntries: TreeElement[] =
			data.attachmentCount !== undefined && data.attachmentCount > 0
				? [
						{
							kind: "attachmentsGroup",
							row,
							root: data.path,
							ref: "",
							count: data.attachmentCount,
						},
					]
				: [];
		return [...notes, ...this.columnsOf(row, data), ...attachmentEntries];
	}

	/**
	 * The column rows of one workbench, in the flow's own declared order.
	 *
	 * Each row also carries the column standing immediately after it in that
	 * order, which is what Pull aims at (dinah-375 OQ-1). The order is the
	 * column axis's declared order, `declaredValues` over `l.Bench.Columns`
	 * in internal/verb/tree.go, which is the same dense-position slice
	 * `downstreamOf` walks in internal/verb/pull.go, so the next node here
	 * names the column the Go side would call downstream. The AXIS_COLUMN
	 * filter runs before the look-ahead so nothing standing among the column
	 * nodes could be mistaken for one.
	 */
	private columnsOf(row: RootRow, data: WorkbenchData): TreeElement[] {
		const nodes = (data.root?.children ?? []).filter(
			(node) => node.axis === AXIS_COLUMN,
		);
		const columns: TreeElement[] = [];
		for (const [index, node] of nodes.entries()) {
			const view = data.columns.get(node.value ?? "");
			if (view === undefined) {
				// Only reachable when a column was deleted between the status
				// and tree calls of one checkpoint. The row decorates with the
				// conservative defaults and self-heals next checkpoint.
				this.deps.log(
					`tree names a column status did not: ${node.value ?? "(unnamed)"}`,
				);
			}
			const nextColumnRef = nodes[index + 1]?.value;
			const nextColumn =
				nextColumnRef === undefined ? undefined : data.columns.get(nextColumnRef);
			columns.push({ kind: "column", row, node, view, nextColumnRef, nextColumn });
		}
		return columns;
	}

	private async resolveCandidate(row: RootRow): Promise<void> {
		const path = row.candidate?.path;
		if (path === undefined) {
			return;
		}
		try {
			row.data = await readWorkbench(
				this.deps.spawner,
				this.deps.exe,
				path,
				this.deps.log,
				row.data,
				this.deps.now,
			);
		} catch (err) {
			row.failure = String(err);
		}
	}

	/** A single-workbench row's own disambiguating path, or nothing. */
	private rootDescription(root: string, folder: string): string {
		const relative = relativeTo(root, folder, this.deps.caseInsensitive);
		return relative === "" ? "" : relative;
	}

	/** The wall clock this provider stamps its answers with. */
	private clock(): number {
		return (this.deps.now ?? Date.now)();
	}

	/**
	 * Asks again whether a dead-end folder really holds no workbench.
	 *
	 * The answer either renews the folder's stamp or leaves it to expire, and
	 * nothing else about the folder changes. A resolution that now names a
	 * workbench, or several, is deliberately not turned into rows here: what
	 * the tree draws for such a folder is a separate surface with its own
	 * card, and the honest thing for this one to report meanwhile is that it
	 * cannot say what is held there, which is what an unrenewed stamp
	 * produces.
	 */
	private async reresolve(state: FolderState): Promise<void> {
		const resolve = this.deps.resolve;
		if (resolve === undefined) {
			return;
		}
		const resolution = await resolve(state.folder);
		if (vacancyAnsweredBy(resolution)) {
			state.membershipAnsweredAt = this.clock();
		}
	}

	/** Turns one folder's resolution into the mode and the stub rows it gets. */
	private blankState(input: FolderInput): FolderState {
		const resolution = input.resolution;
		if (resolution.state === "ok") {
			return {
				folder: input.folder,
				folderName: input.name,
				resolution,
				mode: "single",
				root: resolution.root,
				rows: [],
				// The resolution named the root, and readWorkbench returns a
				// row on every path, so this is stamped by the first refresh.
				// Until one runs, this window has heard nothing about the
				// folder and says so.
				membershipAnsweredAt: undefined,
			};
		}
		// The candidates mode draws the list the resolution carried, so a
		// resolution that could not produce one falls through to the dead-end
		// return below. Taking this branch with nothing to draw is the shape
		// that reads as a folder confirmed to hold nothing.
		if (
			resolution.refusal === AMBIGUOUS_WORKBENCH &&
			!resolution.candidatesUnknown
		) {
			return {
				folder: input.folder,
				folderName: input.name,
				resolution,
				mode: "candidates",
				// The resolution is the answer here: these are the
				// workbenches this folder holds, and it was given just now.
				// Which cards are held inside them stays unheard until a
				// candidate is expanded, and each candidate row says that for
				// itself.
				membershipAnsweredAt: this.clock(),
				rows: (resolution.candidates ?? []).map((candidate) => ({
					rowKind: "workbenchCandidate" as const,
					folder: input.folder,
					folderName: input.name,
					candidate,
					description: relativeTo(
						candidate.path,
						input.folder,
						this.deps.caseInsensitive,
					),
					sole: false,
				})),
			};
		}
		if (resolution.refusal === NO_WORKBENCH_FOUND) {
			// The walk is what closes this case: a folder holding no workbench
			// of its own may still hold several beneath it, and the upward
			// climb never sees them because it walks the other way.
			return {
				folder: input.folder,
				folderName: input.name,
				resolution,
				mode: "forest",
				rows: [],
				// The walk is what establishes membership here, and it has
				// not run. A folder in this state contributing nothing is
				// what let a failed first walk read as an empty hand.
				membershipAnsweredAt: undefined,
			};
		}
		return {
			folder: input.folder,
			folderName: input.name,
			resolution,
			mode: "dead-end",
			rows: [this.deadEndRow(input, resolution.refusal)],
			// Only dinah's own refusal is an answer. Everything else reaching
			// this arm is the window failing to run the binary, time it out
			// or parse it, and each of those wears the same shape as a
			// refusal once the kind has been flattened into the refusal
			// string. Stamping them all was how a folder the extension could
			// not launch dinah in came to be drawn as a folder confirmed to
			// hold nothing.
			membershipAnsweredAt: vacancyAnsweredBy(resolution)
				? this.clock()
				: undefined,
		};
	}

	/** The single informational row a folder with nothing beneath it draws. */
	deadEndRow(input: FolderInput, refusal: string): RootRow {
		return {
			rowKind: "deadEnd",
			folder: input.folder,
			folderName: input.name,
			refusal,
			sentence: this.deps.deadEndSentence(refusal),
			description: "",
			sole: false,
		};
	}

	/**
	 * Marks the sole row, which is the only row drawn expanded by default.
	 *
	 * A window holding one workbench opens onto its columns; a window holding
	 * several opens onto a list of workbenches, because expanding all of them
	 * would bury the list the reader came for.
	 */
	private markSole(): void {
		// A folder whose walk found nothing at all draws the same dead-end row
		// the refusal drew before the walk existed, because a directory with
		// nothing beneath it is exactly that case. This runs before the count
		// below, so a window holding one such folder still counts one row.
		for (const state of this.folders.values()) {
			if (state.mode === "forest" && state.rows.length === 0) {
				state.rows = [
					this.deadEndRow(
						{
							folder: state.folder,
							name: state.folderName,
							resolution: state.resolution,
						},
						NO_WORKBENCH_FOUND,
					),
				];
			}
		}
		const rows = this.rootRows();
		for (const row of rows) {
			row.sole = rows.length === 1;
		}
	}
}

/**
 * The children of a column row or of a state group row, mapped by the kind
 * each child node declares.
 *
 * This is the whole of "draw whatever the tree returned". A column whose
 * children are state groups yields group rows; a column whose children are
 * card leaves yields card rows directly, with no group level and no branch
 * here on the column's kind or on its AwaitingOutside flag. The two shapes
 * travel the same code path, which is why one fixture of the second shape
 * stands for every column that produces it.
 *
 * A column row's children carry one row the tree did not return: the
 * column's own attachments, appended last. The count came with the status
 * answer rather than the tree, and the row sits under the column itself
 * rather than under a state group, because a state group is a heading over
 * cards and the attachments belong to the station.
 */
export function childElements(
	row: RootRow,
	node: TreeNode,
	parent: TreeElement,
	log: (line: string) => void = () => undefined,
): TreeElement[] {
	const column =
		parent.kind === "column"
			? parent.view
			: parent.kind === "group"
				? parent.column
				: undefined;
	const groupValue = parent.kind === "group" ? parent.node.value : undefined;
	const children: TreeElement[] = [];
	for (const child of node.children ?? []) {
		if (child.hidden !== undefined) {
			// A depth or filter default this extension did not ask for. The
			// node is drawn as normal and the reason is reported rather than
			// asserted on, so a later default cannot crash the view.
			log(
				`tree node hides something: kind=${child.kind} axis=${child.axis ?? ""} value=${child.value ?? ""}`,
			);
		}
		if (child.kind === NODE_GROUP) {
			children.push({ kind: "group", row, node: child, column });
		} else if (child.kind === NODE_CARD) {
			children.push({
				kind: "card",
				row,
				node: child,
				view: row.data?.cards.get(child.id ?? ""),
				column,
				groupValue,
			});
		}
	}
	// The column's own attachments, after every card, when the status answer
	// says there are any. Under the column row alone, never repeated beneath
	// each state group, and carrying the count status reported so the row can
	// be drawn before the list it stands for is fetched.
	if (parent.kind === "column") {
		const count = column?.attachment_count ?? 0;
		const root = row.data?.path;
		const ref = column !== undefined ? columnRef(column) : undefined;
		if (count > 0 && root !== undefined && ref !== undefined) {
			children.push({ kind: "attachmentsGroup", row, root, ref, count });
		}
	}
	return children;
}
