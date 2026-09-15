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

/**
 * The two function bodies that serve text, found by the construct that
 * registers each rather than by their position in the file.
 *
 * `provideTextDocumentContent` is the property VS Code calls to fill a served
 * document, and the `resolve` property of the ServedTextRefreshLoop argument
 * is what the poll calls to refetch one. Between them they are the whole of
 * the served-text path: no third caller can serve a document without
 * registering itself through one of these two constructs.
 */
function servedTextSites(file: ts.SourceFile): Map<string, ts.Node> {
	const sites = new Map<string, ts.Node>();
	const propertyOf = (argument: ts.Expression, name: string): ts.Node | undefined => {
		if (!ts.isObjectLiteralExpression(argument)) {
			return undefined;
		}
		for (const property of argument.properties) {
			if (
				(ts.isPropertyAssignment(property) || ts.isMethodDeclaration(property)) &&
				property.name !== undefined &&
				(ts.isIdentifier(property.name) || ts.isStringLiteral(property.name)) &&
				property.name.text === name
			) {
				return property;
			}
		}
		return undefined;
	};
	const find = (node: ts.Node): void => {
		if (
			ts.isCallExpression(node) &&
			ts.isPropertyAccessExpression(node.expression) &&
			node.expression.name.text === "registerTextDocumentContentProvider" &&
			node.arguments.length >= 2
		) {
			const site = propertyOf(node.arguments[1], "provideTextDocumentContent");
			assert.notEqual(site, undefined, "the content provider declares no provideTextDocumentContent");
			assert.equal(sites.has("provideTextDocumentContent"), false, "extension.ts registers more than one served-text content provider");
			sites.set("provideTextDocumentContent", site as ts.Node);
		}
		if (
			ts.isNewExpression(node) &&
			ts.isIdentifier(node.expression) &&
			node.expression.text === "ServedTextRefreshLoop" &&
			node.arguments !== undefined &&
			node.arguments.length >= 1
		) {
			const site = propertyOf(node.arguments[0], "resolve");
			assert.notEqual(site, undefined, "the refresh loop is constructed with no resolve");
			assert.equal(sites.has("refreshLoop.resolve"), false, "extension.ts constructs more than one ServedTextRefreshLoop");
			sites.set("refreshLoop.resolve", site as ts.Node);
		}
		ts.forEachChild(node, find);
	};
	find(file);
	return sites;
}

/**
 * Every lookup keyed by a value inside one served-text site, reported as the
 * text of the thing being looked up in.
 *
 * Two spellings are collected and no others: the subscript `table[kind]` and
 * the one-argument map call `table.get(kind)`. A lookup whose key is a string
 * or numeric literal is not a kind-keyed dispatch and is skipped, which is
 * what keeps an ordinary array index out of the answer. Every other way of
 * reading a value out of a table is invisible here, a computed property in an
 * object binding pattern and `Reflect.get` among them, and the test's own
 * comment carries the list of escapes and says that list is open.
 */
function keyedLookupsIn(site: ts.Node): string[] {
	const found: string[] = [];
	const walk = (node: ts.Node): void => {
		if (ts.isElementAccessExpression(node)) {
			const key = node.argumentExpression;
			if (!ts.isStringLiteral(key) && !ts.isNumericLiteral(key)) {
				found.push(node.expression.getText());
			}
		}
		if (
			ts.isCallExpression(node) &&
			ts.isPropertyAccessExpression(node.expression) &&
			node.expression.name.text === "get" &&
			node.arguments.length === 1
		) {
			found.push(node.expression.expression.getText());
		}
		ts.forEachChild(node, walk);
	};
	walk(site);
	return found;
}

/**
 * Every function declared in this module, keyed by the name a call would
 * reach it under.
 *
 * Two declaration shapes are collected, because both are ordinary here: a
 * `function` declaration with a name, and a `const` bound directly to a
 * function expression or an arrow. A binding whose initializer is anything
 * else is not a function this walk can follow, so it is left out rather than
 * recorded under a body it does not have. "Anything else" includes a factory
 * call, and it also includes the same arrow wrapped in an `as` cast, a
 * `satisfies`, or parentheses, which look like declarations to a reader and
 * are not initializers this walk reads as functions.
 */
function moduleFunctions(file: ts.SourceFile): Map<string, ts.Node> {
	const declared = new Map<string, ts.Node>();
	const find = (node: ts.Node): void => {
		if (ts.isFunctionDeclaration(node) && node.name !== undefined && node.body !== undefined) {
			declared.set(node.name.text, node.body);
		}
		if (
			ts.isVariableDeclaration(node) &&
			ts.isIdentifier(node.name) &&
			node.initializer !== undefined &&
			(ts.isArrowFunction(node.initializer) || ts.isFunctionExpression(node.initializer))
		) {
			declared.set(node.name.text, node.initializer.body);
		}
		ts.forEachChild(node, find);
	};
	find(file);
	return declared;
}

/**
 * The names a site calls directly, meaning a call whose callee is a bare
 * identifier. A call through a property access, `host.serve(kind)`, has no
 * bare name to resolve against a declaration in this module, so it is not
 * collected and the section below says so.
 */
function directCalleesIn(site: ts.Node): string[] {
	const called: string[] = [];
	const walk = (node: ts.Node): void => {
		if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
			called.push(node.expression.text);
		}
		ts.forEachChild(node, walk);
	};
	walk(site);
	return called;
}

