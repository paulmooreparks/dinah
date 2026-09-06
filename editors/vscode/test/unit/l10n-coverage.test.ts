// The completeness proof: no English sentence left standing at a call site
// that shows one to a reader.
//
// A list of converted call sites written into a spec proves nothing about the
// next call site somebody adds, so the claim is produced by a command instead.
// This walks every module under src/ with the TypeScript compiler API and
// fails on a string or template literal sitting where a reader would see it.
//
// The positions it reads are the ones a person actually reads text in: the
// message a host shows, the prompt a host asks for, the placeholder over a
// quick pick, and the label, description, tooltip or placeholder of a row this
// tree draws. A literal anywhere else is not this guard's business, and it
// says so rather than sweeping the whole tree and calling everything a
// finding.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

const extensionRoot = join(__dirname, "..", "..", "..");
const srcDir = join(extensionRoot, "src");

/**
 * The modules this audit does not read, and why each is out.
 *
 * `generated/` is written by esbuild.mjs and carries no prose. `l10n.ts` is
 * the catalogue reader itself, so every message in the product passes through
 * it and a rule about call sites cannot apply to it.
 */
const EXCLUDED_DIRS = ["generated", "test"];
const EXCLUDED_FILES = ["l10n.ts"];

/** The host calls whose message argument a reader sees. */
const SOLE_ARGUMENT_CALLS = ["showError", "showInfo", "input"];

/** The row fields a reader sees on a tree row. */
const ROW_FIELDS = ["label", "description", "tooltip", "placeholder"];

/** Every .ts module under src/ this audit reads. */
function sources(): string[] {
	const found: string[] = [];
	const walk = (dir: string, relative: string): void => {
		for (const entry of readdirSync(dir, { withFileTypes: true })) {
			const here = join(dir, entry.name);
			if (entry.isDirectory()) {
				if (!EXCLUDED_DIRS.includes(entry.name)) {
					walk(here, `${relative}${entry.name}/`);
				}
				continue;
			}
			if (!entry.name.endsWith(".ts") || EXCLUDED_FILES.includes(entry.name)) {
				continue;
			}
			found.push(here);
		}
	};
	walk(srcDir, "");
	return found.sort();
}

/**
 * Whether a node is a literal carrying a word.
 *
 * A template's own text spans are read rather than its interpolations, so
 * `${path}\n${detail}` is composition of two values around a separator and
 * carries no authored sentence, while `held by ${holder}` carries one. An
 * empty string and a bare separator are both left alone for the same reason:
 * there is no word in either for a translator to render.
 */
function carriesAWord(node: ts.Node): boolean {
	if (ts.isStringLiteralLike(node)) {
		return /\p{L}/u.test(node.text);
	}
	if (ts.isTemplateExpression(node)) {
		const spans = [node.head.text, ...node.templateSpans.map((s) => s.literal.text)];
		return spans.some((text) => /\p{L}/u.test(text));
	}
	return false;
}

/**
 * Whether an expression is one this rule allows.
 *
 * A call to the localizer is the point of the exercise. A call to
 * refusalMessage relays the CLI's own wire text, which the CLI itself spells
 * in English under every language setting, so translating this extension's
 * relay of it would show a reader a sentence the tool never said. A bare
 * identifier or property access is a value being passed through: a reference,
 * a path, or a sentence somebody else already composed.
 *
 * A literal is never any of these, which is the point. The clause is written
 * out rather than left implicit so that a reader comparing this code against
 * the criterion it implements finds the same four cases in both.
 */
function isExempt(node: ts.Node): boolean {
	if (ts.isIdentifier(node) || ts.isPropertyAccessExpression(node)) {
		return true;
	}
	if (ts.isCallExpression(node)) {
		const callee = node.expression.getText();
		return (
			callee === "t" || callee === "context.t" || callee === "refusalMessage"
		);
	}
	return false;
}

/**
 * The literals hiding inside a conditional or a fallback chain.
 *
 * `broken ? "damaged" : columnDescription(...)` puts the authored word one
 * level down from the property, and reading only the property's own node would
 * miss it. Only the shapes that choose between alternatives are descended, so
 * this widens what the audit sees without turning it into a sweep of every
 * expression in the tree.
 */
function candidates(node: ts.Expression): ts.Node[] {
	if (ts.isParenthesizedExpression(node)) {
		return candidates(node.expression);
	}
	if (ts.isConditionalExpression(node)) {
		return [...candidates(node.whenTrue), ...candidates(node.whenFalse)];
	}
	if (
		ts.isBinaryExpression(node) &&
		(node.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken ||
			node.operatorToken.kind === ts.SyntaxKind.BarBarToken)
	) {
		return [...candidates(node.left), ...candidates(node.right)];
	}
	return [node];
}

