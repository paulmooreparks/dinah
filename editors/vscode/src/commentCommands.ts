// The one command a comment row contributes: opening the comment's own anchor
// file.
//
// Nothing here imports vscode, for the reason cardCommands.ts's header gives.
// The handler is a function over an injected host, so the unit layer asserts on
// the argv it composes and on the message a refusal produces without a VS Code
// window.
//
// One command and no more, because Dinah offers no more. `dinah comment`
// records a comment on a card, on a column, or on one of a card's items, and
// it refuses a comment's own reference, so a Reply entry here would offer a
// refusal, and there is no verb that edits or deletes a comment for an entry
// to run.

import type { BulkReport } from "./bulk";
import { runBulk } from "./bulk";
import type { CommandHost } from "./cardCommands";
import { isRow, pinnedArgv, refusalMessage, rowRef } from "./cardCommands";
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
