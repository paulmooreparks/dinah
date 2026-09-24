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
	CONTEXT_ATTACHMENT,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_COLLECTION_PREFIX,
	CONTEXT_COMMENT,
	CONTEXT_ITEM_CLOSED,
	CONTEXT_ITEM_LOCKED_SUFFIX,
	CONTEXT_ITEM_PENDING,
	CONTEXT_ITEM_PREFIX,
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
	COMMAND_OPEN_COMMENT,
	COMMAND_OPEN_ITEM,
} from "./identity";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type {
	AttachmentListing,
	AttachmentView,
	CardView,
	ColumnView,
	CommentView,
	DesignatedComment,
	ForestAnswer,
	ItemView,
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
	/**
	 * True when something beneath this row waits on the operator, which draws
	 * the glyph composed with the attention dot instead of as a theme icon.
	 */
	readonly attention?: true;
}

/**
 * The same icon with the attention dot, or the icon unchanged.
 *
 * `on` false returns `icon` itself, the same object, so a row with nothing
 * waiting keeps a byte-identical spec rather than a new object carrying the
 * same fields.
 */
export function withAttention(icon: IconSpec, on: boolean): IconSpec {
	return on ? { ...icon, attention: true } : icon;
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
	 * Whether this window acts as the workbench's own operator, as `status`
	 * reports it. Held from the last good checkpoint when this one's status
	 * did not answer, for the reason `actor` beside it is.
	 *
	 * The item rows read it, because an item the operator owns can be settled
	 * by nobody else and a row that offered the act anyway would be offering
	 * a refusal.
	 */
	readonly isOperator?: boolean;
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

/** One workbench this window has resolved, as the MCP provider needs it. */
export interface McpTarget {
	/** The workbench's own absolute root, as `dinah --json status` reported it. */
	readonly root: string;
	/** The workbench's own title, empty when it has none. Never substituted. */
	readonly title: string;
}

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
			/**
			 * This card's own checklist items, fetched once per checkpoint when
			 * the card's view carries operator_pending greater than zero and this
			 * window is the operator's, absent otherwise and absent when the read
			 * failed. It is what a card's hover names its waiting branches from
			 * without a further call once the branch is expanded.
			 */
			readonly checklist?: readonly ItemView[];
	  }
	| {
			/**
			 * One collection under an entity row: the Comments, Checklist or
			 * Attachments heading a reader opens to reach its members.
			 *
			 * One arm for all three, because the grammar decides which
			 * collections an entity has and the extension states no list of
			 * its own. What varies between them is the kind they hold.
			 */
			readonly kind: "collection";
			readonly row: RootRow;
			/** The workbench root this collection's own fetch is pinned to. */
			readonly root: string;
			/** The reference of the entity this collection hangs from. */
			readonly holder: string;
			/** The row kind of the element this collection was drawn under. */
			readonly holderKind: TreeElement["kind"];
			/** The entity kind this collection holds, as `contents` spells it. */
			readonly memberKind: string;
			/**
			 * The stored item kind a published judgement branch narrows by,
			 * absent on a legacy collection the extension grouped itself.
			 * It decides the row's label and nothing else; the order the
			 * branches draw in is the payload's.
			 */
			readonly narrow?: string;
			/**
			 * The direct member count the CLI published, absent on a legacy
			 * collection. Preferred over the member array's length, which at a
			 * depth cut is the number of rows that came rather than the number
			 * the collection holds.
			 */
			readonly memberCount?: number;
			/** The member nodes, in the order `contents` returned them. */
			readonly members: readonly TreeNode[];
			/**
			 * The collection's own reference.
			 *
			 * It serves two jobs, and which one it is doing is decided by
			 * whether the row also carries members. A published judgement
			 * branch carries both: the reference is what tells three branches
			 * of one card apart in a key, and the members are already here. The
			 * workbench root's own collections carry the reference and no
			 * members, because they are composed from ROOT_COLLECTIONS before
			 * any call is made, and that is the row whose expansion goes and
			 * asks. A collection the extension grouped itself from a flat run
			 * carries no reference at all.
			 */
			readonly ref?: string;
			/**
			 * The holding card's own checklist items, carried on the collection
			 * row so a judgement branch or the legacy Checklist row can name its
			 * waiting items without a further call. Set on the same terms the
			 * card element's own `checklist` is (dinah-599 section 5.4), absent
			 * on every collection whose memberKind is not `item`.
			 */
			readonly checklist?: readonly ItemView[];
	  }
	| {
			readonly kind: "comment";
			readonly row: RootRow;
			readonly root: string;
			/** The reference of the entity the comment hangs from. */
			readonly holder: string;
			/** The `contents` node, which carries the ref and the count below. */
			readonly node: TreeNode;
			/** The joined view, absent when the detail call did not answer. */
			readonly view?: CommentView;
	  }
	| {
			/**
			 * The row a member whose kind this extension has no view for still
			 * gets (dinah-519 section 3.3).
			 *
			 * Unreachable against the grammar as it stands, and not dead code:
			 * it is what makes the claim true that a kind added to
			 * `containment` reaches the tree with no edit here.
			 */
			readonly kind: "entity";
			readonly row: RootRow;
			readonly root: string;
			readonly holder: string;
			readonly node: TreeNode;
	  }
	| {
			readonly kind: "attachment";
			readonly row: RootRow;
			/**
			 * The workbench root the listing that produced this row was pinned
			 * to, taken from the collection element's own `root` rather than
			 * from `row.folder`. A forest row's folder holds several
			 * workbenches, so the folder is not a workbench root at all there.
			 */
			readonly root: string;
			/**
			 * The reference of the entity this attachment hangs from, taken
			 * from the listing's own `ref` rather than from the collection's.
			 *
			 * The listing resolves the workbench's own reference to the literal
			 * `workbench`, so only the listing's answer composes a reference a
			 * later call can resolve.
			 */
			readonly owner: string;
			/** The `contents` node, which carries the ref and the title below. */
			readonly node: TreeNode;
			/** The joined view, absent when the listing call did not answer. */
			readonly view?: AttachmentView;
	  }
	| {
			readonly kind: "item";
			readonly row: RootRow;
			readonly root: string;
			/** The card the item hangs from, so a later call composes. */
			readonly card: string;
			/** The `contents` node, which carries the ref and the count below. */
			readonly node: TreeNode;
			/** The joined view, absent when the detail call did not answer. */
			readonly view?: ItemView;
			/** Whether this window acts as the workbench's operator. */
			readonly isOperator: boolean;
	  };

/**
 * The character that joins the parts of an element key.
 *
 * A NUL cannot appear in a path, a reference, a column identifier or a row's
 * drawn text on any platform this extension runs on, so two different rows
 * cannot compose one key by their parts running together.
 */
const KEY_SEPARATOR = "\u0000";

/** The workbench a root row stands for, however the row resolved. */
function rootPathOf(row: RootRow): string {
	return row.data?.path ?? row.candidate?.path ?? row.folder;
}

/**
 * A row's identity, for deduplicating a selection against the invoked row.
 *
 * The editor hands a command the row it was executed on and, separately, the
 * rows that were selected, and nothing in TreeViewOptions.canSelectMany
 * promises that the same object appears in both. So targetsFor deduplicates by
 * a key rather than by object identity, and this is the key the tree's own
 * rows compose: the kind, then the workbench root, then whichever of the
 * reference, the node reference, the attachment identifier or the text the row
 * carries.
 *
 * It is a key rather than a display string and nothing renders it. The
 * reference a run records for a row is a different thing and already exists:
 * treeItemFor(element, t).label is the label the reader sees in the tree.
 */
export function elementKey(element: TreeElement): string {
	const parts: readonly (string | undefined)[] = keyPartsOf(element);
	return [element.kind, ...parts]
		.map((part) => part ?? "")
		.join(KEY_SEPARATOR);
}

/** The parts that tell two rows of one kind apart. */
function keyPartsOf(element: TreeElement): readonly (string | undefined)[] {
	switch (element.kind) {
		case "root":
			return [rootPathOf(element.row)];
		case "note":
			return [rootPathOf(element.owner), element.text];
		case "column":
			return [rootPathOf(element.row), element.view?.id ?? element.node.value];
		case "group":
			return [
				rootPathOf(element.row),
				element.column?.id,
				element.node.value,
			];
		case "card":
			return [rootPathOf(element.row), element.view?.ref ?? element.node.ref];
		case "collection":
			// The reference first, because three judgement branches below one
			// card all hold items and would otherwise share a key. A legacy
			// collection the extension grouped itself has no reference of its
			// own and keys as it always did.
			return [element.root, element.holder, element.ref ?? element.memberKind];
		case "attachment":
			return [element.root, element.owner, element.view?.id ?? element.node.ref];
		case "comment":
		case "entity":
			return [element.root, element.node.ref];
		case "item":
			return [element.root, element.node.ref ?? element.view?.ref];
	}
}

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