interface Finding {
	readonly where: string;
	readonly text: string;
}

/** What one file's walk found, plus how many positions it looked at. */
interface Audit {
	readonly findings: Finding[];
	readonly inspected: number;
	readonly rowFunctions: number;
}

function auditFile(file: string): Audit {
	const text = readFileSync(file, "utf8");
	const source = ts.createSourceFile(
		file,
		text,
		ts.ScriptTarget.ES2022,
		true,
		ts.ScriptKind.TS,
	);
	const findings: Finding[] = [];
	let inspected = 0;
	let rowFunctions = 0;

	const at = (node: ts.Node): string => {
		const { line } = source.getLineAndCharacterOfPosition(node.getStart(source));
		return `${file.slice(extensionRoot.length + 1)}:${String(line + 1)}`;
	};

	const inspect = (position: ts.Expression, label: string): void => {
		inspected += 1;
		for (const candidate of candidates(position)) {
			if (isExempt(candidate)) {
				continue;
			}
			if (carriesAWord(candidate)) {
				findings.push({ where: `${at(candidate)} (${label})`, text: candidate.getText() });
			}
		}
	};

	/** Whether a call's receiver is one of the injected hosts. */
	const onAHost = (callee: ts.PropertyAccessExpression): boolean =>
		/(^|\.)host$/.test(callee.expression.getText());

	const visitCall = (node: ts.CallExpression): void => {
		if (!ts.isPropertyAccessExpression(node.expression)) {
			return;
		}
		const callee = node.expression;
		if (!onAHost(callee)) {
			return;
		}
		const name = callee.name.text;
		const args = node.arguments;
		if (SOLE_ARGUMENT_CALLS.includes(name) && args.length >= 1) {
			inspect(args[0], `host.${name}`);
			return;
		}
		if (name === "showWarning" && args.length >= 1) {
			inspect(args[0], "host.showWarning message");
			return;
		}
		if (name === "pick" && args.length >= 2) {
			inspect(args[1], "host.pick placeholder");
		}
	};

	/** Whether a function says in its own signature that it composes a row. */
	const composesARow = (node: ts.Node): boolean => {
		const typed = node as ts.SignatureDeclaration;
		return (
			typed.type !== undefined && typed.type.getText().includes("TreeItemSpec")
		);
	};

	const visitRowFunction = (node: ts.Node): void => {
		rowFunctions += 1;
		const walkRow = (inner: ts.Node): void => {
			if (
				ts.isPropertyAssignment(inner) &&
				ts.isIdentifier(inner.name) &&
				ROW_FIELDS.includes(inner.name.text)
			) {
				inspect(inner.initializer, `row field ${inner.name.text}`);
			}
			ts.forEachChild(inner, walkRow);
		};
		ts.forEachChild(node, walkRow);
	};

	const visit = (node: ts.Node): void => {
		if (ts.isCallExpression(node)) {
			visitCall(node);
		}
		if (
			(ts.isFunctionDeclaration(node) ||
				ts.isMethodDeclaration(node) ||
				ts.isArrowFunction(node) ||
				ts.isFunctionExpression(node)) &&
			composesARow(node)
		) {
			visitRowFunction(node);
		}
		ts.forEachChild(node, visit);
	};
	visit(source);

	return { findings, inspected, rowFunctions };
}

test("no English sentence is left standing where a reader would see it", () => {
	const files = sources();
	assert.ok(files.length > 0, "the audit found no modules to read");

	const findings: Finding[] = [];
	let inspected = 0;
	let rowFunctions = 0;
	for (const file of files) {
		const audit = auditFile(file);
		findings.push(...audit.findings);
		inspected += audit.inspected;
		rowFunctions += audit.rowFunctions;
	}

	// A vacuous pass is the failure mode a coverage audit has, and it has two
	// shapes: a parser that matched nothing, and a rule that selected nothing.
	// Both are reported here rather than left to look like a clean sweep.
	assert.ok(
		rowFunctions > 0,
		"no function declaring a TreeItemSpec return was found, so the row half of this rule read nothing",
	);
	assert.ok(
		inspected > 0,
		"no reader-facing position was found at all, so this audit read nothing",
	);

	assert.deepEqual(
		findings.map((finding) => `${finding.where}: ${finding.text}`),
		[],
		"each line above is English a reader sees; route it through the injected localizer",
	);
});
