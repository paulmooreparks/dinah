// dinah-519: opening a comment row opens the comment's own anchor file.
//
// Its own file rather than an addition to items.test.ts, because the two
// handlers are separate registrations in separate modules and an
// implementation could get one right and the other wrong. items.test.ts pins
// the item half on the same terms.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import { COMMAND_OPEN_COMMENT } from "../../src/identity";
import type { TreeElement } from "../../src/tree";
import { treeItemFor } from "../../src/tree";
import { ENGLISH } from "../../src/l10n";
import type { CheckResults, DraftLog, HostLog } from "../support/rows";
import {
	ROOT,
	catalogueWithKinds,
	commentRow,
	emptyDraftLog,
	emptyLog,
	itemRow,
	ok,
	refused,
	wiringFor,
} from "../support/rows";

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(...args: string[]): string[] {
	return ["--json", "--workbench", ROOT, ...args];
}

interface Run {
	readonly log: HostLog;
	readonly calls: string[][];
}

/** Drives the entry the editor registers for this command id. */
async function invoke(
	id: string,
	elements: readonly TreeElement[],
	answer: (argv: readonly string[]) => SpawnOutcome = () => ok(),
): Promise<Run> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === id);
	assert.notEqual(entry, undefined, `no table entry carries the id ${id}`);
	const log: HostLog = emptyLog();
	const drafts: DraftLog = emptyDraftLog();
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return answer(argv);
	};
	const results: CheckResults = { applied: [] };
	await (entry as (typeof ROW_COMMAND_TABLE)[number]).invoke(
		elements,
		wiringFor(log, spawner, results, drafts, catalogueWithKinds()),
	);
	return { log, calls };
}

// ---------------------------------------------------------------------------
// dinah-519/criteria/11: one call, the comment's own reference, no holder
// ---------------------------------------------------------------------------

test("Open Comment asks path for the comment's own reference and opens what it answered", async () => {
	// THE COMMENT'S HOLDER IS AN ITEM, deliberately, which is what arms the
	// single-argv assertion. Every earlier draft of this work resolved a
	// comment by cutting the trailing /comments/<n> off its reference and
	// asking `show <holder> --fields comments`, falling back to `show <holder>`
	// when that was refused, and the fallback existed precisely because a card
	// holder and an item holder take different forms. An implementation that
	// still cuts a holder out of the reference records two argvs on this
	// fixture and fails on the count; one that records one but records a show
	// rather than a path fails on the argv. A build that happened to open the
	// right file after two calls is still wrong and this says so.
	const ref = "wb-1/questions/1/comments/2";
	const path = "C:/work/bench/cards/aa/checklist/bb/comments/cc/comment.md";
	const run = await invoke(COMMAND_OPEN_COMMENT, [commentRow({ ref }, 0, "wb-1/questions/1")], () =>
		ok({ path }),
	);

	assert.deepEqual(run.calls, [pinned("path", ref)]);
	assert.deepEqual(run.log.opened, [path]);
	assert.deepEqual(run.log.served, [], "opening an anchor file served a composed page");
});

test("a refused path shows the refusal and opens no document", async () => {
	const ref = "wb-1/comments/9";
	const run = await invoke(COMMAND_OPEN_COMMENT, [commentRow({ ref })], () =>
		refused("dinah.unknown-comment", ref),
	);
	assert.deepEqual(run.calls, [pinned("path", ref)]);
	assert.deepEqual(run.log.opened, []);
	assert.deepEqual(run.log.errors, [`dinah.unknown-comment: ${ref}`]);
});

