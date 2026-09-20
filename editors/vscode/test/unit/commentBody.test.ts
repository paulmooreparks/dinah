// dinah-525/criteria/12, the extension's own arm: creating a comment with the
// empty form and opening its file, saving through the verb, drawing the
// designated comment on a checklist row, and abandoning the draft.
//
// Everything here drives the pure modules over a recording spawner. No VS Code
// window is built, on the terms every other unit case in this directory takes:
// the thing under test is the argv a command composes and what it does with
// the answer, and both are visible without one.

import assert from "node:assert/strict";
import test from "node:test";

import type { Spawner, SpawnOutcome } from "../../src/cli";
import type { OpenComments } from "../../src/commentBody";
import {
	composeComment,
	forgetComment,
	noteOpenComment,
	saveCommentBody,
	splitAnchorBody,
} from "../../src/commentBody";
import { ENGLISH } from "../../src/l10n";
import { itemTooltip } from "../../src/tree";
import type { ItemView } from "../../src/wire";

const ROOT = "C:/bench";
const FOLDER = "C:/work";
const COMMENT_PATH = "C:/bench/cards/c1/comments/a1/comment.md";

/** Everything the fake host was asked to do. */
interface Log {
	readonly opened: string[];
	readonly errors: string[];
	readonly infos: string[];
	readonly checkpoints: string[];
	readonly logged: string[];
}

function emptyLog(): Log {
	return { opened: [], errors: [], infos: [], checkpoints: [], logged: [] };
}

function host(log: Log) {
	return {
		t: ENGLISH,
		openDocument: async (path: string) => {
			log.opened.push(path);
		},
		showError: (message: string) => {
			log.errors.push(message);
		},
		showInfo: (message: string) => {
			log.infos.push(message);
		},
		appendLines: () => undefined,
		checkpoint: async (folder: string) => {
			log.checkpoints.push(folder);
		},
		log: (line: string) => {
			log.logged.push(line);
		},
	};
}

/**
 * A spawner that records every invocation and answers the two calls composing
 * a comment makes.
 *
 * The stdin is recorded beside the argv, because the save path's whole claim
 * is that the body reaches the verb down a pipe rather than being left on
 * disk, and an argv alone cannot answer that.
 */
function recorder(): {
	spawner: Spawner;
	calls: { argv: string[]; stdin?: string }[];
} {
	const calls: { argv: string[]; stdin?: string }[] = [];
	const spawner: Spawner = async (_exe, argv, options): Promise<SpawnOutcome> => {
		calls.push({ argv: [...argv], stdin: options?.stdin });
		const stdout = argv.includes("path")
			? JSON.stringify({ path: COMMENT_PATH })
			: JSON.stringify({ outcome: "ok", verb: "comment", detail: "a00000000001" });
		return { code: 0, stdout, stderr: "" };
	};
	return { spawner, calls };
}

test("the empty form mints the comment and the extension opens its file", async () => {
	const log = emptyLog();
	const { spawner, calls } = recorder();
	const opened: OpenComments = new Map();

	await composeComment(host(log), spawner, "dinah", opened, {
		root: ROOT,
		folder: FOLDER,
		ref: "wb-1",
	});

	// The comment verb, carrying no text. A dash in that slot is the form
	// that reads a body from a pipe, and sending one would make the extension
	// compose the comment rather than the author.
	const minted = calls[0]?.argv ?? [];
	assert.ok(minted.includes("comment"), `the first call is ${minted.join(" ")}`);
	assert.ok(minted.includes("wb-1"), "the comment verb was not aimed at the holder");
	assert.equal(minted.includes("-"), false, "the comment verb was handed a dash");
	assert.equal(calls[0]?.stdin, undefined, "the comment verb was handed a pipe");

	// Then the file, asked for by the identifier the answer carried.
	const located = calls[1]?.argv ?? [];
	assert.ok(located.includes("path"), `the second call is ${located.join(" ")}`);
	assert.ok(located.includes("a00000000001"), "the path call named some other comment");
	assert.deepEqual(log.opened, [COMMENT_PATH]);
	assert.equal(log.errors.length, 0, log.errors.join("\n"));

	// And the extension remembers it, which is what makes a later save of the
	// tab reach the verb rather than leaving the editor's own bytes on disk.
	assert.equal(opened.get(COMMENT_PATH)?.ref, "a00000000001");
	assert.equal(opened.get(COMMENT_PATH)?.root, ROOT);
});

test("saving a comment's file writes its body through the verb", async () => {
	const log = emptyLog();
	const { spawner, calls } = recorder();
	const opened: OpenComments = new Map();
	noteOpenComment(opened, COMMENT_PATH, {
		root: ROOT,
		folder: FOLDER,
		ref: "wb-1/comments/1",
	});

	const saved = await saveCommentBody(
		host(log),
		spawner,
		"dinah",
		opened,
		COMMENT_PATH,
		"---\nts: 2026-08-01T09:00:00Z\nauthor: ana\n---\nThe body somebody typed.\n",
	);

	assert.equal(saved, true, "the save reported that it wrote nothing");
	const written = calls[0]?.argv ?? [];
	assert.ok(written.includes("set"), `the call is ${written.join(" ")}`);
	assert.ok(written.includes("wb-1/comments/1"), "the write named some other comment");
	assert.ok(written.includes("body"), "the write named some other field");
	// The body alone, down a pipe. The front matter is the tool's to write,
	// and sending the whole file would store the header as the comment.
	assert.equal(calls[0]?.stdin, "The body somebody typed.\n");
});

