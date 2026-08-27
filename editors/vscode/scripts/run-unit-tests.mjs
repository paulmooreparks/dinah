// Runs the unit layer, and refuses to call an empty run a pass.
//
// The script this replaces was `node --test "out/test/unit/*.test.js"`. The
// pattern is quoted, so no shell expands it and Node is left to, and Node's
// own glob expansion arrived after the version CI pins. On a runner the whole
// layer matched nothing. It went red only because Node happened to treat an
// unmatched pattern as an error; a harness that matched nothing quietly would
// have reported a green job that ran zero tests, which is the failure this
// file exists to make impossible.
//
// So two things are checked here that an exit code cannot check. The test
// files are enumerated from the compiled output rather than described by a
// pattern, so "no files" is a condition this script observes instead of a
// condition it hands to somebody else's matcher. And the runner's own summary
// is read back, so a run that starts, executes nothing and exits zero fails.
//
// The enumeration is also why the argument is a list of files rather than the
// directory `out/test/unit/`. Node 24.4.1 on Windows treats a directory
// argument as a module path and dies with MODULE_NOT_FOUND, so the directory
// form would have traded a break on the pinned version for a break on the
// current one. Explicit file arguments are the one form every version runs.

import { spawnSync } from "node:child_process";
import { readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const unitDir = join(here, "..", "out", "test", "unit");

function die(message) {
	process.stderr.write(`${message}\n`);
	process.exit(1);
}

let entries;
try {
	entries = readdirSync(unitDir);
} catch (err) {
	die(`the compiled unit tests are not at ${unitDir}: ${String(err)}`);
}

const files = entries
	.filter((name) => name.endsWith(".test.js"))
	.sort()
	.map((name) => join(unitDir, name));

if (files.length === 0) {
	die(`no compiled unit tests under ${unitDir}, so this run would have tested nothing`);
}

// The reporter is pinned rather than left to the default, because the default
// differs across Node versions and this script reads the output back.
const result = spawnSync(
	process.execPath,
	["--test", "--test-reporter=tap", ...files],
	{ stdio: ["inherit", "pipe", "inherit"], encoding: "utf8" },
);

const output = result.stdout ?? "";
process.stdout.write(output);

if (result.error) {
	die(`the test runner did not start: ${String(result.error)}`);
}

/** Reads one `# <label> <count>` line out of the TAP summary. */
function summaryCount(label) {
	const match = new RegExp(`^# ${label} (\\d+)$`, "m").exec(output);
	return match === null ? undefined : Number(match[1]);
}

const executed = summaryCount("tests");
const passed = summaryCount("pass");
const failed = summaryCount("fail");

if (result.status !== 0) {
	die(`the unit layer failed: ${String(failed ?? "some")} of ${String(executed ?? "?")} test(s)`);
}

// A missing summary is a failure and not a reason to skip the check. If the
// reporter's format ever changes, this goes red and somebody updates the
// parse, rather than the count check quietly ceasing to exist.
if (executed === undefined || passed === undefined || failed === undefined) {
	die("the runner exited zero but printed no TAP summary, so the count could not be read");
}

// What this does and does not catch, measured rather than assumed. It catches
// a layer that ran nothing, which is the dinah-263 failure. It does not catch
// one file among several registering nothing, because Node reports such a
// file as one passing test rather than as zero, so the summary count cannot
// tell that case apart from a file holding a single test.
if (executed === 0 || passed === 0) {
	die(
		`the unit layer exited zero having run ${String(executed)} test(s) across ${String(files.length)} file(s), which is not a pass`,
	);
}

process.stdout.write(
	`Ran ${String(executed)} unit test(s) from ${String(files.length)} file(s), ${String(passed)} passing.\n`,
);
