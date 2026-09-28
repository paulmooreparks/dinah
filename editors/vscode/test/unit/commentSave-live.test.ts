// dinah-525/criteria/21: saving a comment from the extension works, driven
// against a real dinah and a real store.
//
// The unit file beside this one asserts the argv the save composes, and that
// is exactly the shape that let a save path refusing every write ship green:
// the argv was right and the answer was a refusal. What decides whether the
// save works is what the verb says back and what the store holds afterwards,
// and neither is visible to a mocked spawner. So this file builds a binary,
// makes a workbench, mints an empty comment through the extension's own
// compose path, hands the real save handler the document the dinah-member
// provider serves with the author's text below it, as a save of the tab does,
// and reads the store.
//
// This unit file starts a process, and test/unit/layers.test.ts carries the
// exemption with the reason. The cost is one `go build`, shared across the
// file, and a handful of short-lived spawns.

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { after, test } from "node:test";

import type { Spawner } from "../../src/cli";
import { runDinah } from "../../src/cli";
import type { CommentBodyHost, OpenComments } from "../../src/commentBody";
import {
	composeComment,
	recordedDigest,
	saveCommentBody,
	splitAnchorBody,
} from "../../src/commentBody";
import { ENGLISH } from "../../src/l10n";
import { readMember } from "../../src/memberDocument";
import { nodeSpawner } from "../../src/spawn";
import type { FixtureRoot } from "../support/fixtures";
import { buildBinary, fixtureEnv, initBench } from "../support/fixtures";

const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");

/** The binary every test here reads, built once. */
let built: FixtureRoot | undefined;

function binary(): FixtureRoot {
	built ??= buildBinary(repoRoot);
	return built;
}

after(() => {
	if (built !== undefined) {
		rmSync(built.tempRoot, { recursive: true, force: true });
	}
});

/** Everything the fake window was asked to do. */
interface Log {
	readonly opened: string[];
	readonly errors: string[];
	readonly infos: string[];
}

function host(log: Log): CommentBodyHost {
	return {
		t: ENGLISH,
		readFile: async (path) => {
			try {
				return readFileSync(path, "utf8");
			} catch {
				return undefined;
			}
		},
		openDocument: async (path) => {
			log.opened.push(path);
		},
		showError: (message) => {
			log.errors.push(message);
		},
		showInfo: (message) => {
			log.infos.push(message);
		},
		appendLines: () => undefined,
		checkpoint: async () => undefined,
		log: () => undefined,
	};
}

/**
 * A workbench with one card, and an empty comment on it composed through the
 * extension's own path.
 *
 * Composing rather than planting, because the path under test begins at the
 * open: what the save hands the verb is the digest composeComment remembered,
 * and a planted session would be the test choosing that value for itself.
 */
function fixture(t: { name: string }): {
	root: FixtureRoot;
	bench: string;
	path: string;
	opened: OpenComments;
	log: Log;
} {
	const root = binary();
	const holder = join(root.tempRoot, t.name.replace(/[^a-z0-9]+/gi, "-"));
	initBench(root, holder, "fx");
	// `dinah init` puts the workbench in a .dinah container under the
	// directory it was run in, and every call below pins itself with
	// --workbench, so what is wanted is the workbench's own directory rather
	// than the one holding it.
	const container = join(holder, ".dinah");
	const minted = readdirSync(container);
	assert.equal(minted.length, 1, `the container holds ${String(minted.length)} workbenches`);
	const bench = join(container, minted[0]);
	run(root, bench, ["add", "A card"]);

	const log: Log = { opened: [], errors: [], infos: [] };
	const opened: OpenComments = new Map();
	return { root, bench, path: "", opened, log };
}

/**
 * nodeSpawner with the fixture's own environment underneath it.
 *
 * The bare spawner inherits whatever environment the run was started in, so
 * these cases passed on a machine that already had a Dinah identity configured
 * and were refused on one that did not. That is green-on-your-machine, and the
 * two calls into the extension's own code are the ones that reach the binary
 * without going through `run` and its fixtureEnv.
 */