// ---------------------------------------------------------------------------
// Checklist items: their labels, their hold direction and their contextValues
// ---------------------------------------------------------------------------

/** How long an item's one-line label may run before it is elided. */
const ITEM_LABEL_LIMIT = 120;

/**
 * An item's text as one bounded line.
 *
 * An item's own text is prose and may carry paragraphs, and a tree row is one
 * line whatever it is given, so the whitespace is collapsed rather than left
 * to the editor to flatten however it likes. It is exported so a unit test
 * drives the function the tree calls rather than a copy of it.
 */
export function itemLabel(text: string): string {
	const collapsed = text.replace(/\s+/g, " ").trim();
	return collapsed.length > ITEM_LABEL_LIMIT
		? `${collapsed.slice(0, ITEM_LABEL_LIMIT - 1)}\u2026`
		: collapsed;
}

/** The RootRow any element belongs to, whichever member its arm spells it in. */
function rowOf(element: TreeElement): RootRow {
	return element.kind === "note" ? element.owner : element.row;
}

/**
 * A row of `contents` children, grouped by the kind each one declares.
 *
 * The groups come out in the order the kinds were first met, which is the
 * order `contents` emitted them, which is the mount order the containment
 * table declares. Sorting them here, or ordering them from a table of this
 * extension's own, would be a second statement of the grammar; under a card
 * the answer already reads Comments, Checklist, Attachments.
 */
export function partitionByKind(
	nodes: readonly TreeNode[],
): readonly (readonly [string, readonly TreeNode[]])[] {
	const groups = new Map<string, TreeNode[]>();
	for (const node of nodes) {
		const existing = groups.get(node.kind);
		if (existing === undefined) {
			groups.set(node.kind, [node]);
		} else {
			existing.push(node);
		}
	}
	return [...groups.entries()];
}

/**
 * A detail answer's views indexed by the reference the join reads.
 *
 * By reference rather than by position, because `contents` and the detail
 * readers can disagree about which members exist and a positional join shifts
 * every row after the divergence onto somebody else's view.
 */
function indexByRef<T extends { readonly ref: string }>(
	views: readonly T[] | undefined,
): Map<string, T> {
	const byRef = new Map<string, T>();
	for (const view of views ?? []) {
		byRef.set(view.ref, view);
	}
	return byRef;
}

/**
 * A detail answer's views indexed by the attachment's identifier.
 *
 * An identifier is the attachment's identity, where a reference is one of
 * the addresses that reach it, and the workbench's own attachments print
 * under two different references from the containment walk and from the
 * attachment listing (dinah-554). Joining by identifier finds the same
 * attachment under either spelling; joining by reference does not.
 */
function indexById<T extends { readonly id: string }>(
	views: readonly T[] | undefined,
): Map<string, T> {
	const byId = new Map<string, T>();
	for (const view of views ?? []) {
		byId.set(view.id, view);
	}
	return byId;
}

/**
 * A comment's opening words as one bounded line.
 *
 * A comment has no title, so this is the whole of what a row can be named by,
 * and the author and the timestamp go in the grey description instead: every
 * comment on this workbench but a handful is the agent's, and a label opening
 * with the author would spend the left edge of a narrow panel, which is the
 * part clipping takes last, on a constant string.
 *
 * Three steps and no more. The text up to the first newline; then a leading
 * ATX heading marker stripped, meaning a run of one or more `#` followed by at
 * least one space; then the whitespace collapsed and the line elided, which is
 * itemLabel's own body and limit. General Markdown stripping is refused: a
 * guard widened by example fits only its examples, and the heading marker is
 * the one construct the data shows, five of one card's thirty-one comments
 * opening `## WHAT SHIPPED`.
 *
 * A comment whose label collapses to nothing falls back to the reference the
 * caller passed, so the row is still addressable.
 */
