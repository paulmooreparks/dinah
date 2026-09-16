// Composing a comment in the editor, and posting it through the verb. The
// holder is a checklist item or a column, and this module does not know which:
// it is handed a reference and it records an invocation.
//
// The operator ruled on 2026-09-15 that VS Code is an editor, so a prose
// comment is written in one. The house pattern for that already exists twice,
// in editColumnInstructions and editWorkbenchDefinition, and both ask dinah
// where a file is and hand the path to the window. Two things make a comment
// different, and they are the whole of what this module decides.
//
// Composing is not editing. Those two commands open a file that already
// exists. A comment does not exist until somebody has written it, so there is
// no path to ask for and something has to decide when the author is finished.
//
// The verb writes, not the editor. Library.Comment stamps the author and the
// clock, mints the comment's identifier, appends an event to the journal of
// the entity the comment hangs below, which is the card for an item comment
// and the workbench for a column comment, and stamps a locator naming the
// holder on that event. An extension writing
// the comment file itself would produce a comment with no journal entry and a
// reference it invented. So the buffer supplies the text and `dinah comment`
// still does the writing.
//
// Nothing here imports vscode. Every filesystem call and every window call
// arrives through DraftHost, which extension.ts binds to vscode.workspace.fs
// and to the window, on the same terms cardCommands.ts takes its CommandHost.
// DraftHost satisfies VerbContext's host without an assertion, because runVerb
// asks for the two members a spawn needs rather than for a whole CommandHost.

import type { VerbContext } from "./cardCommands";
import { runVerb } from "./cardCommands";
import type { Spawner } from "./cli";
import { fingerprint } from "./fingerprint";
import type { Localizer } from "./l10n";

/** The directory under the extension's global storage that holds every draft. */
export const DRAFTS_DIRNAME = "comment-drafts";

/**
 * What every draft file's name ends in.
 *
 * The `.md` half is what gives the tab Markdown highlighting with no
 * setTextDocumentLanguage call, and the segment before it is what the
 * manifest's own when-clause matches, so the two draft commands are offered on
 * a draft and nowhere else.
 */
export const DRAFT_SUFFIX = ".dinah-comment.md";

/** The globalState key the whole index is stored under. */
export const DRAFT_INDEX_KEY = "dinah.commentDrafts";

/** What one open draft is going to do when its author posts it. */
export interface DraftEntry {
	/** The workbench the post is pinned to, sent as --workbench. */
	readonly root: string;
	/** The folder the post's checkpoint runs against. */
	readonly folder: string;
	/** The invocation the draft completes, with `-` in the text slot. */
	readonly argv: readonly string[];
	/** What the messages name, which is the item's reference. */
	readonly target: string;
}

/** The whole index, keyed by the draft file's absolute path. */
export type DraftIndex = Record<string, DraftEntry>;

/**
 * What the draft commands ask the window and the disk for.
 *
 * Every filesystem call goes through vscode.workspace.fs rather than through
 * node:fs, which keeps the filesystem in the one module already holding every
 * other vscode value. That is the posture the repository's spawn-sites check
 * enforces for child processes: one module reaches the outside world and the
 * rest of src/ asks it to.
 *
 * readIndex is synchronous because Memento.get is, and writeIndex is not
 * because Memento.update returns a thenable. The index is read fresh on every
 * command rather than cached, which keeps one window from acting on a picture
 * of the index it took minutes ago. That freshness is a convenience and
 * carries no correctness argument: VS Code documents Memento as storage that
 * persists across reloads and documents nothing about when a write in one
 * window becomes visible in another, so nothing here depends on its doing so.
 * Where the index and the disk disagree about a draft, the disk is right.
 */
