// The one act a column row offers, as a pure function over an injected host.
//
// This module mirrors workbenchCommands.ts rather than inventing a second
// shape. A column command is pinned to a column standing in a workbench, which
// is the workbench root plus the column's own reference, while a workbench
// command is pinned to the workbench alone, so the two take different contexts
// and folding one into the other would widen a context neither handler can use
// without narrowing it again. That is the same separation dinah-330 drew
// between cardCommands.ts and workbenchCommands.ts.
//
// This is the column row's first context-menu entry of any kind. dinah-330
// left the row with none because its one plausible act then was the queue
// pull, which is state-dependent and waits on dinah-280. Editing a column's
// instructions carries no comparable gate: every column, in every state, has
// an instructions file that can be opened, so this command does not wait on
// dinah-280 for the reason Check and Copy Path did not.
//
// The command checkpoints nothing. A checkpoint exists to repaint the tree
// after the board moved, and opening a file for editing moves nothing; the
// save that follows is the operator's own and the extension's `**/*.md`
// watcher already fires on it.

import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import { refusalMessage, isRow } from "./cardCommands";
import { COMMAND_EDIT_COLUMN_INSTRUCTIONS } from "./identity";
import type { TreeElement } from "./tree";
import { treeItemFor } from "./tree";
import type { PathAnswer } from "./wire";

/** The action the toast offers when there is something in the channel to read. */
export const OPEN_OUTPUT = "Open Output";

/** The window calls the column row's command makes, injected so tests watch them. */
export interface ColumnCommandHost {
	readonly showWarning: (
		message: string,
		actions: readonly string[],
	) => Promise<string | undefined>;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly revealOutput: () => void;
	/** Opens a file as an ordinary, writable text document. */
	readonly openDocument: (path: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/** What the column row's command needs: how to spawn, and which column. */
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
		label: treeItemFor(element).label,
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
): Promise<void> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		["--workbench", context.root, "path", context.columnRef],
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.appendLines([
			`${context.label}: could not open this column's instructions file. ${refusalMessage(outcome)}`,
		]);
		const picked = await context.host.showWarning(
			`${context.label}: could not open this column's instructions file. See the Dinah output channel for details.`,
			[OPEN_OUTPUT],
		);
		if (picked !== undefined) {
			context.host.revealOutput();
		}
		return;
	}
	const path = (outcome.json as PathAnswer).path;
	if (path === undefined || path === "") {
		context.host.log(`${COMMAND_EDIT_COLUMN_INSTRUCTIONS} answered with no path`);
		return;
	}
	await context.host.openDocument(path);
}
