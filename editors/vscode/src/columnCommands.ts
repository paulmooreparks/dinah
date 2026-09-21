// The two acts a column row offers, as pure functions over an injected host.
//
// This module mirrors workbenchCommands.ts rather than inventing a second
// shape. A column command is pinned to a column standing in a workbench, which
// is the workbench root plus the column's own reference, while a workbench
// command is pinned to the workbench alone, so the two take different contexts
// and folding one into the other would widen a context neither handler can use
// without narrowing it again. That is the same separation dinah-330 drew
// between cardCommands.ts and workbenchCommands.ts.
//
// This was the column row's first context-menu entry of any kind. dinah-330
// left the row with none because its one plausible act then was the queue
// pull, which is state-dependent and waited on dinah-280. Editing a column's
// instructions carried no comparable gate: every column, in every state, has
// an instructions file that can be opened. The queue pull arrived with
// dinah-375, on the destination dinah-280 published, and lives in
// pullCommands.ts because it mutates the board and needs a host that
// checkpoints, which this module's own host deliberately does not.
//
// The command checkpoints nothing. A checkpoint exists to repaint the tree
// after the board moved, and opening a file for editing moves nothing; the
// save that follows is the operator's own and the extension's `**/*.md`
// watcher already fires on it.

import type { BulkReport, RowOutcome } from "./bulk";
import { runBulk } from "./bulk";
import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import { refusalMessage, isRow, rowRef } from "./cardCommands";
import type { Wiring } from "./commandTable";
import { composeComment } from "./commentBody";
import { COMMAND_EDIT_COLUMN_INSTRUCTIONS } from "./identity";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { ReporterHost } from "./reporter";
import type { TreeElement } from "./tree";
import { treeItemFor } from "./tree";
import type { PathAnswer } from "./wire";

/**
 * The action the toast offers when there is something in the channel to read.
 *
 * workbenchCommands.ts declares its own, reading the same catalogue key. The
 * duplication predates this card and is left where it stands rather than
 * merged into one shared declaration, which is its own change.
 */
export function openOutputLabel(t: Localizer = ENGLISH): string {
	return t("dialog.openOutput.label");
}

/**
 * The window calls the column row's command makes, injected so tests watch them.
 *
 * It extends ReporterHost rather than declaring its own reporting members, so
 * that a run over several column rows can collect what each row would have
 * shown (dinah-490 D-25). showError and showInfo arrive with that, and they
 * are two of the four message bindings this card adds in extension.ts.
 */
