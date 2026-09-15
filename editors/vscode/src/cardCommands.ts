// The card and attachment commands the tree contributes, as pure argv
// composition and pure outcome handling.
//
// Nothing here imports vscode. Each handler is a function over an injected
// host, so the unit layer asserts on the argv a command composes and on the
// message a refusal produces without a VS Code window, and extension.ts binds
// the host to the real window.
//
// Every mutating call is followed by one off-cycle checkpoint, whatever the
// outcome. A command that mutates nothing runs neither, which is why the copy
// family below reaches for the host directly instead of runVerb. The receipt
// is not read back into the tree: the next checkpoint repaints it from a fresh
// read, which is one rule rather than two and cannot drift from what the board
// actually says.
//
// Each contributed command appears here twice over: as the function that acts
// on one resolved row, and as the invoke at the foot of this file that the
// registration loop calls with every row the reader aimed at. The split is
// what lets one confirmation, one prompt or one clipboard write stand over a
// whole selection while the act itself stays a function of one row.

import type { BulkDeps, BulkReport, RowOutcome } from "./bulk";
import { runBulk } from "./bulk";
import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import type { CliOutcome } from "./cli";
import type { Wiring } from "./commandTable";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { ReporterHost } from "./reporter";
import { KIND_HISTORY, KIND_INSTRUCTIONS } from "./servedText";
import { nodeSpawner } from "./spawn";
import type { TreeElement } from "./tree";
import { treeItemFor } from "./tree";
import type { DetailAnswer, LegalMove, ServedAnswer } from "./wire";
import { ATTACHMENTS_SEGMENT, BACKWARD, FORWARD } from "./wire";

/** One entry of a quick-pick, as the host renders it. */
export interface PickItem {
	readonly label: string;
	readonly detail?: string;
	/** The value the caller gets back, which is never shown. */
	readonly value: string;
	/**
	 * Marks a label-only row a reader cannot choose.
	 *
	 * The command palette's verb list ends in one when a tool was left out
	 * because this build cannot draw its arguments (dinah-420). The modules
	 * that compose pick items import no vscode symbol, so they cannot build a
	 * vscode.QuickPickItemKind value; extension.ts's own `pick` binding turns
	 * this marker into the editor's separator before the array reaches
	 * showQuickPick. An item without it is an ordinary, choosable row.
	 */
	readonly kind?: "separator";
}

/**
 * The window calls these commands make, injected so tests can watch them.
 *
 * It extends ReporterHost rather than declaring its own reporting members, so
 * that one wrapper can collect any host's reporting and a command that can
 * speak is a command whose messages a run can collect (dinah-490 D-25). The
 * localizer, showError, showInfo, showWarning, appendLines and revealOutput
 * all arrive from there.
 */
