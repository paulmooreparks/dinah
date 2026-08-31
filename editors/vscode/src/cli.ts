// The only place in this extension that runs a child process.
//
// Two rules live here rather than in a review comment. Every invocation is
// composed with `--json` in front of it, so nothing above this module can read
// dinah's human output by accident; and every invocation is classified on its
// exit code before anything looks at its content, using the four outcome codes
// the profile declares (0 ok, 2 refused, 3 stale, 4 unreachable) and, for a
// structural read, the fifth code that read's own outcome table adds (5,
// meaning the read completed and found defects to report).
//
// The spawner is a parameter rather than an import, so the unit tests drive
// every branch below without a real process.

import type { Candidate } from "./api";

/**
 * Flags that change how dinah renders rather than what it computes. An argv
 * carrying one of these is refused: the extension reads the machine form and
 * only the machine form, so a caller asking for a rendering has asked the
 * wrong module for the wrong thing.
 *
 * `--json` is not in this list because this module puts it there itself; an
 * argv that already carries it is refused separately, since a caller writing
 * it by hand has misunderstood who owns the flag.
 */
export const RENDERING_FLAGS: readonly string[] = ["--lang", "--quiet"];

/** Raised when a caller hands this module an argv it will not run. */
export class ArgvError extends Error {}

/** What a spawn produced, whether or not the process ever started. */
export interface SpawnOutcome {
	/** The process exit code, or null when it never ran or was signalled. */
	readonly code: number | null;
	readonly stdout: string;
	readonly stderr: string;
	/** Set when the process could not be started at all. */
	readonly spawnError?: { readonly code?: string; readonly message: string };
}

/** Where a spawn runs and what it inherits. */
export interface SpawnOptions {
	readonly cwd?: string;
	readonly env?: NodeJS.ProcessEnv;
}

/** Runs one process and resolves with what it produced. */
export type Spawner = (
	exe: string,
	argv: readonly string[],
	options: SpawnOptions,
) => Promise<SpawnOutcome>;

/**
 * What one dinah invocation came to. The exit code decides the arm before
 * anything reads the content, and stdout is parsed as JSON in every arm
 * because the refusal envelope is on stdout under `--json` too.
 */
export type CliOutcome =
	| { readonly kind: "ok"; readonly json: unknown }
	| {
			readonly kind: "refused";
			readonly refusal: string;
			readonly detail?: string;
			readonly context?: Record<string, string>;
			readonly workbenches?: Candidate[];
	  }
	| { readonly kind: "stale"; readonly detail: string }
	| { readonly kind: "unreachable"; readonly detail: string }
	| {
			readonly kind: "spawn-failed";
			readonly errno?: string;
			readonly detail: string;
	  }
	| { readonly kind: "not-json"; readonly detail: string };

/**
 * Composes the argv dinah is actually invoked with: `--json` ahead of
 * everything the caller asked for.
 *
 * The flag goes first because dinah's session flags are position-agnostic and
 * a leading flag cannot be swallowed by a command's free-text tail.
 */
export function composeArgv(args: readonly string[]): string[] {
	for (const arg of args) {
		if (arg === "--json" || arg.startsWith("--json=")) {
			throw new ArgvError(
				"cli.ts composes --json itself; an argv reaching it must not carry the flag",
			);
		}
		for (const flag of RENDERING_FLAGS) {
			if (arg === flag || arg.startsWith(`${flag}=`)) {
				throw new ArgvError(
					`the extension never reads dinah's rendered output, so ${flag} may not be passed`,
				);
			}
		}
	}
	return ["--json", ...args];
}

/** Parses stdout as JSON, returning undefined when it is not JSON at all. */
function parseJson(stdout: string): unknown | undefined {
	const text = stdout.trim();
	if (text === "") {
		return undefined;
	}
	try {
		return JSON.parse(text);
	} catch {
		return undefined;
	}
}

/** Reads the refusal envelope dinah writes to stdout under `--json`. */
function readRefusal(parsed: unknown, stderr: string): CliOutcome {
	const envelope = parsed as
		| {
				refusal?: unknown;
				detail?: unknown;
				context?: unknown;
				workbenches?: unknown;
		  }
		| undefined;
	if (!envelope || typeof envelope.refusal !== "string") {
		return {
			kind: "not-json",
			detail:
				stderr.trim() ||
				"dinah refused but wrote no machine-readable refusal to stdout",
		};
	}
	return {
		kind: "refused",
		refusal: envelope.refusal,
		detail: typeof envelope.detail === "string" ? envelope.detail : undefined,
		context:
			envelope.context && typeof envelope.context === "object"
				? (envelope.context as Record<string, string>)
				: undefined,
		workbenches: Array.isArray(envelope.workbenches)
			? (envelope.workbenches as Candidate[])
			: undefined,
	};
}