test("a file this window opened no comment into is left alone", async () => {
	const log = emptyLog();
	const { spawner, calls } = recorder();
	const opened: OpenComments = new Map();

	const saved = await saveCommentBody(
		host(log),
		spawner,
		"dinah",
		opened,
		"C:/work/README.md",
		"# A readme\n",
	);

	assert.equal(saved, false, "a file the extension never opened was written through the verb");
	assert.deepEqual(calls, [], "saving an unrelated file spawned dinah");
});

test("closing a comment's tab forgets it, so a later save of that path writes nothing", async () => {
	const log = emptyLog();
	const { spawner, calls } = recorder();
	const opened: OpenComments = new Map();
	noteOpenComment(opened, COMMENT_PATH, {
		root: ROOT,
		folder: FOLDER,
		ref: "wb-1/comments/1",
	});
	forgetComment(opened, COMMENT_PATH);

	const saved = await saveCommentBody(
		host(log),
		spawner,
		"dinah",
		opened,
		COMMENT_PATH,
		"---\nauthor: ana\n---\nwords\n",
	);
	assert.equal(saved, false);
	assert.deepEqual(calls, []);
});

test("the body is cut on the closing fence, not by counting lines", () => {
	// A front matter block holds as many keys as the entity has, and a
	// sequence value carries more lines still, so a cut that counted lines
	// would send part of the header as the comment.
	assert.equal(
		splitAnchorBody("---\nts: t\nauthor: ana\nordinal: 1\ndigest: ff\n---\nthe words\n"),
		"the words\n",
	);
	// A body carrying its own fence keeps it: the cut is the first closing
	// fence, and everything after it is the comment.
	assert.equal(
		splitAnchorBody("---\nauthor: ana\n---\nabove\n---\nbelow\n"),
		"above\n---\nbelow\n",
	);
	// A file somebody emptied is all body, which is what a reader would mean
	// by it and what an empty comment's file looks like once its header goes.
	assert.equal(splitAnchorBody("just words\n"), "just words\n");
	assert.equal(splitAnchorBody(""), "");
	// Carriage returns survive, because the store keeps what it is given and
	// the only way a comment reads as its author wrote it is to send what its
	// author wrote.
	assert.equal(
		splitAnchorBody("---\r\nauthor: ana\r\n---\r\nthe words\r\n"),
		"the words\r\n",
	);
});

/** One checklist item view, with whatever the case under test carries. */
function itemView(extra: Partial<ItemView>): ItemView {
	return {
		id: "b00000000001",
		ordinal: 1,
		ref: "wb-1/questions/1",
		kind: "open_question",
		state: "resolved",
		text: "Does the deadline move?",
		...extra,
	};
}

test("a settled item's row draws the answer it designates", () => {
	// The full read carried the comment, so the row draws the words.
	const whole = itemTooltip(
		itemView({
			resolution: "wb-1/questions/1/comments/1",
			designated: {
				ref: "wb-1/questions/1/comments/1",
				author: "ana",
				body: "the operator ruled it does not",
			},
		}),
		"nothing",
		"Doing",
		false,
		ENGLISH,
	);
	assert.match(whole, /ana: the operator ruled it does not/);

	// The indexed read carried the reference alone, so the row says an answer
	// exists and how to reach it, and no comment was opened to draw it.
	const indexed = itemTooltip(
		itemView({ resolution: "wb-1/questions/1/comments/1" }),
		"nothing",
		"Doing",
		false,
		ENGLISH,
	);
	assert.match(indexed, /wb-1\/questions\/1\/comments\/1/);

	// A comment the store cannot attribute is drawn as one the store cannot
	// name rather than as a blank or as an invented author.
	const authorless = itemTooltip(
		itemView({
			resolution: "wb-1/questions/1/comments/1",
			designated: {
				ref: "wb-1/questions/1/comments/1",
				author_unrecoverable: true,
				body: "the answer somebody wrote",
			},
		}),
		"nothing",
		"Doing",
		false,
		ENGLISH,
	);
	assert.match(authorless, /an author this workbench cannot name: the answer somebody wrote/);

	// A pending item designates nothing and its row says nothing about an
	// answer, which is the control the three rows above rest on.
	const pending = itemTooltip(
		itemView({ state: "pending" }),
		"nothing",
		"Doing",
		false,
		ENGLISH,
	);
	assert.equal(
		/Answer:/.test(pending),
		false,
		`a pending item's row claims an answer:\n${pending}`,
	);
});
