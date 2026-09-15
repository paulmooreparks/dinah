// What extension.ts binds, read off its own syntax tree.
//
// dinah-490 AC-1, AC-3, AC-17, AC-29, AC-31 and AC-34. Every assertion here
// compares node counts, parameter names and identifier text in an AST parsed
// at test time, never a string in a file, which is the same shape
// test/unit/l10n-keys.test.ts and test/unit/l10n-coverage.test.ts already use
// to sweep src/.
//
// The honest limit, stated so nobody credits these with more: they prove the
// source says so, not that VS Code honoured it. Only the integration suite
// could prove that, and this workbench never runs it. What these do buy is
// that the functions the other unit files drive are the functions the editor
// is wired to, so a criterion cannot pass by driving a path the product never
// takes.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

import * as identity from "../../src/identity";
import { EDITOR_COMMANDS } from "../../src/identity";
import { PROMPT_CHANNELS, REPORT_CHANNELS } from "../../src/reporter";
import { SELECTION_POLICIES } from "../../src/selection";

// This file is compiled to out/test/unit/, so the extension root is three up.
const srcDir = join(__dirname, "..", "..", "..", "src");

/** One module's syntax tree, parsed fresh. */
function parse(file: string): ts.SourceFile {
	return ts.createSourceFile(
		file,
		readFileSync(file, "utf8"),
		ts.ScriptTarget.ES2022,
		true,
		ts.ScriptKind.TS,
	);
}

const extensionFile = join(srcDir, "extension.ts");
const extension = parse(extensionFile);

/** Every node of one tree, in source order. */
function nodes(root: ts.Node): ts.Node[] {
	const found: ts.Node[] = [];
	const visit = (node: ts.Node): void => {
		found.push(node);
		ts.forEachChild(node, visit);
	};
	visit(root);
	return found;
}

/** Every call expression of one tree. */
function calls(root: ts.Node): ts.CallExpression[] {
	return nodes(root).filter(ts.isCallExpression);
}

/** The nearest ancestor of this node for which the test answers true. */
function ancestor(node: ts.Node, wanted: (n: ts.Node) => boolean): ts.Node | undefined {
	let here: ts.Node | undefined = node.parent;
	while (here !== undefined) {
		if (wanted(here)) {
			return here;
		}
		here = here.parent;
	}
	return undefined;
}

// ---------------------------------------------------------------------------
// AC-1: the tree view selects many
// ---------------------------------------------------------------------------

test("the one createTreeView call sets canSelectMany to the true keyword", () => {
	const created = calls(extension).filter(
		(call) => call.expression.getText() === "vscode.window.createTreeView",
	);
	// The count first, so a walk that matched nothing fails rather than
	// reporting a clean sweep over no calls at all.
	assert.equal(created.length, 1, "extension.ts does not create exactly one tree view");
	const options = created[0].arguments[1];
	assert.ok(
		options !== undefined && ts.isObjectLiteralExpression(options),
		"the tree view was not given an options object literal",
	);
	const property = options.properties.find(
		(member): member is ts.PropertyAssignment =>
			ts.isPropertyAssignment(member) &&
			ts.isIdentifier(member.name) &&
			member.name.text === "canSelectMany",
	);
	assert.notEqual(property, undefined, "the tree view options carry no canSelectMany");
	assert.equal(
		property?.initializer.kind,
		ts.SyntaxKind.TrueKeyword,
		"canSelectMany is present but is not the true keyword",
	);
});

// ---------------------------------------------------------------------------
// AC-3: five registrations, four of them rowless, one loop over the table
// ---------------------------------------------------------------------------

/** Every call to the local register helper. */
function registrations(): ts.CallExpression[] {
	return calls(extension).filter(
		(call) => ts.isIdentifier(call.expression) && call.expression.text === "register",
	);
}

/** The command id an identifier in identity.ts names. */
function idNamed(name: string): string | undefined {
	const value = (identity as unknown as Record<string, unknown>)[name];
	return typeof value === "string" ? value : undefined;
}

