// The one command a comment row contributes: opening the comment's own anchor
// file.
//
// Nothing here imports vscode, for the reason cardCommands.ts's header gives.
// The handler is a function over an injected host, so the unit layer asserts on
// the argv it composes and on the message a refusal produces without a VS Code
// window.
//
// One command and no more, because Dinah offers no more. `dinah comment`
// records a comment on a card or on one of its items and refuses a comment's
// own reference, so a Reply entry here would offer a refusal, and there is no
// verb that edits or deletes a comment for an entry to run.

import type { BulkReport } from "./bulk";
import { runBulk } from "./bulk";
import type { CommandHost } from "./cardCommands";
import { isRow, rowRef } from "./cardCommands";
import type { Spawner } from "./cli";
import type { Wiring } from "./commandTable";
import { openAnchorFile } from "./itemCommands";
import type { TreeElement } from "./tree";

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
 * Opens the comment's own anchor file.
 *
 * A comment is prose somebody wrote, and a page this extension composes over
 * it is the defect this work was filed about. Opening `comment.md` is what
 * opening `card.md` already is: the thing itself, editable, with its front
 * matter above the prose.
 */
export async function openComment(context: CommentCommandContext): Promise<void> {
	await openAnchorFile(
		context.spawner,
		context.exe,
		context.host,
		context.root,
		context.ref,
	);
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
			await openComment({ ...context, host });
			return { kind: "done" } as const;
		},
	);
}
