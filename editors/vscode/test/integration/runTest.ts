// Launches a real VS Code once per fixture workspace.
//
// One launch per workspace rather than one launch with several folders,
// because the question one of these suites asks is whether the extension
// activated at all, and that can only be asked of a window that opened with
// nothing else in it.

import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { runTests } from "@vscode/test-electron";

import { assertUnderTempRoot } from "../support/env";
import {
	BINARY_NAME,
	addCard,
	buildBinary,
	moveCard,
	initBench,
	pinBinary,
} from "../support/fixtures";

const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");
const suitePath = join(__dirname, "suite", "index");

/** One VS Code launch: which suite runs, and in which workspace folder. */
interface Launch {
	readonly suite: string;
	readonly folder: string;
	readonly env?: Record<string, string | undefined>;
}

async function main(): Promise<void> {
	const root = buildBinary(repoRoot);
	const fixtures = join(root.tempRoot, "fixtures");

	// AC-11: a workspace folder that is itself a workbench.
	const active = join(fixtures, "active");
	initBench(root, active);
	pinBinary(root, active);

	// AC-8: the workbench is the PARENT of the opened folder, which is the
	// dinah-241 shape.
	const nested = join(fixtures, "nested");
	initBench(root, nested);
	const nestedWork = join(nested, "work");
	mkdirSync(nestedWork, { recursive: true });
	writeFileSync(join(nestedWork, "notes.txt"), "a folder below a workbench\n", "utf8");
	pinBinary(root, nestedWork);

	// AC-9: one .dinah container holding two workbenches.
	const ambiguous = join(fixtures, "ambiguous");
	initBench(root, ambiguous, "alpha");
	initBench(root, ambiguous, "beta");
	pinBinary(root, ambiguous);

	// AC-10: a folder with no workbench.md anywhere below it, and no .vscode
	// settings either, so nothing in it can wake the extension.
	const bare = join(fixtures, "bare");
	mkdirSync(bare, { recursive: true });
	writeFileSync(join(bare, "readme.txt"), "nothing to do with dinah\n", "utf8");

	// dinah-265: a workbench holding cards, so the tree suite draws rows from
	// what this commit's binary emits rather than from a JSON literal.
	const tree = join(fixtures, "tree");
	initBench(root, tree);
	pinBinary(root, tree);
	const treeCards = ["Draw the guides", "Translate the headings", "Retire the second map"].map(
		(title) => addCard(root, tree, title),
	);
	// One card is carried into the work column, so the suite sees both
	// take-up spellings. Every card dinah add files lands in intake, which is
	// a queue column that takes no work up, and a fixture whose cards all sit
	// there can only assert the no-Claim half.
	moveCard(root, tree, treeCards[0], "doing");

	const launches: Launch[] = [
		{ suite: "active", folder: active },
		{ suite: "nested", folder: nestedWork },
		{ suite: "ambiguous", folder: ambiguous },
		{ suite: "inactive", folder: bare },
		{ suite: "tree", folder: tree },
	];

	let failed = false;
	for (const launch of launches) {
		const overrides: Record<string, string | undefined> = {
			// VS Code is an Electron application, and Electron obeys this
			// variable by running the binary as plain Node against the first
			// positional argument. Any terminal hosted inside another Electron
			// application exports it, so a developer running these tests from
			// one gets a Node process that tries to require the fixture folder
			// as a module. Clearing it is not a workaround for a defect in
			// this repository; it is refusing to inherit a variable that means
			// something entirely different to the child than it did to the
			// parent. Node drops an environment entry whose value is
			// undefined, so this unsets it rather than blanking it.
			ELECTRON_RUN_AS_NODE: undefined,
			DINAH_HOME: root.home,
			DINAH_ACTOR: "fixture",
			DINAH_SUITE: launch.suite,
			DINAH_FIXTURE_BINARY: join(root.tempRoot, BINARY_NAME),
			DINAH_FIXTURE_ROOT: root.tempRoot,
			...launch.env,
		};
		// The same guard the fixture builders run under. A test host without
		// DINAH_HOME inside the temp root would write into the operator's own
		// user base and pass anyway.
		assertUnderTempRoot(root.tempRoot, overrides);

		process.stdout.write(`\n=== ${launch.suite}: ${launch.folder}\n`);
		try {
			await runTests({
				extensionDevelopmentPath: extensionRoot,
				extensionTestsPath: suitePath,
				extensionTestsEnv: overrides,
				launchArgs: [
					launch.folder,
					"--disable-extensions",
					"--disable-gpu",
					`--user-data-dir=${join(root.tempRoot, "user-data", launch.suite)}`,
					`--extensions-dir=${join(root.tempRoot, "extensions", launch.suite)}`,
				],
			});
		} catch {
			process.stderr.write(`suite ${launch.suite} failed\n`);
			failed = true;
		}
	}

	if (!failed) {
		rmSync(root.tempRoot, { recursive: true, force: true });
	} else {
		process.stderr.write(`fixtures left in place at ${root.tempRoot}\n`);
		process.exit(1);
	}
}

void main();
