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
// The body is written through the verb, and the save is a compare-and-swap on
// the digest rather than on the body. An editor writes the file when its
// author saves, so by the time a save handler runs the body has already
// changed and a verb comparing the body against the digest refuses the
// author's own edit; the first shape of this module did exactly that and
// refused every save. What survives the editor's write is the front matter, so
// what this module remembers when it opens a comment is the digest the anchor
// records, and what it hands the verb on save is that remembered value as the
// digest it expects to still find there. Unmoved, and the change on disk is
// this session's own, so the verb takes the body as it stands, re-stamps and
// journals. Moved, and somebody wrote the comment through a verb while the
// author was typing, so the save is refused rather than overwriting work the
// author never saw.
//
// This does not weaken the divergence refusal. A hand edit made outside a
// session this module opened has no remembered digest to match, so it meets
// the verb's own body comparison exactly as before.
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
	/**
	 * The file's text, or undefined where it cannot be read. Reading is what
	 * tells a session which digest the anchor records when it opens a
	 * comment, and an unreadable file answers undefined rather than throwing,
	 * because the one question asked of it is what the header says and "it
	 * does not say" is an answer the caller handles.
	 */
	readonly readFile: (path: string) => Promise<string | undefined>;
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
	/**
	 * The digest the anchor recorded when this session opened the comment,
	 * which the save hands back as the value it expects to still find there.
	 *
	 * It is read out of the header rather than computed from the body, and
	 * the difference is the whole mechanism: the body is gone by the time a
	 * save handler runs, and the header is not.
	 */
	readonly digest: string;
}

/**
 * The digest an anchor's front matter records, or the empty string where it
 * records none.
 *
 * Read with a line scan rather than by parsing the block, because one key is
 * wanted out of a header whose other keys this module has no business
 * knowing, and a comment written before the digest existed carries none at
 * all. An empty answer is a fact rather than a failure: it says the comment
 * has never been written by a verb that stamps one, and a save then falls to
 * the verb's own body comparison.
 */
export function recordedDigest(text: string): string {
	const newline = text.includes("\r\n") ? "\r\n" : "\n";
	const opening = FENCE + newline;
	if (!text.startsWith(opening)) {
		return "";
	}
	const closing = newline + FENCE + newline;
	const end = text.indexOf(closing, opening.length - newline.length);
	const header = end < 0 ? text : text.slice(opening.length, end + newline.length);
	for (const line of header.split(newline)) {
		const [key, ...rest] = line.split(":");
		if (key.trim() === "digest") {
			return rest.join(":").trim();
		}
	}
	return "";
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
	// The comment was minted a moment ago, so the digest on its header is the
	// one over an empty body and reading the file is the honest way to learn
	// it rather than composing it here. host.readFile answers undefined for a
	// file it cannot read, which leaves the session with no remembered digest
	// and the save falling to the verb's own body comparison.
	const digest = recordedDigest((await host.readFile(path)) ?? "");
	opened.set(path, { root: target.root, folder: target.folder, ref, digest });
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

/**
 * Opens an existing comment's file and remembers the digest its anchor
 * records, so that saving the tab reaches the verb the same way a comment
 * composed here does.
 *
 * A comment already diverged when the session opens is not adopted. Its
 * remembered digest would match the header, so the compare-and-swap would
 * pass and the save would write over somebody's hand edit and re-stamp it,
 * which is the silent absorption the divergence refusal exists to prevent.
 * The file is still opened, because restoring the body by hand is the remedy
 * the format names first and the author needs the file to do it; what is
 * withheld is the session, so a save leaves the editor's bytes where they are
 * and dinah check goes on reporting them.
 */
export async function openExistingComment(
	host: CommentBodyHost,
	opened: OpenComments,
	path: string,
	standing: Omit<OpenComment, "digest">,
): Promise<void> {
	const text = (await host.readFile(path)) ?? "";
	const digest = recordedDigest(text);
	if (digest !== "" && digest !== (await digestOf(splitAnchorBody(text)))) {
		host.showError(host.t("comment.divergedOnOpen", { ref: standing.ref }));
	} else {
		opened.set(path, { ...standing, digest });
	}
	await host.openDocument(path);
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
	const argv = ["set", standing.ref, "body", "-"];
	if (standing.digest !== "") {
		argv.push("--expect-digest", standing.digest);
	}
	const written = await runVerb(context, argv, body);
	if (written.kind !== "ok") {
		return false;
	}
	// The verb re-stamped the digest, so the value this session remembers has
	// to move with it or the next save of the same tab would hand the verb a
	// digest that is one edit stale and be refused. The new value is computed
	// rather than read back, because the verb hashes the body it was given
	// and that is the body in hand.
	opened.set(path, { ...standing, digest: await digestOf(body) });
	return true;
}

/**
 * The digest of one body, computed the way the tool computes it: a
 * hex-encoded SHA-256 over the bytes, through the platform's own subtle
 * crypto.
 *
 * It is computed rather than read back off the file because the answer is
 * wanted before anybody would read the file again, and because a read would
 * be a second source of truth for a value the verb has just derived from the
 * bytes this function is given.
 */
async function digestOf(body: string): Promise<string> {
	const bytes = new TextEncoder().encode(body);
	const sum = await crypto.subtle.digest("SHA-256", bytes);
	return [...new Uint8Array(sum)]
		.map((byte) => byte.toString(16).padStart(2, "0"))
		.join("");
}