export interface CommandHost extends ReporterHost {
	/** Puts text on the system clipboard. */
	readonly copyToClipboard: (text: string) => Promise<void>;
	readonly pick: (
		items: readonly PickItem[],
		placeholder: string,
	) => Promise<PickItem | undefined>;
	readonly input: (prompt: string) => Promise<string | undefined>;
	readonly openDocument: (path: string) => Promise<void>;
	/** Opens a file for the editor to render, which decides how itself. */
	readonly openFile: (path: string) => Promise<void>;
	/** Opens a native file picker and answers the chosen file's absolute path. */
	readonly pickFile: () => Promise<string | undefined>;
	/**
	 * Opens served text of one kind as a read-only tab, under the given title.
	 *
	 * The kind and the title travel together and are resolved by the caller
	 * rather than inside the tab machinery, because the title is a catalogue
	 * key and only the caller knows which kind it is asking for. A later kind
	 * resolves its own title key and calls this same method.
	 */
	readonly openServedText: (
		kind: string,
		root: string,
		ref: string,
		title: string,
	) => Promise<void>;
	/**
	 * Asks the reader to confirm an act that cannot be undone, and answers
	 * whether they did.
	 *
	 * Two arguments rather than one because a modal dialog supplies its own
	 * Cancel and takes the affirmative button's label from the caller, and both
	 * strings are prose a reader meets, so both go through the localizer.
	 */
	readonly confirmDestructive: (
		message: string,
		confirmLabel: string,
	) => Promise<boolean>;
	/** Runs one off-cycle checkpoint for the folder the card stands in. */
	readonly checkpoint: (folder: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/**
 * The two calls runVerb itself makes, and nothing else.
 *
 * Only one of the two is a window call. showError is; checkpoint is not, and
 * extension.ts binds it to the off-cycle refresh rather than to anything on
 * vscode.window, which is why this interface is not named for the window.
 *
 * A caller that only ever spawns a verb should not have to hand over a
 * clipboard, a quick pick or a file dialog it does not own. The comment draft
 * host is the case that made this explicit: it carries neither, it posts
 * through runVerb, and asserting it into CommandHost bought a compiling call
 * at the price of the check that would catch the next member runVerb reads.
 */
export interface VerbHost {
	readonly showError: (message: string) => void;
	/** Runs one off-cycle checkpoint for the folder the card stands in. */
	readonly checkpoint: (folder: string) => Promise<void>;
}

/** What spawning a verb needs: how to spawn, and where the card stands. */
export interface VerbContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: VerbHost;
	/** The workspace folder the card's row belongs to. */
	readonly folder: string;
	/** The workbench the card stands in, which the call is pinned to. */
	readonly root: string;
	/** The card's own reference, which every verb below takes. */
	readonly ref: string;
}

/** What every command needs, which is a verb context with a full host. */
export interface CommandContext extends VerbContext {
	readonly host: CommandHost;
}

/**
 * The sentence a refusal shows.
 *
 * The refusal's own name leads, because it is the stable handle a reader can
 * search for and the detail is prose that may be reworded. This is the same
 * minimal composition status.ts already does for the status bar tooltip, and
 * deliberately not a catalog lookup: the extension reads the machine surface
 * and renders its own English, as every other string it ships does.
 */
export function refusalMessage(outcome: CliOutcome): string {
	if (outcome.kind === "refused") {
		return outcome.detail === undefined || outcome.detail === ""
			? outcome.refusal
			: `${outcome.refusal}: ${outcome.detail}`;
	}
	const detail = (outcome as { detail?: string }).detail;
	return detail === undefined || detail === ""
		? outcome.kind
		: `${outcome.kind}: ${detail}`;
}

/**
 * The row a command was aimed at, when it is a row of the kind the command
 * can act on, and undefined otherwise.
 *
 * This is the single place the absent element is checked, and it is checked
 * before any field is read. A command invoked from the Command Palette, from
 * a keybinding, or by another extension arrives with no argument at all, and
 * reading a field off that argument throws a TypeError before a handler's own
 * wrong-row branch can run. dinah-342 found that ordering under six card
 * commands at once and moved the check here, into the module the unit layer
 * can import; dinah-335 found the same ordering in the attachment handler,
 * which that card's branch deliberately did not touch. One guard rather than
 * one per handler is what stops the next command repeating it.
 */
export function isRow<K extends TreeElement["kind"]>(
	element: TreeElement | undefined,
	kind: K,
): element is Extract<TreeElement, { kind: K }> {
	return element !== undefined && element.kind === kind;
}

/**
 * The workbench a card's row stands in, and the folder its checkpoint is
 * keyed under.
 *
 * A card standing in a forest row belongs to that member workbench rather than
 * to the workspace folder the walk started from, so the verb is pinned to the
 * member's own path while the checkpoint still runs against the folder, whose
 * one merged cursor covers every member beneath it.
 *
 * The absent element is checked before anything is read off it, by isRow
 * above, which is the one place that check lives (dinah-342). This function
 * lives here rather than in extension.ts so the unit layer can reach it: it
 * touches no vscode value, and the guard went six commands deep unexercised
 * while it sat in the one module no test can import.
 *
 * The spawner is injected rather than reached for, which is what
 * contextForAttach and contextForColumn already do and for the same reason: a
 * command composing its own context from an element is otherwise a command no
 * unit test can watch spawn, since nothing between the element and the call
 * belongs to the caller. dinah-490 made that bite, because the registration
 * loop now composes every card context through here.
 */
export function contextFor(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner = nodeSpawner,
): CommandContext | undefined {
	if (!isRow(element, "card")) {
		return undefined;
	}
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

/** Composes a call pinned to one workbench, which is how every verb runs. */
export function pinnedArgv(root: string, args: readonly string[]): string[] {
	return ["--workbench", root, ...args];
}

/**
 * Runs one mutating verb, reports a refusal, and checkpoints either way.
 *
 * The checkpoint runs on a refusal too. A refusal often means the board moved
 * under the reader (somebody else claimed the card), so the read that follows
 * is exactly what shows them why.
 *
 * The third parameter is what a verb reading a bare dash takes on stdin, which
 * is how a comment composed in an editor reaches `dinah comment <item> -`.
 * SpawnOptions.stdin already existed for the MCP surface and nodeSpawner
 * already writes and closes the stream, so nothing in the spawn layer changed
 * and every existing caller keeps its current argument list.
 */
export async function runVerb(
	context: VerbContext,
	args: readonly string[],
	stdin?: string,
): Promise<CliOutcome> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, args),
		// The key is spread in rather than written as `stdin` so that a caller
		// passing nothing leaves it absent rather than present and undefined.
		// nodeSpawner writes and closes the stream on a set key, so an
		// always-present key would give every verb an empty stdin it never
		// asked for, and Object.hasOwn is what dinah-506/criteria/30 reads.
		{ cwd: context.root, ...(stdin === undefined ? {} : { stdin }) },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
	}
	await context.host.checkpoint(context.folder);
	return outcome;
}