function fixtureSpawner(root: FixtureRoot): Spawner {
	return (exe, argv, options) =>
		nodeSpawner(exe, argv, { ...options, env: fixtureEnv(root) });
}

/** Runs one invocation against a fixture workbench and fails on a refusal. */
function run(root: FixtureRoot, bench: string, argv: readonly string[]): void {
	execFileSync(root.binary, ["--json", "--workbench", bench, ...argv], {
		cwd: bench,
		env: fixtureEnv(root),
		stdio: "pipe",
	});
}

/** The comment's anchor as it stands on disk. */
function anchor(path: string): string {
	return readFileSync(path, "utf8");
}

/**
 * A comment's anchor as the dinah-member provider serves it, which is what
 * `dinah show` prints for it.
 */
async function shownAnchor(root: FixtureRoot, bench: string, ref: string): Promise<string> {
	const read = await readMember(fixtureSpawner(root), root.binary, { ref, root: bench });
	assert.equal(read.kind, "ok", `show ${ref} was refused: ${JSON.stringify(read)}`);
	return read.kind === "ok" ? read.text : "";
}

/**
 * Every event name the card's journal carries, in order.
 *
 * The file is read rather than asked for. Dinah publishes no verb that prints
 * a card's journal, and the journal is a line-delimited file the format
 * documents, so reading it is the honest route rather than a shortcut.
 */
async function journalEvents(
	root: FixtureRoot,
	bench: string,
	card: string,
): Promise<string[]> {
	const located = await runDinah(nodeSpawner, root.binary, ["path", card], {
		cwd: bench,
		env: fixtureEnv(root),
	});
	if (located.kind !== "ok") {
		return [];
	}
	const anchor = (located.json as { path?: string }).path ?? "";
	if (anchor === "") {
		return [];
	}
	const journal = join(anchor, "..", "journal.ndjson");
	return readFileSync(journal, "utf8")
		.split("\n")
		.filter((line) => line.trim() !== "")
		.map((line) => (JSON.parse(line) as { event?: string }).event ?? "");
}

test("an edit made in the editor is written through the verb and re-stamped", async (t) => {
	const { root, bench, opened, log } = fixture(t);
	const window = host(log);

	await composeComment(window, fixtureSpawner(root), root.binary, opened, {
		root: bench,
		folder: bench,
		ref: "fx-1",
	});
	assert.deepEqual(log.errors, [], "composing the comment reported an error");
	assert.equal(log.opened.length, 1, "the comment's document was not opened");
	const [key, session] = [...opened.entries()][0] ?? ["", undefined];
	assert.notEqual(session, undefined, "no session was opened over the comment");
	const ref = session?.ref ?? "";
	const before = recordedDigest(await shownAnchor(root, bench, ref));
	assert.notEqual(before, "", "the minted comment carries no digest");
	assert.equal(session?.digest, before, "the session did not remember the digest the anchor records");

	// The document the author saves is the anchor they opened with their text
	// below it, which the file system provider hands the save handler.
	const typed = "The author typed this, in the editor.\n";
	const saved = await saveCommentBody(
		window,
		fixtureSpawner(root),
		root.binary,
		opened,
		key,
		(await shownAnchor(root, bench, ref)) + typed,
	);

	// The answer, which is the whole of what this file exists to assert.
	assert.equal(saved, true, `the save was refused: ${log.errors.join("\n")}`);
	assert.deepEqual(log.errors, [], "the save reported an error");

	// And the store afterwards: the body is what was typed, and the digest
	// describes it rather than the empty body it was minted over.
	const after = await shownAnchor(root, bench, ref);
	assert.equal(splitAnchorBody(after), typed);
	assert.notEqual(
		recordedDigest(after),
		before,
		"the digest was not re-stamped, so dinah check reads the comment as hand-edited",
	);
	const report = await runDinah(nodeSpawner, root.binary, ["check"], {
		cwd: bench,
		env: fixtureEnv(root),
	});
	assert.equal(
		report.kind,
		"ok",
		`a store the extension had just saved into does not check clean: ${JSON.stringify(report)}`,
	);

	// The write is journalled, which is what makes the edit attributed rather
	// than merely present.
	const events = await journalEvents(root, bench, "fx-1");
	assert.ok(
		events.includes("comment_updated"),
		`the journal carries no comment_updated: ${events.join(", ")}`,
	);

	// The session's remembered digest moved with the write, so a second save
	// of the same tab is not refused as one edit stale.
	const again = "The author typed this, and then more.\n";
	assert.equal(
		await saveCommentBody(
			window,
			fixtureSpawner(root),
			root.binary,
			opened,
			key,
			(await shownAnchor(root, bench, ref)) + again,
		),
		true,
		`the second save of the same tab was refused: ${log.errors.join("\n")}`,
	);
	assert.equal(splitAnchorBody(await shownAnchor(root, bench, ref)), typed + again);
});

