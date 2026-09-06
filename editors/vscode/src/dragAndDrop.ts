// Dragging a card from one column onto another, as pure decision logic.
//
// Nothing here imports vscode, on the same terms tree.ts and cardCommands.ts
// already set out for themselves. extension.ts holds the two controller
// methods and hands this module the rows they carry, so the unit layer drives
// every payload, every resolved target, every verdict and every refusal
// without a VS Code host.
//
// The sidebar renders the effect of a drop rather than the drop itself. No row
// is moved here and no move is predicted here: the verb runs, the tree is
// checkpointed whatever it answered, and the next read draws the card wherever
// the workbench now says it stands. A refused drop therefore leaves the card
// where it was and looks exactly like a drag that missed, so the refusal
// message is the only thing telling the reader which of the two happened, and
// it is load-bearing rather than a courtesy.
//
// Direction is not consulted anywhere in this module. The operator ruled that
// a card may be dragged to any column, backwards or forwards, so a drop names
// a destination and the verb decides whether the move is allowed. There is no
// comparison of where the card stood against where it landed, because there is
// nothing such a comparison could decide.

import type { CommandContext, CommandHost } from "./cardCommands";
import { isRow, runVerb } from "./cardCommands";
import type { CliOutcome, Spawner } from "./cli";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { ColumnView } from "./wire";
import type { TreeElement } from "./tree";
import { columnRef } from "./tree";

/** The dragged card, carried across the drag as the mime entry's value. */
export interface DragPayload {
	/** The card's own reference, which the move verb takes. */
	readonly ref: string;
	/** The workbench the card stands in, which the call is pinned to. */
	readonly root: string;
	/** The workspace folder the card's row belongs to, which is checkpointed. */
	readonly folder: string;
	/** The identifier of the column the card stands in as the drag begins. */
	readonly columnId: string;
}

/** A drop target resolved to the column it stands under. */
export interface DropTarget {
	readonly view: ColumnView;
	/** The workbench that column belongs to, read off its own root row. */
	readonly root: string;
}

/** What a resolved drop turns out to mean. */
export type DropVerdict =
	| { readonly kind: "act"; readonly destinationRef: string }
	| { readonly kind: "ignore" }
	| { readonly kind: "crossWorkbench" };

/**
 * Whatever the controller can put a mime entry on, narrowed to the one call.
 *
 * The item is a type parameter because the value a real drag carries is a
 * vscode class and this module reaches for none. A test passes its own
 * recorder and gets the same checking a real DataTransfer gets.
 */
export interface DragSink<Item> {
	readonly set: (mime: string, item: Item) => void;
}

/**
 * The dragged card, or undefined when the rows dragged are not one card.
 *
 * Only the first element is read. The view does not set canSelectMany, so a
 * drag carries one row, and this card neither adds multi-select nor makes a
 * column row draggable for reordering.
 *
 * A card missing its reference, its workbench root or its resolved column
 * yields nothing, on the same conservative-miss terms contextFor already
 * applies to every other card command. The absent element is checked by isRow
 * before any field is read off it, which is the one place that check lives
 * (dinah-342).
 */
export function dragPayloadFor(
	source: readonly TreeElement[],
): DragPayload | undefined {
	const element = source[0];
	if (!isRow(element, "card")) {
		return undefined;
	}
	const ref = element.view?.ref ?? element.node.ref;
	const root = element.row.data?.path;
	const columnId = element.column?.id;
	if (ref === undefined || ref === "" || root === undefined) {
		return undefined;
	}
	if (columnId === undefined || columnId === "") {
		return undefined;
	}
	return { ref, root, folder: element.row.folder, columnId };
}

/**
 * Offers the dragged card to the drag, when the rows dragged are one card.
 *
 * The wrapper is injected because the item a drag carries is a vscode value
 * and this module reaches for none. A drag that starts on a column, a state
 * group, a workbench or an attachment row sets no entry at all, so a drop
 * afterward is indistinguishable from a drag this controller never handled.
 */
export function offerDrag<Item>(
	source: readonly TreeElement[],
	mime: string,
	sink: DragSink<Item>,
	wrap: (payload: DragPayload) => Item,
): void {
	const payload = dragPayloadFor(source);
	if (payload === undefined) {
		return;
	}
	sink.set(mime, wrap(payload));
}