/** Claims the card. */
export async function claimCard(context: CommandContext): Promise<CliOutcome> {
	return runVerb(context, ["claim", context.ref]);
}

/** Releases the card. */
export async function releaseCard(
	context: CommandContext,
): Promise<CliOutcome> {
	return runVerb(context, ["release", context.ref]);
}

/** Unblocks the card. */
export async function unblockCard(
	context: CommandContext,
): Promise<CliOutcome> {
	return runVerb(context, ["unblock", context.ref]);
}

/**
 * Puts a whole selection's references on the clipboard, in one write.
 *
 * The reference rather than the title, because the reference is what every
 * dinah verb takes as an argument and what the operator writes when he names
 * a card to somebody else. No dinah invocation and no checkpoint follows, for
 * the reason copyWorkbenchPath makes neither (dinah-330 D-8): a copy reads
 * only what the row already holds and changes nothing on the board, so a
 * checkpoint would repaint a tree that cannot have moved.
 *
 * One write rather than one per row, and one sentence rather than one per row.
 * The copy family is declared `oneCall` for that reason, and it reports
 * nothing inside the loop: reporting per row fills a one-card sentence with a
 * newline-joined list, which is the shape dinah-490's round-5 review found
 * (D-23).
 */
export async function copyCardRefs(
	finished: readonly CommandContext[],
	host: CommandHost,
): Promise<void> {
	await host.copyToClipboard(finished.map((context) => context.ref).join("\n"));
}

/**
 * The sentence a successful Copy Reference shows, whatever the row count.
 *
 * The singular key when exactly one row finished, which is the sentence that
 * ships today, and the plural when more did. Making the copy always plural
 * would regress the one-row gesture to "Copied 1 card references.", which is
 * why both counts are pinned rather than only the plural one.
 */
export function copiedRefMessage(
	finished: readonly CommandContext[],
	t: Localizer,
): string {
	return finished.length === 1
		? t("dialog.card.copiedRef", { ref: finished[0].ref })
		: t("dialog.card.copiedRef.many", { count: String(finished.length) });
}

