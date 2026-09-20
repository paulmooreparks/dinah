// Composing a comment in the editor, and writing its body through the verb.
//
// The operator ruled on 2026-09-15 that VS Code is an editor, so a prose
// comment is written in one. dinah-506 built an apparatus for that: a draft
// file in the extension's own global storage, an index keyed by its path, a
// post command and a discard command, all of it there to decide when an author
// had finished composing something that did not exist yet.
//
// dinah-525 deleted the question rather than the apparatus's answer to it.
// `dinah comment <ref>` with no text mints the comment and writes an empty
// body, so the entity exists from the first keystroke: there is a file to open
// and a reference to name it by, and an author who says nothing after all runs
// `dinah delete` on it. Nothing has to guess when they are finished, so nothing
// here does.
//
// Two things this module still decides, and they are the whole of it.
//
// The body is written through the verb, not by the editor. The digest a
// comment's anchor carries is recomputed by every verb that writes it, and a
// body the editor put there leaves the digest where it was, which `dinah check`
// then reports as a divergence. Saving through `dinah set <comment> body`
// records the new digest, journals the write and attributes it, which is the
// whole of what the record is for.
//
// What is sent is the body alone. The tab holds the anchor file, which is the
// front matter and the prose below it, and the front matter is the tool's to
// write. splitAnchorBody is where that cut is made, and it is made on the
// closing fence rather than by counting lines.
//
// Nothing here imports vscode, on the terms cardCommands.ts's header gives.

import type { VerbContext } from "./cardCommands";
import { pinnedArgv, refusalMessage, runVerb } from "./cardCommands";
import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import type { Localizer } from "./l10n";
import type { PathAnswer } from "./wire";

/**
 * The fence an anchor file's front matter opens and closes with.
 *
 * Three hyphens on a line of their own, which is what every anchor this
 * format writes carries and what the anchor parser reads.
 */
const FENCE = "---";

/**
 * Cuts an anchor file into its front matter and its body.
 *
 * The cut is made on the second fence rather than by counting lines, because a
 * front matter block holds as many keys as the entity has and a sequence value
 * carries more lines still. A text that does not open with a fence is all
 * body, which is what a file somebody emptied looks like and what a reader
 * would mean by it.
 *
 * The body is returned verbatim: no trim, no normalisation of line endings and
 * no stripping of a trailing newline. The store keeps what it is given and the
 * only way a comment reads as its author wrote it is to send what its author
 * wrote.
 */
export function splitAnchorBody(text: string): string {
	const newline = text.includes("\r\n") ? "\r\n" : "\n";
	const opening = FENCE + newline;
	if (!text.startsWith(opening)) {
		return text;
	}
	const closing = newline + FENCE + newline;
	const at = text.indexOf(closing, opening.length - newline.length);
	if (at < 0) {
		return text;
	}
	return text.slice(at + closing.length);
}

