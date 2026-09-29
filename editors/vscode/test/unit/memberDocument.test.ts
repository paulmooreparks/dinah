// dinah-637/criteria/18: a comment and an item open as dinah-member documents
// read through `dinah show` and written through the verbs, the extension
// drives storage format 12, and its watcher watches the journals.
//
// Everything here drives the pure modules over a recording spawner, on the
// terms every other unit case in this directory takes: the file system
// provider extension.ts registers hands its reads to readMember and its saves
// to saveCommentBody and saveItemDocument, and those are what is asserted.

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import test from "node:test";

import { WATCHED_FILES } from "../../src/changes";
import type { Spawner, SpawnOutcome } from "../../src/cli";
import type { CommentBodyHost, OpenComments } from "../../src/commentBody";
import { openExistingComment, recordedDigest, saveCommentBody } from "../../src/commentBody";
import { ENGLISH } from "../../src/l10n";
import type { OpenItems } from "../../src/memberDocument";
import {
	MEMBER_SCHEME,
	memberAddress,
	memberKey,
	memberLocation,
	openItemDocument,
	saveItemDocument,
} from "../../src/memberDocument";
import { SUPPORTED_FORMATS } from "../../src/version";

const ROOT = "C:/work/bench with space";
const FOLDER = "C:/work";
const COMMENT = "wb-1/questions/1/comments/2";
const ITEM = "wb-1/questions/1";
const BODY = "The words as they stood.\n";
// The digest the anchor records is the one the tool computes over the body,
// so the comment opens undiverged and its session is adopted.
const DIGEST = createHash("sha256").update(BODY).digest("hex");
const COMMENT_ANCHOR = `${["---", "ts: 2026-08-01T09:00:00Z", "author: ana", "ordinal: 2", `digest: ${DIGEST}`, "---"].join("\n")}\n${BODY}`;
const ITEM_ANCHOR = ["---", "kind: open_question", "state: pending", "owner: operator", "---", "Which way do we go?", ""].join("\n");

/** Everything the fake host was asked to do. */
interface Log {
	readonly opened: string[];
	readonly errors: string[];
}

function host(log: Log): CommentBodyHost {
	return {
		t: ENGLISH,
		readFile: async () => undefined,
		openDocument: async (location: string) => {
			log.opened.push(location);
		},
		showError: (message: string) => {
			log.errors.push(message);
		},
		showInfo: () => undefined,
		appendLines: () => undefined,
		checkpoint: async () => undefined,
		log: () => undefined,
	};
}

/**
 * A spawner that records every invocation, answers `show` as the binary does
 * (a comment's anchor as raw text, an item's detail as JSON), and answers a
 * write ok.
 */
function recorder(): { spawner: Spawner; calls: { argv: string[]; stdin?: string }[] } {
	const calls: { argv: string[]; stdin?: string }[] = [];
	const spawner: Spawner = async (_exe, argv, options): Promise<SpawnOutcome> => {
		calls.push({ argv: [...argv], stdin: options?.stdin });
		const shown = argv.indexOf("show");
		if (shown >= 0) {
			const ref = argv[shown + 1];
			const stdout = ref === COMMENT ? COMMENT_ANCHOR : JSON.stringify({ ref: ITEM, text: ITEM_ANCHOR });
			return { code: 0, stdout, stderr: "" };
		}
		return { code: 0, stdout: JSON.stringify({ outcome: "ok", verb: "set" }), stderr: "" };
	};
	return { spawner, calls };
}

test("a member location spells the reference as its path and the workbench in its query", () => {
	const location = memberLocation({ ref: COMMENT, root: ROOT });
	assert.ok(location.startsWith(`${MEMBER_SCHEME}:/${COMMENT}.md?root=`), location);
	const query = location.slice(location.indexOf("?") + 1);
	const path = location.slice(`${MEMBER_SCHEME}:`.length, location.indexOf("?"));
	assert.deepEqual(memberAddress(path, query), { ref: COMMENT, root: ROOT });
	assert.equal(memberAddress("/no-extension", query), undefined);
	assert.equal(memberAddress(`/${COMMENT}.md`, ""), undefined);
});

