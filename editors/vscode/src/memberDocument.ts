// The document a comment or a checklist item opens as.
//
// From storage format 12 a comment and an item are lines of their card's
// journal and have no file of their own, so `dinah path` refuses them. The
// extension opens each as a document of its own scheme instead, `dinah-member`,
// whose URI names the member's reference and the workbench it stands in:
// `dinah-member:/<ref>.md?root=<URL-encoded workbench root>`. Reading the
// document runs `dinah show <ref>`, which answers the anchor the older layout
// stored, composed from the journal, and saving it runs the verb that writes
// the member. The same route serves a store below format 12, where `show`
// prints the member's own file, so nothing here asks which layout it meets.
//
// A comment's save is a compare-and-swap on the digest its anchor carried when
// the document was opened, which is the mechanism commentBody.ts describes.
// An item's save writes changed text through `set <ref> text`; the operator
// ruled on dinah-637/questions/2 that a save whose front matter differs from
// what was opened is refused, naming the verbs that change those fields, since
// a document edit is not how an item's state or owner moves.
//
// Nothing here imports vscode, on the terms cardCommands.ts's header gives;
// extension.ts registers the file system provider over these functions.

import type { VerbContext } from "./cardCommands";
import { pinnedArgv, refusalMessage, runVerb } from "./cardCommands";
import type { Spawner } from "./cli";
import { runDinahText } from "./cli";
import type { CommentBodyHost } from "./commentBody";
import { splitAnchorBody } from "./commentBody";

/** The URI scheme a comment or an item opens under. */
export const MEMBER_SCHEME = "dinah-member";

/** The member a document names and the workbench it stands in. */
export interface MemberAddress {
	readonly ref: string;
	readonly root: string;
}

/**
 * The location a member opens at, spelled as the URI the provider serves.
 *
 * The reference is the path, so a tab's title reads as the reference, and the
 * workbench root travels in the query, URL-encoded, because a root is a
 * filesystem path and carries characters a URI path would reinterpret.
 */
export function memberLocation(address: MemberAddress): string {
	return `${MEMBER_SCHEME}:/${address.ref}.md?root=${encodeURIComponent(address.root)}`;
}

/**
 * The member a URI's path and query name, or undefined where they name none.
 * The two parts are taken apart rather than the whole string, because a URI
 * library hands them over already split and decoded differently.
 */
export function memberAddress(path: string, query: string): MemberAddress | undefined {
	const trimmed = path.replace(/^\/+/, "");
	if (!trimmed.endsWith(".md")) {
		return undefined;
	}
	const ref = trimmed.slice(0, -".md".length);
	const encoded = new URLSearchParams(query).get("root");
	if (ref === "" || encoded === null || encoded === "") {
		return undefined;
	}
	return { ref, root: encoded };
}

/** The key an open member is remembered under. */
export function memberKey(address: MemberAddress): string {
	return `${address.root}\n${address.ref}`;
}

/** What reading a member came to. */
export type MemberRead =
	| { readonly kind: "ok"; readonly text: string }
	| { readonly kind: "refused"; readonly message: string };

/**
 * Reads a member's document text through `dinah show`. A comment's answer is
 * its composed anchor as the command prints it, which opens with a front
 * matter fence; an item's is a JSON object whose text member is its anchor.
 * The two are told apart by what came back rather than by the reference,
 * because the identifier `dinah comment` answers a new comment with does not
 * say it is a comment.
 */
export async function readMember(
	spawner: Spawner,
	exe: string,
	address: MemberAddress,
): Promise<MemberRead> {
	const argv = pinnedArgv(address.root, ["show", address.ref]);
	const shown = await runDinahText(spawner, exe, argv, { cwd: address.root });
	if (shown.kind !== "ok") {
		return { kind: "refused", message: refusalMessage(shown) };
	}
	if (!shown.text.trimStart().startsWith("{")) {
		return { kind: "ok", text: shown.text };
	}
	try {
		const text = (JSON.parse(shown.text) as { text?: unknown }).text;
		return { kind: "ok", text: typeof text === "string" ? text : "" };
	} catch {
		return { kind: "ok", text: shown.text };
	}
}

/**
 * The front matter of an anchor's text: everything before the body
 * splitAnchorBody cuts off, which is empty for a text that opens with no
 * fence.
 */
export function anchorHeader(text: string): string {
	const body = splitAnchorBody(text);
	return text.slice(0, text.length - body.length);
}

/** Where one open item stands, so a save of its document can reach the verb. */
export interface OpenItem {
	readonly address: MemberAddress;
	/** The workspace folder the write's checkpoint runs against. */
	readonly folder: string;
	/** The front matter the document carried when it was opened. */
	readonly header: string;
}

/** The items open in this window, keyed by memberKey. */
export type OpenItems = Map<string, OpenItem>;

/**
 * Opens an item's document and remembers the front matter it opened with, so
 * a save can tell a change of the text, which it writes, from a change of a
 * field, which it refuses.
 */
export async function openItemDocument(
	host: CommentBodyHost,
	spawner: Spawner,
	exe: string,
	opened: OpenItems,
	address: MemberAddress,
	folder: string,
): Promise<void> {
	const read = await readMember(spawner, exe, address);
	if (read.kind !== "ok") {
		host.showError(read.message);
		return;
	}
	opened.set(memberKey(address), { address, folder, header: anchorHeader(read.text) });
	await host.openDocument(memberLocation(address));
}

/**
 * Writes a saved item document through the verb: its text through
 * `set <ref> text -`, and nothing where the front matter differs from what was
 * opened, which is refused naming the verbs that change an item's fields. A
 * document this window did not open is left alone and answers false.
 */
export async function saveItemDocument(
	host: CommentBodyHost,
	spawner: Spawner,
	exe: string,
	opened: OpenItems,
	address: MemberAddress,
	text: string,
): Promise<boolean> {
	const standing = opened.get(memberKey(address));
	if (standing === undefined) {
		return false;
	}
	if (anchorHeader(text) !== standing.header) {
		host.showError(host.t("member.itemFieldsChanged", { ref: address.ref }));
		return false;
	}
	const context: VerbContext = {
		spawner,
		exe,
		host,
		folder: standing.folder,
		root: address.root,
		ref: address.ref,
	};
	const written = await runVerb(context, ["set", address.ref, "text", "-"], splitAnchorBody(text));
	return written.kind === "ok";
}
