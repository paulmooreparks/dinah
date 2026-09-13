// The loop, the report it builds, and the one message a run produces.
//
// dinah-490 AC-20 through AC-28, AC-32 and AC-36. Everything here drives
// src/bulk.ts directly: the per-command wiring is held by
// test/unit/rowCommands.test.ts and the registration boundary by
// test/unit/wiring.test.ts.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

import type { BulkReport, RowOutcome } from "../../src/bulk";
import { collectingHost, runBulk, runOverRows, summaryFor } from "../../src/bulk";
import type { CommandHost } from "../../src/cardCommands";
import type { ColumnCommandHost } from "../../src/columnCommands";
import { ENGLISH } from "../../src/l10n";
import { REPORT_CHANNELS } from "../../src/reporter";
import type { WorkbenchCommandHost } from "../../src/workbenchCommands";
import { cardHost, columnHost, emptyLog, workbenchHost } from "../support/rows";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const srcDir = join(extensionRoot, "src");

const DONE: RowOutcome = { kind: "done" };

/** The reference a row of this file's fixtures carries. */
const refOf = (row: string): string => row;

// ---------------------------------------------------------------------------
// AC-20: one entry per row, attributed to that row, at every length
// ---------------------------------------------------------------------------

test("runOverRows records one entry per row, in order, at every length and every mix", async () => {
	// The invariant the whole design turns on, asserted against the module
	// that is supposed to establish it by construction. The comparison is
	// index by index rather than by count, so a run that lost row three and
	// gained a duplicate of row four adds up and still fails here.
	const kinds = ["done", "failed", "skipped", "throw"] as const;
	let exercised = 0;
	for (let length = 0; length <= 5; length += 1) {
		// Every assignment of four outcomes to `length` rows, read off the
		// base-four digits of the combination's own index.
		const total = 4 ** length;
		for (let combination = 0; combination < total; combination += 1) {
			const rows = Array.from({ length }, (_unused, index) => `row-${String(index)}`);
			const plan = rows.map((_row, index) => kinds[Math.floor(combination / 4 ** index) % 4]);
			const report = await runOverRows(rows, refOf, async (row): Promise<RowOutcome> => {
				const kind = plan[rows.indexOf(row)];
				if (kind === "throw") {
					throw new Error(`${row} threw`);
				}
				if (kind === "failed") {
					return { kind: "failed", failure: `${row} refused` };
				}
				if (kind === "skipped") {
					return { kind: "skipped", why: "not this kind of row" };
				}
				return DONE;
			});
			assert.equal(report.selected, rows.length);
			assert.equal(report.entries.length, report.selected);
			for (const [index, entry] of report.entries.entries()) {
				assert.equal(entry.ref, refOf(rows[index]));
			}
			exercised += 1;
		}
	}
	// A generator with an off-by-one bound produces a universal claim over an
	// empty set, which is true, so the number of runs is asserted against the
	// number the bound promises.
	const expected = [0, 1, 2, 3, 4, 5].reduce((sum, length) => sum + 4 ** length, 0);
	assert.equal(exercised, expected);
	assert.equal(exercised, 1365);
});

// ---------------------------------------------------------------------------
// AC-21: a rejecting act is one failed row, not a lost run
// ---------------------------------------------------------------------------

test("an act that rejects is recorded as failed and the loop advances", async () => {
	const rows = ["one", "two", "three"];
	const report = await runOverRows(rows, refOf, async (row) => {
		if (row === "two") {
			throw new Error("the editor refused to open it");
		}
		return DONE;
	});
	assert.equal(report.entries.length, 3);
	assert.deepEqual(report.entries[1].outcome, {
		kind: "failed",
		failure: "the editor refused to open it",
	});
	// The third entry is what proves the loop advanced rather than that the
	// rejection was merely swallowed.
	assert.deepEqual(report.entries[2].outcome, DONE);
	const summary = summaryFor(report, ENGLISH);
	assert.equal(summary.level, "warning");
	assert.ok(summary.message.includes("1"), summary.message);
});

test("a rejection carrying something other than an Error still records readable text", async () => {
	const report = await runOverRows(["one"], refOf, async () => {
		// A rejection carrying a string rather than an Error, which is what a
		// spawner outside this extension can hand back.
		throw "the spawner answered a string";
	});
	assert.deepEqual(report.entries[0].outcome, {
		kind: "failed",
		failure: "the spawner answered a string",
	});
});

// ---------------------------------------------------------------------------
// AC-23: a report that cannot add up produces no message at all
// ---------------------------------------------------------------------------

