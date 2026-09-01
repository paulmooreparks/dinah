// The tool's own release number is displayed and never compared.
//
// version.ts states the rule in its header comment and nothing enforced it.
// `tool` reads "0.1.0" in every build dinah is compiled from source, and only
// a release build overwrites it, so a gate reading it refuses every
// contributor's own binary while passing every released one. `format` and
// `profile` are what this extension compares, and a second implementation of
// the format can answer both of them.
//
// This is the TypeScript half of the pair. cmd/dinah/version_guard_test.go is
// the Go half, and the two are written to the same shape on purpose.

import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { test } from "node:test";
import * as ts from "typescript";

const extensionRoot = join(__dirname, "..", "..", "..");
const srcRoot = join(extensionRoot, "src");

/** The six operators that compare two values. */
const COMPARISONS: ReadonlySet<ts.SyntaxKind> = new Set([
	ts.SyntaxKind.EqualsEqualsEqualsToken,
	ts.SyntaxKind.ExclamationEqualsEqualsToken,
	ts.SyntaxKind.LessThanToken,
	ts.SyntaxKind.LessThanEqualsToken,
	ts.SyntaxKind.GreaterThanToken,
	ts.SyntaxKind.GreaterThanEqualsToken,
]);

/** Every .ts file beneath a directory. */
function walk(dir: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(dir)) {
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) {
			found.push(...walk(full));
		} else if (entry.endsWith(".ts")) {
			found.push(full);
		}
	}
	return found;
}

/** Whether an expression reads a member named `tool` off something. */
function readsTool(node: ts.Expression): boolean {
	return ts.isPropertyAccessExpression(node) && node.name.text === "tool";
}

/** One site the sweep found, named the way a reader opens it. */
interface Site {
	readonly where: string;
	readonly text: string;
}

/**
 * Sweeps src/ and reports every comparison of a `.tool` member, together with
 * how many `.tool` reads of any kind were seen. The second number is what
 * keeps a green result meaningful: a sweep that finds no reads at all has
 * stopped looking at the member it was written for.
 */
function sweep(): { comparisons: Site[]; reads: number } {
	const comparisons: Site[] = [];
	let reads = 0;
	for (const file of walk(srcRoot)) {
		const rel = relative(srcRoot, file).split("\\").join("/");
		const text = readFileSync(file, "utf8");
		const source = ts.createSourceFile(rel, text, ts.ScriptTarget.Latest, true);
		const visit = (node: ts.Node): void => {
			if (ts.isPropertyAccessExpression(node) && node.name.text === "tool") {
				reads += 1;
			}
			if (ts.isBinaryExpression(node) && COMPARISONS.has(node.operatorToken.kind)) {
				const operands = [node.left, node.right];
				// `typeof report.tool !== "string"` is how readReport decides
				// whether a payload carries the member at all, which is a
				// check on the shape of an answer rather than a gate on the
				// release number in it.
				const guardsShape = operands.some((operand) => ts.isTypeOfExpression(operand));
				if (!guardsShape && operands.some((operand) => readsTool(operand))) {
					const line = source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1;
					comparisons.push({ where: `${rel}:${String(line)}`, text: node.getText(source) });
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(source);
	}
	return { comparisons, reads };
}

/**
 * What this guard does not catch, stated here rather than left for a later
 * reader to discover the hard way. It does not see a value copied into an
 * intermediate variable before the comparison, so `const t = version.tool; if
 * (t < floor)` passes. It does not see a comparison expressed through a call
 * rather than an operator, so `semverLess(version.tool, floor)`,
 * `version.tool.startsWith(...)` and a membership test against a set of
 * allowed release strings all pass. This is the class of limitation dinah-343
 * already tracks for guards that recognise a shape rather than a meaning, and
 * a green run here is not a proof that nothing anywhere compares the release
 * number some third way.
 */
test("nothing under src compares the tool's own release number", () => {
	const { comparisons } = sweep();
	assert.deepEqual(
		comparisons,
		[],
		`these sites compare the release number, which is displayed and never compared: ${comparisons
			.map((site) => `${site.where} ${site.text}`)
			.join(", ")}`,
	);
});

test("the sweep still sees the member, so the check is not vacuous", () => {
	// An absence assertion that passes because nothing anywhere reads the
	// member would go on passing after the member was renamed, which is the
	// failure mode spawn-sites.test.ts guards against with the same companion.
	const { reads } = sweep();
	assert.ok(
		reads > 0,
		"the sweep found no read of a `.tool` member anywhere under src, so the comparison check asserted nothing",
	);
});