/**
 * The exit code a structural read carries when it completed and found defects,
 * as contract.ExitCodeForRead spells it.
 *
 * 5 rather than 2 because 2 already means the invocation was refused, and 5
 * rather than 1 because 1 stays reserved for a response nothing conforming
 * can produce (dinah-346).
 */
export const EXIT_READ_FINDINGS = 5;

/**
 * Classifies an invocation that did not exit 0, which every caller reads the
 * same way.
 *
 * Exit 0 is deliberately not handled here. Each caller says for itself what an
 * exit it treats as success but cannot parse means, and those sentences differ:
 * a `version` probe that answers prose is a binary that is not dinah, while a
 * check that answers prose is a report that went missing.
 */
function classifyFailure(outcome: SpawnOutcome, parsed: unknown): CliOutcome {
	switch (outcome.code) {
		case 2:
			return readRefusal(parsed, outcome.stderr);
		case 3:
			return {
				kind: "stale",
				detail: outcome.stderr.trim() || "dinah reported stale knowledge",
			};
		case 4:
			return {
				kind: "unreachable",
				detail:
					outcome.stderr.trim() || "dinah could not reach what it was asked for",
			};
		default:
			return {
				kind: "not-json",
				detail:
					outcome.stderr.trim() ||
					`dinah exited ${String(outcome.code)}, which is not an outcome the profile declares`,
			};
	}
}

/**
 * Runs one dinah invocation and classifies it.
 *
 * `args` is what the caller wants dinah to do, without `--json`; this function
 * composes that. The environment is passed through untouched, because the
 * extension sets no `DINAH_*` variable of its own and a variable it invented
 * would put the editor and the user's terminal on different workbenches.
 */
export async function runDinah(
	spawner: Spawner,
	exe: string,
	args: readonly string[],
	options: SpawnOptions = {},
): Promise<CliOutcome> {
	const outcome = await spawner(exe, composeArgv(args), options);

	if (outcome.spawnError) {
		return {
			kind: "spawn-failed",
			errno: outcome.spawnError.code,
			detail: outcome.spawnError.message,
		};
	}

	const parsed = parseJson(outcome.stdout);
	if (outcome.code === 0) {
		if (parsed === undefined) {
			return {
				kind: "not-json",
				detail:
					"this binary is not dinah, or is too old to answer `--json version`",
			};
		}
		return { kind: "ok", json: parsed };
	}
	return classifyFailure(outcome, parsed);
}

/**
 * Runs `dinah check` against one workbench and classifies what came back.
 *
 * A structural read answers on two exit codes rather than one: 0 when the
 * workbench read cleanly and EXIT_READ_FINDINGS when it read fine and found
 * defects to report. Both are the read having happened, so both are `ok` here
 * and the caller tells them apart by the report's own `outcome` member. Which
 * of the two a run took is not re-derived from the body anywhere, because
 * dinah-346 landed the distinction on the CLI and a client that reimplemented
 * it would be a second place for the rule to live.
 *
 * Exit 2 means the invocation itself was refused and nothing else, which is
 * what dinah-346 freed it to mean, so it goes through the same refusal
 * envelope every other call does. The remaining codes are runDinah's own,
 * shared rather than repeated.
 */
export async function runCheck(
	spawner: Spawner,
	exe: string,
	root: string,
	options: SpawnOptions = {},
): Promise<CliOutcome> {
	const outcome = await spawner(
		exe,
		composeArgv(["--workbench", root, "check"]),
		options,
	);

	if (outcome.spawnError) {
		return {
			kind: "spawn-failed",
			errno: outcome.spawnError.code,
			detail: outcome.spawnError.message,
		};
	}

	const parsed = parseJson(outcome.stdout);
	if (outcome.code === 0 || outcome.code === EXIT_READ_FINDINGS) {
		if (parsed === undefined) {
			return {
				kind: "not-json",
				detail:
					outcome.stderr.trim() ||
					`dinah check exited ${String(outcome.code)} and wrote no JSON report`,
			};
		}
		return { kind: "ok", json: parsed };
	}
	return classifyFailure(outcome, parsed);
}