/**
 * Asks once for the reason the block verb requires, over the whole selection.
 *
 * An empty reason cancels rather than sending one, because `dinah block` takes
 * the reason as an argument and a blank one would record a block nobody can
 * act on.
 *
 * The prompt names how many cards resolved rather than asking about "this
 * card", because one sentence is raised over the whole selection and the
 * shipped English asks about one card. The singular key is unchanged and is
 * still what a one-card gesture shows.
 */
export async function askBlockReason(
	resolved: readonly CommandContext[],
	host: CommandHost,
): Promise<string | undefined> {
	const reason = await host.input(
		resolved.length === 1
			? host.t("dialog.block.reasonPrompt")
			: host.t("dialog.block.reasonPrompt.many", {
					count: String(resolved.length),
				}),
	);
	if (reason === undefined || reason.trim() === "") {
		return undefined;
	}
	return reason.trim();
}

/** Blocks one card with the reason the reader gave for the whole selection. */
export async function blockCardWith(
	context: CommandContext,
	reason: string,
): Promise<CliOutcome> {
	return runVerb(context, ["block", context.ref, reason]);
}

/**
 * Orders the destinations a move offers: forward first in the array's own
 * order, then backward.
 *
 * The array order within each direction is the workbench's own declared flow
 * order, so it is preserved rather than sorted. Forward leads because a card
 * moves forward far more often than it goes back, and a reader scanning the
 * list should meet the ordinary case first.
 */
export function orderLegalMoves(
	moves: readonly LegalMove[],
): readonly LegalMove[] {
	const forward = moves.filter((move) => move.direction === FORWARD);
	const backward = moves.filter((move) => move.direction === BACKWARD);
	const other = moves.filter(
		(move) => move.direction !== FORWARD && move.direction !== BACKWARD,
	);
	return [...forward, ...backward, ...other];
}

/** The quick-pick entry one legal move renders as. */
export function movePick(move: LegalMove, t: Localizer = ENGLISH): PickItem {
	return {
		label: move.title,
		detail: move.direction === BACKWARD ? t("dialog.move.backward") : undefined,
		// The destination the move verb takes is the entry's own Column, which
		// is the column's identifier. Not its Ref, which is what a person
		// types, and not its Title, which is what a person reads.
		value: move.column,
	};
}

/**
 * The destinations every card in the selection will accept, in the first
 * card's order.
 *
 * The intersection is keyed by `move.column`, which movePick already
 * establishes as the value the move verb takes. The titles and the direction
 * shown are the first card's, because a destination is one column whatever
 * card is looking at it and the first card's rendering is as true as any
 * other's.
 */
export function sharedLegalMoves(
	perCard: readonly (readonly LegalMove[])[],
): readonly LegalMove[] {
	const first = perCard[0];
	if (first === undefined) {
		return [];
	}
	const shared = new Set(first.map((move) => move.column));
	for (const moves of perCard.slice(1)) {
		const here = new Set(moves.map((move) => move.column));
		for (const column of [...shared]) {
			if (!here.has(column)) {
				shared.delete(column);
			}
		}
	}
	return first.filter((move) => shared.has(move.column));
}

/**
 * Reads every resolved card's legal moves and asks once for the destination
 * they share.
 *
 * Asking once rather than once per card is the decision (D-4). A prompt per
 * card over a dozen cards is the shape a reader abandons halfway, and
 * abandoning halfway is exactly the partial result this card exists to make
 * visible. The `instructions` calls are reads, so a reader who cancels the
 * single prompt has changed nothing on the board.
 *
 * Every branch keys on the number of cards that resolved rather than on the
 * number of rows targeted, which is what the signature enforces: this function
 * is handed the resolved list and can read no other count.
 */