test("a report whose selected count was raised after the fact throws rather than reporting", async () => {
	const report = await runOverRows(["a", "b", "c"], refOf, async () => DONE);
	// Spread rather than a hand-built literal, because object spread copies
	// own symbol keys: the copy carries the report mark and reaches the
	// reconciliation arm rather than being refused by the mark arm, which is
	// the attack this criterion is about.
	const forged: BulkReport = { ...report, selected: 5 };
	assert.throws(
		() => summaryFor(forged, ENGLISH),
		(err: Error) => {
			assert.ok(err.message.includes("5"), err.message);
			assert.ok(err.message.includes("3"), err.message);
			return true;
		},
	);
	// The accepting partner: a summaryFor that threw on everything would
	// satisfy the half above.
	assert.equal(summaryFor(report, ENGLISH).level, "info");
});

// ---------------------------------------------------------------------------
// AC-25: a report is a value only src/bulk.ts can mint
// ---------------------------------------------------------------------------

test("a report the bulk layer did not build produces no message, however well formed", async () => {
	const real = await runOverRows(["a", "b"], refOf, async () => DONE);
	// Every field the interface names, all of them consistent, and no mark.
	const fabricated = {
		selected: 2,
		entries: real.entries,
		cancelled: false,
		emptyRun: "speak",
		notes: [],
	} as unknown as BulkReport;
	assert.throws(
		() => summaryFor(fabricated, ENGLISH),
		(err: Error) => {
			assert.ok(
				err.message.includes("not built by the bulk layer"),
				err.message,
			);
			return true;
		},
	);
	assert.equal(summaryFor(real, ENGLISH).level, "info");
});

test("the report mark is not exported, so nothing outside bulk.ts can obtain one", () => {
	// Read off the module's own export statements rather than off a runtime
	// import, because exporting it would reopen the door while leaving every
	// runtime assertion above green.
	const file = join(srcDir, "bulk.ts");
	const source = ts.createSourceFile(
		file,
		readFileSync(file, "utf8"),
		ts.ScriptTarget.ES2022,
		true,
		ts.ScriptKind.TS,
	);
	const exported: string[] = [];
	for (const statement of source.statements) {
		const modifiers = ts.canHaveModifiers(statement)
			? (ts.getModifiers(statement) ?? [])
			: [];
		if (!modifiers.some((m) => m.kind === ts.SyntaxKind.ExportKeyword)) {
			continue;
		}
		if (ts.isVariableStatement(statement)) {
			for (const declaration of statement.declarationList.declarations) {
				if (ts.isIdentifier(declaration.name)) {
					exported.push(declaration.name.text);
				}
			}
			continue;
		}
		if (
			(ts.isFunctionDeclaration(statement) ||
				ts.isInterfaceDeclaration(statement) ||
				ts.isTypeAliasDeclaration(statement)) &&
			statement.name !== undefined
		) {
			exported.push(statement.name.text);
		}
	}
	// The walk found something, so an empty result is a real answer rather
	// than a walk that read nothing.
	assert.ok(exported.length > 0, "no export statement was read at all");
	assert.ok(exported.includes("summaryFor"), exported.join(", "));
	assert.ok(!exported.includes("reportMark"), exported.join(", "));
});

// ---------------------------------------------------------------------------
// AC-24: a declined prompt cancels the whole run, and says nothing more
// ---------------------------------------------------------------------------

test("a run whose ask answers undefined spawns nothing and reconciles anyway", async () => {
	const log = emptyLog();
	const host = cardHost(log);
	const rows = ["a", "b", "c", "d", "e"];
	let acted = 0;
	const report = await runBulk<string, string, true, CommandHost>(
		rows,
		refOf,
		(row) => row,
		{ host, t: ENGLISH, skipReason: "not this kind of row" },
		async () => undefined,
		async () => {
			acted += 1;
			return DONE;
		},
	);
	assert.equal(acted, 0);
	assert.equal(report.selected, 5);
	assert.equal(report.entries.length, 5);
	assert.ok(report.entries.every((entry) => entry.outcome.kind === "skipped"));
	assert.equal(report.cancelled, true);
	// The cancelled report reconciles like any other, so the shape that used
	// to throw is no longer spellable.
	assert.equal(summaryFor(report, ENGLISH).level, "none");
	assert.deepEqual(log.infos, []);
	assert.deepEqual(log.warnings, []);
});

test("a hand-built cancelled report still throws, so cancellation is no hole", () => {
	const fabricated = {
		selected: 5,
		entries: [],
		cancelled: true,
	} as unknown as BulkReport;
	assert.throws(() => summaryFor(fabricated, ENGLISH));
});

// ---------------------------------------------------------------------------
// AC-27: one targeted row that yields no context writes a line and shows nothing
// ---------------------------------------------------------------------------

