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

import type { BulkReport, RowOutcome } from "./bulk";
import { runBulk } from "./bulk";
import type { CliOutcome, Spawner } from "./cli";
import { runCheck, runDinah } from "./cli";
import { refusalMessage, isRow, rowOutcomeFor, rowRef } from "./cardCommands";
import type { Wiring } from "./commandTable";
import { COMMAND_EDIT_WORKBENCH_DEFINITION } from "./identity";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { ReporterHost } from "./reporter";
import type { TreeElement } from "./tree";
import { treeItemFor } from "./tree";
import type { CheckAnswer, CheckFinding, PathAnswer } from "./wire";
import { READ_FINDINGS } from "./wire";

/**
 * The window calls a workbench-row command makes, injected so tests watch them.
 *
 * It extends ReporterHost rather than declaring its own reporting members, so
 * that a run over several workbench rows can collect what each row would have
 * shown (dinah-490 D-25). showError arrives with that, and it is one of the
 * four message bindings this card adds in extension.ts.
 */
export interface WorkbenchCommandHost extends ReporterHost {
	readonly copyToClipboard: (text: string) => Promise<void>;
	/** Opens a file as an ordinary, writable text document. */
	readonly openDocument: (path: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/** What every workbench-row command needs: how to spawn, and which workbench. */
export interface WorkbenchCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	/**
	 * How the resolved binary described itself when the extension probed it at
	 * activation, as version.ts's describeVersion composes that line, and empty
	 * when no binary was resolved.
	 *
	 * Carried so that a message about an answer can name the binary that gave
	 * it. This is display and nothing else: no branch here reads it, splits it,
	 * or measures it against anything, which is the rule version.ts's own
	 * header states for the release tag inside it.
	 */
	readonly toolDescription: string;
	readonly host: WorkbenchCommandHost;
	/** The value `--workbench` takes, never the row's on-screen description. */
	readonly path: string;
	/** The row's own drawn label, reused rather than composed a second time. */
	readonly label: string;
}

/**
 * The action a toast offers when there is something in the channel to read.
 *
 * columnCommands.ts declares its own, reading the same catalogue key. The
 * duplication predates this card and is left where it stands rather than
 * merged into one shared declaration, which is its own change.
 */
export function openOutputLabel(t: Localizer = ENGLISH): string {
	return t("dialog.openOutput.label");
}

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
	toolDescription: string,
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
	return {
		spawner,
		exe,
		toolDescription,
		host,
		path,
		label: treeItemFor(element, host.t).label,
	};
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
	const picked = await context.host.showWarning(message, [
		openOutputLabel(context.host.t),
	]);
	if (picked !== undefined) {
		context.host.revealOutput();
	}
}

/**
 * Names the binary that answered, so a reader who has been told the answer
 * could not be read learns which executable to look at.
 *
 * A message about a disagreement over the shape of an answer leaves a reader
 * knowing that this build and some dinah disagree, and knowing nothing about
 * which dinah. That was the whole of what the operator could see when a binary
 * predating dinah-346 answered check on a workbench carrying defects. The path
 * and the self-reported version are both already in hand here, and printing
 * them decides nothing: no version is parsed, compared, or read as a statement
 * about what the tool can do.
 */
