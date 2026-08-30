// The seven commands the tree contributes, as pure argv composition and pure
// outcome handling.
//
// Nothing here imports vscode. Each handler is a function over an injected
// host, so the unit layer asserts on the argv a command composes and on the
// message a refusal produces without a VS Code window, and extension.ts binds
// the host to the real window.
//
// Every mutating call is followed by one off-cycle checkpoint, whatever the
// outcome. The receipt is not read back into the tree: the next checkpoint
// repaints it from a fresh read, which is one rule rather than two and cannot
// drift from what the board actually says.

import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import type { CliOutcome } from "./cli";
import { nodeSpawner } from "./spawn";
import type { TreeElement } from "./tree";
import type { DetailAnswer, LegalMove, ServedAnswer } from "./wire";
import { BACKWARD, FORWARD } from "./wire";

/** One entry of a quick-pick, as the host renders it. */
export interface PickItem {
	readonly label: string;
	readonly detail?: string;
	/** The value the caller gets back, which is never shown. */
	readonly value: string;
}

/** The window calls these commands make, injected so tests can watch them. */
export interface CommandHost {
	readonly showError: (message: string) => void;
	readonly pick: (
		items: readonly PickItem[],
		placeholder: string,
	) => Promise<PickItem | undefined>;
	readonly input: (prompt: string) => Promise<string | undefined>;
	readonly openDocument: (path: string) => Promise<void>;
	/** Runs one off-cycle checkpoint for the folder the card stands in. */
	readonly checkpoint: (folder: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/** What every command needs: how to spawn, and where the card stands. */
export interface CommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the card's row belongs to. */
	readonly folder: string;
	/** The workbench the card stands in, which the call is pinned to. */
	readonly root: string;
	/** The card's own reference, which every verb below takes. */
	readonly ref: string;
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
 * The workbench a card's row stands in, and the folder its checkpoint is
 * keyed under.
 *
 * A card standing in a forest row belongs to that member workbench rather than
 * to the workspace folder the walk started from, so the verb is pinned to the
 * member's own path while the checkpoint still runs against the folder, whose
 * one merged cursor covers every member beneath it.
 *
 * The absent element is checked before anything is read off it. A command
 * invoked from the Command Palette, from a keybinding, or by another
 * extension arrives with no argument at all, and reading a field off that
 * argument threw a TypeError one line before the row-names-no-card branch
 * below could run (dinah-342). This function lives here rather than in
 * extension.ts so the unit layer can reach it: it touches no vscode value,
 * and the guard went six commands deep unexercised while it sat in the one
 * module no test can import.
 */
export function contextFor(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
): CommandContext | undefined {
	if (element === undefined || element.kind !== "card") {
		return undefined;
	}
	const ref = element.view?.ref ?? element.node.ref;
	const root = element.row.data?.path;
	if (ref === undefined || ref === "" || root === undefined) {
		return undefined;
	}
	return {
		spawner: nodeSpawner,
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
 */
async function runVerb(
	context: CommandContext,
	args: readonly string[],
): Promise<CliOutcome> {
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
 * Blocks the card, after asking for the reason the verb requires.
 *
 * An empty reason cancels rather than sending one, because `dinah block` takes
 * the reason as an argument and a blank one would record a block nobody can
 * act on.
 */
export async function blockCard(
	context: CommandContext,
): Promise<CliOutcome | undefined> {
	const reason = await context.host.input("Why is this card blocked?");
	if (reason === undefined || reason.trim() === "") {
		return undefined;
	}
	return runVerb(context, ["block", context.ref, reason.trim()]);
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
export function movePick(move: LegalMove): PickItem {
	return {
		label: move.title,
		detail: move.direction === BACKWARD ? "backward" : undefined,
		// The destination the move verb takes is the entry's own Column, which
		// is the column's identifier. Not its Ref, which is what a person
		// types, and not its Title, which is what a person reads.
		value: move.column,
	};
}

/**
 * Moves the card, after asking which destination.
 *
 * The destination list is fetched when the item is invoked rather than
 * eagerly for every card in the tree, because a tree of two hundred cards
 * would otherwise cost two hundred spawns to draw.
 */
export async function moveCard(
	context: CommandContext,
): Promise<CliOutcome | undefined> {
	const served = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["instructions", context.ref]),
		{ cwd: context.root },
	);
	if (served.kind !== "ok") {
		context.host.showError(refusalMessage(served));
		return served;
	}
	const moves = (served.json as ServedAnswer).legal_moves ?? [];
	if (moves.length === 0) {
		context.host.showError(
			`${context.ref} has no legal moves from where it stands.`,
		);
		return undefined;
	}
	const picked = await context.host.pick(
		orderLegalMoves(moves).map(movePick),
		`Move ${context.ref} to`,
	);
	if (picked === undefined) {
		return undefined;
	}
	return runVerb(context, ["move", context.ref, picked.value]);
}

/**
 * Opens the card's own file.
 *
 * The path is read off `show`'s own Detail.Path rather than composed from the
 * reference. `dinah path` would be the obvious verb and has no machine form
 * (dinah-272), and a path this extension built itself would be a second
 * spelling of a layout the binary already owns.
 */
export async function openCard(context: CommandContext): Promise<void> {
	const outcome = await runDinah(
		context.spawner,
		context.exe,
		pinnedArgv(context.root, ["show", context.ref]),
		{ cwd: context.root },
	);
	if (outcome.kind !== "ok") {
		context.host.showError(refusalMessage(outcome));
		return;
	}
	const path = (outcome.json as DetailAnswer).path;
	if (path === undefined || path === "") {
		context.host.log(`dinah show ${context.ref} answered with no path`);
		return;
	}
	await context.host.openDocument(path);
}