test("a single unusable row writes one channel line and shows no message", async () => {
	const log = emptyLog();
	const host = cardHost(log);
	let acted = 0;
	const report = await runBulk<string, string, true, CommandHost>(
		["a note row"],
		refOf,
		() => undefined,
		{ host, t: ENGLISH, skipReason: "names no card" },
		async () => true,
		async () => {
			acted += 1;
			return DONE;
		},
	);
	assert.equal(acted, 0);
	assert.deepEqual(log.errors, []);
	assert.deepEqual(log.infos, []);
	assert.deepEqual(log.warnings, []);
	assert.deepEqual(log.lines, ["a note row: names no card"]);
	assert.equal(report.selected, 1);
	assert.equal(report.entries.length, 1);
	assert.equal(report.entries[0].outcome.kind, "skipped");
	assert.equal(summaryFor(report, ENGLISH).level, "none");
});

// ---------------------------------------------------------------------------
// AC-36: a failing run writes one line per row and shows one message
// ---------------------------------------------------------------------------

test("three failing rows write three lines and show exactly one message", async () => {
	const log = emptyLog();
	const host = cardHost(log);
	const rows = ["one", "two", "three"];
	const report = await runBulk<string, string, true, CommandHost>(
		rows,
		refOf,
		(row) => row,
		{ host, t: ENGLISH, skipReason: "names no card" },
		async () => true,
		async (row, _answer, running) => {
			// The act shows its own error and records the same text, which is
			// what every act-reachable site in src/ does today. The collected
			// error is discarded, so the channel carries one line per row
			// rather than two.
			const text = `${row} refused`;
			running.showError(text);
			return { kind: "failed", failure: text };
		},
	);
	assert.deepEqual(log.lines, [
		"one: one refused",
		"two: two refused",
		"three: three refused",
	]);
	assert.equal(shownCount(log.errors, log.infos, log.warnings), 1);
	assert.deepEqual(log.errors, []);
	assert.equal(log.warnings.length, 1);
	assert.equal(
		log.warnings[0],
		ENGLISH("dialog.bulk.partial", {
			succeeded: "0",
			selected: "3",
			refused: "3",
			skipped: "0",
		}),
	);
	assert.equal(report.entries.length, 3);
});

/** How many messages of any level reached the reader. */
function shownCount(
	errors: readonly string[],
	infos: readonly string[],
	warnings: readonly string[],
): number {
	return errors.length + infos.length + warnings.length;
}

// ---------------------------------------------------------------------------
// AC-32: the refusal reaches the reader, and every host shape is collected
// ---------------------------------------------------------------------------

test("a refusal stated inside ask reaches the reader, and the summary stays silent", async () => {
	// The ask runs on the real host, before the loop and before the wrapping.
	// Handing it the collecting host makes a refused run say nothing at all,
	// which no assertion stated over the summary could see.
	const log = emptyLog();
	const host = cardHost(log);
	const report = await runBulk<string, string, true, CommandHost>(
		["a", "b"],
		refOf,
		(row) => row,
		{ host, t: ENGLISH, skipReason: "names no card" },
		async (_resolved, real) => {
			real.showError("Dinah runs this command on one row at a time.");
			return undefined;
		},
		async () => DONE,
	);
	assert.deepEqual(log.errors, ["Dinah runs this command on one row at a time."]);
	assert.equal(shownCount([], log.infos, log.warnings), 0);
	assert.equal(summaryFor(report, ENGLISH).level, "none");
});

/** One host shape, built with the members that shape declares in the source. */
interface Shape {
	readonly name: string;
	/** The PROMPT_CHANNELS members this shape actually declares. */
	readonly prompts: readonly string[];
	readonly build: () => CommandHost | WorkbenchCommandHost | ColumnCommandHost;
}