function answeringBinary(context: WorkbenchCommandContext): string {
	if (context.exe === "") {
		return context.host.t("dialog.workbench.noBinary");
	}
	return context.toolDescription === ""
		? context.host.t("dialog.workbench.answeringBinary", { exe: context.exe })
		: context.host.t("dialog.workbench.answeringBinaryDescribed", {
				exe: context.exe,
				description: context.toolDescription,
			});
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
		// A check with no report is never silent. The reason goes to the
		// channel and the toast points at it, on the same terms a dirty run
		// does, so a reader learns the difference between a clean workbench and
		// a question that was never answered.
		//
		// The leading clause says what is true on every arm this branch takes.
		// A refusal, a spawn that failed and a stale cursor all left this
		// extension with no report; so did the answer a binary predating
		// dinah-346 gives for a workbench carrying defects, which is a report
		// this build cannot read rather than an absent one. The clause it
		// replaced said the check could not run, and on that last arm the check
		// ran and found exactly what it was asked to find.
		//
		// The channel line and the toast were one shared English fragment
		// before dinah-379, so that the two could not drift apart. They are two
		// catalogue entries now, because a fragment spliced into two sentences
		// fixes an English word order every other language has to fight, and
		// the two entries sit adjacent in every catalogue where a translator
		// reads them together.
		context.host.appendLines([
			context.host.t("dialog.workbench.noReport.channel", {
				workbench: context.label,
				detail: refusalMessage(outcome),
			}),
			answeringBinary(context),
		]);
		await offerOutput(
			context,
			context.host.t("dialog.workbench.noReport.toast", {
				workbench: context.label,
			}),
		);
		return outcome;
	}

	const answer = outcome.json as CheckAnswer;
	if (answer.outcome !== READ_FINDINGS) {
		context.host.showInfo(
			context.host.t("dialog.workbench.checkClean", { workbench: context.label }),
		);
		return outcome;
	}

	const findings = answer.findings ?? [];
	const count = String(findings.length);
	context.host.appendLines([
		context.host.t("dialog.workbench.checkFindings.channel", {
			workbench: context.label,
			count,
		}),
		...findings.map(findingLine),
	]);
	await offerOutput(
		context,
		context.host.t("dialog.workbench.checkFindings.toast", {
			workbench: context.label,
			count,
		}),
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
export async function copyWorkbenchPaths(
	finished: readonly WorkbenchCommandContext[],
	host: WorkbenchCommandHost,
): Promise<void> {
	await host.copyToClipboard(finished.map((context) => context.path).join("\n"));
}

/**
 * The sentence a successful Copy Path shows, whatever the row count.
 *
 * The singular key when exactly one row finished, which is the sentence that
 * ships today, and the plural when more did. Filling the singular key's
 * {path} with a newline-joined list is the defect this pair replaces.
 */
export function copiedPathMessage(
	finished: readonly WorkbenchCommandContext[],
	t: Localizer,
): string {
	return finished.length === 1
		? t("dialog.workbench.copiedPath", { path: finished[0].path })
		: t("dialog.workbench.copiedPath.many", { count: String(finished.length) });
}

/**
 * Opens the workbench's own definition file for editing.
 *
 * No narrower surface is built (dinah-332 D-1). workbench.md carries no
 * witness convention to protect, because witnessing in this codebase is a
 * per-card position reconciler and the format's own documentation calls a
 * hand edit of a definition file legal and deliberately unjournaled. So this
 * opens the raw file exactly as openCard opens a card's own file, and a
 * narrower editor would have to re-derive the ordering and slug rules the
 * columns list already single-authorities.
 *
 * The generic runDinah rather than a wrapper of its own: path refuses through
 * the ordinary refusal envelope and carries none of check's exit-code
 * overload, so readRefusal classifies it with no special casing.
 *
 * This command also recovers a workbench whose own workbench.md, or any of
 * its columns' files, is malformed, and it needs no code change here to do so
 * (dinah-332's recovery-gap section). The ok and non-ok arms below are already
 * generic, and dinah-272's AC-8 and AC-9 are what make `path workbench` answer
 * ok in that case by resolving from the discovered root alone. Until those
 * land on the trunk, a broken definition file refuses here like any other
 * refusal and the reader is told so.
 */
export async function editWorkbenchDefinition(
	context: WorkbenchCommandContext,
): Promise<RowOutcome> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		["--workbench", context.path, "path", "workbench"],
		{ cwd: context.path },
	);
	if (outcome.kind !== "ok") {
		context.host.appendLines([
			context.host.t("dialog.workbench.definitionUnreadable.channel", {
				workbench: context.label,
				detail: refusalMessage(outcome),
			}),
		]);
		await offerOutput(
			context,
			context.host.t("dialog.workbench.definitionUnreadable.toast", {
				workbench: context.label,
			}),
		);
		return { kind: "failed", failure: refusalMessage(outcome) };
	}
	const path = (outcome.json as PathAnswer).path;
	if (path === undefined || path === "") {
		context.host.log(
			`${COMMAND_EDIT_WORKBENCH_DEFINITION} answered with no path`,
		);
		return { kind: "failed", failure: "path answered with no path" };
	}
	await context.host.openDocument(path);
	return { kind: "done" };
}

// ---------------------------------------------------------------------------
// What the registration loop calls: one invoke per workbench-row command
// ---------------------------------------------------------------------------

/** The channel line a row that names no workbench gets. */
const NO_WORKBENCH = "names no workbench";

/** One run of a workbench-row command over the rows it was aimed at. */
async function workbenchRun<A>(
	elements: readonly TreeElement[],
	wiring: Wiring,
	ask: (
		resolved: readonly WorkbenchCommandContext[],
		host: WorkbenchCommandHost,
	) => Promise<A | undefined>,
	act: (
		context: WorkbenchCommandContext,
		answer: A,
		host: WorkbenchCommandHost,
	) => Promise<RowOutcome>,
	extra: {
		readonly finish?: (
			finished: readonly WorkbenchCommandContext[],
			host: WorkbenchCommandHost,
		) => Promise<void>;
		readonly successMessage?: (
			finished: readonly WorkbenchCommandContext[],
			t: Localizer,
		) => string;
	} = {},
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForWorkbench(
				element,
				wiring.exe,
				wiring.binaryLabel,
				wiring.workbenchHost,
				wiring.spawner,
			),
		{
			host: wiring.workbenchHost,
			t: wiring.t,
			skipReason: NO_WORKBENCH,
			...extra,
		},
		ask,
		act,
	);
}

/** A fanOut workbench command asks nothing, so its ask answers a unit value. */
async function noQuestion(): Promise<true> {
	return true;
}

/**
 * Checks each selected workbench, and updates the Problems panel from the
 * answer each check already fetched.
 *
 * The manual check pays for one invocation and both surfaces read it, which is
 * the arrangement that shipped before multi-select and is unchanged here.
 */
export async function invokeCheckWorkbench(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return workbenchRun(
		elements,
		wiring,
		noQuestion,
		async (context, _answer, host) => {
			const outcome = await checkWorkbench({ ...context, host });
			await wiring.applyCheckResult(context.path, context.label, outcome);
			return rowOutcomeFor(outcome);
		},
	);
}

/** Puts every selected workbench's path on the clipboard, in one write. */
export async function invokeCopyWorkbenchPath(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return workbenchRun(
		elements,
		wiring,
		noQuestion,
		async () => ({ kind: "done" }),
		{ finish: copyWorkbenchPaths, successMessage: copiedPathMessage },
	);
}

/** Opens each selected workbench's own definition file. */
export async function invokeEditWorkbenchDefinition(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return workbenchRun(
		elements,
		wiring,
		noQuestion,
		async (context, _answer, host) =>
			editWorkbenchDefinition({ ...context, host }),
	);
}