export interface DraftHost {
	readonly t: Localizer;
	/** globalStorageUri.fsPath, which this module joins onto. */
	readonly storageRoot: string;
	/** createDirectory, which is documented to create missing parents. */
	readonly ensureDirectory: (path: string) => Promise<void>;
	/** writeFile with the UTF-8 bytes of text. */
	readonly writeDraft: (path: string, text: string) => Promise<void>;
	/** readFile decoded as UTF-8, or undefined when there is no such file. */
	readonly readDraft: (path: string) => Promise<string | undefined>;
	/** delete, which the successful post and Discard both reach. */
	readonly deleteDraft: (path: string) => Promise<void>;
	/** TextDocument.save() for the document at path, false when it did not save. */
	readonly saveDocument: (path: string) => Promise<boolean>;
	readonly openDocument: (path: string) => Promise<void>;
	readonly readIndex: () => DraftIndex;
	readonly writeIndex: (index: DraftIndex) => Promise<void>;
	readonly confirmDestructive: (
		message: string,
		confirmLabel: string,
	) => Promise<boolean>;
	readonly showError: (message: string) => void;
	readonly showInfo: (message: string) => void;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly checkpoint: (folder: string) => Promise<void>;
	readonly log: (line: string) => void;
}

/**
 * Joins path segments the way every platform this extension runs on reads
 * them.
 *
 * A backslash separator is what Windows prints and what a reader recognises in
 * a message naming a file, so the platform's own separator is used rather than
 * a forward slash everywhere. The separator is taken from storageRoot itself:
 * the editor hands a path spelled the platform's way, so its own spelling is
 * the right one to continue in, and this module needs no node:path import to
 * find it out.
 */
export function joinDraftPath(base: string, ...segments: readonly string[]): string {
	const separator = base.includes("\\") && !base.includes("/") ? "\\" : "/";
	return [base.replace(/[\\/]+$/, ""), ...segments].join(separator);
}

/**
 * The draft file's name, which is display and is never parsed back.
 *
 * This is the split servedText.ts already states for its own URIs, where the
 * authority and the query say what to fetch and the path is the tab's label.
 * Identity lives in the index, keyed by the absolute path, so a reader who
 * renames a draft breaks nothing and a filename naming the wrong item posts to
 * the right one.
 */
export function draftFilename(itemRef: string): string {
	return `${itemRef.split("/").join(".")}${DRAFT_SUFFIX}`;
}

/**
 * Where one item's draft lives.
 *
 * The directory is per workbench and the filename is per item. fingerprint is
 * a pure function of its argument's UTF-8 bytes, so fingerprint(root) is a
 * stable directory name for one workbench and two workbenches that both hold a
 * dinah-506 do not collide.
 *
 * The extension's own global storage rather than the workbench or the
 * workspace. A draft is not workbench content and must not land in the
 * workbench, where `dinah check` would meet it and a commit would carry it,
 * and it is not workspace content either, because a reader with no folder open
 * has no workspace storage at all and the tree works for them today.
 */
export function draftPathFor(
	storageRoot: string,
	root: string,
	itemRef: string,
): string {
	return joinDraftPath(
		storageRoot,
		DRAFTS_DIRNAME,
		fingerprint(root),
		draftFilename(itemRef),
	);
}

/** What Comment needs in order to compose a draft for one item. */
export interface DraftTarget {
	/** The workbench the post will be pinned to. */
	readonly root: string;
	/** The workspace folder the post's checkpoint will run against. */
	readonly folder: string;
	/** The item's own reference, which the argv and the messages both name. */
	readonly ref: string;
}

/**
 * Opens a draft for one item, and spawns nothing.
 *
 * The existence test reads the disk, because the file is the fact and the
 * index is bookkeeping. Deciding from the index alone destroys unsent prose
 * whenever the two disagree in the file-present, entry-absent direction, and
 * that direction is reachable: two windows open on the same workbench differ
 * in exactly it unless a Memento write in one is visible in the other, which
 * no VS Code documentation promises. This project rests no correctness
 * argument on undocumented behaviour of an external system, so the question is
 * dissolved rather than answered.
 *
 * The two disagreements that remain are both harmless. A file with no entry is
 * adopted, which writes the missing entry and opens what is there. An entry
 * with no file is the ordinary state after somebody deletes a draft by hand,
 * and it is written over with a fresh entry for the same invocation, which is
 * the entry that would have been written anyway.
 *
 * A new buffer is created empty. No template, no header line and no comment
 * marker, because anything written into the buffer is text the author has to
 * delete and text the post would otherwise send. The reader knows what they
 * are writing into from the tab's name, which carries the item's reference,
 * and from the message.
 */
