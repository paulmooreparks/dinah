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
//
// The view selects many (dinah-490), so a drag carries every selected row once
// any of them is a card, and a drop moves each dragged card independently
// against the one target while recording the rows it could not move. The two
// halves of a drag are two separate calls with no shared state: handleDrop is
// handed a target and a data transfer and no selection, so the mime entry is
// the only channel there is and it carries the whole gesture.

import type { BulkReport, RowOutcome } from "./bulk";
import { runBulk } from "./bulk";
import type { CommandContext, CommandHost } from "./cardCommands";
import { isRow, refusalMessage, runVerb } from "./cardCommands";
import type { Spawner } from "./cli";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { ColumnView } from "./wire";
import type { TreeElement } from "./tree";
import { columnRef, treeItemFor } from "./tree";

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

/** One dragged row: a card that composes a payload, or a row that does not. */
export type DragRow =
	| {
			readonly kind: "card";
			readonly ref: string;
			readonly payload: DragPayload;
	  }
	| { readonly kind: "other"; readonly ref: string };

/**
 * The payload one dragged card composes, or undefined when the row is not a
 * card the move verb could be aimed at.
 *
 * A card missing its reference, its workbench root or its resolved column
 * yields nothing, on the same conservative-miss terms contextFor already
 * applies to every other card command. The absent element is checked by isRow
 * before any field is read off it, which is the one place that check lives
 * (dinah-342).
 *
 * Internal to this module. dragRowsFor is what the controller reaches for,
 * because a drag now carries every dragged row rather than one payload.
 */