export async function askMoveDestination(
	resolved: readonly CommandContext[],
	host: CommandHost,
): Promise<string | undefined> {
	const perCard: (readonly LegalMove[])[] = [];
	for (const context of resolved) {
		const served = await runDinah(
			context.spawner,
			context.exe,
			pinnedArgv(context.root, ["instructions", context.ref]),
			{ cwd: context.root },
		);
		if (served.kind !== "ok") {
			host.showError(refusalMessage(served));
			return undefined;
		}
		perCard.push((served.json as ServedAnswer).legal_moves ?? []);
	}
	// No card resolved at all, which is the mixed or all-column selection.
	// Nothing is spawned, nothing is asked and nothing is said here: the run
	// proceeds with every row skipped and the summary tells the reader that
	// the gesture came to nothing. Answering undefined instead would suppress
	// that summary and leave a menu invocation saying nothing at all.
	if (resolved.length === 0) {
		return "";
	}
	const shared = sharedLegalMoves(perCard);
	// Two branches rather than one sentence with a ternary in it. One card
	// with no legal moves is today's shipped message and names that card;
	// several cards sharing no destination is this card's new sentence and
	// names how many of them resolved.
	if (shared.length === 0 && resolved.length === 1) {
		host.showError(host.t("dialog.move.noLegalMoves", { ref: resolved[0].ref }));
		return undefined;
	}
	if (shared.length === 0) {
		host.showError(
			host.t("dialog.move.noSharedDestination", {
				count: String(resolved.length),
			}),
		);
		return undefined;
	}
	const picked = await host.pick(
		orderLegalMoves(shared).map((move) => movePick(move, host.t)),
		resolved.length === 1
			? host.t("dialog.move.placeholder", { ref: resolved[0].ref })
			: host.t("dialog.move.placeholder.many", {
					count: String(resolved.length),
				}),
	);
	if (picked === undefined) {
		return undefined;
	}
	return picked.value;
}

/** Moves one card to the destination the reader chose for the selection. */
export async function moveCardTo(
	context: CommandContext,
	column: string,
): Promise<CliOutcome> {
	return runVerb(context, ["move", context.ref, column]);
}

/**
 * Opens the card's served instruction chain as a read-only editor tab.
 *
 * Opening is an explicit act rather than something the claim drags along
 * behind it (dinah-270 D-1). A tab that reopened itself every time the board
 * moved would take the reader's focus away from whatever they were editing,
 * which is the intrusion a panel does not commit.
 */
export async function openInstructions(context: CommandContext): Promise<void> {
	await context.host.openServedText(
		KIND_INSTRUCTIONS,
		context.root,
		context.ref,
		context.host.t("servedText.title.instructions", { ref: context.ref }),
	);
}

/**
 * Opens the card's own journal as a read-only editor tab.
 *
 * This mirrors openInstructions field for field, and it does so because the
 * two acts differ only in which kind of text the tab serves. The reasoning
 * above about opening being an explicit act applies here unchanged, and a card
 * that opened its history on every claim would take the reader's focus for the
 * same reason (dinah-422 D-1).
 */
export async function openHistory(context: CommandContext): Promise<void> {
	await context.host.openServedText(
		KIND_HISTORY,
		context.root,
		context.ref,
		context.host.t("servedText.title.history", { ref: context.ref }),
	);
}

/**
 * Opens the card's own file.
 *
 * The path is read off `show`'s own Detail.Path rather than composed from the
 * reference. `dinah path` would be the obvious verb and has no machine form
 * (dinah-272), and a path this extension built itself would be a second
 * spelling of a layout the binary already owns.
 */
export async function openCard(context: CommandContext): Promise<RowOutcome> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["show", context.ref]),
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
		return { kind: "failed", failure: refusalMessage(outcome) };
	}
	const path = (outcome.json as DetailAnswer).path;
	if (path === undefined || path === "") {
		context.host.log(`dinah show ${context.ref} answered with no path`);
		return { kind: "failed", failure: "show answered with no path" };
	}
	await context.host.openDocument(path);
	return { kind: "done" };
}

