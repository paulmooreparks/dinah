// Comment on Card: what composeComment is handed, and which selections it
// refuses or skips.
//
// The spawner is the recording fake from test/support/rows.ts, so no test
// here starts the real binary and none can inherit an ambient Dinah identity.
// What reaches composeComment is read from the two places it leaves a trace:
// the argv of the comment verb, which carries the workbench root and the
// card's reference, and the entry composeComment records for the file it
// opened, which carries the folder as well.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { invokeCommentOnCard } from "../../src/cardCommands";
import { COMMAND_COMMENT_ON_CARD } from "../../src/identity";
import { SELECTION_POLICIES } from "../../src/selection";
import { ENGLISH } from "../../src/l10n";
import type { OpenComments } from "../../src/commentBody";
import {
	FOLDER,
	ROOT,
	cardRow,
	emptyCommentLog,
	emptyLog,
	noteRow,
	ok,
	spawnerLog,
	wiringFor,
} from "../support/rows";

const COMMENT_ID = "a00000000001";
const COMMENT_PATH = `${ROOT}/cards/wb1/comments/a1/comment.md`;

/** A spawner answering the mint with a comment id and the path call with a file. */
function composingSpawner(): ReturnType<typeof spawnerLog> {
	const spawned = spawnerLog();
	spawned.queue.push(
		ok({ outcome: "ok", verb: "comment", detail: COMMENT_ID }),
		ok({ path: COMMENT_PATH }),
	);
	return spawned;
}

test("Comment on Card is a rowOnly command", () => {
	assert.deepEqual(SELECTION_POLICIES[COMMAND_COMMENT_ON_CARD], { policy: "rowOnly" });
});

test("Comment on Card is offered on the card row after the filing commands and hidden from the palette", () => {
	const manifest = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "package.json"), "utf8"),
	) as {
		contributes: {
			menus: Record<string, { command: string; when: string; group?: string }[]>;
		};
	};
	const rows = manifest.contributes.menus["view/item/context"].filter(
		(item) => item.command === COMMAND_COMMENT_ON_CARD,
	);
	assert.deepEqual(rows, [
		{
			command: COMMAND_COMMENT_ON_CARD,
			when: "view == dinah.workbenchView && viewItem =~ /^dinah\\.card\\./",
			group: "2_create@4",
		},
	]);
	const palette = manifest.contributes.menus.commandPalette.filter(
		(item) => item.command === COMMAND_COMMENT_ON_CARD,
	);
	assert.deepEqual(palette, [{ command: COMMAND_COMMENT_ON_CARD, when: "false" }]);
});

test("Comment on Card hands composeComment the card's root, folder and reference", async () => {
	const log = emptyLog();
	const comments = emptyCommentLog();
	const openComments: OpenComments = new Map();
	const spawned = composingSpawner();
	const wiring = wiringFor(log, spawned.spawner, undefined, comments, undefined, openComments);

	const report = await invokeCommentOnCard([cardRow("wb-1")], wiring);

	assert.equal(report.entries.length, 1);
	assert.equal(report.entries[0].outcome.kind, "done");
	assert.equal(spawned.calls.length, 2, `spawned ${JSON.stringify(spawned.calls)}`);
	assert.deepEqual(
		spawned.calls[0],
		["--json", "--workbench", ROOT, "comment", "wb-1"],
		"the comment verb was not aimed at this card, on this workbench, with no text after it",
	);
	assert.deepEqual(spawned.calls[1], ["--json", "--workbench", ROOT, "path", COMMENT_ID]);
	assert.deepEqual(comments.opened, [COMMENT_PATH], "the minted comment's file was not opened");
	const recorded = openComments.get(COMMENT_PATH);
	assert.equal(recorded?.root, ROOT, "the opened comment records some other workbench");
	assert.equal(recorded?.folder, FOLDER, "the opened comment records some other folder");
	assert.equal(recorded?.ref, COMMENT_ID);
	assert.deepEqual(log.errors, []);
});

test("Comment on Card refuses more than one card row and spawns nothing", async () => {
	const log = emptyLog();
	const comments = emptyCommentLog();
	const spawned = composingSpawner();
	const wiring = wiringFor(log, spawned.spawner, undefined, comments);

	const report = await invokeCommentOnCard([cardRow("wb-1"), cardRow("wb-2")], wiring);

	assert.deepEqual(log.errors, [ENGLISH("dialog.bulk.oneRowOnly")]);
	assert.deepEqual(spawned.calls, [], "a refused selection still spawned the binary");
	assert.deepEqual(comments.opened, []);
	assert.equal(report.cancelled, true);
	assert.ok(report.entries.every((entry) => entry.outcome.kind === "skipped"));
});

test("Comment on Card skips a row naming no card with NO_CARD and comments on the card beside it", async () => {
	// The accepting case sits beside the skip, so a build that skipped every
	// row would fail here rather than pass.
	const log = emptyLog();
	const comments = emptyCommentLog();
	const spawned = composingSpawner();
	const wiring = wiringFor(log, spawned.spawner, undefined, comments);

	const report = await invokeCommentOnCard([noteRow("a note"), cardRow("wb-1")], wiring);

	assert.equal(report.entries.length, 2);
	const skipped = report.entries.filter((entry) => entry.outcome.kind === "skipped");
	assert.equal(skipped.length, 1);
	assert.deepEqual(skipped[0].outcome, { kind: "skipped", why: "names no card" });
	assert.notEqual(
		(skipped[0].outcome as { why: string }).why,
		ENGLISH("skip.notACardRow"),
		"the skip names a checklist as acceptable, which this command does not act on",
	);
	assert.deepEqual(log.errors, []);
	assert.deepEqual(spawned.calls[0], ["--json", "--workbench", ROOT, "comment", "wb-1"]);
	assert.deepEqual(comments.opened, [COMMENT_PATH]);
});