export function commentLabel(text: string, fallback = ""): string {
	const firstLine = text.split("\n", 1)[0] ?? "";
	const stripped = firstLine.replace(/^#+[ \t]+/, "");
	const label = itemLabel(stripped);
	return label === "" ? fallback : label;
}

/**
 * A comment row's tooltip: its reference, who wrote it and when, and the body.
 *
 * The body verbatim rather than a prefix of it, because the tooltip is where a
 * reader settles whether this is the comment they wanted without opening a
 * tab.
 */
export function commentTooltip(
	view: CommentView,
	ref: string,
	t: Localizer = ENGLISH,
): string {
	return [
		ref,
		t("comment.row.author", { author: view.author, ts: view.ts }),
		view.body,
	].join("\n");
}

/**
 * A comment row's description: its position in the thread, its author and its
 * time.
 *
 * The ordinal is the trailing segment of the reference, which is the comment's
 * one-based position among its holder's comments, so the number a reader sees
 * is the one they would type. The timestamp is rendered exactly as Dinah
 * stores it, because every other Dinah surface prints it that way and a second
 * spelling is a second thing to translate and to test.
 */
export function commentDescription(
	view: CommentView,
	ref: string,
	t: Localizer = ENGLISH,
): string {
	const ordinal = ref.slice(ref.lastIndexOf("/") + 1);
	return t("comment.row.description", {
		ordinal,
		author: view.author,
		ts: view.ts,
	});
}

/**
 * The label a collection row carries, by the entity kind it holds.
 *
 * The one per-kind list this design keeps at the structural level, and it
 * states no containment: an entry missing from it costs a translated noun and
 * not a row, because the fallback is the kind token itself.
 *
 * Each entry names its key as a literal rather than composing one, because the
 * guard in test/unit/l10n-keys.test.ts reads call sites and an interpolated
 * key is a key it cannot check.
 */
const COLLECTION_LABELS: Readonly<
	Record<string, (t: Localizer) => string>
> = {
	comment: (t) => t("tree.collection.comments"),
	item: (t) => t("tree.checklistGroup.label"),
	attachment: (t) => t("tree.attachments.label"),
};

/**
 * The label a judgement branch draws, keyed by the item kind it narrows by.
 *
 * A lookup and nothing more. It decides what a row is called and never what
 * order the rows come in, which is the payload's to decide: the CLI publishes
 * the branches in the order its own declaration puts them, and this extension
 * draws what it is given. A table here that also ordered them would be the
 * second statement of the grammar that dinah-519 took out.
 */
/**
 * The kind token a published collection node carries, which is the one token
 * this extension reads to tell a branch from an entity. It is the CLI's own
 * spelling and is never translated.
 */
const KIND_COLLECTION = "collection";

const NARROW_LABELS: Readonly<Record<string, (t: Localizer) => string>> = {
	open_question: (t) => t("tree.collection.questions"),
	acceptance_criterion: (t) => t("tree.collection.criteria"),
	decision: (t) => t("tree.collection.decisions"),
};

/**
 * The icon a collection row carries, keyed by member kind.
 *
 * Keyed the same way COLLECTION_LABELS is keyed, so a kind this extension
 * gains is given its noun and its glyph in one place rather than two.
 */
const COLLECTION_ICONS: Readonly<Record<string, IconSpec>> = {
	comment: { id: "comment-discussion" },
	item: { id: "checklist" },
	attachment: { id: "files" },
};

/** The icon a judgement branch carries, keyed by the kind it narrows by. */
const NARROW_ICONS: Readonly<Record<string, IconSpec>> = {
	open_question: { id: "comment-unresolved" },
	acceptance_criterion: { id: "verified" },
	decision: { id: "lightbulb" },
};

/**
 * A collection row's icon, resolved in the order collectionLabel resolves
 * its noun: the narrowed kind first, then the member kind.
 *
 * A collection this extension has no glyph for draws none, because an icon
 * says what the members are and a guess about an unknown kind would say
 * something false. The label falls through to printing the token in the
 * same case, and a row carrying its raw token is already telling the
 * reader that this extension does not know the kind.
 */
export function collectionIcon(
	memberKind: string,
	narrow?: string,
): IconSpec | undefined {
	if (narrow !== undefined) {
		const branch = NARROW_ICONS[narrow];
		if (branch !== undefined) {
			return branch;
		}
	}
	return COLLECTION_ICONS[memberKind];
}

/**
 * A collection row's label: the narrowed kind's translated noun where the row
 * is a judgement branch, otherwise its member kind's, otherwise the token
 * itself.
 *
 * An unknown narrow token falls through to the member kind rather than being
 * printed raw, so a kind the CLI gains before this extension knows its name
 * draws as Checklist rather than as `risk`.
 *
 * Attachment 1 of dinah-536 said such a token would be rendered verbatim, and
 * this is a deliberate departure from it. A raw `risk` in the tree is a token
 * a reader cannot act on and did not ask to see, where Checklist is at least
 * true: the row does hold checklist items. The spec's own reasoning was that
 * this follows the existing fallback for an unknown member kind, and that
 * fallback is still here, one step further in, for a member kind nobody has
 * a noun for at all.
 */
export function collectionLabel(
	memberKind: string,
	t: Localizer = ENGLISH,
	narrow?: string,
): string {
	if (narrow !== undefined) {
		const branch = NARROW_LABELS[narrow];
		if (branch !== undefined) {
			return branch(t);
		}
	}
	const named = COLLECTION_LABELS[memberKind];
	return named === undefined ? memberKind : named(t);
}

/**
 * One of the workbench's own collections, as the root row has to know it
 * before any call is made.
 */
interface RootCollection {
	/**
	 * The collection's directory name, which is the trailing segment of the
	 * reference the row is opened with: `workbench/<dir>`.
	 */
	readonly dir: string;
	/**
	 * The entity kind this collection holds, as `contents` spells it and
	 * therefore singular where `dir` is plural.
	 *
	 * The collection row needs a memberKind at compose time, for its label,
	 * its contextValue and its partition, and the root composes its element
	 * before it has any member to read a kind off. Deriving it from `dir`
	 * would be either a dir-to-kind mapping, which is a second statement of
	 * the grammar, or the plural directory name, which misses
	 * COLLECTION_LABELS and yields a contextValue no other collection row
	 * carries. So it is written down.
	 */
	readonly memberKind: string;
	/**
	 * How many members the checkpoint says the collection holds, read only to
	 * decide whether the row is drawn at all.
	 */
	readonly count: (data: WorkbenchData) => number;
}

/**
 * The workbench's own collections the grouped projection does not already
 * draw. Columns and cards are absent because the kanban draws them.
 *
 * This table is the one place in this extension that states a piece of the
 * containment grammar, and it is here because `contents` counts rank from the
 * workbench: no depth of it answers the root's own mounts without also
 * answering every card. The remedy is a depth counted from the named
 * reference, or a kind filter, either of which changes the verb and belongs to
 * a card of its own.
 *
 * A directory is listed here only so the root can skip the call when the count
 * is zero, which today it is for every one of them. The member rows come from
 * `contents workbench/<dir> --depth all` and never from this table; only the
 * row that holds them is composed from it.
 */
const ROOT_COLLECTIONS: readonly RootCollection[] = [
	{
		dir: "attachments",
		memberKind: "attachment",
		count: (data) => data.attachmentCount ?? 0,
	},
];

/**
 * The six things an item's column can be doing to the card that carries it.
 *
 * An object rather than a union, because two catalogue groups are keyed by
 * these six tokens and the guard in test/unit/l10n-keys.test.ts reads a
 * family's members off a module-level object literal's property names. Writing
 * the six here once is what keeps `item.hold.*` and
 * `form.file.column.detail.*` from each needing a hand-copied list.
 */
export const HOLD_DIRECTIONS = {
	entryAhead: true,
	entryPassed: true,
	exitHere: true,
	exitAhead: true,
	exitPassed: true,
	nothing: true,
};

/** One of the six, as every surface that reads the direction spells it. */
export type HoldDirection = keyof typeof HOLD_DIRECTIONS;

/**
 * What a column declaring `hold` does to a card standing at `cardIndex`, given
 * that the column stands at `candidateIndex` in the same declared flow order.
 *
 * Twelve inputs and twelve answers, which is the four hold values crossed with
 * the three positions. The table is written out rather than reduced to a pair
 * of nested conditions, because three of the twelve overlap two plausible
 * rules and a reduction would let branch order settle them silently.
 *
 * The `both` row is where that matters. A column holding both ways offers two
 * true sentences at each position, so each of its three cells is a call rather
 * than a consequence. Ahead of the card the answer is the entry, because both
 * holds are still in the card's future and entry is the one it reaches first.
 * At the card's own column the answer is the exit, and that cell is not a
 * matter of wording: the card is standing there, so the entry has happened,
 * and saying the entry is passed would claim the item stops the card where it
 * stands when the item will in fact refuse the card's next move out. Behind
 * the card both sentences are true and differ only in which event they name,
 * so that cell goes to the exit, the later of the two events the card passed.
 *
 * The first parameter is ColumnView.hold as wire.ts declares it, which is
 * `string | undefined` like every other optional wire member, so this function
 * has to answer for a value outside the three typed words. It answers
 * "nothing", the way it answers for the two spellings that already mean no
 * hold, rather than inventing a policy of its own.
 */
export function holdDirection(
	hold: string | undefined,
	candidateIndex: number,
	cardIndex: number,
): HoldDirection {
	const ahead = candidateIndex > cardIndex;
	const here = candidateIndex === cardIndex;
	switch (hold) {
		case "on":
			return ahead ? "entryAhead" : "entryPassed";
		case "out":
			if (ahead) {
				return "exitAhead";
			}
			return here ? "exitHere" : "exitPassed";
		case "both":
			if (ahead) {
				return "entryAhead";
			}
			return here ? "exitHere" : "exitPassed";
		default:
			return "nothing";
	}
}

/**
 * The columns of one workbench in the flow's own declared order, by the ref a
 * tree node names them with.
 *
 * It reads the same AXIS_COLUMN nodes columnsOf draws the column rows from, so
 * the positions this answers are the positions the reader sees.
 */
export function columnOrderOf(
	data: WorkbenchData | undefined,
): readonly string[] {
	return (data?.root?.children ?? [])
		.filter((node) => node.kind === NODE_GROUP && node.axis === AXIS_COLUMN)
		.map((node) => node.value ?? "");
}

/**
 * The ref of the column a card stands in, given the workbench's own data.
 *
 * CardView.column carries the column's identifier and the flow order carries
 * the ref, which is the slug where a column has one, so the two are joined
 * through the ColumnView rather than compared directly.
 */
export function cardColumnRefOf(
	data: WorkbenchData | undefined,
	cardRef: string,
): string | undefined {
	if (data === undefined) {
		return undefined;
	}
	let columnId: string | undefined;
	for (const view of data.cards.values()) {
		if (view.ref === cardRef) {
			columnId = view.column;
			break;
		}
	}
	if (columnId === undefined || columnId === "") {
		return undefined;
	}
	for (const [ref, view] of data.columns) {
		if (view.id === columnId) {
			return ref;
		}
	}
	return undefined;
}

/** The ref of the column an item names, joined through the ColumnView map. */
function itemColumnRefOf(
	data: WorkbenchData | undefined,
	item: ItemView,
): string | undefined {
	if (item.column === undefined || item.column === "") {
		return undefined;
	}
	for (const [ref, view] of data?.columns ?? []) {
		if (view.id === item.column || ref === item.column) {
			return ref;
		}
	}
	return undefined;
}

/**
 * What one item's own column is doing to the card that carries it.
 *
 * A column neither the item nor the card resolves to answers "nothing", which
 * is what a column declaring no hold answers. An item filed against a column
 * this workbench no longer declares is one dinah-501 refuses every claim over,
 * and saying so is that card's sentence rather than this one's; what this line
 * can honestly say is that it knows of no stop.
 */
export function itemHoldDirection(
	data: WorkbenchData | undefined,
	cardRef: string,
	item: ItemView,
): HoldDirection {
	const order = columnOrderOf(data);
	const cardColumn = cardColumnRefOf(data, cardRef);
	const itemColumn = itemColumnRefOf(data, item);
	if (cardColumn === undefined || itemColumn === undefined) {
		return "nothing";
	}
	const candidateIndex = order.indexOf(itemColumn);
	const cardIndex = order.indexOf(cardColumn);
	if (candidateIndex < 0 || cardIndex < 0) {
		return "nothing";
	}
	return holdDirection(
		data?.columns.get(itemColumn)?.hold,
		candidateIndex,
		cardIndex,
	);
}

/**
 * The word for an item's kind, which every item surface shares.
 *
 * It answers the rendered word rather than the catalogue key, so each of the
 * three keys is spelled as a literal at the one call site the guard in
 * test/unit/l10n-keys.test.ts can read. A reader who meets "Open question" in
 * the filing form and "question" on the row has been given two names for one
 * thing, which is why the form calls this too.
 */
export function itemKindWord(kind: string, t: Localizer): string {
	switch (kind) {
		case "open_question":
			return t("item.kind.question");
		case "decision":
			return t("item.kind.decision");
		default:
			return t("item.kind.criterion");
	}
}

/**
 * The word for an item's state.
 *
 * A state outside the four the format declares reads as pending, which is the
 * direction that says least: a damaged anchor is shown as unsettled rather
 * than as settled by somebody.
 */
export function itemStateWord(state: string, t: Localizer): string {
	switch (state) {
		case "resolved":
			return t("item.state.resolved");
		case "verified":
			return t("item.state.verified");
		case "failed":
			return t("item.state.failed");
		default:
			return t("item.state.pending");
	}
}

/** The kind segment of a contextValue, which is one word per item kind. */
function contextKindOf(kind: string): string {
	switch (kind) {
		case "open_question":
			return "question";
		case "decision":
			return "decision";
		default:
			return "criterion";
	}
}

/**
 * The stored kinds a judgement branch may narrow by, which are the three the
 * CLI publishes as branches.
 *
 * The membership test is what keeps contextKindOf's own fallback out of this
 * path. That fallback answers `criterion` for anything it does not know,
 * which is the conservative answer on an item row and the wrong one here: a
 * branch narrowing by a kind this extension has no name for would otherwise
 * offer Add an Acceptance Criterion, and a reader would file the wrong kind
 * from a row that said something else. Such a branch falls back to the
 * unnarrowed value instead, which offers all three and asks the reader.
 */
const NARROWABLE_KINDS: readonly string[] = [
	"open_question",
	"decision",
	"acceptance_criterion",
];

/**
 * The contextValue one collection row carries.
 *
 * A collection that narrows by an item kind carries a third segment naming
 * that kind, so the menus can offer one filing command on a Questions branch
 * rather than all three. A collection that narrows by nothing carries the two
 * segments it has always carried, which is the value an older binary's flat
 * checklist still produces.
 */
export function collectionContextValue(
	memberKind: string,
	narrow?: string,
): string {
	const base = `${CONTEXT_COLLECTION_PREFIX}.${memberKind}`;
	return narrow !== undefined && NARROWABLE_KINDS.includes(narrow)
		? `${base}.${contextKindOf(narrow)}`
		: base;
}

/**
 * The contextValue one item row carries.
 *
 * The state axis collapses the three closed states to one word, because all
 * three offer exactly Reopen, and a state outside the four the format declares
 * reads as closed too: a damaged anchor then offers Reopen rather than a
 * terminal verb, which is the conservative direction.
 *
 * The `.locked` suffix withholds the terminal verbs from a window that is not
 * the operator, on an item the operator owns. Only a pending value takes it,
 * because closeItem is where the owner is checked and Reopen is deliberately
 * left open. The row is not hidden and is not greyed: its tooltip says the
 * item is the operator's to settle, and Open and Comment stay on it, so
 * anybody can read the question and argue on it.
 *
 * One race is accepted here rather than guarded. isOperator comes from the
 * last checkpoint, so a window whose status has gone stale can offer a verb
 * the tool then refuses by name, and the next checkpoint repaints the row.
 * That is the race actionsFor already accepts for a card somebody else claimed
 * between the paint and the click.
 */
export function itemContextValue(view: ItemView, isOperator: boolean): string {
	const pending = view.state === "pending";
	const state = pending ? CONTEXT_ITEM_PENDING : CONTEXT_ITEM_CLOSED;
	const base = `${CONTEXT_ITEM_PREFIX}.${contextKindOf(view.kind)}.${state}`;
	return pending && view.owner === ITEM_OWNER_OPERATOR && !isOperator
		? `${base}.${CONTEXT_ITEM_LOCKED_SUFFIX}`
		: base;
}

/** The one owner value dinah enforces against the actor. */
export const ITEM_OWNER_OPERATOR = "operator";

/** The icon a row draws for one item, by its kind and its state. */
function itemIcon(view: ItemView): { readonly id: string } {
	if (view.state === "failed") {
		return { id: "error" };
	}
	if (view.state !== "pending") {
		return { id: "pass" };
	}
	switch (view.kind) {
		case "open_question":
			return { id: "question" };
		case "decision":
			return { id: "lightbulb" };
		default:
			return { id: "circle-large-outline" };
	}
}

// ---------------------------------------------------------------------------
// The attention indicator: who is waiting on the operator, and what a row
// says about it
// ---------------------------------------------------------------------------

/**
 * True when an item is in the operator's queue, by the CLI's own rule.
 *
 * This mirrors bench.ItemAwaitsOperator in internal/bench/entity.go: a
 * pending open question or decision naming the operator or naming no owner
 * at all. An absent owner reads as empty, exactly as the Go side reads an
 * absent frontmatter key. An acceptance criterion never qualifies, because
 * Test verifies a criterion rather than the operator.
 */
export function itemNeedsOperator(view: ItemView | undefined): boolean {
	if (view === undefined) {
		return false;
	}
	if (view.state !== "pending") {
		return false;
	}
	if (view.kind !== "open_question" && view.kind !== "decision") {
		return false;
	}
	const owner = view.owner ?? "";
	return owner === ITEM_OWNER_OPERATOR || owner === "";
}

/**
 * True when a card waits on the operator: blocked, or holding his items.
 *
 * A card standing in Acceptance carries no special case here (dinah-599
 * decisions/4): it carries the dot exactly when this test says so, whatever
 * column it stands in.
 */
export function cardNeedsOperator(view: CardView | undefined): boolean {
	if (view === undefined) {
		return false;
	}
	return view.state === STATE_BLOCKED || (view.operator_pending ?? 0) > 0;
}

/**
 * The heading, up to three names one per line, and "and others" past three.
 *
 * Given an empty list it returns the empty string, and a caller never passes
 * one, since a row with no names carries no dot.
 */
export function attentionLines(names: readonly string[], t: Localizer): string {
	if (names.length === 0) {
		return "";
	}
	const lines = [t("tree.attention.heading"), ...names.slice(0, 3)];
	if (names.length > 3) {
		lines.push(t("tree.attention.others"));
	}
	return lines.join("\n");
}

/**
 * A row's ordinary tooltip with its attention lines put first, ahead of
 * everything the tooltip already said.
 *
 * A row whose ordinary tooltip is empty, which is the state group and the
 * collection rows, gets the attention lines as its whole tooltip rather than
 * a line with nothing following it.
 */
function withAttentionTooltip(attention: string, tooltip: string): string {
	if (attention === "") {
		return tooltip;
	}
	return tooltip === "" ? attention : `${attention}\n${tooltip}`;
}

/**
 * The card and state-group rows' own names: the references of the cards
 * beneath a node that need the operator, in the order the tree draws them.
 *
 * `cards` is the workbench's own card-view map, absent for a row whose
 * checkpoint carried none, in which case every card beneath reads as absent
 * and contributes nothing.
 */
function cardRefsNeedingOperator(
	node: TreeNode,
	cards: ReadonlyMap<string, CardView> | undefined,
): string[] {
	const names: string[] = [];
	const walk = (n: TreeNode): void => {
		if (n.kind === NODE_CARD) {
			const view = cards?.get(n.id ?? "");
			if (view !== undefined && cardNeedsOperator(view)) {
				const ref = view.ref ?? n.ref ?? "";
				if (ref !== "") {
					names.push(ref);
				}
			}
			return;
		}
		for (const child of n.children ?? []) {
			walk(child);
		}
	};
	for (const child of node.children ?? []) {
		walk(child);
	}
	return names;
}

/**
 * The card row's own branch names: which of a card's judgement branches hold
 * an item waiting on the operator, Questions before Decisions.
 *
 * `checklist` absent means the card's view says something waits but which
 * branch holds it was never read, so the row names the Checklist label
 * instead of a branch.
 */
function branchNamesNeedingOperator(
	checklist: readonly ItemView[] | undefined,
	t: Localizer,
): string[] {
	if (checklist === undefined) {
		return [collectionLabel("item", t)];
	}
	const kinds = new Set(
		checklist.filter((item) => itemNeedsOperator(item)).map((item) => item.kind),
	);
	return ["open_question", "decision"]
		.filter((kind) => kinds.has(kind))
		.map((kind) => collectionLabel("item", t, kind));
}

/**
 * A card row's attention lines: the blocked sentence, the heading and branch
 * names, or both when a card is blocked and also holds an item for the
 * operator.
 */
function cardAttentionLines(
	view: CardView | undefined,
	checklist: readonly ItemView[] | undefined,
	t: Localizer,
): string {
	if (!cardNeedsOperator(view)) {
		return "";
	}
	const lines: string[] = [];
	if (view?.state === STATE_BLOCKED) {
		lines.push(t("tree.attention.blocked"));
	}
	if ((view?.operator_pending ?? 0) > 0) {
		const block = attentionLines(branchNamesNeedingOperator(checklist, t), t);
		if (block !== "") {
			lines.push(block);
		}
	}
	return lines.join("\n");
}

/** An item row's attention line: the one sentence it draws when it waits. */
function itemAttentionLines(view: ItemView | undefined, t: Localizer): string {
	return itemNeedsOperator(view) ? t("tree.attention.answer") : "";
}

/** The description beside an item's label: its kind and its state. */
export function itemDescription(view: ItemView, t: Localizer): string {
	return t("item.row.description", {
		kind: itemKindWord(view.kind, t),
		state: itemStateWord(view.state, t),
	});
}

/**
 * Everything an item row says on hover, one fact per line.
 *
 * The hold sentence is the one a reader can get nowhere else, and it is keyed
 * by the same holdDirection the filing form reads, so the two surfaces cannot
 * disagree about what a column is doing.
 */
/**
 * The designated comment drawn on one line: who wrote it, and what it says.
 *
 * A comment the store cannot attribute is drawn as one the store cannot name
 * rather than as a blank or as an invented author, which is the whole of what
 * recording the absence buys. The two cases are two sentences from the
 * catalogue rather than one with a hole in it.
 */
function answerLine(designated: DesignatedComment, t: Localizer): string {
	const body = designated.body ?? "";
	if (designated.author === undefined || designated.author === "") {
		return t("item.answer.unattributed", { body });
	}
	return t("item.answer.by", { author: designated.author, body });
}

export function itemTooltip(
	view: ItemView,
	direction: HoldDirection,
	columnTitle: string,
	locked: boolean,
	t: Localizer,
): string {
	const lines = [view.text];
	// The state, which rode the row's description and was clipped with it.
	// The kind survives that loss because the row's icon carries it, and
	// nothing carries the state, so the tooltip does (dinah-517).
	lines.push(`${t("item.stateLabel")} ${itemStateWord(view.state, t)}`);
	// The answer a settled item designates, drawn where the retired note was
	// drawn. The reference is carried by both checklist reads and the comment
	// itself only by the full one, so the tooltip says what it has: the words
	// where a read opened them and the reference to reach them where it did
	// not.
	if (view.designated !== undefined) {
		lines.push(`${t("item.answer")} ${answerLine(view.designated, t)}`);
	} else if (view.resolution !== undefined && view.resolution !== "") {
		lines.push(`${t("item.answer")} ${view.resolution}`);
	}
	lines.push(t(`item.hold.${direction}`, { 0: columnTitle }));
	if (view.owner !== undefined && view.owner !== "") {
		lines.push(`${t("item.owner")} ${view.owner}`);
	}
	if (view.comment_count !== undefined && view.comment_count > 0) {
		lines.push(t("item.comments", { count: String(view.comment_count) }));
	}
	if (locked) {
		lines.push(t("item.locked"));
	}
	return lines.join("\n");
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
			return { id: "record-small", color: "charts.blue" };
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

/**
 * The icon a state group carries, keyed by the same axis value its label
 * is keyed by.
 *
 * A group whose value this extension does not recognise draws nothing,
 * which is what groupLabel does with the same value when it prints the
 * token rather than a name for it.
 */
export function groupIcon(value: string | undefined): IconSpec | undefined {
	switch (value) {
		case STATE_READY:
			return { id: "watch" };
		case STATE_ACTIVE:
			return { id: "play-circle" };
		case STATE_BLOCKED:
			return { id: "debug-pause" };
		default:
			return undefined;
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
const WORKBENCH_ICON: IconSpec = { id: "tools" };

/** The icon a column row carries when the last good read cached it. */
const COLUMN_ICON: IconSpec = { id: "split-horizontal" };

/**
 * The icon the row a member of an unknown kind receives.
 *
 * A collection whose kind this extension has no name for draws no icon at
 * all, because a wrong glyph asserts something about the members and an
 * absent one asserts nothing. This row is the opposite case. The kind is
 * unknown, and the row still has to be told apart from the rows around it.
 */
const ENTITY_ICON: IconSpec = { id: "symbol-misc" };

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
	return "collapsed";
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
			const isOperator = element.row.data?.isOperator === true;
			const names =
				isOperator && !broken
					? cardRefsNeedingOperator(element.node, element.row.data?.cards)
					: [];
			const attention = attentionLines(names, t);
			return {
				label: view?.title ?? element.node.value ?? "",
				description: broken
					? t("tree.column.damaged")
					: columnDescription(view, element.node, t),
				tooltip: withAttentionTooltip(
					attention,
					broken
						? columnBrokenTooltip(view, element.row.data, t)
						: columnTooltip(
								view,
								element.node,
								element.nextColumn,
								element.nextColumnRef,
								t,
							),
				),
				contextValue: columnActionsFor(view, element.nextColumnRef),
				collapsibleState: "collapsed",
				icon: broken ? WARNING_ICON : withAttention(COLUMN_ICON, names.length > 0),
			};
		}
		case "group": {
			const isOperator = element.row.data?.isOperator === true;
			const names = isOperator
				? cardRefsNeedingOperator(element.node, element.row.data?.cards)
				: [];
			const attention = attentionLines(names, t);
			const icon = groupIcon(element.node.value);
			return {
				label: groupLabel(element.node.value, t),
				description: String(element.node.count),
				tooltip: withAttentionTooltip(attention, ""),
				contextValue: CONTEXT_STATE_GROUP,
				collapsibleState: "collapsed",
				icon: icon === undefined ? undefined : withAttention(icon, names.length > 0),
			};
		}
		case "card": {
			const state = cardState(element.view, element.groupValue);
			const ref = element.view?.ref ?? element.node.ref ?? "";
			const isOperator = element.row.data?.isOperator === true;
			const needsOperator = isOperator && cardNeedsOperator(element.view);
			const attention = isOperator
				? cardAttentionLines(element.view, element.checklist, t)
				: "";
			return {
				label: cardLabel(ref, element.node.title ?? element.view?.title),
				description: cardDescription(element.view),
				tooltip: withAttentionTooltip(
					attention,
					cardTooltip(element.node, element.view, element.column, state, t),
				),
				contextValue: actionsFor({ state, column: element.column }),
				// An arrow only when the checkpoint's own total says something
				// is there to expand.
				collapsibleState: cardExpands(element.view) ? "collapsed" : "none",
				icon: withAttention(cardIcon(state), needsOperator),
				command: {
					command: COMMAND_OPEN_CARD,
					title: "Open Card",
					args: [element],
				},
			};
		}
		case "collection": {
			const isOperator = element.row.data?.isOperator === true;
			const names =
				isOperator && element.memberKind === "item"
					? (element.checklist ?? [])
							.filter(
								(item) =>
									itemNeedsOperator(item) &&
									(element.narrow === undefined || item.kind === element.narrow),
							)
							.map((item) => itemLabel(item.text))
					: [];
			const attention = attentionLines(names, t);
			const icon = collectionIcon(element.memberKind, element.narrow);
			return {
				label: collectionLabel(element.memberKind, t, element.narrow),
				description: String(element.memberCount ?? element.members.length),
				tooltip: withAttentionTooltip(attention, ""),
				contextValue: collectionContextValue(
					element.memberKind,
					element.narrow,
				),
				collapsibleState: "collapsed",
				icon: icon === undefined ? undefined : withAttention(icon, names.length > 0),
			};
		}
		case "comment": {
			const ref = element.node.ref ?? "";
			const view = element.view;
			return {
				label: commentLabel(view?.body ?? element.node.title ?? "", ref),
				// The description is the one thing the view supplies, so it is
				// the one thing a refused detail call costs the row.
				...(view === undefined
					? {}
					: {
							description: commentDescription(view, ref, t),
							tooltip: commentTooltip(view, ref, t),
						}),
				// The same glyph on every comment row, whoever wrote it. An
				// icon marking the operator's own comments was specified for
				// four rounds and withdrawn on 2026-09-15: a reader opens one
				// card and reads one thread, and on eighteen of the nineteen
				// cards carrying comments every comment has the same author,
				// so the column would have drawn one repeated glyph.
				icon: { id: "comment" },
				contextValue: CONTEXT_COMMENT,
				// The grammar's own count and never the view's attachments,
				// which answer a different question and are absent on a row
				// whose detail call was refused.
				collapsibleState: element.node.count > 0 ? "collapsed" : "none",
				command: {
					command: COMMAND_OPEN_COMMENT,
					title: "Open Comment",
					args: [element],
				},
			};
		}
		case "entity": {
			const count = element.node.count;
			return {
				label: element.node.title ?? element.node.ref ?? "",
				...(count > 0 ? { description: String(count) } : {}),
				// No contextValue at all rather than an empty string, because
				// toTreeItem already relies on that distinction and a `when`
				// clause comparing against "" would match an empty one.
				collapsibleState: count > 0 ? "collapsed" : "none",
				icon: ENTITY_ICON,
			};
		}
		case "item": {
			const view = element.view;
			if (view === undefined) {
				// The detail call did not answer. The description, the icon
				// and the context value are all composed out of an ItemView,
				// so the row keeps its label and loses the three of them.
				return {
					label: itemLabel(element.node.title ?? ""),
					collapsibleState: element.node.count > 0 ? "collapsed" : "none",
					command: {
						command: COMMAND_OPEN_ITEM,
						title: "Open Item",
						args: [element],
					},
				};
			}
			const direction = itemHoldDirection(element.row.data, element.card, view);
			const contextValue = itemContextValue(view, element.isOperator);
			const attention = element.isOperator ? itemAttentionLines(view, t) : "";
			return {
				label: itemLabel(view.text),
				description: itemDescription(view, t),
				tooltip: withAttentionTooltip(
					attention,
					itemTooltip(
						view,
						direction,
						view.column_title ?? view.column ?? "",
						contextValue.endsWith(`.${CONTEXT_ITEM_LOCKED_SUFFIX}`),
						t,
					),
				),
				icon: withAttention(itemIcon(view), element.isOperator && itemNeedsOperator(view)),
				contextValue,
				// An item's own comments are rows now, and the arrow comes
				// from the grammar's count rather than from comment_count,
				// which the row's description already carries and which says
				// nothing about the mounts a later grammar might add.
				collapsibleState: element.node.count > 0 ? "collapsed" : "none",
				command: {
					command: COMMAND_OPEN_ITEM,
					title: "Open Item",
					args: [element],
				},
			};
		}
		case "attachment": {
			const view = element.view;
			if (view === undefined) {
				// The listing did not answer for this member. The filename,
				// the description, the tooltip, the icon and the open command
				// are all composed out of an AttachmentView, so the row keeps
				// the contents node's title and loses every one of them. No
				// contextValue either, because the delete verb addresses the
				// attachment by an identifier only the listing carries.
				return {
					label: element.node.title ?? element.node.ref ?? "",
					collapsibleState: "none",
				};
			}
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
				// Set whatever the payload says, because an attachment Dinah
				// cannot open is the one a reader most wants gone, and a value
				// gated on openability would withhold Delete from exactly that
				// row (dinah-451 D-4).
				contextValue: CONTEXT_ATTACHMENT,
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
	const isOperator = data?.isOperator === true;
	const attentionNames = isOperator ? workbenchAttentionNames(data) : [];
	const attention = attentionLines(attentionNames, t);
	return {
		label,
		description,
		tooltip: withAttentionTooltip(
			attention,
			detail === "" ? path : `${path}\n${detail}`,
		),
		contextValue,
		collapsibleState,
		icon: withAttention(WORKBENCH_ICON, attentionNames.length > 0),
	};
}

/**
 * The workbench row's own names: the titles of the columns holding a card
 * that needs the operator, in the workbench's own declared column order.
 *
 * The order comes from `data.root`'s own children, filtered to the column
 * axis exactly as columnsOf filters them, which is the flow's declared order
 * columnsOf already reads.
 */
function workbenchAttentionNames(data: WorkbenchData | undefined): string[] {
	const columnNodes = (data?.root?.children ?? []).filter(
		(node) => node.axis === AXIS_COLUMN,
	);
	const names: string[] = [];
	for (const node of columnNodes) {
		const refs = cardRefsNeedingOperator(node, data?.cards);
		if (refs.length === 0) {
			continue;
		}
		const view = data?.columns.get(node.value ?? "");
		names.push(view?.title ?? node.value ?? "");
	}
	return names;
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
		runDinah(spawner, exe, pinned(root, ["list", "cards"]), { cwd: root }),
	]);

	for (const [name, outcome] of [
		["status", status],
		["tree", tree],
		["list cards", listing],
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
): Pick<WorkbenchData, "actor" | "isOperator" | "holding" | "fetchedAt"> {
	if (status === undefined) {
		return {
			actor: held?.actor,
			isOperator: held?.isOperator,
			holding: held?.holding ?? [],
			fetchedAt: held?.fetchedAt,
		};
	}
	return {
		actor: status.actor ?? held?.actor,
		// A binary older than the field omits it, which arrives as undefined
		// and is read as the last checkpoint's answer rather than as false.
		// Reading an absent key as false would lock the operator out of his
		// own items on the checkpoint after an upgrade.
		isOperator: status.is_operator ?? held?.isOperator,
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
		runDinah(spawner, exe, ["list", "cards", "--root", folder], { cwd: folder }),
	]);

	for (const [name, outcome] of [
		["status --root", status],
		["tree --root", tree],
		["list cards --root", listing],
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
 * The reference is the collection's own holder, whatever kind that is, so a
 * comment's attachments are asked for against the comment and the workbench's
 * against the literal `workbench`. Asking the card instead would succeed,
 * answer the card's own attachments, and join none of them to the comment's
 * rows, so the call and not the output is where this has to be right.
 *
 * The answer is never cached (dinah-335's Decision 2): the call runs on every
 * expansion of the row, so an attachment added or renamed since the last
 * expansion is shown as it now stands, and the only cost of that freshness is
 * one call a row somebody opened once more.
 */
export async function readAttachments(
	spawner: Spawner,
	exe: string,
	root: string,
	ref: string,
	log: (line: string) => void,
): Promise<AttachmentListing | undefined> {
	// The workbench's own attachments are asked for by the roster word, which
	// is the shorter of the two spellings that answer the same bytes and the
	// one the references guide teaches. Everything else is the entity's
	// attachments collection, addressed below it.
	const args =
		ref === "" || ref === "workbench"
			? ["list", "attachments"]
			: ["list", `${ref}/attachments`];
	const outcome = await runDinah(spawner, exe, pinned(root, args), { cwd: root });
	if (outcome.kind !== "ok") {
		log(`dinah list ${ref}/attachments at ${root}: ${outcome.kind}`);
		return undefined;
	}
	return outcome.json as AttachmentListing;
}

/**
 * The children one entity holds, as the containment grammar answers them.
 *
 * `--depth all` rather than the default, and that is forced rather than
 * preferred. contentsLimit counts ranks from the workbench and not from the
 * reference the caller named, so the default level yields the children of a
 * card and yields nothing at all below an item or a comment: such a row would
 * draw an arrow, because the node's count is filled whatever depth the walk
 * was cut at, and open onto nothing. Choosing the level per kind would mean
 * holding a copy of rankOfKind here, which is the second statement of the
 * grammar this whole design removes.
 *
 * The answer is never cached, on the terms readAttachments is not: the call
 * runs on every expansion, so a comment posted since the last one is drawn as
 * it now stands.
 */
export async function readContents(
	spawner: Spawner,
	exe: string,
	root: string,
	ref: string,
	log: (line: string) => void,
): Promise<readonly TreeNode[] | undefined> {
	const outcome = await runDinah(
		spawner,
		exe,
		pinned(root, ["list", ref, "--depth", "all"]),
		{ cwd: root },
	);
	if (outcome.kind !== "ok") {
		log(`dinah list ${ref} --depth all at ${root}: ${outcome.kind}`);
		return undefined;
	}
	return (outcome.json as TreeAnswer).root.children ?? [];
}

/**
 * One entity's comments, fetched when a reader opens the thread.
 *
 * Which call serves them depends on what holds them, and the holder's row kind
 * is what decides: `show <card> --fields comments` answers a card's, and
 * `show <item>` answers an item's, because only a card takes a field selector
 * and `show <item> --fields comments` is refused outright. A column's and the
 * workbench's comments have no structured reader at all, so they are not asked
 * for and their rows draw from their `contents` nodes alone.
 *
 * The holder's kind is read off the element the tree already composed, never
 * out of the shape of the reference. Reading a reference's shape is the
 * extension restating the containment grammar, which is the shape this whole
 * design refuses.
 *
 * Three answers rather than two, because a refusal and an unasked question are
 * different events and only the first of them is worth telling a reader about.
 * A single absent answer conflated them, and the day a column's comments reach
 * the grammar every expansion of a column thread would have written a read
 * failure to the channel for a read that never happened.
 */
export type CommentsRead =
	/** The call was made and answered. */
	| { readonly kind: "answered"; readonly views: readonly CommentView[] }
	/** The call was made and refused. */
	| { readonly kind: "refused" }
	/** No call was made, because this holder kind has no structured reader. */
	| { readonly kind: "unasked" };

export async function readComments(
	spawner: Spawner,
	exe: string,
	root: string,
	holder: string,
	holderKind: TreeElement["kind"],
	log: (line: string) => void,
): Promise<CommentsRead> {
	const args =
		holderKind === "card"
			? ["show", holder, "--fields", "comments"]
			: holderKind === "item"
				? ["show", holder]
				: undefined;
	if (args === undefined) {
		return { kind: "unasked" };
	}
	const outcome = await runDinah(spawner, exe, pinned(root, args), { cwd: root });
	if (outcome.kind !== "ok") {
		log(`dinah ${args.join(" ")} at ${root}: ${outcome.kind}`);
		return { kind: "refused" };
	}
	return {
		kind: "answered",
		views: (outcome.json as { comments?: readonly CommentView[] }).comments ?? [],
	};
}

/**
 * Whether a card row draws an expand arrow.
 *
 * One number, summed across every collection the containment grammar gives a
 * card, rather than one per collection. The per-collection counts said nothing
 * about a card whose only content is comments, so such a card drew no arrow,
 * its getChildren was never called, and its comments were unreachable: VS Code
 * asks for the tree item first and decides from what it answers. A mount added
 * to a card is counted in this one number with no edit here.
 */
export function cardExpands(view: CardView | undefined): boolean {
	return (view?.child_count ?? 0) > 0;
}

/**
 * One card's checklist items, fetched when a reader expands the group row.
 *
 * `--fields card,checklist` rather than a bare show, because the item views
 * are the whole of what this row needs and a bare show would carry the card's
 * body, its links, its attachments and its comments on every expansion.
 *
 * The answer is never cached, on the terms readAttachments is not: the call
 * runs on every expansion, so an item somebody answered since the last one is
 * shown as it now stands, and the only cost of that freshness is one call a
 * row somebody opened once more.
 */
export async function readChecklist(
	spawner: Spawner,
	exe: string,
	root: string,
	ref: string,
	log: (line: string) => void,
): Promise<readonly ItemView[] | undefined> {
	const outcome = await runDinah(
		spawner,
		exe,
		pinned(root, ["show", ref, "--fields", "card,checklist"]),
		{ cwd: root },
	);
	if (outcome.kind !== "ok") {
		log(`dinah show ${ref} --fields card,checklist at ${root}: ${outcome.kind}`);
		return undefined;
	}
	return (outcome.json as { checklist?: readonly ItemView[] }).checklist ?? [];
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
	/**
	 * One card's checklist answer, shared by every judgement branch of that
	 * card for the life of one checkpoint.
	 *
	 * A card now draws up to three branches over one checklist, and each of
	 * them needs the same detail answer to put text on its rows. Asking once
	 * per branch would make three calls where the CLI has one answer, and two
	 * branches opened at the same moment would make two calls before either
	 * finished. So what is held here is the promise rather than its result:
	 * a second branch arriving mid-flight joins the first one's call instead
	 * of starting another.
	 *
	 * Refusal and an empty answer are held exactly as success is. A branch
	 * that refused is a branch the next expansion should not retry behind the
	 * reader's back, and the checkpoint is what clears the slate: `refresh`
	 * empties this map, so the next expansion after a refresh asks again.
	 */
	private readonly checklists = new Map<
		string,
		Promise<readonly ItemView[] | undefined>
	>();

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
		// A checkpoint is where a card's checklist stops being what this
		// provider already knows, so every memoized answer goes, successes,
		// empties and refusals alike. Cleared before the unknown-folder
		// return rather than after it, so that a refresh naming a folder this
		// provider does not hold still ends the checkpoint the memo belongs
		// to. The clear is global on purpose: the memo is keyed by workbench
		// root and holding one folder's entries across another's checkpoint
		// would be a cache with two lifetimes.
		this.checklists.clear();
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
	 * Every workbench this window has resolved, for the MCP provider.
	 *
	 * The kept-row predicate is written out rather than borrowed, because
	 * `holdingReportOf` is not exported and its answer would have to be
	 * re-read to be used. It is that function's own condition, clause for
	 * clause: a dead-end row is dropped before its data is looked at, a row
	 * with no data is unheard, and a row whose data carries no path or has
	 * never been fetched is unheard too.
	 *
	 * The title travels unsubstituted, which is why this is not `rootsFor`.
	 * That method hands back the title `holdingReportOf` has already replaced
	 * with UNTITLED_WORKBENCH, so a label built on it reads `Dinah: Dinah` for
	 * a workbench with no title and cannot be told from one genuinely called
	 * Dinah.
	 *
	 * Nothing is folded and nothing is dropped here. Two workspace folders
	 * that both reach one workbench yield two targets, and `planMcpServers`
	 * is what reduces them to one plan, so the deduplication rule is asserted
	 * against the code that performs it.
	 */
	mcpTargets(): readonly McpTarget[] {
		const found: McpTarget[] = [];
		for (const state of this.folders.values()) {
			for (const row of state.rows) {
				if (row.rowKind === "deadEnd") {
					continue;
				}
				const data = row.data;
				if (data === undefined) {
					continue;
				}
				if (data.path === "" || data.fetchedAt === undefined) {
					continue;
				}
				found.push({ root: data.path, title: data.title });
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
			case "group":
				return this.withChecklistsPrefetched(
					childElements(element.row, element.node, element, this.deps.log),
					element.row,
				);
			case "column": {
				// A column's cards come from the grouped projection, because a
				// column does not contain a card in the grammar at all: the
				// workbench contains every card, and the column a card stands
				// at is a field of the card. The column's own mounts come from
				// the grammar like every other entity's, so its comments draw
				// the day the grammar gives it a comments mount, with no edit
				// here.
				const cards = await this.withChecklistsPrefetched(
					childElements(element.row, element.node, element, this.deps.log),
					element.row,
				);
				const root = element.row.data?.path;
				const ref =
					element.view !== undefined ? columnRef(element.view) : undefined;
				if (root === undefined || ref === undefined) {
					return cards;
				}
				return [...cards, ...(await this.collectionsOf(element, root, ref))];
			}
			case "card": {
				const root = element.row.data?.path;
				const ref = element.view?.ref ?? element.node.ref;
				if (root === undefined || ref === undefined || ref === "") {
					return [];
				}
				return this.collectionsOf(element, root, ref);
			}
			case "comment":
			case "entity":
				return this.collectionsOf(
					element,
					element.root,
					element.node.ref ?? "",
				);
			case "item":
				return this.collectionsOf(
					element,
					element.root,
					element.node.ref ?? element.view?.ref ?? "",
				);
			case "attachment":
				return [];
			case "collection":
				return this.collectionMembers(element);
		}
	}

	/**
	 * One entity row's collection rows, from the grammar and from nothing else.
	 *
	 * The children of any reference arrive in one call, partitioned by their
	 * own kind into one collection row per kind, in the order `contents`
	 * returned them, which is the mount order the containment table declares.
	 * No count on the checkpoint decides which rows exist and no per-kind list
	 * is consulted, so a kind the grammar gains reaches the tree here with no
	 * edit: it draws under its own kind token, through the entity row.
	 *
	 * When the holder is a card in an operator window whose view carries
	 * operator_pending greater than zero, it also awaits the card's own
	 * checklist once, through checklistOf's own per-checkpoint cache, and sets
	 * it on every collection row whose memberKind is item: a judgement branch
	 * or the legacy Checklist row, so that row's own hover can name its
	 * waiting items without a further call (dinah-599 section 5.4).
	 */
	private async collectionsOf(
		element: TreeElement,
		root: string,
		ref: string,
	): Promise<TreeElement[]> {
		if (ref === "") {
			return [];
		}
		const children = await readContents(
			this.deps.spawner,
			this.deps.exe,
			root,
			ref,
			this.deps.log,
		);
		if (children === undefined) {
			return [this.contentsNote(rowOf(element))];
		}
		const row = rowOf(element);
		const checklist = await this.checklistForItemCollections(element, root, ref);
		const rows: TreeElement[] = [];
		let flat: TreeNode[] = [];
		// A published collection node becomes one row directly and flushes the
		// run of flat nodes standing in front of it, so the sequence the CLI
		// sent is the sequence a reader sees: comments, then the branches, then
		// attachments. Flat nodes are still grouped by first-seen kind, which
		// is what keeps this extension working against a binary that publishes
		// no branches at all.
		const flush = (): void => {
			if (flat.length === 0) {
				return;
			}
			for (const [memberKind, members] of partitionByKind(flat)) {
				rows.push({
					kind: "collection" as const,
					row,
					root,
					holder: ref,
					holderKind: element.kind,
					memberKind,
					members,
					...(memberKind === "item" && checklist !== undefined ? { checklist } : {}),
				});
			}
			flat = [];
		};
		for (const node of children) {
			if (node.kind !== KIND_COLLECTION) {
				flat.push(node);
				continue;
			}
			flush();
			rows.push({
				kind: "collection" as const,
				row,
				root,
				holder: ref,
				holderKind: element.kind,
				memberKind: node.member_kind ?? "",
				narrow: node.narrow,
				memberCount: node.member_count,
				members: node.children ?? [],
				ref: node.ref,
				...(node.member_kind === "item" && checklist !== undefined ? { checklist } : {}),
			});
		}
		flush();
		return rows;
	}

	/**
	 * The checklist collectionsOf attaches to a holder's own item collections,
	 * absent unless the holder is a card in an operator window whose view says
	 * something waits.
	 */
	private async checklistForItemCollections(
		element: TreeElement,
		root: string,
		ref: string,
	): Promise<readonly ItemView[] | undefined> {
		if (element.kind !== "card") {
			return undefined;
		}
		if (element.row.data?.isOperator !== true) {
			return undefined;
		}
		if ((element.view?.operator_pending ?? 0) <= 0) {
			return undefined;
		}
		return this.checklistOf(root, ref);
	}

	/**
	 * Attaches each card element's own checklist, fetched once per checkpoint,
	 * to every card whose view carries operator_pending greater than zero, in
	 * an operator window. A card row's hover names the branches holding what
	 * waits on him, and a branch row is drawn before its items are, so both
	 * need the card's item views at the moment childElements draws the card
	 * (dinah-599 section 5.4). Every other window, and every card with
	 * nothing waiting, is returned unchanged.
	 */
	private async withChecklistsPrefetched(
		elements: readonly TreeElement[],
		row: RootRow,
	): Promise<TreeElement[]> {
		const data = row.data;
		if (data?.isOperator !== true) {
			return [...elements];
		}
		const root = data.path;
		const pending = elements.filter(
			(el): el is Extract<TreeElement, { kind: "card" }> =>
				el.kind === "card" && (el.view?.operator_pending ?? 0) > 0,
		);
		if (pending.length === 0) {
			return [...elements];
		}
		const fetched = await Promise.all(
			pending.map(
				async (el) =>
					[el, await this.checklistOf(root, el.view?.ref ?? el.node.ref ?? "")] as const,
			),
		);
		const checklists = new Map<TreeElement, readonly ItemView[] | undefined>(fetched);
		return elements.map((el) => {
			const checklist = checklists.get(el);
			return el.kind === "card" && checklist !== undefined ? { ...el, checklist } : el;
		});
	}

	/** The one row a refused `contents` call draws, in place of the members. */
	private contentsNote(row: RootRow): TreeElement {
		// The localizer is bound to a name before it is called, because the
		// guard in test/unit/l10n-keys.test.ts reads a call site off the
		// callee's own name and a parenthesised fallback expression is a call
		// site it cannot see.
		const t = this.deps.t ?? ENGLISH;
		const unreadable = t("tree.contents.unreadable");
		return { kind: "note", owner: row, text: unreadable, tooltip: unreadable };
	}

	/**
	 * One collection row's member rows: the structure from the grammar, the
	 * detail from the kind's own reader, joined on the reference.
	 *
	 * The grammar decides which rows exist. A member the detail call did not
	 * answer for still draws, from its own `contents` node, because the
	 * grammar answered and it is the detail that did not; a view the grammar
	 * named no node for is dropped. The join is on the reference rather than
	 * on position, because `contents` keeps a member `Show` skips, so a
	 * successful detail call can answer one view short and a positional join
	 * would shift every row after it onto somebody else's author.
	 */
	private async collectionMembers(
		element: Extract<TreeElement, { kind: "collection" }>,
	): Promise<TreeElement[]> {
		// The workbench root composes its collection rows before any call is
		// made, so this is where its own members are fetched. Every collection
		// below a card arrived with its holder's own answer.
		let members = element.members;
		// A collection that arrived carrying its members draws them. Only one
		// kind of row arrives without any: the workbench's own collections,
		// which are composed from ROOT_COLLECTIONS before a call is made and
		// carry a reference so that their expansion can go and ask. A
		// published judgement branch carries both its reference and its
		// members, and its reference is its identity rather than an errand:
		// fetching against it here would ask the CLI a second structural
		// question whose answer the first one already held.
		// `narrow` is what tells a published branch from a workbench-root
		// collection, rather than the member count: the CLI omits an empty
		// branch today, but a branch that did arrive empty would otherwise
		// fall through and ask a second structural question whose answer it
		// is already holding.
		if (members.length === 0 && element.narrow === undefined && element.ref !== undefined) {
			const children = await readContents(
				this.deps.spawner,
				this.deps.exe,
				element.root,
				element.ref,
				this.deps.log,
			);
			if (children === undefined) {
				return [this.contentsNote(element.row)];
			}
			members = children;
		}
		switch (element.memberKind) {
			case "comment":
				return this.commentRows(element, members);
			case "item":
				return this.itemRows(element, members);
			case "attachment":
				return this.attachmentRows(element, members);
			default:
				return members.map((node) => ({
					kind: "entity" as const,
					row: element.row,
					root: element.root,
					holder: element.holder,
					node,
				}));
		}
	}

	/** The comment rows of one thread. */
	private async commentRows(
		element: Extract<TreeElement, { kind: "collection" }>,
		members: readonly TreeNode[],
	): Promise<TreeElement[]> {
		const read = await readComments(
			this.deps.spawner,
			this.deps.exe,
			element.root,
			element.holder,
			element.holderKind,
			this.deps.log,
		);
		if (read.kind === "refused") {
			// The rows are drawn from the grammar's own nodes and nothing is
			// appended to say so, because a note row here would stand where a
			// comment belongs and take a row the design does not have. The
			// sentence goes to the output channel instead, which is where a
			// reader who wants to know why a row is bare looks.
			//
			// Only on a refusal. A holder kind with no structured reader was
			// never asked, so there is no failure to report and a sentence
			// here would be a false one.
			const t = this.deps.t ?? ENGLISH;
			this.deps.log(t("tree.comments.unreadable"));
		}
		const byRef = indexByRef(read.kind === "answered" ? read.views : undefined);
		return members.map((node) => ({
			kind: "comment" as const,
			row: element.row,
			root: element.root,
			holder: element.holder,
			node,
			view: byRef.get(node.ref ?? ""),
		}));
	}

	/** The item rows of one card's checklist. */
	private async itemRows(
		element: Extract<TreeElement, { kind: "collection" }>,
		members: readonly TreeNode[],
	): Promise<TreeElement[]> {
		const views = await this.checklistOf(element.root, element.holder);
		if (views === undefined) {
			const t = this.deps.t ?? ENGLISH;
			this.deps.log(t("tree.checklist.unreadable"));
		}
		const byRef = indexByRef(views);
		return members.map((node) => ({
			kind: "item" as const,
			row: element.row,
			root: element.root,
			card: element.holder,
			node,
			view: byRef.get(node.ref ?? ""),
			// An absent answer is read as not-the-operator, which withholds a
			// verb rather than offering one the tool would refuse, and the
			// next checkpoint repaints the row.
			isOperator: element.row.data?.isOperator === true,
		}));
	}

	/**
	 * One card's checklist, read at most once per checkpoint however many
	 * branches ask for it.
	 *
	 * The promise goes into the map before it is awaited, which is what makes
	 * two branches opened in the same tick share one call rather than race to
	 * start two.
	 */
	private checklistOf(
		root: string,
		holder: string,
	): Promise<readonly ItemView[] | undefined> {
		const key = `${root}\u0000${holder}`;
		const held = this.checklists.get(key);
		if (held !== undefined) {
			return held;
		}
		const reading = readChecklist(
			this.deps.spawner,
			this.deps.exe,
			root,
			holder,
			this.deps.log,
		);
		this.checklists.set(key, reading);
		return reading;
	}

	/** The attachment rows of one entity's attachments. */
	private async attachmentRows(
		element: Extract<TreeElement, { kind: "collection" }>,
		members: readonly TreeNode[],
	): Promise<TreeElement[]> {
		const listing = await readAttachments(
			this.deps.spawner,
			this.deps.exe,
			element.root,
			element.holder,
			this.deps.log,
		);
		if (listing === undefined) {
			const t = this.deps.t ?? ENGLISH;
			this.deps.log(t("tree.attachments.unreadable"));
		}
		const byId = indexById(listing?.attachments);
		const byRef = indexByRef(listing?.attachments);
		const owner = listing?.ref ?? element.holder;
		// Every member draws, whether or not the listing answered for it. The
		// filename, the description and the payload path ride the
		// AttachmentView, so a member the listing missed loses those three;
		// what it keeps is the contents node's own title, which is the
		// attachment's description, on the same terms the comment and item
		// arms already degrade. A row the reader can see is what a refusal
		// owes them: dropping the row leaves the collection's own count
		// promising members that open onto nothing (dinah-519 section 4.3).
		//
		// The join is by identifier rather than by reference, because the
		// workbench's own attachments print under two different references
		// from the containment walk and from the attachment listing
		// (dinah-554); an identifier is the attachment's identity and does
		// not vary with which verb printed it. A member carrying no id falls
		// back to the reference join, so nothing that joined before this
		// change stops joining.
		return members.map((node) => ({
			kind: "attachment" as const,
			row: element.row,
			root: element.root,
			owner,
			node,
			view:
				node.id === undefined
					? byRef.get(node.ref ?? "")
					: byId.get(node.id),
		}));
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
		// The workbench's own collections stand after the columns, one row
		// each, composed from ROOT_COLLECTIONS. The table is read only to
		// decide which rows exist and what kind each holds; the members come
		// from `contents workbench/<dir> --depth all` when the row is opened.
		// A count of zero skips the row and therefore skips the call, which is
		// the whole reason the checkpoint's count is read here at all.
		const collections: TreeElement[] = [];
		for (const entry of ROOT_COLLECTIONS) {
			if (entry.count(data) <= 0) {
				continue;
			}
			collections.push({
				kind: "collection",
				row,
				root: data.path,
				holder: "workbench",
				holderKind: "root",
				memberKind: entry.memberKind,
				members: [],
				ref: `workbench/${entry.dir}`,
			});
		}
		return [...notes, ...this.columnsOf(row, data), ...collections];
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
 * The card rows alone. A column's own mounts are collection rows drawn from
 * the containment grammar, which the provider appends after these, because
 * that takes a call and this composition takes none.
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
	return children;
}
