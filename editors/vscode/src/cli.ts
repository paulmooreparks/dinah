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

/**
 * How much of an unreadable stdout body a diagnostic quotes.
 *
 * The number bounds what one VS Code toast and one output-channel line can
 * carry. A toast silently drops whatever it cannot fit, and an output line
 * long enough to wrap several times buries the label that says which
 * workbench the message is about. Two hundred characters is long enough to
 * show a shell error or the opening of an HTML error page, and short enough
 * that the label beside it survives. Raise it and you trade that legibility
 * for more of a body the reader can already see by running dinah themselves;
 * drop it much below eighty and it starts cutting ordinary shell errors in
 * half.
 */
const EXCERPT_LIMIT = 200;

/**
 * Cuts a diagnostic to one line and a bounded length, so a raw stdout dump
 * cannot blow up a toast or a log line.
 *
 * An empty body is named rather than left blank, because a message ending in
 * "stdout: " reads as a message that broke halfway through composing itself.
 */
function excerpt(text: string, max = EXCERPT_LIMIT): string {
	const oneLine = text.replace(/\s+/g, " ").trim();
	if (oneLine === "") {
		return "(empty)";
	}
	return oneLine.length > max ? `${oneLine.slice(0, max)}…` : oneLine;
}

/**
 * Names the JSON shape of a body that parsed but is not an object, so the
 * diagnostic below can say what arrived without listing anything.
 *
 * An array is the case that forced this. `Object.keys` answers an array with
 * its indices, so a body of `[1, 2]` reported as a key listing would say the
 * top-level keys were 0 and 1, which is true of the JavaScript value and
 * tells the reader nothing about the response.
 */
function jsonShape(parsed: unknown): string {
	if (parsed === null) {
		return "null";
	}
	if (Array.isArray(parsed)) {
		return "an array";
	}
	return `a ${typeof parsed}`;
}

/**
 * Reads the refusal envelope dinah writes to stdout under `--json`.
 *
 * When the envelope is there, this says so and nothing here has changed. When
 * it is not, the fallback names what was expected against what arrived, which
 * is the difference between a client saying it could not read a response and
 * a client saying nothing at all. A binary predating dinah-346 answers `check`
 * on a workbench carrying defects with exit 2 and its old report body, and the
 * message that used to come back described neither the exit code nor the body.
 *
 * Naming the mismatch is not body-sniffing. Exit 2 still means refused and
 * means only that (dinah-346), a string `refusal` field still classifies the
 * answer as a refusal, and no branch below decides what a check found. These
 * sentences are reached only once no recognised shape matched, and they report
 * a response this client could not read rather than a verdict about dinah.
 */
function readRefusal(
	parsed: unknown,
	stdout: string,
	stderr: string,
): CliOutcome {
	const envelope = parsed as
		| {
				refusal?: unknown;
				detail?: unknown;
				context?: unknown;
				workbenches?: unknown;
		  }
		| undefined;
	if (envelope && typeof envelope.refusal === "string") {
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
	const stderrText = stderr.trim();
	if (stderrText !== "") {
		return { kind: "not-json", detail: stderrText };
	}
	if (parsed === undefined) {
		return {
			kind: "not-json",
			detail:
				`dinah exited 2 (refused), but stdout was not JSON. Expected a ` +
				`refusal envelope with a string "refusal" field. stdout: ${excerpt(stdout)}`,
		};
	}
	if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
		return {
			kind: "not-json",
			detail:
				`dinah exited 2 (refused), but its JSON was ${jsonShape(parsed)}. ` +
				`Every refusal envelope is an object carrying a string "refusal" field.`,
		};
	}
	const keys = Object.keys(parsed as Record<string, unknown>);
	return {
		kind: "not-json",
		detail:
			`dinah exited 2 (refused), but its JSON carried no string "refusal" ` +
			`field, which every refusal envelope carries. Top-level keys: ` +
			`${keys.length > 0 ? keys.join(", ") : "(none)"}.`,
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
			return readRefusal(parsed, outcome.stdout, outcome.stderr);
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
