// dinah-551: Resolve, Verify and Fail from the extension, driven against a real
// dinah and a real store.
//
// items.test.ts asserts the argv closeItem composes against a fake spawner, and
// that test pinned an argv every one of the three verbs refused: a fake
// spawner accepts whatever it is handed, so the refusal never reached it. What
// decides whether closing an item works is what the verb answers and what the
// store holds afterwards. So this file builds a binary, makes a workbench, files
// an open question and two acceptance criteria, runs each closing verb through
// closeItem, and reads the item back from the store.
//
// An answer of ok and a new comment below the item are not enough on their
// own, because both hold on a build that records a failed criterion as
// verified. So each case also asserts the state the item ends in and that its
// resolution designates the comment the verb created.
//
// This unit file starts a process, and test/unit/layers.test.ts carries the
// exemption with the reason. The cost is one `go build`, shared across the
// file, and a handful of short-lived spawns.

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { after, test } from "node:test";

import type { Spawner } from "../../src/cli";
import { runDinah } from "../../src/cli";
import { splitAnchorBody } from "../../src/commentBody";
import type { ItemCommandContext } from "../../src/itemCommands";
import { closeItem, reopenItem } from "../../src/itemCommands";
import { nodeSpawner } from "../../src/spawn";
import type { FixtureRoot } from "../support/fixtures";
import { buildBinary, fixtureEnv, initBench } from "../support/fixtures";
import type { HostLog } from "../support/rows";
import { cardHost, emptyLog } from "../support/rows";

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

/** The card every fixture files, and the three items it carries. */
const CARD = "fx-1";
const QUESTION = "fx-1/questions/1";
const FIRST_CRITERION = "fx-1/criteria/1";
const SECOND_CRITERION = "fx-1/criteria/2";

/**
 * A workbench with one card carrying an open question and two acceptance
 * criteria, all pending.
 */
function fixture(t: { name: string }): { root: FixtureRoot; bench: string } {
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
	run(root, bench, ["file", CARD, "open_question", "Is this settled?"]);
	run(root, bench, ["file", CARD, "acceptance_criterion", "The first thing holds."]);
	run(root, bench, ["file", CARD, "acceptance_criterion", "The second thing holds."]);
	return { root, bench };
}

/**
 * nodeSpawner with the fixture's own environment underneath it.
 *
 * The bare spawner inherits whatever environment the run was started in, so a
 * case handing it to extension code passes on a machine that already has a
 * Dinah identity configured and is refused on one that does not. Every spawner
 * this file hands to closeItem or reopenItem is this one.
 */
function fixtureSpawner(root: FixtureRoot): Spawner {
	return (exe, argv, options) =>
		nodeSpawner(exe, argv, { ...options, env: fixtureEnv(root) });
}

/** Runs one invocation against a fixture workbench and fails on a refusal. */
function run(root: FixtureRoot, bench: string, argv: readonly string[]): void {
	execFileSync(root.binary, ["--json", ...argv], {
		cwd: bench,
		env: fixtureEnv(root),
		stdio: "pipe",
	});
}

/** Asks the binary one read-only question and answers its JSON. */
async function ask(
	root: FixtureRoot,
	bench: string,
	argv: readonly string[],
): Promise<unknown> {
	const outcome = await runDinah(nodeSpawner, root.binary, ["--workbench", bench, ...argv], {
		cwd: bench,
		env: fixtureEnv(root),
	});
	assert.equal(outcome.kind, "ok", `dinah ${argv.join(" ")} was refused: ${JSON.stringify(outcome)}`);
	return outcome.kind === "ok" ? outcome.json : undefined;
}

/** The anchor file one reference resolves to, as the binary locates it. */
async function anchorPath(root: FixtureRoot, bench: string, ref: string): Promise<string> {
	const path = ((await ask(root, bench, ["path", ref])) as { path?: string }).path ?? "";
	assert.notEqual(path, "", `dinah path ${ref} answered with no path`);
	return path;
}

/**
 * The value one key of an anchor's front matter records, or the empty string
 * where it records none.
 *
 * Modeled on recordedDigest in src/commentBody.ts, which reads the one key the
 * extension needs and is not general. The item's anchor is read off the disk
 * rather than through a verb, because the store is what this file asserts on.
 */