/**
 * The column a drop landed on, however the reader reached it.
 *
 * A card row and a state group row both carry the same column field the column
 * row itself carries, and visually the reader let go somewhere inside that
 * column, so a drop on either is a drop on the column that owns it. Every
 * other row kind names no column to move into. A column row the status/tree
 * join missed carries no ColumnView, and a row whose workbench did not resolve
 * carries no root, so neither resolves to a target.
 */
export function dropColumnFor(
	target: TreeElement | undefined,
): DropTarget | undefined {
	if (target === undefined) {
		return undefined;
	}
	if (
		target.kind !== "column" &&
		target.kind !== "group" &&
		target.kind !== "card"
	) {
		return undefined;
	}
	const view = target.kind === "column" ? target.view : target.column;
	if (view === undefined) {
		return undefined;
	}
	const root = target.row.data?.path;
	if (root === undefined) {
		return undefined;
	}
	return { view, root };
}

/**
 * What to do about a card dropped on a resolved target.
 *
 * The workbench check runs before the column check, so a same-identifier
 * column in another workbench is reported rather than silently ignored. Two
 * workbenches can name a column the same thing, and treating that as a drop
 * onto the card's own column would do nothing and say nothing.
 *
 * A drop that names no column, and a drop back onto the column the card
 * already stands in, both do nothing and show nothing. The workbench orders
 * cards by when they were filed and no verb reorders them, so a drop inside
 * one column would show the reader an ordering the workbench does not record.
 */
export function classifyDrop(
	payload: DragPayload,
	drop: DropTarget | undefined,
): DropVerdict {
	if (drop === undefined) {
		return { kind: "ignore" };
	}
	if (drop.root !== payload.root) {
		return { kind: "crossWorkbench" };
	}
	if (drop.view.id === payload.columnId) {
		return { kind: "ignore" };
	}
	return { kind: "act", destinationRef: columnRef(drop.view) };
}

/**
 * Acts on a verdict, which for an ordinary drop means running `dinah move`.
 *
 * The move goes through runVerb, the one piece of plumbing every mutating card
 * command already shares, so a drag reports a refusal in the same sentence the
 * Move and Pull commands report it in and checkpoints the folder whatever the
 * answer was. A drag is never a pull: `dinah pull` takes a destination and
 * picks its own card, so a pull could move a card other than the one the
 * reader dragged.
 */
export async function applyDropVerdict(
	verdict: DropVerdict,
	payload: DragPayload,
	drop: DropTarget | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): Promise<CliOutcome | undefined> {
	if (verdict.kind === "ignore") {
		return undefined;
	}
	if (verdict.kind === "crossWorkbench") {
		host.showError(crossWorkbenchMessage(payload.ref, drop, host.t));
		return undefined;
	}
	const context: CommandContext = {
		spawner,
		exe,
		host,
		folder: payload.folder,
		root: payload.root,
		ref: payload.ref,
	};
	return runVerb(context, ["move", payload.ref, verdict.destinationRef]);
}

/**
 * The sentence a drop into another workbench shows.
 *
 * A column in a different workbench looks like a plausible destination, so the
 * reader is told rather than left wondering why nothing happened. Both halves
 * are named, because a reader who dragged the wrong row and a reader who aimed
 * at the wrong column need different answers.
 *
 * The target is taken rather than a name composed by the caller, so the one
 * place that decides what a column is called in a message is this module. A
 * target that did not resolve cannot reach this sentence through classifyDrop,
 * which answers crossWorkbench only for a target it did resolve, so the
 * fallback wording covers a caller that composed the verdict itself.
 */
export function crossWorkbenchMessage(
	ref: string,
	drop: DropTarget | undefined,
	t: Localizer = ENGLISH,
): string {
	const destination =
		drop === undefined ? t("dialog.drop.thatColumn") : destinationName(drop.view);
	return t("dialog.drop.crossWorkbench", { ref, destination });
}

/**
 * What a column is called in a message, falling back to the reference a person
 * types when the title is empty, exactly as columnTooltip already falls back.
 */
function destinationName(view: ColumnView): string {
	return view.title !== "" ? view.title : columnRef(view);
}