function dragPayloadFor(element: TreeElement | undefined): DragPayload | undefined {
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
 * Every dragged row, in the order dragged, one entry per row.
 *
 * One row in and one row out, so no caller ever supplies a count and the drop
 * path gets the same by-construction guarantee every other command has: the
 * number of rows the run reports is the number of rows the reader dragged.
 *
 * A row that composes no payload is carried as `other` rather than dropped,
 * and its `ref` is the label the tree drew for it, so the entry the report
 * records names the row instead of being a synthetic hole.
 */
export function dragRowsFor(
	source: readonly TreeElement[],
	t: Localizer = ENGLISH,
): readonly DragRow[] {
	return source.map((element) => {
		const payload = dragPayloadFor(element);
		return payload === undefined
			? { kind: "other" as const, ref: treeItemFor(element, t).label }
			: { kind: "card" as const, ref: payload.ref, payload };
	});
}

/**
 * The dragged rows read back off a mime entry, or undefined when the value is
 * not one offerDrag put there.
 *
 * DataTransferItem.value is declared `any`, and nothing documents that the
 * entry under this mime type is one offerDrag set: the recommended tree mime
 * type is the editor's own, and what the editor does or does not put under it
 * is not something this module may assume in either direction. A single record
 * survived a cast because every field read off it was checked before use. A
 * list does not, because the first thing the drop path does with it is
 * iterate. So this is the reader, and it applies the conservative-miss
 * discipline at the boundary rather than inside the loop.
 *
 * A list holding no card row is refused, which is the same rule as offerDrag
 * setting no entry for such a drag, written once on each side so the two
 * halves cannot drift apart.
 *
 * What this cannot answer, stated so nobody credits it with more: whether this
 * session set the entry. Nothing in the declared API carries that, so the
 * question this function answers is whether the value is a DragRow list this
 * controller could have set.
 */
export function dragRowsFrom(value: unknown): readonly DragRow[] | undefined {
	if (!Array.isArray(value) || value.length === 0) {
		return undefined;
	}
	const rows: DragRow[] = [];
	for (const entry of value) {
		if (typeof entry !== "object" || entry === null) {
			return undefined;
		}
		const row = entry as { kind?: unknown; ref?: unknown; payload?: unknown };
		if (typeof row.ref !== "string") {
			return undefined;
		}
		if (row.kind === "other") {
			rows.push({ kind: "other", ref: row.ref });
			continue;
		}
		if (row.kind !== "card") {
			return undefined;
		}
		const payload = row.payload as Partial<DragPayload> | undefined;
		if (typeof payload !== "object" || payload === null) {
			return undefined;
		}
		if (
			typeof payload.ref !== "string" ||
			typeof payload.root !== "string" ||
			typeof payload.folder !== "string" ||
			typeof payload.columnId !== "string"
		) {
			return undefined;
		}
		rows.push({
			kind: "card",
			ref: row.ref,
			payload: {
				ref: payload.ref,
				root: payload.root,
				folder: payload.folder,
				columnId: payload.columnId,
			},
		});
	}
	if (!rows.some((row) => row.kind === "card")) {
		return undefined;
	}
	return rows;
}

/**
 * Offers every dragged row to the drag, when at least one of them is a card.
 *
 * The wrapper is injected because the item a drag carries is a vscode value
 * and this module reaches for none. A drag holding no card row at all sets no
 * entry, so a drop afterward is indistinguishable from a drag this controller
 * never handled; a drag holding at least one card sets the entry and carries
 * every dragged row, the non-card ones included, so the drop can name the rows
 * it could not act on (D-22).
 *
 * DataTransferItem.value is documented as custom data whose "original object
 * can be retrieved so long as the extension that created the DataTransferItem
 * runs in the same extension host", and handleDrag's own text says a
 * DataTransferItem dropped on another tree item in the same tree is preserved.
 * Both handlers are this extension's, in one host, on one tree, so the object
 * handed to the wrapper is the object read back, and a list is as ordinary a
 * value there as a record.
 */
export function offerDrag<Item>(
	source: readonly TreeElement[],
	mime: string,
	sink: DragSink<Item>,
	wrap: (rows: readonly DragRow[]) => Item,
	t: Localizer = ENGLISH,
): void {
	const rows = dragRowsFor(source, t);
	if (!rows.some((row) => row.kind === "card")) {
		return;
	}
	sink.set(mime, wrap(rows));
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
 * Acts on each dragged row against one resolved drop target.
 *
 * The move goes through runVerb, the one piece of plumbing every mutating card
 * command already shares, so a drag reports a refusal in the same sentence the
 * Move and Pull commands report it in and checkpoints the folder whatever the
 * answer was. A drag is never a pull: `dinah pull` takes a destination and
 * picks its own card, so a pull could move a card other than the one the
 * reader dragged.
 *
 * The whole run goes through runBulk like any other command, so five dragged
 * cards produce one message rather than five. The drop path is the one place
 * that declares `emptyRun: "silent"`: a menu invocation is an explicit request
 * and a run that attempted nothing still answers, while a drag is a gesture
 * that can miss, and a gesture that missed has to go on looking like a gesture
 * that missed (D-19).
 */
export async function applyDropVerdicts(
	rows: readonly DragRow[],
	drop: DropTarget | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): Promise<BulkReport> {
	return runBulk(
		rows,
		(row) => row.ref,
		(row) => (row.kind === "card" ? row.payload : undefined),
		{
			host,
			t: host.t,
			skipReason: "names no card",
			emptyRun: "silent",
		},
		async () => true,
		async (payload, _answer, running): Promise<RowOutcome> => {
			const verdict = classifyDrop(payload, drop);
			if (verdict.kind === "ignore") {
				return { kind: "skipped", why: "dropped where nothing moves" };
			}
			if (verdict.kind === "crossWorkbench") {
				const message = crossWorkbenchMessage(payload.ref, drop, running.t);
				running.showError(message);
				return { kind: "failed", failure: message };
			}
			const context: CommandContext = {
				spawner,
				exe,
				host: running,
				folder: payload.folder,
				root: payload.root,
				ref: payload.ref,
			};
			const outcome = await runVerb(context, [
				"move",
				payload.ref,
				verdict.destinationRef,
			]);
			return outcome.kind === "ok"
				? { kind: "done" }
				: { kind: "failed", failure: refusalMessage(outcome) };
		},
	);
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
