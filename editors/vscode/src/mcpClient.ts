// The extension's JSON-RPC client for `dinah mcp`, and nothing above it.
//
// Two surfaces answer this extension, and this module is the second one.
// cli.ts spawns `dinah --json <verb>` and classifies the result on the
// process exit code, which is what every tree command runs on. The command
// palette cannot use that surface: it enumerates verbs from the tool table
// the MCP head publishes, and that table carries each tool's published name
// rather than the verb spelling a terminal takes, so composing an argv from
// an entry of it would need a second name table maintained by hand. The
// palette therefore enumerates and runs through MCP alone (dinah-420 D1).
//
// Every call here is its own short-lived process. The server loop reads
// line-delimited JSON-RPC from stdin and exits when the stream closes, so a
// call writes `initialize` and the one request it wants answered, closes the
// stream, and reads what the process wrote before it exited. No session is
// held open across a wizard the reader can cancel at any step.
//
// The spawner is the same injected Spawner cli.ts takes, so a unit test drives
// every branch below without a process, and the one real spawner still lives
// in spawn.ts.

import type { SpawnOptions, Spawner } from "./cli";

/** The JSON-RPC protocol version both ends speak. */
const JSONRPC_VERSION = "2.0";

/** The id `initialize` is sent under, which no caller reads an answer from. */
const INITIALIZE_ID = 1;

/** The id the one request a call cares about is sent under. */
export const REQUEST_ID = 2;

/** What one MCP call came to. */
export type McpOutcome =
	| { readonly kind: "ok"; readonly result: Record<string, unknown> }
	| { readonly kind: "spawn-failed"; readonly detail: string }
	| { readonly kind: "transport-error"; readonly detail: string };

/**
 * The two lines one call writes to the server's stdin.
 *
 * `initialize` leads because the protocol says a client sends it before it
 * asks for anything, and its answer is discarded here: this client reads the
 * tool table rather than the server's capabilities, and a server that refused
 * the handshake would fail the request on the next line anyway.
 */
export function composeSession(
	method: string,
	params: Record<string, unknown>,
): string {
	const initialize = {
		jsonrpc: JSONRPC_VERSION,
		id: INITIALIZE_ID,
		method: "initialize",
		params: {},
	};
	const request = { jsonrpc: JSONRPC_VERSION, id: REQUEST_ID, method, params };
	return `${JSON.stringify(initialize)}\n${JSON.stringify(request)}\n`;
}

/** One line of the server's stdout, once it has parsed. */
interface Envelope {
	readonly id?: unknown;
	readonly result?: unknown;
	readonly error?: { readonly code?: number; readonly message?: string };
}

/**
 * Cuts a diagnostic to one line and a bounded length.
 *
 * The same two hundred characters cli.ts's own excerpt uses, and for the same
 * reason: the text is arbitrary output this extension did not produce, quoted
 * inside a sentence that carries more than the text.
 */
function excerpt(text: string, max = 200): string {
	const oneLine = text.replace(/\s+/g, " ").trim();
	if (oneLine === "") {
		return "(empty)";
	}
	return oneLine.length > max ? `${oneLine.slice(0, max)}…` : oneLine;
}

/**
 * Reads the answer to REQUEST_ID out of everything the server wrote.
 *
 * Every line is parsed rather than only the last, because the handshake's own
 * answer arrives first and a server is free to write other lines beside them.
 * A line that is not JSON at all fails the whole read: this transport is
 * line-delimited JSON-RPC, so a line that is not one is a server this client
 * cannot follow, and reading past it would be reading around a defect.
 */
export function readAnswer(stdout: string, stderr: string): McpOutcome {
	const lines = stdout.split("\n").filter((line) => line.trim() !== "");
	if (lines.length === 0) {
		return {
			kind: "transport-error",
			detail: `dinah mcp wrote nothing to stdout. stderr: ${excerpt(stderr)}`,
		};
	}
	const envelopes: Envelope[] = [];
	for (const line of lines) {
		try {
			envelopes.push(JSON.parse(line) as Envelope);
		} catch {
			return {
				kind: "transport-error",
				detail: `dinah mcp wrote a line that is not JSON-RPC: ${excerpt(line)}`,
			};
		}
	}
	const answer = envelopes.find((envelope) => envelope.id === REQUEST_ID);
	if (answer === undefined) {
		return {
			kind: "transport-error",
			detail:
				`dinah mcp answered no request with id ${String(REQUEST_ID)}. ` +
				`stdout: ${excerpt(stdout)}`,
		};
	}
	if (answer.error !== undefined) {
		return {
			kind: "transport-error",
			detail: `dinah mcp refused the call: ${answer.error.message ?? "(no message)"}`,
		};
	}
	const result = answer.result;
	if (result === null || typeof result !== "object" || Array.isArray(result)) {
		return {
			kind: "transport-error",
			detail: `dinah mcp answered with a result that is not an object: ${excerpt(JSON.stringify(result) ?? "undefined")}`,
		};
	}
	return { kind: "ok", result: result as Record<string, unknown> };
}

/**
 * Runs one MCP method against a fresh `dinah mcp` and classifies the answer.
 *
 * The argv carries no `--json`. That flag chooses the machine rendering of the
 * cli head's own output, and this head has one rendering; cli.ts refuses an
 * argv that carries the flag for the same reason it composes it, so nothing
 * here goes through composeArgv.
 */
export async function callMcp(
	spawner: Spawner,
	exe: string,
	method: string,
	params: Record<string, unknown>,
	options: SpawnOptions = {},
): Promise<McpOutcome> {
	const outcome = await spawner(exe, ["mcp"], {
		...options,
		stdin: composeSession(method, params),
	});
	if (outcome.spawnError) {
		return { kind: "spawn-failed", detail: outcome.spawnError.message };
	}
	return readAnswer(outcome.stdout, outcome.stderr);
}
