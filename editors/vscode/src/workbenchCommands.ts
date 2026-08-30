// The two acts a workbench row offers, as pure functions over an injected host.
//
// This module mirrors cardCommands.ts rather than inventing a second shape:
// nothing here imports vscode, each handler takes a context a guard composed,
// and extension.ts binds the host to the real window. The two families are
// separate because they take different contexts. A card command is pinned to a
// card standing in a workbench, while these are pinned to the workbench alone.
//
// Neither act checkpoints. A checkpoint exists to repaint the tree after the
// board moved, and neither of these moves it: check passes no migration flag,
// and copying a path makes no invocation at all (dinah-330 D-8).

import type { CliOutcome, Spawner } from "./cli";
import { runCheck } from "./cli";
import { refusalMessage, isRow } from "./cardCommands";
import type { TreeElement } from "./tree";
import { treeItemFor } from "./tree";
import type { CheckAnswer, CheckFinding } from "./wire";
import { READ_FINDINGS } from "./wire";

/** The window calls a workbench-row command makes, injected so tests watch them. */
export interface WorkbenchCommandHost {
	readonly showInfo: (message: string) => void;
	readonly showWarning: (
		message: string,
		actions: readonly string[],
	) => Promise<string | undefined>;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly revealOutput: () => void;
	readonly copyToClipboard: (text: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/** What every workbench-row command needs: how to spawn, and which workbench. */
export interface WorkbenchCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: WorkbenchCommandHost;
	/** The value `--workbench` takes, never the row's on-screen description. */
	readonly path: string;
	/** The row's own drawn label, reused rather than composed a second time. */
	readonly label: string;
}

/** The action a toast offers when there is something in the channel to read. */
export const OPEN_OUTPUT = "Open Output";

/**
 * The context for a workbench-row command, or undefined when the row named is
 * not a workbench anything can be run against.
 *
 * All three resolved-identity row kinds qualify. A candidate row carries a
 * real path before it is expanded, and a forest row is one member workbench
 * among several sharing a folder rather than a container over them, so each of
 * the three names one concrete workbench that `--workbench` can be aimed at
 * (dinah-330 D-4). A dead end names none, and neither does any row that is not
 * a root row at all.
 *
 * The absent element is checked by isRow before any field is read, which is
 * the guard dinah-335 extracted rather than a second copy of it. A palette
 * invocation, a keybinding and a call from another extension all arrive with
 * no argument, and reading a field off that argument throws before a handler's
 * own wrong-row branch could run.
 */
export function contextForWorkbench(
	element: TreeElement | undefined,
	exe: string,
	host: WorkbenchCommandHost,
	spawner: Spawner,
): WorkbenchCommandContext | undefined {
	if (!isRow(element, "root")) {
		return undefined;
	}
	if (element.row.rowKind === "deadEnd") {
		return undefined;
	}
	// The resolved path first, then the candidate's own, which is the order the
	// row itself draws them in: a candidate that has been expanded carries both
	// and the resolved one is what the walk actually read.
	const path = element.row.data?.path ?? element.row.candidate?.path;
	if (path === undefined || path === "") {
		return undefined;
	}
	return { spawner, exe, host, path, label: treeItemFor(element).label };
}

/** The line one finding renders as, its path first because that is what a reader looks for. */
export function findingLine(finding: CheckFinding): string {
	return finding.Detail === ""
		? `${finding.Path}: ${finding.Key}`
		: `${finding.Path}: ${finding.Key} (${finding.Detail})`;
}

/** Offers the channel to a reader who has something waiting in it. */
async function offerOutput(
	context: WorkbenchCommandContext,
	message: string,
): Promise<void> {
	const picked = await context.host.showWarning(message, [OPEN_OUTPUT]);
	if (picked !== undefined) {
		context.host.revealOutput();
	}
}

/**
 * Runs the workbench's own check and reports what it found.
 *
 * A clean run says so in a toast and writes nothing to the channel, because a
 * reader who asked a question deserves the answer even when the answer is that
 * there is nothing to see. A run that found defects writes one line per finding
 * and offers the channel, which is the shape the view's own welcome text
 * already teaches and which does not fall over on a workbench carrying fifty
 * of them (dinah-330 D-5).
 *
 * Whether the run was clean is read off the report's `outcome` member. Exit
 * codes are cli.ts's business, and the emptiness of the findings array is a
 * second signal that would drift from the first.
 */
export async function checkWorkbench(
	context: WorkbenchCommandContext,
): Promise<CliOutcome> {
	const outcome = await runCheck(context.spawner, context.exe, context.path);
	if (outcome.kind !== "ok") {
		// A check that could not run is never silent. The refusal goes to the
		// channel and the toast points at it, on the same terms a dirty run
		// does, so a reader learns the difference between a clean workbench and
		// a question that was never asked.
		context.host.appendLines([
			`${context.label}: check could not run. ${refusalMessage(outcome)}`,
		]);
		await offerOutput(
			context,
			`${context.label}: check could not run. See the Dinah output channel for details.`,
		);
		return outcome;
	}

	const answer = outcome.json as CheckAnswer;
	if (answer.outcome !== READ_FINDINGS) {
		context.host.showInfo(`${context.label}: check found no defects.`);
		return outcome;
	}

	const findings = answer.findings ?? [];
	const count = String(findings.length);
	context.host.appendLines([
		`${context.label}: check found ${count} defect(s):`,
		...findings.map(findingLine),
	]);
	await offerOutput(
		context,
		`${context.label}: check found ${count} defect(s). See the Dinah output channel for details.`,
	);
	return outcome;
}

/**
 * Copies the workbench's own path to the clipboard.
 *
 * The path rather than the row's description, which is the disambiguating text
 * the tree draws to tell two same-titled workbenches apart and which is empty
 * on the ordinary single-workbench row this command is most often used on
 * (dinah-330 D-3). The path is always present and is the value `--workbench`
 * takes from any working directory, which is what a reader is copying it for.
 */
export async function copyWorkbenchPath(
	context: WorkbenchCommandContext,
): Promise<void> {
	await context.host.copyToClipboard(context.path);
	context.host.showInfo(`Copied ${context.path}`);
}
