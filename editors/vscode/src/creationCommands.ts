// The two creation commands the tree contributes: filing a card into a
// column, and attaching a file to a workbench, a column or a card.
//
// Both mutate the board, so both take cardCommands.ts's own CommandHost, which
// carries checkpoint, rather than workbenchCommands.ts's WorkbenchCommandHost,
// which does not because neither Check nor Copy Path mutates anything
// (dinah-330 D-8). That distinction is what decides which host a command
// needs. Nothing here imports vscode.
//
// Every mutating call is followed by one off-cycle checkpoint, whatever the
// outcome, and the receipt is not read back into the tree. Neither command
// reveals, selects or opens the row it created: the next checkpoint repaints
// it from a fresh read (dinah-331 Decision 5).

import type { CommandHost } from "./cardCommands";
import { isRow, pinnedArgv, refusalMessage } from "./cardCommands";
import type { CliOutcome, Spawner } from "./cli";
import { runDinah } from "./cli";
import type { TreeElement } from "./tree";
import { columnRef } from "./tree";

/** What New Card needs: where the workbench stands, and which column it fills. */
export interface ColumnCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the column's row belongs to. */
	readonly folder: string;
	/** The workbench the column stands in, which the call is pinned to. */
	readonly root: string;
	/** The column identifier `dinah add --column` takes. */
	readonly column: string;
	/** The column's own title, reused in the title prompt rather than composed twice. */
	readonly label: string;
}

/**
 * The context for New Card, or undefined when the row named is not a column
 * this act can be aimed at.
 *
 * A column the status/tree join missed carries no ColumnView and yields no
 * context here, on the same terms columnActionsFor leaves such a row's
 * contextValue at the bare, unsuffixed CONTEXT_COLUMN. Neither function knows
 * the column's capacity, so neither offers the act, and the row self-heals on
 * the next checkpoint.
 *
 * The absent element is checked by isRow before any field is read off it,
 * which is the one place that check lives (dinah-342).
 */
export function contextForColumn(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): ColumnCommandContext | undefined {
	if (!isRow(element, "column") || element.view === undefined) {
		return undefined;
	}
	const root = element.row.data?.path;
	if (root === undefined) {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		folder: element.row.folder,
		root,
		column: columnRef(element.view),
		label: element.view.title,
	};
}

/**
 * Files a new card into the column, after asking for a title.
 *
 * Only a title is asked for (Decision 1). `dinah add` requires nothing else,
 * and no read publishes a workbench's declared severity or priority names for
 * a form to offer safely, so a free-text prompt for either would be guessing
 * against a vocabulary nothing has published.
 */
export async function newCard(
	context: ColumnCommandContext,
): Promise<CliOutcome | undefined> {
	const title = await context.host.input(
		`Title for the new card in ${context.label}`,
	);
	if (title === undefined || title.trim() === "") {
		return undefined;
	}
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, [
			"add",
			title.trim(),
			"--column",
			context.column,
		]),
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
	}
	await context.host.checkpoint(context.folder);
	return outcome;
}

/** What Attach File needs: where the workbench stands, and which entity receives the file. */
export interface AttachCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the row belongs to. */
	readonly folder: string;
	/** The workbench the entity stands in, which the call is pinned to. */
	readonly root: string;
	/** What `attach` takes as its first argument, which is "" for the workbench itself. */
	readonly ref: string;
}

/**
 * The context for Attach File, or undefined when the row named cannot receive
 * one.
 *
 * All three levels this tree can name an attachable entity from resolve here:
 * the workbench root row, whose ref is "" on the same empty-ref-means-workbench
 * convention the attachments group already uses, a column row, and a card row.
 * A comment carries no row in this tree (dinah-335 Decision 1) and stays
 * unreachable here for the same reason.
 */