export async function openCommentDraft(
	host: DraftHost,
	target: DraftTarget,
): Promise<void> {
	const directory = joinDraftPath(
		host.storageRoot,
		DRAFTS_DIRNAME,
		fingerprint(target.root),
	);
	const path = joinDraftPath(directory, draftFilename(target.ref));
	try {
		await host.ensureDirectory(directory);
	} catch (err) {
		host.showError(
			host.t("draft.storageUnavailable", {
				path: directory,
				detail: err instanceof Error ? err.message : String(err),
			}),
		);
		return;
	}
	const entry: DraftEntry = {
		root: target.root,
		folder: target.folder,
		// The entry records the invocation rather than the item, so the post
		// command never learns what a comment is. A later card that composes
		// an item's own text the same way writes a file invocation here and
		// adds no mechanism, because runFile reads a bare dash from stdin
		// exactly as runComment does.
		argv: ["comment", target.ref, "-"],
		target: target.ref,
	};
	const existing = await host.readDraft(path);
	if (existing !== undefined) {
		const index = host.readIndex();
		if (index[path] === undefined) {
			await host.writeIndex({ ...index, [path]: entry });
		}
		await host.openDocument(path);
		host.showInfo(host.t("draft.reopened", { ref: target.ref, path }));
		return;
	}
	await host.writeIndex({ ...host.readIndex(), [path]: entry });
	await host.writeDraft(path, "");
	await host.openDocument(path);
	host.showInfo(host.t("draft.created", { ref: target.ref, path }));
}

/**
 * Posts the draft at one path, and deletes it only when the tool said ok.
 *
 * The document is saved before it is read, which is what makes the bytes on
 * disk equal the bytes that were posted and is the whole of the recovery
 * story: a post refused after three paragraphs leaves those three paragraphs
 * on disk and the tab still open, and the fix is to press the same button
 * again once the refusal is dealt with.
 *
 * The bytes are sent verbatim. No trim, no normalisation of line endings and
 * no stripping of a trailing newline: runComment stores what arrives and the
 * store keeps what it is given, so the only way the comment reads as its
 * author wrote it is to send what its author wrote.
 *
 * Three branches end the run having sent nothing, and none of them removes
 * anything, the index entry included. The post is not the housekeeper: a
 * command that sent nothing removes nothing, and the one place a fileless
 * entry is reaped is the activation sweep below.
 */
export async function postCommentDraft(
	host: DraftHost,
	spawner: Spawner,
	exe: string,
	path: string,
): Promise<void> {
	if (!(await host.saveDocument(path))) {
		host.showError(host.t("draft.post.saveFailed", { path }));
		return;
	}
	// TextDocument.save() is documented to report whether the save happened
	// and is not documented to recreate a file somebody deleted, so a true
	// from it is not read as proof that the file is on disk.
	const text = await host.readDraft(path);
	if (text === undefined) {
		host.showError(host.t("draft.post.missingFile", { path }));
		return;
	}
	if (text.trim() === "") {
		host.showError(host.t("draft.post.empty", { path }));
		return;
	}
	const entry = host.readIndex()[path];
	if (entry === undefined) {
		host.showError(host.t("draft.unknown", { path }));
		return;
	}
	const context: VerbContext = {
		spawner,
		exe,
		host,
		folder: entry.folder,
		root: entry.root,
		ref: entry.target,
	};
	const outcome = await runVerb(context, entry.argv, text);
	if (outcome.kind !== "ok") {
		// runVerb has already shown the refusal through the ordinary path, so
		// what is left to say is that nothing was lost. It is an info rather
		// than a second error, because the error the reader has to act on is
		// the refusal and a second red toast would compete with it.
		host.showInfo(host.t("draft.post.keptAfterRefusal", { path }));
		return;
	}
	try {
		await host.deleteDraft(path);
	} catch (err) {
		// The comment exists at this point, so a message saying the post
		// failed would be false. The entry is dropped either way, because the
		// invocation it records has been carried out and a second press must
		// not repeat it.
		await dropEntry(host, path);
		host.showInfo(
			host.t("draft.post.postedButNotRemoved", {
				ref: entry.target,
				path,
				detail: err instanceof Error ? err.message : String(err),
			}),
		);
		return;
	}
	await dropEntry(host, path);
	host.showInfo(host.t("draft.post.posted", { ref: entry.target }));
}