test("a comment opened through the dinah-member provider saves through set body with the digest it opened with", async () => {
	const log: Log = { opened: [], errors: [] };
	const { spawner, calls } = recorder();
	const opened: OpenComments = new Map();

	await openExistingComment(host(log), spawner, "dinah", opened, { ref: COMMENT, root: ROOT, folder: FOLDER });
	assert.deepEqual(log.errors, []);
	assert.deepEqual(log.opened, [memberLocation({ ref: COMMENT, root: ROOT })]);
	assert.ok(calls[0]?.argv.includes("show") && calls[0]?.argv.includes(COMMENT), `the read was ${calls[0]?.argv.join(" ")}`);
	assert.equal(calls.some((call) => call.argv.includes("path")), false, "opening a comment asked dinah path");
	assert.equal(recordedDigest(COMMENT_ANCHOR), DIGEST);

	// The author edits the document and saves it, which is what the provider's
	// writeFile hands to saveCommentBody under the member's key.
	const edited = COMMENT_ANCHOR.replace("The words as they stood.", "The words as the author left them.");
	const saved = await saveCommentBody(host(log), spawner, "dinah", opened, memberKey({ ref: COMMENT, root: ROOT }), edited);
	assert.equal(saved, true, log.errors.join("\n"));
	const write = calls[calls.length - 1];
	assert.deepEqual(write?.argv, ["--json", "--workbench", ROOT, "set", COMMENT, "body", "-", "--expect-digest", DIGEST]);
	assert.equal(write?.stdin, "The words as the author left them.\n");
});

test("an item opened through the dinah-member provider saves its text and refuses a changed field", async () => {
	const log: Log = { opened: [], errors: [] };
	const { spawner, calls } = recorder();
	const opened: OpenItems = new Map();
	const address = { ref: ITEM, root: ROOT };

	await openItemDocument(host(log), spawner, "dinah", opened, address, FOLDER);
	assert.deepEqual(log.opened, [memberLocation(address)]);

	const retexted = ITEM_ANCHOR.replace("Which way do we go?", "Which way, and by when?");
	assert.equal(await saveItemDocument(host(log), spawner, "dinah", opened, address, retexted), true, log.errors.join("\n"));
	const write = calls[calls.length - 1];
	assert.deepEqual(write?.argv, ["--json", "--workbench", ROOT, "set", ITEM, "text", "-"]);
	assert.equal(write?.stdin, "Which way, and by when?\n");

	const before = calls.length;
	const restated = ITEM_ANCHOR.replace("state: pending", "state: resolved");
	assert.equal(await saveItemDocument(host(log), spawner, "dinah", opened, address, restated), false);
	assert.equal(calls.length, before, "a save changing the item's front matter reached the binary");
	assert.equal(log.errors.length, 1);
	assert.match(log.errors[0] ?? "", /resolve, verify, fail, waive, withdraw, reopen/);
});

/** Whether a path matches a glob of the one shape WATCHED_FILES takes. */
function matchesWatched(path: string): boolean {
	const braces = /^\*\*\/\{(.*)\}$/.exec(WATCHED_FILES);
	assert.notEqual(braces, null, `WATCHED_FILES is ${WATCHED_FILES}, not the shape this reads`);
	const name = path.split("/").pop() ?? "";
	return (braces?.[1] ?? "").split(",").some((alternative) => {
		const pattern = new RegExp(`^${alternative.replace(/\./g, "\\.").replace(/\*/g, ".*")}$`);
		return pattern.test(name);
	});
}

test("the watcher watches every journal beside every anchor, and storage format 12 is driven", () => {
	assert.equal(matchesWatched("cards/aa/journal.ndjson"), true);
	assert.equal(matchesWatched("journal.ndjson"), true);
	assert.equal(matchesWatched("cards/aa/card.md"), true);
	assert.equal(matchesWatched("cards/aa/attachments/bb/payload/notes.txt"), false);
	assert.ok(SUPPORTED_FORMATS.includes(12), `SUPPORTED_FORMATS is ${SUPPORTED_FORMATS.join(", ")}`);
	assert.ok(SUPPORTED_FORMATS.includes(11), "a binary at format 11 is no longer driven");
});
