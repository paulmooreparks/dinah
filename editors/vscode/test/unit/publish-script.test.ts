// publish-extension.ps1's release lookup, exercised by running the script.
//
// The script used to publish whatever version package.json committed, which
// worked only because a maintainer typed the number about to be published into
// the manifest first. The manifest is a floor now, so the script asks the
// release history instead, and three outcomes have to stay apart: the lookup
// failed, the lookup succeeded and found no release, and the lookup found one.
// A release lookup whose "could not answer" and "nothing there" look identical
// to the caller is the defect this repository spent a review round on, and the
// no-release case is not hypothetical, because no extension release exists yet.
//
// Reading the script's source would prove none of that. The three outcomes are
// driven for real, with gh, npm and vsce stubbed on PATH, and the assertion is
// on what the stubs were actually asked to do. The stub for npm records its own
// arguments, so the version reaching the packaging call is observed rather than
// inferred from the text of the line that would produce it.
//
// What this does not do is publish anything or reach the network. gh never runs
// here, and vsce is a stub that records and exits.

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";
import { test } from "node:test";

const extensionRoot = join(__dirname, "..", "..", "..");
const script = join(extensionRoot, "scripts", "publish-extension.ps1");
const onWindows = process.platform === "win32";

/**
 * Writes one stub command into `dir` under a name the platform's shell finds.
 *
 * The bodies are per-platform because there is no one script a cmd.exe search
 * path and a POSIX exec both run. Each stub appends the arguments it was given
 * to a log, so the assertions read what was asked rather than what was written.
 */
function stub(dir: string, name: string, windowsBody: string, posixBody: string): void {
	if (onWindows) {
		writeFileSync(join(dir, `${name}.cmd`), `@echo off\r\n${windowsBody}\r\n`, "utf8");
		return;
	}
	const path = join(dir, name);
	writeFileSync(path, `#!/bin/sh\n${posixBody}\n`, "utf8");
	chmodSync(path, 0o755);
}

/**
 * Whether the log holds a `vsce publish` call.
 *
 * The publisher check calls `vsce ls-publishers` on every run, so a plain search
 * for the word finds that instead and can never fail. Each log line is one
 * command's arguments, so the publish call is a line whose first word is
 * publish.
 */
function publishWasCalled(log: string): boolean {
	return log.split(/\r?\n/u).some((line) => /^publish\b/u.test(line.trim()));
}

/** What one run of the script saw and did. */
interface Run {
	status: number | null;
	stdout: string;
	stderr: string;
	log: string;
}

/**
 * Runs publish-extension.ps1 with stubbed external commands.
 *
 * `ghMode` decides what the release listing does: "fail" exits non-zero the way
 * a network error, an expired token or a rate limit does, "empty" succeeds and
 * returns a list carrying no extension release, "silent" succeeds and returns
 * nothing at all, and "released" succeeds and returns two.
 *
 * "empty" and "silent" are the same case for the version module and different
 * cases for the script. "silent" is what a repository with no releases at all
 * produces, and it is the only one that feeds an empty capture into the
 * pipeline, which is a PowerShell behaviour rather than a module behaviour.
 */