test("every report channel is collected on every host shape, and the prompts are not", async () => {
	// Three shapes that differ in what they carry rather than one case wearing
	// three names. Only CommandHost declares pick, input and
	// confirmDestructive today, so the prompt half of this reaches that shape
	// alone and says so: for the other two, AC-31's heritage and return-type
	// clauses are what hold their shape instead.
	const shapes: readonly Shape[] = [
		{
			name: "CommandHost",
			prompts: ["pick", "input", "confirmDestructive"],
			build: () => cardHost(emptyLog()),
		},
		{ name: "WorkbenchCommandHost", prompts: [], build: () => workbenchHost(emptyLog()) },
		{ name: "ColumnCommandHost", prompts: [], build: () => columnHost(emptyLog()) },
	];
	assert.equal(shapes.length, 3);
	for (const shape of shapes) {
		const log = emptyLog();
		const real =
			shape.name === "CommandHost"
				? cardHost(log)
				: shape.name === "WorkbenchCommandHost"
					? workbenchHost(log)
					: columnHost(log);
		const wrapped = collectingHost(real);
		// Every member REPORT_CHANNELS names, read from the source's own set
		// rather than from a list repeated here.
		assert.deepEqual([...REPORT_CHANNELS], ["showError", "showInfo", "showWarning"]);
		wrapped.host.showError(`${shape.name} error`);
		wrapped.host.showInfo(`${shape.name} info`);
		const answered = await wrapped.host.showWarning(`${shape.name} warning`, [
			"Open Output",
		]);
		// The editor answers undefined when a reader dismisses a toast, so a
		// caller that reveals the channel on assent reveals nothing.
		assert.equal(answered, undefined, shape.name);
		assert.deepEqual(log.errors, [], shape.name);
		assert.deepEqual(log.infos, [], shape.name);
		assert.deepEqual(log.warnings, [], shape.name);
		const drained = wrapped.drain();
		assert.deepEqual(drained.errors, [`${shape.name} error`], shape.name);
		assert.deepEqual(
			drained.notes,
			[
				{ level: "info", text: `${shape.name} info` },
				{ level: "warning", text: `${shape.name} warning` },
			],
			shape.name,
		);
		// The refusing partner: a wrapper that intercepted everything would
		// satisfy the capture clauses above and fail here. The number compared
		// is asserted per shape, so a shape declaring none reports zero rather
		// than reading as a pass on undefined === undefined.
		const carrier = real as unknown as Record<string, unknown>;
		const wrapper = wrapped.host as unknown as Record<string, unknown>;
		const compared = shape.prompts.filter(
			(member) => carrier[member] !== undefined && wrapper[member] === carrier[member],
		);
		assert.deepEqual(compared, [...shape.prompts], shape.name);
		assert.equal(compared.length, shape.prompts.length, shape.name);
	}
});

test("the collecting host records a checkpoint's folder rather than running it", async () => {
	const log = emptyLog();
	const wrapped = collectingHost(cardHost(log));
	await wrapped.host.checkpoint("C:/work");
	await wrapped.host.checkpoint("C:/work");
	assert.deepEqual(log.checkpoints, []);
	assert.deepEqual(wrapped.drain().folders, ["C:/work", "C:/work"]);
});

// ---------------------------------------------------------------------------
// AC-26 and AC-28: the perimeter is one caller, and the silence is one site
// ---------------------------------------------------------------------------

/** Every .ts module under src/, which both sweeps below walk. */
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

/** The files whose AST holds an identifier of this name. */
function filesNaming(name: string): string[] {
	const found: string[] = [];
	for (const file of sources()) {
		const source = ts.createSourceFile(
			file,
			readFileSync(file, "utf8"),
			ts.ScriptTarget.ES2022,
			true,
			ts.ScriptKind.TS,
		);
		let seen = false;
		const visit = (node: ts.Node): void => {
			if (ts.isIdentifier(node) && node.text === name) {
				seen = true;
			}
			ts.forEachChild(node, visit);
		};
		visit(source);
		if (seen) {
			found.push(file.slice(srcDir.length + 1).split("\\").join("/"));
		}
	}
	return found;
}

test("the summary and the host switch have one caller each, and the entry point has many", () => {
	// A future command cannot compose its own message or choose its own host
	// without this going red. The runBulk floor is the half that catches a
	// narrowed walk: a sweep scoped to bulk.ts alone finds both of the others
	// in exactly one file and reports a clean perimeter it never looked at.
	assert.deepEqual(filesNaming("summaryFor"), ["bulk.ts"]);
	assert.deepEqual(filesNaming("collectingHost"), ["bulk.ts"]);
	const callers = filesNaming("runBulk");
	assert.ok(callers.includes("bulk.ts"), callers.join(", "));
	assert.ok(
		callers.length >= 6,
		`runBulk is named in ${String(callers.length)} files: ${callers.join(", ")}`,
	);
});

test("only the drop path declares a silent empty run", () => {
	// An exception any caller may declare is not an exception. The initialiser
	// is compared as a node rather than by searching for the word, so a
	// comment or a message key cannot fire it.
	const silent: string[] = [];
	let skipReasons = 0;
	for (const file of sources()) {
		const source = ts.createSourceFile(
			file,
			readFileSync(file, "utf8"),
			ts.ScriptTarget.ES2022,
			true,
			ts.ScriptKind.TS,
		);
		const visit = (node: ts.Node): void => {
			if (ts.isPropertyAssignment(node) && ts.isIdentifier(node.name)) {
				if (
					node.name.text === "emptyRun" &&
					ts.isStringLiteral(node.initializer) &&
					node.initializer.text === "silent"
				) {
					silent.push(file.slice(srcDir.length + 1).split("\\").join("/"));
				}
				if (node.name.text === "skipReason") {
					skipReasons += 1;
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(source);
	}
	assert.deepEqual(silent, ["dragAndDrop.ts"]);
	// The other half's own count. A collector right about one property name
	// and mis-keyed about the other reads plenty and reports nothing for the
	// half that is broken.
	assert.ok(skipReasons >= 1, "no skipReason property was collected at all");
});
