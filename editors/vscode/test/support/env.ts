// The environment every child process the tests spawn is given.
//
// Two isolation rules bind this layer, and both exist because this repository
// has been bitten by them.
//
// Every fixture workbench is created under the test run's own temp root, never
// inside the checkout, because dinah's discovery walk climbs to the drive root
// and a fixture inside the checkout would put that walk into the repository's
// real .dinah.
//
// Every child process runs with DINAH_HOME pointed inside that same temp root.
// DINAH_HOME does not bound the walk, so it guards configuration writes rather
// than discovery: without it a `dinah init` in a test reaches the operator's
// own user base. This helper refuses to build an environment that breaks
// either rule, which is what makes the guard mechanical rather than a habit.

import { resolve, sep } from "node:path";

/** Raised when an environment would let a test escape its own temp root. */
export class TempRootViolation extends Error {}

/** Reports whether `candidate` lies inside `root`, segment by segment. */
export function underRoot(candidate: string, root: string): boolean {
	const normalise = (value: string): string[] => {
		let text = resolve(value).split(sep).join("/");
		if (process.platform === "win32") {
			text = text.toLowerCase();
		}
		return text.split("/").filter((segment) => segment !== "");
	};
	const inner = normalise(candidate);
	const outer = normalise(root);
	if (inner.length <= outer.length) {
		return false;
	}
	return outer.every((segment, index) => inner[index] === segment);
}

/**
 * Builds the environment a fixture-building child process runs with, refusing
 * one whose DINAH_HOME is missing or outside `tempRoot`.
 *
 * The refusal is the point. A test that forgot DINAH_HOME would pass while
 * writing into the operator's own user base, and nothing downstream would
 * report it.
 */
export function checkedEnv(
	tempRoot: string,
	overrides: Record<string, string | undefined> = {},
): NodeJS.ProcessEnv {
	const held = neutralised(overrides);
	assertUnderTempRoot(tempRoot, held);
	return { ...process.env, ...held };
}

/**
 * The overrides with DINAH_WORKBENCH blanked unless the caller set it.
 *
 * DINAH_WORKBENCH outranks the directory walk, so a child inheriting one
 * from the shell that ran the suite acts on that workbench whatever
 * directory it runs in. A stage that exports it to reach the development
 * workbench and then runs these tests would otherwise have every fixture
 * call that names no --workbench land on it. Blanking rather than
 * unsetting, because dinah's own ladder in internal/bench/config.go skips a
 * value that is empty once trimmed, so a blank is exactly an absence to the
 * binary and survives a host that merges overrides onto its own
 * environment rather than replacing it. A caller that sets the variable
 * keeps its value, and assertUnderTempRoot then holds that value to the
 * temp root.
 */
export function neutralised(
	overrides: Record<string, string | undefined>,
): Record<string, string | undefined> {
	return { DINAH_WORKBENCH: "", ...overrides };
}

/**
 * Asserts that a set of environment overrides keeps a child inside the temp
 * root, without composing the full environment.
 *
 * The VS Code test host takes overrides rather than a whole environment, so
 * the same guard has to be reachable without the merge.
 */
export function assertUnderTempRoot(
	tempRoot: string,
	overrides: Record<string, string | undefined>,
): void {
	const home = overrides.DINAH_HOME;
	if (home === undefined || home.trim() === "") {
		throw new TempRootViolation(
			"a child process in these tests must set DINAH_HOME, or `dinah init` writes into the operator's own user base",
		);
	}
	if (!underRoot(home, tempRoot)) {
		throw new TempRootViolation(
			`DINAH_HOME is ${home}, which is not under this test run's temp root ${tempRoot}`,
		);
	}
	// A named workbench is held to the same root as DINAH_HOME. A blank one
	// is an absence to the binary and passes; so does an unset one.
	const named = overrides.DINAH_WORKBENCH?.trim() ?? "";
	if (named !== "" && !underRoot(named, tempRoot)) {
		throw new TempRootViolation(
			`DINAH_WORKBENCH is ${named}, which is not under this test run's temp root ${tempRoot}`,
		);
	}
}
