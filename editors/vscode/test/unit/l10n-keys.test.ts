// The other direction of the catalogue relation: the code asks for a name, and
// the catalogue has to carry it.
//
// dinah-379 gave the extension a catalogue and four guards that keep the eight
// translations honest against each other. None of them reads the code, so
// `t("tree.group.redy")` compiles, lints, packages and installs, and then
// localizerOver throws while the sidebar is being drawn. A reader meets a
// broken panel rather than a wrong word.
//
// This walks every module under src/ with the TypeScript compiler API, the
// same way l10n-coverage.test.ts does, and deliberately reuses that file's
// shape: the same exclusions, the same conditional descent, the same
// per-population floors. A reader comparing the two finds one idea in both.
//
// Four tests close the relation. Every key the code asks for is a key the
// catalogue carries. Every key expression the sweep meets resolves by a
// declared rule, which is what makes the first claim honest, because an
// expression the sweep cannot read is a key it silently does not check and
// that reads exactly like a key that checked out. Each declared key family
// names exactly the catalogue entries its table reaches. And every catalogue
// key is one the extension actually reaches, because a key nobody renders is
// dead weight seven translators still pay for.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import * as ts from "typescript";

import type { LocaleCatalog } from "../../src/l10n";

const extensionRoot = join(__dirname, "..", "..", "..");
const srcDir = join(extensionRoot, "src");

/**
 * The modules this sweep does not read, and why each is out.
 *
 * `generated/` is written by esbuild.mjs and asks for no key. `locales/` holds
 * the catalogues rather than code. `l10n.ts` is the catalogue reader itself,
 * so every message in the product passes through it and it names no key of its
 * own. `test` names no directory under `src/` today and excludes nothing; it is
 * carried verbatim from l10n-coverage.test.ts, whose list this one mirrors on
 * purpose, so that a test directory added under `src/` later is out of both
 * walks at once.
 */
const EXCLUDED_DIRS = ["generated", "locales", "test"];
const EXCLUDED_FILES = ["l10n.ts"];

/** A key spelled at run time out of a head this file declares and a table the code owns. */
interface KeyFamily {
	/** The literal head of the template, ending at the interpolation. */
	readonly prefix: string;
	/** The module carrying the table whose property names are the members. */
	readonly module: string;
	/** The module-level object literal whose property names are the members. */
	readonly table: string;
}

/**
 * The families, declared here rather than in src/.
 *
 * A family declares where its members come from and does not copy them:
 * familyMembers reads the named object literal's property names off the AST,
 * so HISTORY_ROWS stays the one place the event names are written. The two
 * hand-written fields both fail loudly when wrong. A prefix matching no
 * catalogue key fails the family test, and a template head matching no family
 * fails as an unresolvable key expression.
 *
 * This lives in the test layer because it is build-time data no reader's text
 * passes through, and src/ is bundled into dist/extension.js.
 */
const KEY_FAMILIES: readonly KeyFamily[] = [
	{ prefix: "history.event.", module: "servedText.ts", table: "HISTORY_ROWS" },
];

/** What one sweep read, and what it made of it. */
interface KeySweep {
	/** Modules read. */
	readonly modules: number;
	/** Localizer call sites matched. */
	readonly sites: number;
	/** Key expressions inspected, after the conditional descent. */
	readonly expressions: number;
	/** Every literal key resolved by rule 1 or rule 2, in source order. */
	readonly literals: string[];
	/** One `file:line key` per family template resolved by rule 3. */
	readonly family: string[];
	/** One `file:line expression` per key expression no rule resolves. */
	readonly unresolved: string[];
}

/**
 * Every .ts module under src/ this sweep reads.
 *
 * Taken from l10n-coverage.test.ts, which walks the same tree for a different
 * rule. The two exclusion lists differ by `locales`, which carries JSON rather
 * than code and so is invisible to a walk that only collects .ts.
 */