export function contextForAttach(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): AttachCommandContext | undefined {
	if (element === undefined) {
		return undefined;
	}
	if (element.kind === "root") {
		if (element.row.rowKind === "deadEnd") {
			return undefined;
		}
		const root = element.row.data?.path ?? element.row.candidate?.path;
		if (root === undefined || root === "") {
			return undefined;
		}
		return {
			spawner,
			exe,
			host,
			folder: element.row.folder,
			root,
			ref: "",
		};
	}
	if (element.kind === "column") {
		if (element.view === undefined) {
			return undefined;
		}
		const root = element.row.data?.path;
		if (root === undefined) {
			return undefined;
		}
		return {
			spawner,
			exe,
			host,
			folder: element.row.folder,
			root,
			ref: columnRef(element.view),
		};
	}
	if (element.kind === "card") {
		// The listing's ref first and the tree node's behind it. The two
		// answers are joined per checkpoint, so either can be missing, and
		// where both are present the listing is the fresher read.
		const ref = element.view?.ref ?? element.node.ref;
		const root = element.row.data?.path;
		if (ref === undefined || ref === "" || root === undefined) {
			return undefined;
		}
		return {
			spawner,
			exe,
			host,
			folder: element.row.folder,
			root,
			ref,
		};
	}
	return undefined;
}

/**
 * The dialog Attach File opens, spelled here rather than inside extension.ts.
 *
 * extension.ts is the one module that imports vscode, and nothing in this
 * layer can stub that import, so anything spelled inside it is reachable by a
 * test only as source text. These options and the selection read below are the
 * two parts of the picker that carry a decision, so they live here, where a
 * test drives them directly, and extension.ts is left holding the call to
 * vscode.window.showOpenDialog and nothing else (dinah-331 AC-12).
 *
 * One file rather than many, because `dinah attach` takes one payload path. A
 * file rather than a folder, because the verb refuses a directory. The label
 * names the act rather than the dialog's default "Open", which would say the
 * wrong verb for what the button does.
 */
export const ATTACH_DIALOG_OPTIONS = {
	canSelectMany: false,
	canSelectFiles: true,
	canSelectFolders: false,
	openLabel: "Attach",
} as const;

/** One entry of a file dialog's answer, as far as the read below needs it. */
export interface PickedFile {
	/** The selection as a filesystem path, which is what `attach` takes. */
	readonly fsPath: string;
}

/**
 * The path the operator chose, or undefined when the operator chose nothing.
 *
 * Three answers reach here and each means something different. Undefined is a
 * cancelled dialog, which VS Code documents. An empty array is not documented
 * either way, so it is read as a cancellation rather than trusted not to
 * arrive. One or more entries is a selection, and the first is the only one
 * this command can use, since `attach` takes a single payload.
 *
 * fsPath rather than path: the latter answers a URI path, which carries a
 * leading slash before a Windows drive letter, and `attach` would refuse that
 * on Windows and nowhere else.
 */
export function pickedFilePath(
	picked: readonly PickedFile[] | undefined,
): string | undefined {
	if (picked === undefined || picked.length === 0) {
		return undefined;
	}
	return picked[0].fsPath;
}

/**
 * Attaches a file to the entity, after asking which file and, once one is
 * picked, an optional description.
 *
 * `pickFile` is a parameter rather than a CommandHost field read implicitly,
 * so a test drives it exactly like every other prompt this module makes. File
 * first and description second (Decision 4), so a cancelled file pick never
 * leaves a typed description stranded. An empty description submission is a
 * real answer meaning no description and the call still proceeds; only a
 * cancellation, which arrives as undefined, aborts the act at either step.
 *
 * The ref goes into the argv as its own element even when it is the empty
 * string, because `runAttach` reads its two words positionally with no
 * shift-on-omission, so an explicit empty first word is how the workbench
 * itself is named. The spawner takes an argv array with no shell in between,
 * so that element reaches the process as a genuine empty argument.
 */
export async function attachFile(
	context: AttachCommandContext,
	pickFile: () => Promise<string | undefined>,
): Promise<CliOutcome | undefined> {
	const file = await pickFile();
	if (file === undefined) {
		return undefined;
	}
	const description = await context.host.input(
		"Description for this attachment, or leave blank",
	);
	if (description === undefined) {
		return undefined;
	}
	const trimmed = description.trim();
	const args =
		trimmed === ""
			? ["attach", context.ref, file]
			: ["attach", context.ref, file, `--description=${trimmed}`];
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, args),
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
	}
	await context.host.checkpoint(context.folder);
	return outcome;
}