test("a write landing between the open and the save refuses the save", async (t) => {
	const { root, bench, opened, log } = fixture(t);
	const window = host(log);

	await composeComment(window, fixtureSpawner(root), root.binary, opened, {
		root: bench,
		folder: bench,
		ref: "fx-1",
	});
	const [key, session] = [...opened.entries()][0] ?? ["", undefined];
	const ref = session?.ref ?? "";
	const opening = await shownAnchor(root, bench, ref);

	// Somebody else writes the comment through a verb while the author is
	// still typing, which moves the digest the anchor records away from the
	// one this session remembered.
	run(root, bench, ["set", "fx-1/comments/1", "body", "somebody else's words"]);
	assert.notEqual(
		recordedDigest(await shownAnchor(root, bench, ref)),
		session?.digest,
		"the fixture did not move the recorded digest, so this case proves nothing",
	);

	// Then the author saves the document they opened, with their text below.
	const typed = "What the author was typing.\n";
	const saved = await saveCommentBody(
		window,
		fixtureSpawner(root),
		root.binary,
		opened,
		key,
		opening + typed,
	);

	assert.equal(saved, false, "the save overwrote work the author never saw");
	assert.ok(
		log.errors.some((message) => message.includes("fx-1/comments/1")),
		`the refusal does not name the comment: ${log.errors.join("\n")}`,
	);
	// Nothing was written through a verb, so the record is what the other
	// write made it, and the author's text is still in their open tab.
	assert.ok(
		splitAnchorBody(await shownAnchor(root, bench, ref)).startsWith("somebody else's words"),
		"the refused save overwrote the other write",
	);
});

test("a hand edit outside any editing session is still refused", async (t) => {
	// The property the compare-and-swap must not cost. A session that never
	// opened the comment has no remembered digest to hand over, so the write
	// falls to the verb's own body comparison exactly as before.
	const { root, bench, opened, log } = fixture(t);
	const window = host(log);
	run(root, bench, ["comment", "fx-1", "what the verb wrote"]);
	const located = await runDinah(
		fixtureSpawner(root),
		root.binary,
		["path", "fx-1/comments/1"],
		{ cwd: bench, env: fixtureEnv(root) },
	);
	assert.equal(located.kind, "ok");
	const path = (located.json as { path?: string }).path ?? "";
	assert.notEqual(path, "");

	// Somebody edits the file by hand, with no session open over it.
	writeFileSync(path, anchor(path) + "and a line nobody journalled.\n", "utf8");
	// A session is then noted with no remembered digest, which is what a
	// comment carrying none would give and is the weakest state the save can
	// be asked to act in.
	opened.set(path, { root: bench, folder: bench, ref: "fx-1/comments/1", digest: "" });

	const saved = await saveCommentBody(
		window,
		fixtureSpawner(root),
		root.binary,
		opened,
		path,
		anchor(path),
	);
	assert.equal(saved, false, "a write over a hand edit was absorbed");
	assert.ok(
		log.errors.some((message) => message.includes("fx-1/comments/1")),
		`the refusal does not name the comment: ${log.errors.join("\n")}`,
	);
});