/** What the comment commands ask the window for. */
export interface CommentBodyHost {
	readonly t: Localizer;
	readonly openDocument: (path: string) => Promise<void>;
	readonly showError: (message: string) => void;
	readonly showInfo: (message: string) => void;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly checkpoint: (folder: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/** Where one open comment stands, so a save of its file can reach the verb. */
export interface OpenComment {
	/** The workbench the write is pinned to. */
	readonly root: string;
	/** The workspace folder the write's checkpoint runs against. */
	readonly folder: string;
	/** The comment's own reference, which the write names. */
	readonly ref: string;
}

/**
 * What the extension knows about the comment files it has opened, keyed by the
 * file's absolute path.
 *
 * A map rather than a resolution from the path, because a path under a
 * workbench says which comment it is only by being parsed, and this extension
 * does not parse references out of paths anywhere else: it asks `dinah path`
 * which file a reference names and remembers the answer. A file this extension
 * did not open is one it writes nothing for, which is the honest answer rather
 * than a restrictive one, since a person who opened `comment.md` from the
 * explorer is editing a file by hand and `dinah check` is what tells them so.
 */
export type OpenComments = Map<string, OpenComment>;

/**
 * The identifier an ok envelope carries in its detail, which is what the
 * comment verb answers with and what the next call resolves.
 *
 * A comment's identifier resolves exactly as its positional reference does, so
 * nothing here composes `<holder>/comments/<n>`: the ordinal a reference
 * carries is a position in a collection, and reading the identifier off the
 * answer is what stops this module counting comments to find out which one it
 * just made.
 */
function detailOf(json: unknown): string {
	if (typeof json !== "object" || json === null) {
		return "";
	}
	const detail = (json as { detail?: unknown }).detail;
	return typeof detail === "string" ? detail : "";
}

/** What composing a comment needs: the holder and where it stands. */
export interface ComposeTarget {
	readonly root: string;
	readonly folder: string;
	/** The holder's reference, which is a card, a column or a checklist item. */
	readonly ref: string;
}

/**
 * Mints an empty comment below one holder and opens its file.
 *
 * Two calls and no state. `dinah comment <ref>` with no text answers with the
 * new comment's own identifier, which resolves like any other reference, and
 * `dinah path` turns that into the file the window opens. What used to be a
 * draft in global storage, an index entry and two commands is now the entity
 * itself, in the workbench, where `dinah show` can see it and where an author
 * who abandons it runs `dinah delete`.
 */
export async function composeComment(
	host: CommentBodyHost,
	spawner: Spawner,
	exe: string,
	opened: OpenComments,
	target: ComposeTarget,
): Promise<void> {
	const context: VerbContext = {
		spawner,
		exe,
		host,
		folder: target.folder,
		root: target.root,
		ref: target.ref,
	};
	const minted = await runVerb(context, ["comment", target.ref]);
	if (minted.kind !== "ok") {
		return;
	}
	const ref = detailOf(minted.json);
	if (ref === "") {
		host.showError(host.t("comment.composeNoReference", { ref: target.ref }));
		return;
	}
	const located = await runDinah(spawner, exe, pinnedArgv(target.root, ["path", ref]), {
		cwd: target.root,
	});
	if (located.kind !== "ok") {
		host.showError(refusalMessage(located));
		return;
	}
	const path = (located.json as PathAnswer).path ?? "";
	if (path === "") {
		host.showError(host.t("comment.composeNoPath", { ref }));
		return;
	}
	opened.set(path, { root: target.root, folder: target.folder, ref });
	await host.openDocument(path);
	host.showInfo(host.t("comment.composed", { ref, path }));
}

/**
 * Remembers that one comment's file is open, so a later save of it reaches the
 * verb rather than leaving the editor's own bytes on disk.
 *
 * Opening an existing comment goes through this for the same reason composing
 * one does: the two end in the same place, a tab holding `comment.md`, and a
 * save of that tab has to mean the same thing whichever way the reader got
 * there.
 */
export function noteOpenComment(
	opened: OpenComments,
	path: string,
	standing: OpenComment,
): void {
	opened.set(path, standing);
}

/** Forgets a comment file, which is what closing its tab comes to. */
export function forgetComment(opened: OpenComments, path: string): void {
	opened.delete(path);
}

/**
 * Writes the body of a saved comment file through the verb.
 *
 * The editor has already put its own bytes on disk by the time this runs, and
 * that is the state this call exists to correct rather than to prevent: the
 * write below re-renders the anchor from the body it is given and records the
 * digest over it, so the record and the file agree again and the journal says
 * who changed it. An implementation that let the editor's save stand would
 * leave a comment `dinah check` reports as diverged on every save.
 *
 * A file this extension did not open is left alone and answers false, which is
 * what lets the caller keep its listener cheap.
 */
export async function saveCommentBody(
	host: CommentBodyHost,
	spawner: Spawner,
	exe: string,
	opened: OpenComments,
	path: string,
	text: string,
): Promise<boolean> {
	const standing = opened.get(path);
	if (standing === undefined) {
		return false;
	}
	const body = splitAnchorBody(text);
	const context: VerbContext = {
		spawner,
		exe,
		host,
		folder: standing.folder,
		root: standing.root,
		ref: standing.ref,
	};
	const written = await runVerb(context, ["set", standing.ref, "body", "-"], body);
	return written.kind === "ok";
}