test("the served-text path dispatches through that one table and consults no second registry", () => {
	// This is the structural form of the claim, and it replaces a regular
	// expression over `resolvers[...]` sites that a reviewer defeated on
	// 2026-09-15 by writing a second table named itemPages beside the first,
	// consulted ahead of the resolvers lookup and serving a composed page for
	// kind item. Matching the spelling of the lookup that is there says
	// nothing about a lookup that is not.
	//
	// What is asserted instead is the shape: the served-text path is the two
	// function bodies VS Code and the refresh loop call, together with the
	// bodies of the functions those two call directly by name and that
	// extension.ts declares itself. Inside that region every value-keyed
	// lookup is against `resolvers` and nothing else.
	//
	// The one step of call-following is here because a reviewer defeated the
	// body-only form of this walk on 2026-09-15 by extracting the same second
	// table's lookup into a module-scope helper and calling the helper from
	// provideTextDocumentContent. The region is one step and stops there,
	// because a fixed-point walk over call names alone resolves nothing and
	// would have to guess at shadowing, and the honest report of a bounded
	// region is worth more than an unbounded one nobody can characterise.
	//
	// WHAT THIS GUARD DOES NOT SEE, stated plainly because a guard that
	// cannot see something is worse than useless while it reads as though it
	// can. Two bounds hold it, distance and spelling, and both are real.
	// Distance is the region described above. Spelling is the closed list of
	// syntactic constructs the three walks recognise, and every other way of
	// writing the same operation escapes them wherever it sits, inside the
	// region as much as outside it.
	//
	// The positive list is short and exact, so read it as the statement of
	// reach and read the shapes below as illustration. keyedLookupsIn
	// collects an element access whose key is not a string or numeric
	// literal, and a one-argument call through a property named `get`.
	// directCalleesIn collects a call whose callee is a bare identifier, and
	// the walk follows such a call exactly one step. moduleFunctions
	// resolves that name only against a named `function` declaration or a
	// variable whose initializer is directly a function expression or an
	// arrow.
	//
	// Six escapes are on the record, every one of them written into
	// extension.ts and run against this file rather than argued from
	// reading the walks. THIS LIST IS NOT KNOWN TO BE COMPLETE
	// and must not be read as a partition of what escapes: it records the
	// shapes somebody has actually run, and nothing here rules out a
	// seventh, because the positive list is a list of constructs and the
	// language spells an indexed read in more ways than anybody has
	// enumerated here. Each entry carries its reproduction.
	//
	//   - A dispatch that consults no table at all, of the shape
	//     `if (parsed.kind === "item") { return renderItem(...); }` written
	//     into either site. The walk reads lookups, and that is not one.
	//   - A lookup two calls out, where a followed helper calls a second
	//     helper that holds the table. Write the module-scope helper of the
	//     2026-09-15 defeat and put a third helper between it and the table.
	//   - A lookup in a helper extension.ts imports rather than declares, or
	//     obtains from a factory call, or binds through an `as` cast or a
	//     `satisfies`, since none of those is an initializer moduleFunctions
	//     reads as a function.
	//   - A lookup behind a call through a property access, `host.serve(k)`,
	//     whose callee carries no bare name to resolve.
	//   - A second table read by computed destructuring in the site's own
	//     body, `const { [parsed.kind]: page } = itemPages;`. Zero calls out
	//     and squarely inside the region, and it escapes on spelling alone,
	//     because a computed property in an object binding pattern is not an
	//     element access.
	//   - `Reflect.get(itemPages, parsed.kind)`, which is a call but not a
	//     call through a property named `get`. A two-argument `.get(k, d)`
	//     falls the same way, on the argument count.
	//
	// What refuses a good many of those is the other half of this pair, and
	// it refuses them by a different route: any of them serves a kind, and a
	// served kind has to be reachable from a URI the tree composes, which is
	// why openItem's own registration and the absence of every
	// item.document.* catalogue key are pinned separately. That pairing is
	// the reason a hole here is survivable, and it is not a reason to
	// describe this walk as reaching further than it does.
	const extensionSource = join(__dirname, "..", "..", "..", "src", "extension.ts");
	const file = ts.createSourceFile(
		extensionSource,
		readFileSync(extensionSource, "utf8"),
		ts.ScriptTarget.Latest,
		true,
	);
	const sites = servedTextSites(file);
	assert.deepEqual(
		[...sites.keys()].sort(),
		["provideTextDocumentContent", "refreshLoop.resolve"],
		"the walk did not find both served-text sites, so it read nothing it claims to read",
	);
	const declared = moduleFunctions(file);
	assert.ok(
		declared.size > 0,
		"the walk found no function declared in extension.ts, so it can follow no call and read nothing",
	);
	for (const [name, site] of sites) {
		const callees = directCalleesIn(site);
		assert.ok(
			callees.length > 0,
			`${name} calls nothing by name, so the call-following half of this walk read nothing`,
		);
		const lookups = keyedLookupsIn(site);
		for (const callee of callees) {
			const body = declared.get(callee);
			if (body !== undefined) {
				lookups.push(...keyedLookupsIn(body));
			}
		}
		assert.ok(lookups.length > 0, `${name} performs no keyed lookup at all, so this walk read nothing`);
		assert.deepEqual(
			[...new Set(lookups)].sort(),
			["resolvers"],
			`${name} consults a registry other than resolvers: ${lookups.join(", ")}`,
		);
	}
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