/** What opening an attachment needs, which is its own file's path. */
export interface AttachmentOpenContext {
	readonly path: string;
}

/**
 * The path an attachment row names, when it names one.
 *
 * An attachment element carries no CommandContext, because that shape names a
 * card and the workbench the card stands in, while an attachment's path is the
 * whole of what opening one needs and it already rides the element the row was
 * drawn from. The absent element is checked by isRow before any field is read
 * off it, which is the one place that check lives (dinah-342).
 */
export function contextForAttachmentOpen(
	element: TreeElement | undefined,
): AttachmentOpenContext | undefined {
	if (!isRow(element, "attachment")) {
		return undefined;
	}
	// A row the listing did not answer for carries no view and so names no
	// path. It is drawn, so that a refusal is visible where the reader is
	// looking, and it opens nothing.
	if (element.view === undefined) {
		return undefined;
	}
	const path = element.view.path;
	if (path === undefined || path === "") {
		return undefined;
	}
	return { path };
}

/**
 * Opens an attachment's own file, handing the editor a path and nothing else.
 *
 * `openFile` rather than `openDocument`, because an attachment is arbitrary
 * bytes and the editor is the one to decide how to render them (dinah-335's
 * Decision 3). The plain click is the whole of what this handler offers; the
 * row's context menu arrived with dinah-451 and is the delete below.
 */
export async function openAttachment(
	context: AttachmentOpenContext,
	host: CommandHost,
): Promise<RowOutcome> {
	await host.openFile(context.path);
	return { kind: "done" };
}

/**
 * An attachment row's context, and the two strings its confirmation names.
 *
 * The verb takes `ref`, which is composed from the attachment's identifier so
 * that a position shifting under a concurrent delete cannot address a
 * different file. The confirmation shows `filename` and `drawnRef`, which are
 * what the row itself was drawn from, because a reader recognises the file by
 * its name and the address they would type rather than by an identifier.
 */
export interface AttachmentCommandContext extends CommandContext {
	readonly filename: string;
	/** The address the listing drew, which is what the singular confirmation names. */
	readonly drawnRef: string;
}

/**
 * The context an attachment row's own verbs run in.
 *
 * The reference is composed from the attachment's identifier rather than from
 * `view.ref`. `view.ref` ends in the attachment's position within its
 * collection, and a position shifts when an earlier attachment is deleted, so
 * a row drawn before somebody else's delete-then-attach would address a
 * different file with nothing refused. The identifier the listing reported is
 * what the row was drawn from and it names one attachment for as long as that
 * attachment exists.
 *
 * The absent element is checked by isRow before any field is read, which is
 * the ordering dinah-342 found wrong under six commands at once.
 *
 * The spawner is injected rather than reached for, which is what
 * contextForAttach and contextForColumn already do for the same reason: a
 * command composing its own context from an element is otherwise a command no
 * unit test can watch spawn, since nothing between the element and the call
 * belongs to the caller.
 */
export function contextForAttachment(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): AttachmentCommandContext | undefined {
	if (!isRow(element, "attachment")) {
		return undefined;
	}
	// The verb addresses the attachment by the identifier the listing
	// reported, so a degraded row drawn without a view offers it nothing to
	// address and is refused here as well as being drawn without a menu.
	if (element.view === undefined) {
		return undefined;
	}
	if (element.owner === "" || element.root === "" || element.view.id === "") {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		folder: element.row.folder,
		root: element.root,
		ref: `${element.owner}/${ATTACHMENTS_SEGMENT}/${element.view.id}`,
		filename: element.view.filename,
		drawnRef: element.view.ref,
	};
}

