// The one act a queue column row offers: pulling its head-of-ready card into
// the column the workbench says a pull from there lands in (dinah-375).
//
// It mutates the board, so it takes cardCommands.ts's own CommandHost, which
// carries checkpoint, rather than columnCommands.ts's file-opening host, on
// the same terms creationCommands.ts already sets out. Nothing here imports
// vscode.
//
// The command sits on the column row rather than on a card row because
// `dinah pull` takes a destination column and picks the head of that
// destination's upstream itself. No argument names a card, so a per-card item
// would promise a targeting the verb cannot honour and would move a different
// card than the one the reader clicked.

import type { CommandHost } from "./cardCommands";
import { isRow, pinnedArgv, refusalMessage } from "./cardCommands";
import type { CliOutcome, Spawner } from "./cli";
import { runDinah } from "./cli";
import type { TreeElement } from "./tree";
import type { PullAnswer } from "./wire";

/** What Pull needs: where the workbench stands, and which queue and destination. */
export interface PullCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the column's row belongs to. */
	readonly folder: string;
	/** The workbench the queue stands in, which the call is pinned to. */
	readonly root: string;
	/** What ColumnView.pull_destination named: the column the card lands in. */
	readonly destination: string;
	/** The queue's own title, which the empty-pull message's upstream half falls back to. */
	readonly label: string;
}

/**
 * The context for Pull, or undefined when the row named cannot be pulled from.
 *
 * A column the status/tree join missed carries no ColumnView and yields no
 * context, on the same terms contextForColumn already takes for New Card. A
 * work column, and a queue publishing no destination, are declined here as
 * well rather than left to the caller having read the contextValue correctly:
 * a command is reachable by a keybinding and by another extension, neither of
 * which consults a menu clause.
 *
 * The absent element is checked by isRow before any field is read off it,
 * which is the one place that check lives (dinah-342).
 */
export function contextForPull(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): PullCommandContext | undefined {
	if (!isRow(element, "column") || element.view === undefined) {
		return undefined;
	}
	if (element.view.takes_work_up) {
		return undefined;
	}
	const destination = element.view.pull_destination;
	if (destination === undefined || destination === "") {
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
		destination,
		label: element.view.title,
	};
}

/**
 * Pulls the head-of-ready card out of this queue, without claiming it.
 *
 * `--no-claim` is dinah-375 D-1. Clearing a queue forward is not a declaration
 * that whoever right-clicked is now doing the card's work, so the card lands
 * ready in the destination exactly where an ordinary move would leave it, and
 * the timeline records the one thing that happened. Every refusal a claiming
 * pull answers is still answered here; the marker skips only the unresolved-item
 * check, which guards a claim this form does not take.
 *
 * A refusal is shown as every other row command shows one. An ok answer
 * carrying no card means the queue's upstream held nothing ready, and that is
 * reported rather than left to look identical to a pull that worked, since a
 * pull that silently does nothing is worse than no pull at all. An ordinary
 * successful pull reports nothing: the card's arrival in its new column is the
 * trace, as it is for Claim, Release, Move and Unblock.
 */
export async function pullFromColumn(
	context: PullCommandContext,
): Promise<CliOutcome> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["pull", context.destination, "--no-claim"]),
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
	} else {
		const answer = outcome.json as PullAnswer;
		if (answer.card === undefined) {
			context.host.showInfo(
				emptyPullMessage(answer, context.label, context.destination),
			);
		}
	}
	await context.host.checkpoint(context.folder);
	return outcome;
}

/**
 * The sentence an ok-but-empty pull shows.
 *
 * The two titles are read off the answer's own message_values, which is where
 * okEmpty puts the upstream's title and the destination's, so the reader is
 * told which column was looked in rather than which column was clicked. The
 * English is composed here rather than read from dinah's message catalog,
 * which is what every other string this extension shows already does.
 *
 * Each half falls back to what the caller already holds for that half rather
 * than to one value for both. The clicked queue is the column the pull draws
 * from, so it stands in for a missing upstream. The destination the argv was
 * built from stands in for a missing destination, unresolved to a title but
 * still naming the column the card would land in; columnTooltip falls back to
 * the same raw reference for the same reason. Falling back to the queue for
 * both halves would compose "Nothing in Design Queue is ready to pull into
 * Design Queue.", which tells the reader the card goes back where it already
 * is.
 */
export function emptyPullMessage(
	answer: PullAnswer,
	label: string,
	destination: string,
): string {
	const from = answer.message_values?.upstream ?? label;
	const into = answer.message_values?.destination ?? destination;
	return `Nothing in ${from} is ready to pull into ${into}.`;
}