test("extension.ts registers in exactly three shapes and no others", () => {
	const registered = registrations();
	// The literal is the doubling: a walk that matched nothing would otherwise
	// satisfy every classification below by vacuity.
	assert.equal(
		registered.length,
		7,
		`extension.ts holds ${String(registered.length)} register calls`,
	);

	const rowless = registered.filter((call) => {
		const first = call.arguments[0];
		if (first === undefined || !ts.isIdentifier(first)) {
			return false;
		}
		const id = idNamed(first.text);
		return id !== undefined && SELECTION_POLICIES[id]?.policy === "noRow";
	});
	assert.equal(
		rowless.length,
		6,
		`${String(rowless.length)} register calls name a command declaring noRow`,
	);
	// A rowless command is one of two things, and dinah-506 is where the
	// second appeared. A global command reads nothing at all and so declares
	// no parameter, which is what this guard asserted when every rowless
	// command was global. An editor command reads the active editor, and the
	// editor title bar hands its command a Uri, so it declares one parameter
	// and reads it as a Uri rather than as a row. Both shapes are pinned,
	// because a global command that grew a parameter would be reading
	// something a palette invocation never supplies.
	for (const call of rowless) {
		const handler = call.arguments[1];
		assert.ok(
			handler !== undefined && ts.isArrowFunction(handler),
			"a rowless command registers something other than an arrow function",
		);
		const id = idNamed((call.arguments[0] as ts.Identifier).text) ?? "";
		const editor = (EDITOR_COMMANDS as readonly string[]).includes(id);
		assert.equal(
			handler.parameters.length,
			editor ? 1 : 0,
			`${call.arguments[0].getText()} registers a handler declaring ${String(handler.parameters.length)} parameters`,
		);
		if (editor) {
			assert.match(
				handler.getText(),
				/draftPathOf\(/,
				`${call.arguments[0].getText()} reads its argument some other way than through draftPathOf`,
			);
		}
	}
	// The one helper both editor commands read their argument through, which
	// is where the Uri test and the active-editor fallback live. A command
	// invoked with no draft in front of the reader has been asked for nothing,
	// so both handlers return without calling the pure module.
	assert.equal(
		(extension.getText().match(/argument instanceof vscode\.Uri/g) ?? []).length,
		1,
		"the Uri test is spelled more than once, or not at all",
	);
	assert.equal(
		(EDITOR_COMMANDS as readonly string[]).length,
		2,
		"the editor classification is empty or has grown, so the shapes above read nothing",
	);

	// The one remaining call is the loop, and what it iterates is the claim
	// worth making: a loop over a second array of the same shape satisfies
	// every other assertion here while ROW_COMMAND_TABLE wires nothing.
	const loops = registered.filter((call) => !rowless.includes(call));
	assert.equal(loops.length, 1);
	const forOf = ancestor(loops[0], ts.isForOfStatement) as ts.ForOfStatement | undefined;
	assert.notEqual(forOf, undefined, "the remaining register call sits in no for-of");
	assert.ok(
		forOf !== undefined &&
			ts.isIdentifier(forOf.expression) &&
			forOf.expression.text === "ROW_COMMAND_TABLE",
		`the loop iterates ${forOf?.expression.getText() ?? "nothing"}`,
	);

	// Imported from the module that declares it, rather than shadowed locally.
	const imported = extension.statements.some(
		(statement) =>
			ts.isImportDeclaration(statement) &&
			ts.isStringLiteral(statement.moduleSpecifier) &&
			statement.moduleSpecifier.text === "./commandTable" &&
			statement.getText().includes("ROW_COMMAND_TABLE"),
	);
	assert.ok(imported, "ROW_COMMAND_TABLE is not imported from ./commandTable");

	// The loop binds invoke out of the value it is iterating, so the function
	// the handler calls is the entry's own rather than a lookup beside it.
	const binding = (forOf.initializer as ts.VariableDeclarationList).declarations[0].name;
	assert.ok(ts.isObjectBindingPattern(binding), "the loop binds no object pattern");
	const bound = binding.elements.map((element) => element.name.getText());
	assert.deepEqual([...bound].sort(), ["id", "invoke"]);

	// The handler declares both parameters, passes both to targetsFor, and
	// calls the invoke the loop bound. A handler that declares a second
	// parameter and then ignores it satisfies an arity check and still acts on
	// one row, so the identifiers are compared rather than counted.
	const handler = loops[0].arguments[1];
	assert.ok(handler !== undefined && ts.isArrowFunction(handler));
	assert.equal(handler.parameters.length, 2);
	const parameterNames = handler.parameters.map((parameter) => parameter.name.getText());
	const targeting = calls(handler).filter(
		(call) => ts.isIdentifier(call.expression) && call.expression.text === "targetsFor",
	);
	assert.equal(targeting.length, 1);
	assert.deepEqual(
		targeting[0].arguments.slice(0, 2).map((argument) => argument.getText()),
		parameterNames,
	);
	const invoking = calls(handler).filter(
		(call) => ts.isIdentifier(call.expression) && call.expression.text === "invoke",
	);
	assert.equal(invoking.length, 1);
	assert.equal(invoking[0].arguments[0], targeting[0]);
});

// ---------------------------------------------------------------------------
// AC-29: the drag boundary is wired to the functions AC-15 drives
// ---------------------------------------------------------------------------

/** The object literal extension.ts hands the tree view as its drag controller. */
function dragController(): ts.ObjectLiteralExpression {
	const declared = nodes(extension)
		.filter(ts.isVariableDeclaration)
		.find((declaration) => declaration.name.getText() === "dragAndDropController");
	assert.notEqual(declared, undefined, "extension.ts declares no drag controller");
	const literal = declared?.initializer;
	assert.ok(
		literal !== undefined && ts.isObjectLiteralExpression(literal),
		"the drag controller is not an object literal",
	);
	return literal;
}

/** One named member of the controller, as the arrow function it is. */
function handlerNamed(name: string): ts.ArrowFunction {
	const members = dragController().properties.filter(
		(member): member is ts.PropertyAssignment =>
			ts.isPropertyAssignment(member) &&
			ts.isIdentifier(member.name) &&
			member.name.text === name,
	);
	assert.equal(members.length, 1, `the controller holds ${String(members.length)} ${name}`);
	const handler = members[0].initializer;
	assert.ok(ts.isArrowFunction(handler), `${name} is not an arrow function`);
	return handler;
}

test("handleDrag offers the rows it was handed, rather than a list it built", () => {
	const handler = handlerNamed("handleDrag");
	const offering = calls(handler).filter(
		(call) => ts.isIdentifier(call.expression) && call.expression.text === "offerDrag",
	);
	assert.equal(offering.length, 1);
	assert.equal(
		offering[0].arguments[0].getText(),
		handler.parameters[0].name.getText(),
		"offerDrag is handed something other than handleDrag's own source parameter",
	);
});

test("handleDrop reads its own data transfer and acts on what came back", () => {
	const handler = handlerNamed("handleDrop");
	const reading = calls(handler).filter(
		(call) => ts.isIdentifier(call.expression) && call.expression.text === "dragRowsFrom",
	);
	assert.equal(reading.length, 1);
	const transfer = handler.parameters[1].name.getText();
	const argument = reading[0].arguments[0].getText();
	assert.ok(argument.includes(transfer), `${argument} does not read ${transfer}`);
	assert.ok(
		argument.includes("DRAG_MIME_TYPE"),
		`${argument} does not read the drag mime entry`,
	);
	// The result is bound to a name, and that same name is what the drop acts
	// on. A handler that read the entry and then acted on a freshly built list
	// satisfies a call-count check and wires nothing.
	const bound = ancestor(reading[0], ts.isVariableDeclaration) as
		| ts.VariableDeclaration
		| undefined;
	assert.notEqual(bound, undefined, "the mime read is bound to no name");
	const applying = calls(handler).filter(
		(call) =>
			ts.isIdentifier(call.expression) && call.expression.text === "applyDropVerdicts",
	);
	assert.equal(applying.length, 1);
	assert.equal(applying[0].arguments[0].getText(), bound?.name.getText());
});

test("the three singular-drag functions are called nowhere in extension.ts", () => {
	// A diff that adds the new path and leaves the old one live passes every
	// other clause on this card. The names are compared exactly, because
	// applyDropVerdict is a prefix of applyDropVerdicts and a containment
	// check would read the new call as the old one.
	const retired = ["dragPayloadFor", "classifyDrop", "applyDropVerdict"];
	const live = calls(extension)
		.map((call) => (ts.isIdentifier(call.expression) ? call.expression.text : ""))
		.filter((name) => retired.includes(name));
	assert.deepEqual(live, [], `extension.ts still calls: ${live.join(", ")}`);
	// The sweep read something, so the empty result above is a real answer.
	assert.ok(calls(extension).length > 0, "no call expression was read at all");
});

// ---------------------------------------------------------------------------
// AC-31: every message binding sits on a declared channel of a declared host
// ---------------------------------------------------------------------------

/** Every `vscode.window.show*Message` call extension.ts makes. */
function messageBindings(): ts.CallExpression[] {
	return calls(extension).filter((call) => {
		const callee = call.expression;
		if (!ts.isPropertyAccessExpression(callee)) {
			return false;
		}
		// The suffix is what keeps showQuickPick, showInputBox,
		// showTextDocument and showOpenDialog out by construction rather than
		// by an exemption list somebody maintains.
		return (
			callee.expression.getText() === "vscode.window" &&
			callee.name.text.endsWith("Message")
		);
	});
}

test("extension.ts binds exactly thirteen message calls, one per declared channel per host", () => {
	const bound = messageBindings();
	// Thirteen is forced rather than chosen. The return-type clause below
	// forbids a shared bindings helper spread into the factories, so each
	// spells its own: commandHost 4, workbenchCommandHost 3, columnCommandHost
	// 3, and draftCommandHost 3. An implementer meeting a different number
	// raises the criterion rather than binding fewer, because binding fewer
	// leaves a host short of the channels its own interface declares, which is
	// the hole the perimeter exists to close.
	//
	// draftCommandHost carries three rather than four because it reports and
	// confirms but never warns: DraftHost declares showError, showInfo and
	// confirmDestructive, and the one destructive act it offers is Discard.
	assert.equal(
		bound.length,
		13,
		`extension.ts holds ${String(bound.length)} vscode.window message calls`,
	);

	const channels = [...REPORT_CHANNELS, ...PROMPT_CHANNELS];
	const names: string[] = [];
	for (const call of bound) {
		const assignment = ancestor(call, ts.isPropertyAssignment) as
			| ts.PropertyAssignment
			| undefined;
		assert.notEqual(assignment, undefined, `${call.getText()} sits in no property`);
		const name = assignment?.name.getText() ?? "";
		assert.ok(
			channels.includes(name as (typeof channels)[number]),
			`${name} is a message binding on no declared channel`,
		);
		names.push(name);

		const owner = ancestor(
			call,
			(node) => ts.isFunctionDeclaration(node) && node.type !== undefined,
		) as ts.FunctionDeclaration | undefined;
		const returns = owner?.type?.getText() ?? "";
		assert.ok(
			["CommandHost", "WorkbenchCommandHost", "ColumnCommandHost", "DraftHost"].includes(
				returns,
			),
			`a message binding sits in a function returning ${returns}`,
		);
	}
	assert.deepEqual(
		[...new Set(names)].sort(),
		["confirmDestructive", "showError", "showInfo", "showWarning"],
	);
});

test("all three host interfaces extend ReporterHost, and the two channel sets are disjoint", () => {
	const heritage = new Map<string, string[]>();
	for (const module of ["cardCommands.ts", "workbenchCommands.ts", "columnCommands.ts"]) {
		for (const node of nodes(parse(join(srcDir, module)))) {
			if (!ts.isInterfaceDeclaration(node)) {
				continue;
			}
			heritage.set(
				node.name.text,
				(node.heritageClauses ?? []).flatMap((clause) =>
					clause.types.map((type) => type.expression.getText()),
				),
			);
		}
	}
	for (const host of ["CommandHost", "WorkbenchCommandHost", "ColumnCommandHost"]) {
		assert.ok(
			heritage.get(host)?.includes("ReporterHost"),
			`${host} does not name ReporterHost in a heritage clause`,
		);
	}
	assert.equal(REPORT_CHANNELS.length, 3);
	const shared = REPORT_CHANNELS.filter((name) =>
		(PROMPT_CHANNELS as readonly string[]).includes(name),
	);
	assert.deepEqual(shared, []);
});

// ---------------------------------------------------------------------------
// AC-34: the report-channel call sites in the command modules
// ---------------------------------------------------------------------------

/** Every .ts module under src/, excluding the generated and locale trees. */
function sources(): string[] {
	const found: string[] = [];
	const walk = (dir: string): void => {
		for (const entry of readdirSync(dir, { withFileTypes: true })) {
			const here = join(dir, entry.name);
			if (entry.isDirectory()) {
				if (entry.name !== "generated" && entry.name !== "locales") {
					walk(here);
				}
				continue;
			}
			if (entry.name.endsWith(".ts")) {
				found.push(here);
			}
		}
	};
	walk(srcDir);
	return found.sort();
}

test("the command modules hold exactly thirty-five report-channel call sites", () => {
	// A tripwire rather than a correctness check. A per-row report site added
	// after this card cannot land silently, because its author has to raise
	// this figure and, in doing so, decide whether the new site belongs inside
	// a run. It is an exact figure rather than a floor, because a floor would
	// pass a run that lost sites and losing a site is how a command stops
	// reporting at all.
	const sites: string[] = [];
	const files = new Set<string>();
	for (const file of sources()) {
		const relative = file.slice(srcDir.length + 1).split("\\").join("/");
		if (relative === "bulk.ts" || relative === "extension.ts") {
			continue;
		}
		for (const call of calls(parse(file))) {
			const callee = call.expression;
			if (!ts.isPropertyAccessExpression(callee)) {
				continue;
			}
			if (!(REPORT_CHANNELS as readonly string[]).includes(callee.name.text)) {
				continue;
			}
			sites.push(`${relative}: ${callee.getText()}`);
			files.add(relative);
		}
	}
	assert.equal(
		sites.length,
		35,
		`the command modules hold ${String(sites.length)} report-channel call sites:\n${sites.join("\n")}`,
	);
	// Stated as at least six, which is what the criterion declares, so a file
	// legitimately losing its last call does not redden the sweep while a walk
	// that read almost nothing still does. Nine is the figure today, after
	// dinah-506 added commentDrafts.ts and itemCommands.ts, and dinah-519
	// added openAnchorFile's own showError in itemCommands.ts.
	assert.ok(
		files.size >= 6,
		`the sites are spread over ${String(files.size)} files: ${[...files].join(", ")}`,
	);
});

// ---------------------------------------------------------------------------
// AC-17: the prose the change falsified is gone
// ---------------------------------------------------------------------------

test("no comment in src/ claims the view is single-select or that a mixed drag sets no entry", () => {
	// The weaker kind of check, and it says so: a search finds copies of a
	// phrase rather than copies of a claim, and a stale sentence spelled some
	// other way passes it. What makes the claim survivable is that neither
	// sentence is load-bearing after this card, because AC-1 reads the call
	// site itself. The positive half, that the three replacement comments
	// describe what a multi-row drag does, is a human read at Test.
	const stale = [
		"The view does not set canSelectMany",
		"sets no entry at all, so a drop afterward is indistinguishable",
		"draggable for reordering",
	];
	const found: string[] = [];
	let scanned = 0;
	for (const file of sources()) {
		scanned += 1;
		const body = readFileSync(file, "utf8");
		for (const sentence of stale) {
			if (body.includes(sentence)) {
				found.push(`${file.slice(srcDir.length + 1)}: ${sentence}`);
			}
		}
	}
	assert.ok(scanned > 0, "no module was scanned at all");
	assert.deepEqual(found, [], found.join("\n"));
});

test("the drag module's three comments describe a drag of several rows", () => {
	const body = readFileSync(join(srcDir, "dragAndDrop.ts"), "utf8");
	assert.ok(body.includes("The view selects many"), "the header says nothing about multi-select");
	assert.ok(
		body.includes("Every dragged row, in the order dragged, one entry per row."),
		"dragRowsFor's own comment does not describe what it answers",
	);
	assert.ok(
		body.includes("when at least one of them is a card"),
		"offerDrag's own comment does not describe when it sets the entry",
	);
});