/**
 * Asks once, before anything spawns, whether to delete what was selected.
 *
 * The confirmation is the extension's own, because the tool's answer to the
 * same question is a required `--yes` marker rather than a prompt, and a
 * marker composed in code asks nobody anything. The singular sentence names
 * the file and the address the row was drawn from, since an attachment row is
 * labelled by filename alone and one entity may carry several; the plural
 * names how many attachments resolved, because a sentence naming one filename
 * can fill nothing from a run over several.
 *
 * The count is the resolved one rather than the targeted one, which is what
 * the signature enforces: this function is handed the resolved list, so a
 * selection of two attachments and a column row asks about two.
 */
export async function askDeleteAttachmentConfirmation(
	resolved: readonly AttachmentCommandContext[],
	host: CommandHost,
): Promise<true | undefined> {
	const confirmed = await host.confirmDestructive(
		resolved.length === 1
			? host.t("dialog.attachment.delete.confirm", {
					filename: resolved[0].filename,
					ref: resolved[0].drawnRef,
				})
			: host.t("dialog.attachment.delete.confirm.many", {
					count: String(resolved.length),
				}),
		host.t("dialog.attachment.delete.action"),
	);
	return confirmed ? true : undefined;
}

/** Deletes one attachment the reader has already confirmed. */
export async function deleteAttachmentAt(
	context: AttachmentCommandContext,
): Promise<CliOutcome> {
	return runVerb(context, ["delete", context.ref, "--yes"]);
}

/**
 * Archives one card, which takes it off the board.
 *
 * A sibling of claimCard and releaseCard, running the verb the CLI already
 * carries. `dinah archive` takes exactly one reference per invocation, so a
 * selection is a loop rather than a flag, and the confirmation is deliberately
 * not inside this function: a later card offering Archive on a column row
 * writes its own ask, because the sentence a column needs is not this one, and
 * reuses this verb and the whole bulk layer unchanged (D-2).
 */
export async function archiveCard(context: CommandContext): Promise<CliOutcome> {
	return runVerb(context, ["archive", context.ref]);
}

/**
 * Asks once, before anything spawns, whether to archive what was selected.
 *
 * The copy says outright that this extension cannot show the archive or put
 * the card back, because that is true until the companion card lands and a
 * reader deserves to know it at the moment they act rather than afterwards
 * (D-9). It names `dinah restore` as the route back, which was confirmed by a
 * run rather than read off the help.
 */
export async function askArchiveConfirmation(
	resolved: readonly CommandContext[],
	host: CommandHost,
): Promise<true | undefined> {
	const confirmed = await host.confirmDestructive(
		resolved.length === 1
			? host.t("dialog.archive.confirm.one", { ref: resolved[0].ref })
			: host.t("dialog.archive.confirm.many", {
					count: String(resolved.length),
				}),
		host.t("dialog.archive.action"),
	);
	return confirmed ? true : undefined;
}

// ---------------------------------------------------------------------------
// What the registration loop calls: one invoke per contributed card command
// ---------------------------------------------------------------------------
//
// Each function below takes the rows the reader aimed at, whole and
// unfiltered, and hands them to runBulk. No command filters its own rows:
// resolve inside runBulk is the one place this extension reads whether a row
// yielded a context, and it records the row either way (D-16). So the number
// of rows a run reports is the number of rows the reader aimed at.

/** The channel line a row that names no card gets. */
const NO_CARD = "names no card";

/** The channel line a row that names no attachment gets. */
const NO_ATTACHMENT = "names no attachment";

/** What a verb's answer comes to for the row that ran it. */
export function rowOutcomeFor(outcome: CliOutcome): RowOutcome {
	// A row is failed when the outcome's kind is anything other than ok.
	// Defining failure as a refusal would record a vanished binary
	// (spawn-failed) or an out-of-date one (stale) as a run of successes.
	return outcome.kind === "ok"
		? { kind: "done" }
		: { kind: "failed", failure: refusalMessage(outcome) };
}

/** The label a row shows in the tree, which is what a run records for it. */
export function rowRef(element: TreeElement, t: Localizer): string {
	return treeItemFor(element, t).label;
}