/**
 * Throws one draft away, after the index has owned it and the reader has
 * confirmed, in that order.
 *
 * A post the tool answered ok and this command are the only two things that
 * delete a draft. Not a closed tab, not a window reload, not a refusal, not a
 * timeout, not a spawn failure and not an age. An author saves to make their
 * words safe, which is what a save means everywhere else in the editor, so no
 * save handler, no watcher and no timer posts or removes anything here.
 *
 * The index lookup is what keeps this command off a file the extension never
 * created, and it is the same lookup postCommentDraft makes for the same
 * reason. The manifest offers both commands on any file whose name ends in
 * DRAFT_SUFFIX, wherever that file sits, because no clause the manifest can
 * carry answers the directory question accurately. A resource-scoped test
 * does exist, since VS Code documents resourcePath and resourceDirname as
 * when-clause context keys, but the drafts live under globalStorageUri,
 * whose layout VS Code documents nothing about, so any directory literal
 * written into the manifest would have to be a pattern loose enough to be
 * wrong. The other route is a custom context key set through setContext,
 * which is one value for the whole window rather than one per resource, so
 * the key answers for the active editor and gets a second editor's title bar
 * wrong. So a reader with an unrelated notes.dinah-comment.md open is
 * offered the command, and what happens when they press it is decided here.
 * An earlier form of this function confirmed and then deleted whatever path
 * it was handed, which destroyed that file.
 *
 * The refusal comes before the confirmation rather than after it. A modal
 * asking whether to throw away a file the extension does not own has already
 * told the reader something false about what it is about to do, and the
 * answer does not depend on which button they press.
 */
export async function discardCommentDraft(
	host: DraftHost,
	path: string,
): Promise<void> {
	if (host.readIndex()[path] === undefined) {
		host.showError(host.t("draft.unknown", { path }));
		return;
	}
	const confirmed = await host.confirmDestructive(
		host.t("draft.discard.confirm", { path }),
		host.t("draft.discard.confirmLabel"),
	);
	if (!confirmed) {
		return;
	}
	await host.deleteDraft(path);
	await dropEntry(host, path);
}

/** Removes one path from the index, leaving every other entry where it is. */
async function dropEntry(host: DraftHost, path: string): Promise<void> {
	const index = host.readIndex();
	if (index[path] === undefined) {
		return;
	}
	const remaining: DraftIndex = { ...index };
	delete remaining[path];
	await host.writeIndex(remaining);
}

/**
 * Drops index entries whose files no longer exist, which activation runs.
 *
 * Drafts are never pruned by age. A file holding somebody's unsent argument is
 * not rubbish, and an extension that tidies it away on a timer has taken a
 * decision that is not its own. This sweep drops entries and never files,
 * which is the same direction openCommentDraft runs in: a file is never
 * removed on the strength of the index saying nothing about it.
 */
export async function sweepDraftIndex(host: DraftHost): Promise<number> {
	const index = host.readIndex();
	const remaining: DraftIndex = {};
	let dropped = 0;
	for (const [path, entry] of Object.entries(index)) {
		if ((await host.readDraft(path)) === undefined) {
			dropped += 1;
			host.log(`comment draft index: dropping the entry for ${path}`);
			continue;
		}
		remaining[path] = entry;
	}
	if (dropped > 0) {
		await host.writeIndex(remaining);
	}
	return dropped;
}