test("a row that is not a comment is skipped and spawns nothing", async () => {
	const run = await invoke(COMMAND_OPEN_COMMENT, [itemRow({ ref: "wb-1/questions/1" })]);
	assert.deepEqual(run.calls, []);
	assert.deepEqual(run.log.opened, []);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/7's menu half: the row carries the value its clause names
// ---------------------------------------------------------------------------

test("a comment row carries the contextValue its context menu is registered against", () => {
	// One value and no axes, on the terms every attachment row carries one. A
	// comment has no state, no owner and no kind, so there is nothing for an
	// axis to say, and the manifest clause matching it is an equality.
	assert.equal(treeItemFor(commentRow(), ENGLISH).contextValue, "dinah.comment");
	assert.equal(
		treeItemFor(commentRow({}, 0, "wb-1", ROOT, false), ENGLISH).contextValue,
		"dinah.comment",
		"a row whose detail call did not answer lost the value its menu is registered against",
	);
});

// ---------------------------------------------------------------------------
// dinah-519/criteria/10: the composed item document is gone, not reduced
// ---------------------------------------------------------------------------

/**
 * The key set of the served-text resolver table the extension registers at
 * activation.
 *
 * Parsed off the source rather than imported, because extension.ts loads
 * vscode as a value and no unit test can import it, and because the table is a
 * local of activate(). What is read is the REGISTRATION and not an export: a
 * source assertion that servedText.ts no longer exports renderItemMarkdown is
 * defeated by keeping the function under another name, while a kind nothing
 * registers cannot serve a page however the renderer is spelled.
 *
 * EVERY PROPERTY NAME IS READ, computed or literal. The first version of this
 * walk collected `[KIND_X]:` entries alone, and re-registering the item kind
 * under a bare `item:` key passed it: an implementer who deletes the renderer
 * and leaves a stub registered writes exactly that. Reading the object's own
 * property names off the syntax tree is what closes it, because a property is
 * a property whichever way it is spelled.
 */
function registeredKinds(): string[] {
	const extensionSource = join(__dirname, "..", "..", "..", "src", "extension.ts");
	const file = ts.createSourceFile(
		extensionSource,
		readFileSync(extensionSource, "utf8"),
		ts.ScriptTarget.Latest,
		true,
	);
	let table: ts.ObjectLiteralExpression | undefined;
	const find = (node: ts.Node): void => {
		if (
			ts.isVariableDeclaration(node) &&
			ts.isIdentifier(node.name) &&
			node.name.text === "resolvers" &&
			node.initializer !== undefined &&
			ts.isObjectLiteralExpression(node.initializer)
		) {
			assert.equal(table, undefined, "extension.ts declares more than one resolver table");
			table = node.initializer;
		}
		ts.forEachChild(node, find);
	};
	find(file);
	assert.notEqual(table, undefined, "extension.ts declares no served-text resolver table");

	const served = readFileSync(
		join(__dirname, "..", "..", "..", "src", "servedText.ts"),
		"utf8",
	);
	const kinds = (table as ts.ObjectLiteralExpression).properties.map((property) => {
		const name = property.name;
		assert.notEqual(name, undefined, "a resolver entry carries no property name at all");
		if (name !== undefined && ts.isComputedPropertyName(name)) {
			const constant = name.expression.getText();
			const declared = new RegExp(`export const ${constant} = "([^"]+)"`).exec(served);
			assert.notEqual(declared, null, `servedText.ts declares no ${constant}`);
			return (declared as RegExpExecArray)[1];
		}
		if (name !== undefined && (ts.isIdentifier(name) || ts.isStringLiteral(name))) {
			return name.text;
		}
		assert.fail(`a resolver entry is keyed by something this walk cannot read: ${String(name?.getText())}`);
	});
	assert.ok(kinds.length > 0, "the walk found no resolver entry, so it read nothing");
	return kinds;
}

test("no served-text kind is registered for an item or for a comment", () => {
	// The half-deletion is what this refuses: an implementer who adds the two
	// file handlers and leaves the resolver registered ships both surfaces and
	// passes every other assertion on this card. An implementation that
	// deletes renderItemMarkdown and leaves the kind registered against a stub
	// fails here and nowhere else.
	const kinds = registeredKinds();
	assert.equal(kinds.includes("item"), false, `the table still registers item: ${kinds.join(", ")}`);
	assert.equal(
		kinds.includes("comment"),
		false,
		`the table registers a comment kind, which this card adds none of: ${kinds.join(", ")}`,
	);
	// The kinds it does carry are unchanged from trunk, so a build that
	// deletes one kind and quietly adds another fails rather than passing on
	// the two absences alone.
	assert.deepEqual([...kinds].sort(), ["guide", "history", "instructions"]);
});

test("the refresh loop resolves through that one table and holds no second registry", () => {
	// The refresh loop reads a kind and asks the same resolvers object for it,
	// so the key set above is the whole of what any served-text surface can be
	// keyed by. registeredKinds refuses a second declaration of the table
	// outright; this names the two sites that read it, so a lookup against
	// some other map is visible here rather than making the key set partial.
	const source = readFileSync(
		join(__dirname, "..", "..", "..", "src", "extension.ts"),
		"utf8",
	);
	const lookups = [...source.matchAll(/resolvers\[[a-zA-Z.]+\]/g)].map((hit) => hit[0]);
	assert.deepEqual(
		[...new Set(lookups)].sort(),
		["resolvers[kind]", "resolvers[parsed.kind]"],
		`the resolver table is read from an unexpected site: ${lookups.join(", ")}`,
	);
});

test("no module under src/ renders a composed page for an item", () => {
	// The renderer and its labels type go with the kind. This is the weaker
	// kind of check and says so: it finds copies of a name rather than copies
	// of the idea, and the registration test above is what actually refuses a
	// page keyed to an item. It is here because a dangling export is dead
	// weight a later reader has to work out the fate of.
	const dir = join(__dirname, "..", "..", "..", "src");
	const offenders: string[] = [];
	const walk = (at: string): void => {
		for (const entry of readdirSync(at, { withFileTypes: true })) {
			const here = join(at, entry.name);
			if (entry.isDirectory()) {
				if (entry.name !== "generated" && entry.name !== "locales") {
					walk(here);
				}
				continue;
			}
			if (!entry.name.endsWith(".ts")) {
				continue;
			}
			const text = readFileSync(here, "utf8");
			if (text.includes("renderItemMarkdown") || text.includes("ItemLabels")) {
				offenders.push(entry.name);
			}
		}
	};
	walk(dir);
	assert.deepEqual(offenders, []);
});