/** One run of a card-row command over the rows it was aimed at. */
async function cardRun<A>(
	elements: readonly TreeElement[],
	wiring: Wiring,
	ask: (
		resolved: readonly CommandContext[],
		host: CommandHost,
	) => Promise<A | undefined>,
	act: (
		context: CommandContext,
		answer: A,
		host: CommandHost,
	) => Promise<RowOutcome>,
	extra: Partial<BulkDeps<CommandContext, CommandHost>> = {},
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) => contextFor(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: NO_CARD,
			...extra,
		},
		ask,
		act,
	);
}

/** A fanOut card command asks nothing, so its ask answers a unit value. */
async function noQuestion(): Promise<true> {
	return true;
}

/** Opens each selected card's own file. */
export async function invokeOpenCard(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) =>
		openCard({ ...context, host }),
	);
}

/** Claims each selected card, continuing past a refusal. */
export async function invokeClaim(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) =>
		rowOutcomeFor(await claimCard({ ...context, host })),
	);
}

/** Releases each selected card. */
export async function invokeRelease(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) =>
		rowOutcomeFor(await releaseCard({ ...context, host })),
	);
}

/** Unblocks each selected card. */
export async function invokeUnblock(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) =>
		rowOutcomeFor(await unblockCard({ ...context, host })),
	);
}

/** Opens each selected card's served instruction chain as a tab. */
export async function invokeOpenInstructions(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) => {
		await openInstructions({ ...context, host });
		return { kind: "done" };
	});
}

/** Opens each selected card's own journal as a tab. */
export async function invokeOpenHistory(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(elements, wiring, noQuestion, async (context, _answer, host) => {
		await openHistory({ ...context, host });
		return { kind: "done" };
	});
}

/** Asks once for a destination every selected card accepts, then moves each. */
export async function invokeMove(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(
		elements,
		wiring,
		askMoveDestination,
		async (context, column, host) =>
			rowOutcomeFor(await moveCardTo({ ...context, host }, column)),
	);
}

/** Asks once for a reason, then blocks every selected card with it. */
export async function invokeBlock(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(
		elements,
		wiring,
		askBlockReason,
		async (context, reason, host) =>
			rowOutcomeFor(await blockCardWith({ ...context, host }, reason)),
	);
}

/** Asks once, then archives every selected card. */
export async function invokeArchiveCard(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(
		elements,
		wiring,
		askArchiveConfirmation,
		async (context, _answer, host) =>
			rowOutcomeFor(await archiveCard({ ...context, host })),
		// Archive alone lines every attempted row, the finished ones included,
		// because the confirmation promises the channel carries every
		// reference it archived and `dinah restore` is the route back (D-17).
		{ lineEveryRow: true, doneLine: "archived" },
	);
}

/** Puts every selected card's reference on the clipboard, in one write. */
export async function invokeCopyCardRef(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return cardRun(
		elements,
		wiring,
		noQuestion,
		// The per-row act performs no effect and reports nothing. The one
		// clipboard write happens in finish and the one sentence comes from
		// successMessage, which is what keeps the summary the only route this
		// command speaks through (D-23).
		async () => ({ kind: "done" }),
		{ finish: copyCardRefs, successMessage: copiedRefMessage },
	);
}

/** Opens each selected attachment's own file. */
export async function invokeOpenAttachment(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		contextForAttachmentOpen,
		{ host: wiring.cardHost, t: wiring.t, skipReason: NO_ATTACHMENT },
		noQuestion,
		async (context, _answer, host) => openAttachment(context, host),
	);
}

/** Asks once, then deletes every selected attachment. */
export async function invokeDeleteAttachment(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForAttachment(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{ host: wiring.cardHost, t: wiring.t, skipReason: NO_ATTACHMENT },
		askDeleteAttachmentConfirmation,
		async (context, _answer, host) =>
			rowOutcomeFor(await deleteAttachmentAt({ ...context, host })),
	);
}