function sources(): string[] {
	const found: string[] = [];
	const walk = (dir: string): void => {
		for (const entry of readdirSync(dir, { withFileTypes: true })) {
			const here = join(dir, entry.name);
			if (entry.isDirectory()) {
				if (!EXCLUDED_DIRS.includes(entry.name)) {
					walk(here);
				}
				continue;
			}
			if (!entry.name.endsWith(".ts") || EXCLUDED_FILES.includes(entry.name)) {
				continue;
			}
			found.push(here);
		}
	};
	walk(srcDir);
	return found.sort();
}

/**
 * Whether a call is a call to the injected localizer.
 *
 * The callee's name decides, and no receiver spelling is whitelisted. Four
 * spellings are live: the bare `t`, `context.host.t`, `host.t` and
 * `this.deps.t`. A whitelist that missed a fifth would skip those call sites
 * in silence, which is the exact failure this file exists to close, so the
 * rule errs toward reading more rather than fewer. An unrelated function named
 * `t` has its argument reported as an unresolvable key expression, and the
 * remedy is to rename that function rather than to exempt it.
 */
function isLocalizerCall(node: ts.CallExpression): boolean {
	const callee = node.expression;
	if (ts.isIdentifier(callee)) {
		return callee.text === "t";
	}
	if (ts.isPropertyAccessExpression(callee)) {
		return callee.name.text === "t";
	}
	return false;
}

/**
 * The key expressions hiding inside a conditional or a fallback chain.
 *
 * This is l10n-coverage.test.ts's own `candidates` applied to the first
 * argument instead of to the message. runVerbCommand.ts passes a conditional
 * between two literal keys at three call sites, so without the descent three
 * key expressions would read as unresolvable.
 */
function keyCandidates(node: ts.Expression): ts.Expression[] {
	if (ts.isParenthesizedExpression(node)) {
		return keyCandidates(node.expression);
	}
	if (ts.isConditionalExpression(node)) {
		return [...keyCandidates(node.whenTrue), ...keyCandidates(node.whenFalse)];
	}
	if (
		ts.isBinaryExpression(node) &&
		(node.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken ||
			node.operatorToken.kind === ts.SyntaxKind.BarBarToken)
	) {
		return [...keyCandidates(node.left), ...keyCandidates(node.right)];
	}
	return [node];
}

/** The module-level `const NAME = "literal"` bindings of one source file. */
function moduleConstants(source: ts.SourceFile): Map<string, string> {
	const found = new Map<string, string>();
	for (const statement of source.statements) {
		if (!ts.isVariableStatement(statement)) {
			continue;
		}
		for (const declaration of statement.declarationList.declarations) {
			const initializer = declaration.initializer;
			if (
				ts.isIdentifier(declaration.name) &&
				initializer !== undefined &&
				ts.isStringLiteralLike(initializer)
			) {
				found.set(declaration.name.text, initializer.text);
			}
		}
	}
	return found;
}

/** The property names of a module-level object literal in one of src/'s modules. */
function familyMembers(family: KeyFamily): string[] {
	const file = join(srcDir, family.module);
	const source = ts.createSourceFile(
		file,
		readFileSync(file, "utf8"),
		ts.ScriptTarget.ES2022,
		true,
		ts.ScriptKind.TS,
	);
	const members: string[] = [];
	for (const statement of source.statements) {
		if (!ts.isVariableStatement(statement)) {
			continue;
		}
		for (const declaration of statement.declarationList.declarations) {
			if (
				!ts.isIdentifier(declaration.name) ||
				declaration.name.text !== family.table ||
				declaration.initializer === undefined ||
				!ts.isObjectLiteralExpression(declaration.initializer)
			) {
				continue;
			}
			for (const property of declaration.initializer.properties) {
				const name = property.name;
				if (name !== undefined && (ts.isIdentifier(name) || ts.isStringLiteralLike(name))) {
					members.push(name.text);
				}
			}
		}
	}
	return members.sort();
}