function headerValue(text: string, key: string): string {
	const newline = text.includes("\r\n") ? "\r\n" : "\n";
	const opening = "---" + newline;
	if (!text.startsWith(opening)) {
		return "";
	}
	const closing = newline + "---" + newline;
	const end = text.indexOf(closing, opening.length - newline.length);
	const header = end < 0 ? text : text.slice(opening.length, end + newline.length);
	for (const line of header.split(newline)) {
		const [name, ...rest] = line.split(":");
		if (name.trim() === key) {
			return rest.join(":").trim();
		}
	}
	return "";
}

/** The references of the comments below one item, as the binary lists them. */
async function commentsBelow(root: FixtureRoot, bench: string, item: string): Promise<string[]> {
	const listing = (await ask(root, bench, ["list", `${item}/comments`])) as {
		members?: { ref?: string }[];
	};
	return (listing.members ?? []).map((member) => member.ref ?? "");
}

/** The context the tree would hand closeItem for one item row. */
function contextFor(root: FixtureRoot, bench: string, ref: string, log: HostLog): ItemCommandContext {
	return {
		spawner: fixtureSpawner(root),
		exe: root.binary,
		host: cardHost(log),
		folder: bench,
		root: bench,
		ref,
		card: CARD,
	};
}

/**
 * Closes one item through closeItem and asserts the three things this file
 * exists for: the verb answered ok, the item ends in the state the verb sets,
 * and its resolution designates the one comment the verb created, whose body
 * is the text the reader gave.
 */
async function closeAndCheck(
	root: FixtureRoot,
	bench: string,
	verb: string,
	item: string,
	state: string,
	text: string,
): Promise<void> {
	assert.deepEqual(
		await commentsBelow(root, bench, item),
		[],
		`${item} carried a comment before ${verb} ran, so the designation below proves nothing`,
	);

	const log = emptyLog();
	const outcome = await closeItem(contextFor(root, bench, item, log), verb, text);

	// The answer.
	assert.equal(outcome.kind, "ok", `${verb} ${item} was refused: ${JSON.stringify(outcome)}`);
	assert.deepEqual(log.errors, [], `${verb} ${item} reported an error`);

	// The state the store records for the item.
	const record = readFileSync(await anchorPath(root, bench, item), "utf8");
	const recorded = headerValue(record, "state");
	assert.equal(recorded, state, `${item} ended ${recorded}, where ${verb} sets ${state}`);

	// The designation. The verb created exactly one comment, and the item's
	// resolution names that comment. The two references are compared by the
	// anchor each resolves to, because the listing and the resolution are free
	// to spell one comment's reference through different branches of the card.
	const created = await commentsBelow(root, bench, item);
	assert.equal(created.length, 1, `${verb} left ${String(created.length)} comments below ${item}`);
	const resolution = headerValue(record, "resolution");
	assert.notEqual(resolution, "", `${item} records no resolution after ${verb}`);
	const designated = await anchorPath(root, bench, resolution);
	const comment = await anchorPath(root, bench, created[0]);
	assert.equal(
		designated,
		comment,
		`${item}'s resolution ${resolution} is not the comment ${verb} created, ${created[0]}`,
	);
	assert.equal(
		splitAnchorBody(readFileSync(comment, "utf8")).trim(),
		text,
		`the comment ${verb} created does not carry the text the reader gave`,
	);
}

test("Resolve records the question resolved, answered by the comment it created, and Reopen reopens it", async (t) => {
	const { root, bench } = fixture(t);
	await closeAndCheck(root, bench, "resolve", QUESTION, "resolved", "Yes, this is settled.");

	const log = emptyLog();
	const reopened = await reopenItem(
		contextFor(root, bench, QUESTION, log),
		"Reopened for another look.",
	);
	assert.equal(reopened.kind, "ok", `reopen ${QUESTION} was refused: ${JSON.stringify(reopened)}`);
	assert.deepEqual(log.errors, [], `reopen ${QUESTION} reported an error`);
});

test("Verify records the criterion verified, answered by the comment it created", async (t) => {
	const { root, bench } = fixture(t);
	await closeAndCheck(root, bench, "verify", FIRST_CRITERION, "verified", "Checked by hand.");
});

test("Fail records the criterion failed, answered by the comment it created", async (t) => {
	const { root, bench } = fixture(t);
	await closeAndCheck(root, bench, "fail", SECOND_CRITERION, "failed", "It does not hold.");
});
