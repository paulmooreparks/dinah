// Every contributed command declares what it does with a selection, and the
// rows a command was aimed at are resolved one way.
//
// dinah-490 AC-2 and AC-4.

import assert from "node:assert/strict";
import { test } from "node:test";

import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import { TREE_COMMANDS } from "../../src/identity";
import { SELECTION_POLICIES, targetsFor } from "../../src/selection";

// ---------------------------------------------------------------------------
// AC-2: every contributed command declares a policy, and nothing else does
// ---------------------------------------------------------------------------

test("the policy table and the command roster are the same set", () => {
	// Both directions, reported separately, because the two failures mean
	// different things: a command contributed with no declared policy is a
	// command multi-select reaches undecided, and a policy for a command
	// nobody contributes is a stale entry nothing exercises.
	const declared = Object.keys(SELECTION_POLICIES);
	const contributed = [...TREE_COMMANDS];
	const undeclared = contributed.filter((id) => !declared.includes(id)).sort();
	const unexpected = declared.filter((id) => !contributed.includes(id)).sort();
	assert.deepEqual(
		undeclared,
		[],
		`these contributed commands declare no selection policy: ${undeclared.join(", ")}`,
	);
	assert.deepEqual(
		unexpected,
		[],
		`these selection policies name no contributed command: ${unexpected.join(", ")}`,
	);
});

test("the roster holds the thirty-two commands this extension contributes", () => {
	// The literal is the doubling the set comparison above cannot do on its
	// own: a command deleted from both lists at once leaves them equal, and
	// only a count notices. A later card adding a command edits this number,
	// and that is the intended cost.
	assert.equal(TREE_COMMANDS.length, 32);
});

test("every policy but noRow carries an entry in the table the editor iterates", () => {
	// The link between the declaration and the wiring, in both directions. A
	// command given a table entry but no policy fails as unexpected, and one
	// given a policy but no entry fails as missing, so neither list can drift
	// under the other.
	const wired = ROW_COMMAND_TABLE.map((entry) => entry.id).sort();
	const expected = Object.entries(SELECTION_POLICIES)
		.filter(([, entry]) => entry.policy !== "noRow")
		.map(([id]) => id)
		.sort();
	const missing = expected.filter((id) => !wired.includes(id));
	const unexpected = wired.filter((id) => !expected.includes(id));
	assert.deepEqual(
		missing,
		[],
		`these commands declare a row policy and are wired to nothing: ${missing.join(", ")}`,
	);
	assert.deepEqual(
		unexpected,
		[],
		`these table entries declare no row policy: ${unexpected.join(", ")}`,
	);
});

test("every fanOut entry declares an effect and no other entry does", () => {
	// The effect field is what the per-command tests read their expectation
	// from, so an entry carrying one it has no business carrying would put a
	// second, unread declaration in the same record.
	for (const [id, entry] of Object.entries(SELECTION_POLICIES)) {
		if (entry.policy === "fanOut") {
			assert.notEqual(entry.effect, undefined, `${id} declares no effect`);
		} else {
			assert.equal(entry.effect, undefined, `${id} declares an effect it cannot use`);
		}
	}
});

// ---------------------------------------------------------------------------
// AC-4: the rows a command was aimed at
// ---------------------------------------------------------------------------

/** A row that is nothing but its key, since targetsFor reads nothing else. */
interface Row {
	readonly key: string;
}

const keyOf = (row: Row): string => row.key;

const A: Row = { key: "a" };
const B: Row = { key: "b" };
const C: Row = { key: "c" };

test("no element and no selection answers the empty array", () => {
	assert.deepEqual(targetsFor<Row>(undefined, undefined, keyOf), []);
	assert.deepEqual(targetsFor<Row>(undefined, [], keyOf), []);
});

test("an element with no selection answers that element alone", () => {
	// This is the Command Palette invocation and the single-row invocation,
	// and it keeps today's behaviour byte for byte.
	assert.deepEqual(targetsFor(A, undefined, keyOf), [A]);
	assert.deepEqual(targetsFor(A, [], keyOf), [A]);
});

test("an element already in the selection answers the selection, deduplicated, in its own order", () => {
	// Deduplication is by the key rather than by object identity, because
	// nothing documents that the editor hands back the same object in both
	// arguments, so the duplicate here is a second object carrying one key.
	const echo: Row = { key: "a" };
	assert.deepEqual(targetsFor(A, [B, echo, C, B], keyOf), [B, echo, C]);
	assert.deepEqual(
		targetsFor(A, [B, echo, C, B], keyOf).map(keyOf),
		["b", "a", "c"],
	);
});

test("an element outside the selection leads, and the whole selection follows", () => {
	// The editor's declaration says the array holds the selected items and
	// says nothing about the invoked one, so the only reading that cannot
	// silently drop the row the reader aimed at is to include it. The length
	// is asserted as well as the order, because a regression narrowing the
	// answer to the selection alone would still produce a plausible list.
	const answer = targetsFor(A, [B, C], keyOf);
	assert.deepEqual(answer, [A, B, C]);
	assert.equal(answer.length, 3);
	assert.equal(answer[0], A);
});
