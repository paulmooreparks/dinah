// The two commands a comment row contributes: Open Comment, which opens the
// comment's own anchor file, and Delete Comment, which runs `dinah delete` on
// the comment after the reader confirms it.
//
// Nothing here imports vscode, for the reason cardCommands.ts's header gives.
// The handlers are functions over an injected host, so the unit layer asserts
// on the argv they compose and on the message a refusal produces without a
// VS Code window.
//
// A comment's body is edited by saving the file Open Comment opens, which
// writes it through the verb. No Reply entry is offered, because `dinah
// comment` records a comment on a card, on a column, or on one of a card's
// items, and it refuses a comment's own reference, so a Reply entry here would
// offer a refusal.

import type { BulkReport, RowOutcome } from "./bulk";
import { runBulk } from "./bulk";
import type { CommandHost } from "./cardCommands";
import {
	isRow,
	pinnedArgv,
	refusalMessage,
	rowOutcomeFor,
	rowRef,
	runVerb,
} from "./cardCommands";
import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import type { Wiring } from "./commandTable";
import type { CommentBodyHost, OpenComments } from "./commentBody";
import { openExistingComment } from "./commentBody";
import type { TreeElement } from "./tree";
import type { PathAnswer } from "./wire";

/** What a comment command acts on, which is one comment and where it stands. */
export interface CommentCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the comment's row belongs to. */
	readonly folder: string;
	/** The workbench the comment stands in, which the call is pinned to. */
	readonly root: string;
	/** The comment's own reference, which is the whole of what opening it needs. */
	readonly ref: string;
}

/**
 * The context for a comment command, or undefined for a row that is not a
 * comment.
 *
 * No holder is composed and none is needed. Every earlier draft of this work
 * resolved a comment by cutting the trailing `/comments/<n>` off its reference
 * and asking about whatever was left, which meant working out what kind of
 * thing the holder was; `path` takes the comment's own reference and resolves
 * it whatever holds it, so the question does not arise.
 */
export function contextForComment(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): CommentCommandContext | undefined {
	if (!isRow(element, "comment")) {
		return undefined;
	}
	const ref = element.node.ref ?? "";
	if (ref === "") {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		folder: element.row.folder,
		root: element.root,
		ref,
	};
}

/**
 * Opens the comment's own anchor file, and starts an editing session over it.
 *
 * A comment is prose somebody wrote, and a page this extension composes over
 * it is the defect this work was filed about. Opening `comment.md` is what
 * opening `card.md` already is: the thing itself, editable, with its front
 * matter above the prose.
 *
 * The session is what parts this from opening a card. A comment's body carries
 * a digest of itself, so a save that left the editor's own bytes on the file
 * would leave a comment `dinah check` reports as hand-edited. Reaching the
 * file through openExistingComment remembers the digest the anchor records, so
 * the save handler can hand it back to the verb and have the write attributed
 * rather than refused. The two routes to an open comment, composing a new one
 * and opening one that exists, therefore end in the same state.
 */
export async function openComment(
	context: CommentCommandContext,
	commentHost: CommentBodyHost,
	opened: OpenComments,
): Promise<void> {
	const located = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["path", context.ref]),
		{ cwd: context.root },
	);
	if (located.kind !== "ok") {
		context.host.showError(refusalMessage(located));
		return;
	}
	const path = (located.json as PathAnswer).path ?? "";
	if (path === "") {
		context.host.log(`dinah path ${context.ref} answered with no path`);
		return;
	}
	await openExistingComment(commentHost, opened, path, {
		root: context.root,
		folder: context.folder,
		ref: context.ref,
	});
}

/**
 * The channel line a row that names no comment gets.
 *
 * A module constant rather than a catalogue key, on the model of
 * cardCommands.ts's own NO_CARD and NO_ATTACHMENT. It is a channel line for a
 * row the reader did not aim this command at, not a sentence the panel shows.
 */
const NO_COMMENT = "names no comment";

/** Opens the anchor file of every selected comment, one tab each. */
export async function invokeOpenComment(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForComment(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: NO_COMMENT,
		},
		async () => true as const,
		async (context, _answer, host) => {
			await openComment(
				{ ...context, host },
				wiring.commentHost,
				wiring.openComments,
			);
			return { kind: "done" } as const;
		},
	);
}

/**
 * Asks once, before anything spawns, whether to delete what was selected.
 *
 * askDeleteAttachmentConfirmation with the comment strings. One comment is
 * named by the reference the reader sees drawn in the tree, and several are
 * counted; the count is the resolved one, because this function is handed the
 * resolved list.
 */
export async function askDeleteCommentConfirmation(
	resolved: readonly CommentCommandContext[],
	host: CommandHost,
): Promise<true | undefined> {
	const confirmed = await host.confirmDestructive(
		resolved.length === 1
			? host.t("dialog.comment.delete.confirm", { ref: resolved[0].ref })
			: host.t("dialog.comment.delete.confirm.many", {
					count: String(resolved.length),
				}),
		host.t("dialog.comment.delete.action"),
	);
	return confirmed ? true : undefined;
}

/** The refusal `dinah delete` gives a comment that is an item's answer of record. */
const NOT_DESIGNATABLE = "dinah.not-designatable";

/**
 * Deletes one comment the reader has already confirmed, and reports the
 * outcome that is final.
 *
 * A comment can be the answer of record for a checklist item, and `dinah
 * delete` refuses such a comment with dinah.not-designatable unless it is
 * forced, when it deletes the comment and reopens the item in one act. So the
 * first attempt runs quietly rather than through runVerb, which would show the
 * refusal as an error on a single row and leave it on screen after the forced
 * delete succeeded. The call is built exactly as runVerb builds it.
 *
 * Every path reports at most once and checkpoints exactly once. The forced
 * delete reports and checkpoints through runVerb. A declined second
 * confirmation shows the first refusal and checkpoints, and so does every
 * other outcome that is not ok. The item named in the second confirmation is
 * the refusal's own context.item, and a refusal carrying none raises no second
 * confirmation, because the prompt must not name an item the extension did
 * not receive.
 */
export async function deleteCommentAt(
	context: CommentCommandContext,
): Promise<RowOutcome> {
	const first = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["delete", context.ref, "--yes"]),
		{ cwd: context.root },
	);
	if (first.kind === "ok") {
		await context.host.checkpoint(context.folder);
		return { kind: "done" };
	}
	const item =
		first.kind === "refused" && first.refusal === NOT_DESIGNATABLE
			? (first.context?.item ?? "")
			: "";
	if (item !== "") {
		const confirmed = await context.host.confirmDestructive(
			context.host.t("dialog.comment.delete.designated.confirm", {
				ref: context.ref,
				item,
			}),
			context.host.t("dialog.comment.delete.designated.action"),
		);
		if (confirmed) {
			return rowOutcomeFor(
				await runVerb(context, ["delete", context.ref, "--yes", "--force"]),
			);
		}
	}
	context.host.showError(refusalMessage(first));
	await context.host.checkpoint(context.folder);
	return rowOutcomeFor(first);
}

/** Asks once, then deletes every selected comment. */
export async function invokeDeleteComment(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForComment(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: NO_COMMENT,
		},
		askDeleteCommentConfirmation,
		async (context, _answer, host) => deleteCommentAt({ ...context, host }),
	);
}