/** The key out of a `file:line key` entry, which is everything after the last space. */
function keyOf(entry: string): string {
	return entry.slice(entry.lastIndexOf(" ") + 1);
}

/** Sweeps the given modules, which the tests pass so a fixture can drive it. */
function sweepKeys(files: readonly string[]): KeySweep {
	const literals: string[] = [];
	const family: string[] = [];
	const unresolved: string[] = [];
	let sites = 0;
	let expressions = 0;

	for (const file of files) {
		const source = ts.createSourceFile(
			file,
			readFileSync(file, "utf8"),
			ts.ScriptTarget.ES2022,
			true,
			ts.ScriptKind.TS,
		);
		const constants = moduleConstants(source);
		const at = (node: ts.Node): string => {
			const { line } = source.getLineAndCharacterOfPosition(node.getStart(source));
			return `${file.slice(extensionRoot.length + 1)}:${String(line + 1)}`;
		};

		// The three rules, and anything else is a failure rather than a
		// silence: a string or no-substitution template literal, an identifier
		// bound to a module-level string const in the same file, and a
		// template whose head is a declared family's prefix with exactly one
		// interpolation and an empty tail.
		const resolve = (node: ts.Expression): void => {
			if (ts.isStringLiteralLike(node)) {
				literals.push(`${at(node)} ${node.text}`);
				return;
			}
			if (ts.isIdentifier(node)) {
				const bound = constants.get(node.text);
				if (bound !== undefined) {
					literals.push(`${at(node)} ${bound}`);
					return;
				}
			}
			if (
				ts.isTemplateExpression(node) &&
				node.templateSpans.length === 1 &&
				node.templateSpans[0].literal.text === "" &&
				KEY_FAMILIES.some((declared) => declared.prefix === node.head.text)
			) {
				family.push(`${at(node)} ${node.head.text}`);
				return;
			}
			unresolved.push(`${at(node)} ${node.getText()}`);
		};

		const visit = (node: ts.Node): void => {
			if (ts.isCallExpression(node) && isLocalizerCall(node) && node.arguments.length >= 1) {
				sites += 1;
				for (const candidate of keyCandidates(node.arguments[0])) {
					expressions += 1;
					resolve(candidate);
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(source);
	}

	return { modules: files.length, sites, expressions, literals, family, unresolved };
}

/** The base catalogue's keys, read off disk as the catalogue was committed. */
function catalogueKeys(): Set<string> {
	const catalog = JSON.parse(
		readFileSync(join(srcDir, "locales", "en.json"), "utf8"),
	) as LocaleCatalog;
	return new Set(Object.keys(catalog.entries));
}

/**
 * The floors, one per population, asserted before anything is compared.
 *
 * scripts/run-unit-tests.mjs says in its own comment that it cannot see one
 * file among several registering nothing, because Node reports such a file as
 * one passing test rather than as zero. So the floors inside this file are the
 * only thing standing between an empty sweep and a green run, and each
 * population gets its own rather than one combined counter standing in for
 * two.
 */
function floors(sweep: KeySweep, keys: Set<string>): void {
	assert.ok(sweep.modules > 0, "the sweep read no module at all");
	assert.ok(sweep.sites > 0, "the sweep matched no localizer call site");
	assert.ok(sweep.expressions > 0, "the sweep inspected no key expression");
	assert.ok(sweep.literals.length > 0, "the sweep resolved no literal key");
	assert.ok(keys.size > 0, "the catalogue read no key");
}

test("every message name the extension asks for is one the catalogue carries", () => {
	const sweep = sweepKeys(sources());
	const keys = catalogueKeys();
	floors(sweep, keys);

	assert.deepEqual(
		sweep.literals.filter((entry) => !keys.has(keyOf(entry))),
		[],
		"each line above names a file, a line and a key src/locales/en.json does not carry, which throws at a reader while the panel is being drawn",
	);
});

test("every key expression the sweep meets resolves by a declared rule", () => {
	const sweep = sweepKeys(sources());
	assert.ok(sweep.expressions > 0, "the sweep inspected no key expression");

	// This is what makes the test above honest. Without it, a key expression
	// the sweep cannot read is a key the sweep silently does not check, and
	// that reads exactly like a key that checked out.
	assert.deepEqual(
		sweep.unresolved,
		[],
		"each line above is a key expression no rule resolves; spell the key as a literal, bind it to a module-level const, or declare its family in KEY_FAMILIES",
	);
});

test("each declared key family names exactly the catalogue entries its table reaches", () => {
	const keys = catalogueKeys();
	const sweep = sweepKeys(sources());
	assert.ok(KEY_FAMILIES.length > 0, "no key family is declared");
	assert.ok(keys.size > 0, "the catalogue read no key");

	for (const family of KEY_FAMILIES) {
		const members = familyMembers(family);
		assert.ok(
			members.length > 0,
			`${family.table} in ${family.module} yielded no member, so this family read nothing`,
		);

		// A key under the prefix the code asks for directly belongs on the
		// reached side too: history.event.unknown is spelled out at a call
		// site rather than composed, so the family needs no hand-written
		// extras and carries none.
		const reached = new Set([
			...members.map((member) => `${family.prefix}${member}`),
			...sweep.literals
				.map((entry) => keyOf(entry))
				.filter((key) => key.startsWith(family.prefix)),
		]);
		const carried = [...keys].filter((key) => key.startsWith(family.prefix));

		assert.deepEqual(
			[
				...[...reached]
					.filter((key) => !keys.has(key))
					.sort()
					.map((key) => `${family.table} reaches ${key}, no catalogue entry carries it`),
				...carried
					.filter((key) => !reached.has(key))
					.sort()
					.map((key) => `catalogue carries ${key}, nothing reaches it`),
			],
			[],
		);
	}
});

test("every catalogue key is one the extension actually reaches", () => {
	const sweep = sweepKeys(sources());
	const keys = catalogueKeys();
	assert.ok(sweep.literals.length > 0, "the sweep resolved no literal key");
	assert.ok(keys.size > 0, "the catalogue read no key");

	const reached = new Set(sweep.literals.map((entry) => keyOf(entry)));
	for (const family of KEY_FAMILIES) {
		for (const member of familyMembers(family)) {
			reached.add(`${family.prefix}${member}`);
		}
	}

	// l10n.test.ts's "every held-card key the catalogue declares is one the
	// rendering reaches" stays as it is. It names its nine status.holding.*
	// keys one at a time and reports better for them than a whole-catalogue
	// set difference does.
	assert.deepEqual(
		[...keys].filter((key) => !reached.has(key)).sort(),
		[],
		"each key above is one nothing renders, which is dead weight seven translators still pay for",
	);
});

test("a key expression no rule resolves is reported, and a module constant still resolves", () => {
	// The refusing case and the clean case in one fixture, so a sweep that
	// reported every identifier as unresolvable cannot satisfy the first half.
	//
	// The fixture is named .ts.txt so sources() never sweeps it and
	// tsconfig.test.json's exclude keeps tsc off it, exactly as
	// confirmDestructive-call-site.ts.txt is handled. sweepKeys parses it
	// because it passes ScriptKind.TS explicitly.
	const fixture = join(extensionRoot, "test", "fixtures", "unresolvable-key-call-site.ts.txt");
	const sweep = sweepKeys([fixture]);

	// The line is asserted as a line rather than as a number. Two fixtures in
	// this repository key on source line numbers and shift when a diff adds a
	// line above them, and a comment added to the fixture is not a defect this
	// test should report.
	assert.equal(sweep.unresolved.length, 1);
	assert.match(
		sweep.unresolved[0].replace(/^.*[/\\]/, ""),
		/^unresolvable-key-call-site\.ts\.txt:\d+ chosenKey$/,
	);
	assert.deepEqual(
		sweep.literals.map((entry) => keyOf(entry)),
		["fixture.resolved"],
	);
	assert.equal(sweep.sites, 2);
});