export interface ColumnCommandHost extends ReporterHost {
	/** Opens a file as an ordinary, writable text document. */
	readonly openDocument: (path: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/**
 * What a column row's commands need: how to spawn, which column, and which
 * workspace folder the row belongs to.
 */
export interface ColumnCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: ColumnCommandHost;
	/** The value `--workbench` takes. */
	readonly root: string;
	/**
	 * The column's own id, slug or title, whatever the row carries, because
	 * ColumnByRef resolves any of the three.
	 */
	readonly columnRef: string;
	/** The row's own drawn label, reused rather than composed a second time. */
	readonly label: string;
	/**
	 * The workspace folder the column's row belongs to. composeComment hands it
	 * to the checkpoint after the comment is minted, so the refresh reaches the
	 * folder the row was drawn from; the verb itself runs in the workbench
	 * root. It is the field ItemCommandContext already carries for the same
	 * purpose.
	 */
	readonly folder: string;
}

/**
 * The context for the column row's command, or undefined when the row named is
 * not a column anything can be run against.
 *
 * The absent element is checked by isRow before any field is read, which is
 * the guard dinah-342 extracted and dinah-335 found a second copy of. A
 * palette invocation, a keybinding and a call from another extension all
 * arrive with no argument at all, and reading a field off that argument throws
 * before this function's own wrong-row branch could run.
 *
 * Two fields can name this column and the order between them is load-bearing.
 * `view.id` is read first because it is the column's raw identifier, which is
 * what resolves precisely. `node.value` carries the column's own Ref(), which
 * tree.go fills with the slug when the column has one and with the identifier
 * only otherwise, so the two strings differ on every slugged column and
 * preferring node.value would answer the ordinary path with the slug.
 *
 * The fallback is not a defensive nicety either. columnsOf documents `view` as
 * undefined precisely when a column was deleted between the status and tree
 * calls of one checkpoint, and in that race node.value is still the field that
 * resolves this column's own file, because tree.go fills a column node's Value
 * independently of whether the status join later finds a matching view. A
 * fallback reading node.id would do nothing at the one moment it exists to
 * help: tree.ts's own columnRef comment states TreeNode.ID is absent on every
 * node but a card leaf, so a column node's id is undefined and this function
 * would decline instead of resolving.
 */
export function contextForColumn(
	element: TreeElement | undefined,
	exe: string,
	host: ColumnCommandHost,
	spawner: Spawner,
): ColumnCommandContext | undefined {
	if (!isRow(element, "column")) {
		return undefined;
	}
	const columnRef = element.view?.id ?? element.node.value;
	// The resolved path first, then the candidate's own, which is the order
	// contextForWorkbench reads them in and the order the row itself draws
	// them: an expanded candidate carries both, and the resolved one is what
	// the walk actually read.
	const root = element.row.data?.path ?? element.row.candidate?.path;
	if (columnRef === undefined || columnRef === "") {
		return undefined;
	}
	if (root === undefined || root === "") {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		root,
		columnRef,
		label: treeItemFor(element, host.t).label,
		folder: element.row.folder,
	};
}

/**
 * Opens this column's own instructions file for editing.
 *
 * The raw columns/<id>/column.md, which is the posture editWorkbenchDefinition
 * takes toward workbench.md and for the same reason (dinah-332 D-1): no
 * witness convention covers either file, so nothing narrower is built. A
 * column's instructions text and a card's checklist are two separate stores,
 * and editing one never touches the other.
 *
 * Unlike editWorkbenchDefinition, this command is not a recovery path for a
 * broken column.md (dinah-332 D-4). Resolving a column reference still needs a
 * fully opened bench, and one unparsable column file refuses the whole open,
 * so the very corruption this command would otherwise fix is what stops it
 * answering.
 */
export async function editColumnInstructions(
	context: ColumnCommandContext,
): Promise<RowOutcome> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		["--workbench", context.root, "path", context.columnRef],
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.appendLines([
			context.host.t("dialog.column.instructionsUnreadable.channel", {
				column: context.label,
				detail: refusalMessage(outcome),
			}),
		]);
		const picked = await context.host.showWarning(
			context.host.t("dialog.column.instructionsUnreadable.toast", {
				column: context.label,
			}),
			[openOutputLabel(context.host.t)],
		);
		if (picked !== undefined) {
			context.host.revealOutput();
		}
		return { kind: "failed", failure: refusalMessage(outcome) };
	}
	const path = (outcome.json as PathAnswer).path;
	if (path === undefined || path === "") {
		context.host.log(`${COMMAND_EDIT_COLUMN_INSTRUCTIONS} answered with no path`);
		return { kind: "failed", failure: "path answered with no path" };
	}
	await context.host.openDocument(path);
	return { kind: "done" };
}

// ---------------------------------------------------------------------------
// What the registration loop calls
// ---------------------------------------------------------------------------

/** The channel line a row that names no column gets. */
const NO_COLUMN = "names no column";

/** Opens each selected column's own instructions file. */
export async function invokeEditColumnInstructions(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForColumn(element, wiring.exe, wiring.columnHost, wiring.spawner),
		{ host: wiring.columnHost, t: wiring.t, skipReason: NO_COLUMN },
		async () => true,
		async (context, _answer, host) =>
			editColumnInstructions({ ...context, host }),
	);
}

/**
 * Mints an empty comment on the one selected column and opens its file.
 *
 * This is invokeCommentOnItem with a column row in place of an item row.
 * Nothing is asked and nothing is confirmed. The comment exists from the
 * moment the command runs, so the author writes into the entity itself and a
 * save of that tab writes its body through the verb. An author who decides
 * to say nothing after all deletes it with Delete Comment on the comment's
 * own row.
 *
 * It resolves through contextForColumn rather than through the creation
 * commands' own resolver, because commenting reads no ColumnView field: the
 * reference is enough, and contextForColumn answers one on a row the status
 * join has not reached yet. That is the same reasoning that puts Comment on
 * Column under the when-clause Edit Column Instructions carries rather than
 * the narrower one Attach File carries.
 */
export async function invokeCommentOnColumn(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForColumn(element, wiring.exe, wiring.columnHost, wiring.spawner),
		{
			host: wiring.columnHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notAColumnRow"),
		},
		async (resolved, host) => {
			if (resolved.length > 1) {
				host.showError(host.t("dialog.bulk.oneRowOnly"));
				return undefined;
			}
			return resolved.length === 1 ? (true as const) : undefined;
		},
		async (context) => {
			await composeComment(
				wiring.commentHost,
				wiring.spawner,
				wiring.exe,
				wiring.openComments,
				{
					root: context.root,
					folder: context.folder,
					ref: context.columnRef,
				},
			);
			return { kind: "done" } as const;
		},
	);
}