function runPublish(ghMode: "fail" | "empty" | "silent" | "released"): Run {
	const dir = mkdtempSync(join(tmpdir(), "dinah-publish-"));
	try {
		const log = join(dir, "calls.log");
		const record = onWindows
			? `>>"${log}" echo %*`
			: `printf '%s\\n' "$*" >> "${log}"`;

		if (ghMode === "fail") {
			stub(
				dir,
				"gh",
				`${record}\r\necho the releases could not be listed 1>&2\r\nexit /b 1`,
				`${record}\necho "the releases could not be listed" 1>&2\nexit 1`,
			);
		} else {
			const tags =
				ghMode === "silent"
					? []
					: ghMode === "empty"
						? ["v1.2.3", "dinah-v1.0.5"]
						: ["vscode-v1.2.5", "v1.2.3", "vscode-v1.3.0"];
			stub(
				dir,
				"gh",
				`${record}\r\n${tags.map((tag) => `echo ${tag}`).join("\r\n")}`,
				`${record}\n${tags.map((tag) => `echo "${tag}"`).join("\n")}`,
			);
		}

		// npm records and succeeds. Nothing here installs, compiles or packages,
		// so the archive directory stays absent and the script fails after the
		// packaging call on the released path. That is fine: the question this
		// answers is which arguments reached npm, and the log holds them whether
		// the run finished or not.
		stub(dir, "npm", record, record);
		// vsce answers the publisher check and records everything else. A run
		// that reached a real publish would show up in the log.
		stub(
			dir,
			"vsce",
			`${record}\r\nif "%1"=="ls-publishers" echo paulmooreparks`,
			`${record}\nif [ "$1" = "ls-publishers" ]; then echo paulmooreparks; fi`,
		);

		const shell = onWindows ? "pwsh.exe" : "pwsh";
		const result = spawnSync(
			shell,
			["-NoProfile", "-NonInteractive", "-File", script, "-Tag", "v0.1.42-dev", "-DryRun"],
			{
				encoding: "utf8",
				env: { ...process.env, PATH: `${dir}${delimiter}${process.env.PATH ?? ""}` },
			},
		);
		assert.equal(
			result.error,
			undefined,
			`pwsh did not start, so this check ran nothing: ${String(result.error)}`,
		);
		return {
			status: result.status,
			stdout: result.stdout ?? "",
			stderr: result.stderr ?? "",
			log: existsSync(log) ? readFileSync(log, "utf8") : "",
		};
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
}

test("a lookup that could not answer fails the script and says so", () => {
	const run = runPublish("fail");
	assert.notEqual(run.status, 0, "a failed release lookup let the script carry on");
	assert.match(
		`${run.stdout}${run.stderr}`,
		/could not be listed/iu,
		"the script did not report that the release list could not be retrieved",
	);
	assert.ok(
		!/no extension release exists yet/iu.test(`${run.stdout}${run.stderr}`),
		"a lookup that could not answer was reported as an absent release, which is the trap this check exists for",
	);
	assert.ok(
		!run.log.includes("run package"),
		"the script packaged an archive after the release lookup failed",
	);
	assert.ok(
		!publishWasCalled(run.log),
		"the script published after the release lookup failed",
	);
});

test("a list carrying no extension release fails the script with its own message", () => {
	const run = runPublish("empty");
	assert.notEqual(run.status, 0, "a list carrying no extension release let the script carry on");
	assert.match(
		`${run.stdout}${run.stderr}`,
		/no extension release exists yet/iu,
		"the script did not report that no extension release exists yet",
	);
	assert.ok(
		!/could not be listed/iu.test(`${run.stdout}${run.stderr}`),
		"an absent release was reported as a lookup failure",
	);
	assert.ok(
		!run.log.includes("run package"),
		"the script packaged an archive with no released version to package",
	);
	assert.ok(
		!publishWasCalled(run.log),
		"the script published with no released version to publish",
	);
});

test("a listing that returns nothing at all fails the script with the same message", () => {
	// A repository with no releases whatever prints no line, so the capture is
	// empty and PowerShell feeds an empty pipeline into the version script. The
	// module cannot tell this from a list of non-extension tags, and the script
	// can: this is the only mode where nothing crosses the pipe.
	const run = runPublish("silent");
	assert.notEqual(run.status, 0, "an empty listing let the script carry on");
	assert.match(
		`${run.stdout}${run.stderr}`,
		/no extension release exists yet/iu,
		"an empty listing did not report that no extension release exists yet",
	);
	assert.ok(
		!/could not be listed/iu.test(`${run.stdout}${run.stderr}`),
		"an empty listing was reported as a lookup failure",
	);
	assert.ok(
		!run.log.includes("run package"),
		"the script packaged an archive with no released version to package",
	);
	assert.ok(
		!publishWasCalled(run.log),
		"the script published with no released version to publish",
	);
});

test("the newest released version reaches the packaging call", () => {
	const run = runPublish("released");
	// The stub npm produces no archive, so the script fails after this call.
	// What matters is the argument list it was handed, which the log holds.
	assert.match(
		run.log,
		/run package -- --version 1\.3\.0/u,
		`the packaging call did not carry the newest released version; the stubs were asked for:\n${run.log}`,
	);
	assert.ok(
		!run.log.includes("--published"),
		"the packaging call still passes the retired --published flag, which selects nothing and packages the committed floor",
	);
});
