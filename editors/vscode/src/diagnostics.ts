// The Problems panel's copy of what `dinah check` last said about a workbench.
//
// This module projects findings rather than analysing anything. The checker
// already answers with a structured report, so nothing here re-derives a
// finding from text, invents a severity the checker does not carry, or
// fabricates a line and column a Finding never had.
//
// Nothing here imports vscode, on the same terms as changes.ts and
// countdown.ts: extension.ts owns the one DiagnosticCollection and hands this
// module a function that writes to it, so `node --test` drives every branch
// below without an extension host.
//
// Two halves of the problem are easy to forget and both are answered here
// rather than at the call site. A finding that was fixed has to be cleared,
// because a collection replaces one URI's entries and leaves every other URI
// exactly as it was, so a file the previous run wrote to and this run does not
// mention would keep a defect that no longer exists. And a panel that is empty
// because no check has completed looks exactly like a panel that is empty
// because the workbench is clean, so a placeholder says which one it is until
// the first run answers.

import type { CliOutcome, Spawner } from "./cli";
import { runCheck } from "./cli";
import type { Localizer } from "./l10n";
import { findingLine } from "./workbenchCommands";
import type { CheckAnswer } from "./wire";

/**
 * One projected diagnostic, in the shape extension.ts turns into a real
 * vscode.Diagnostic.
 */
export interface DiagnosticEntry {
	/**
	 * [startLine, startChar, endLine, endChar], always [0, 0, 0, 0].
	 *
	 * A Finding carries no line and no column, and a fabricated range would
	 * assert a precision the checker never gave. The whole of the first line
	 * is what the editor draws for a range of zero width at the start of the
	 * document, which is the honest answer to "somewhere in this file".
	 */
	readonly range: readonly [number, number, number, number];
	readonly message: string;
	readonly severity: "warning";
	/** A literal tool identifier, not routed through the catalogue (D3). */
	readonly source: string;
}

/** What one workbench's diagnostics currently look like, keyed by absolute file path. */
export type DiagnosticPlan = ReadonlyMap<string, readonly DiagnosticEntry[]>;

/**
 * The identifier every diagnostic this module produces carries.
 *
 * It is a literal rather than a catalogue key, in the same register as the
 * output channel's own name and the manifest's displayName, and in the register
 * VS Code's own convention for this field uses (`eslint`, `tsc`).
 */
export const SOURCE = "dinah check";

/** The catalogue key the placeholder diagnostic renders from. */
export const UNCERTAIN_KEY = "diagnostics.check.uncertain";

/** Resolves whether a Finding's own Path is directly addressable. */
export type StatKind = "file" | "directory" | "missing";

/** The range every entry carries, written once because every entry carries it. */
const WHOLE_FILE: readonly [number, number, number, number] = [0, 0, 0, 0];

/**
 * Composes the message a finding attached to its own file reads as.
 *
 * The path is left out because the Problems panel already groups by file and
 * repeating it would say the same thing twice in one row. `Key` and `Detail`
 * pass through raw, which is what the machine surface carries (D1).
 */
function ownFileMessage(key: string, detail: string): string {
	return detail === "" ? key : `${key} (${detail})`;
}

/**
 * Turns one check report into the diagnostics it projects to.
 *
 * The result is total over the answer. Every finding lands under exactly one
 * path, either its own or the fallback, and a finding that can reach neither
 * is written to the log rather than dropped.
 *
 * `fallbackPath` is resolved by the caller and passed in, because it is
 * per-root state cached across runs and this function owns none.
 */
export async function planFor(
	answer: CheckAnswer,
	fallbackPath: string | undefined,
	statKind: (path: string) => Promise<StatKind>,
	log: (line: string) => void,
): Promise<DiagnosticPlan> {
	const byPath = new Map<string, DiagnosticEntry[]>();
	const add = (path: string, message: string): void => {
		const entries = byPath.get(path) ?? [];
		entries.push({
			range: WHOLE_FILE,
			message,
			severity: "warning",
			source: SOURCE,
		});
		byPath.set(path, entries);
	};
	for (const finding of answer.findings ?? []) {
		const kind = await statKind(finding.Path);
		if (kind === "file") {
			add(finding.Path, ownFileMessage(finding.Key, finding.Detail));
			continue;
		}
		// The finding names a directory, or names something that is not there
		// at all, and neither is a document the editor can open a diagnostic
		// on. It moves to the workbench's own definition file, and the message
		// grows the path back because the row no longer sits where the defect
		// is.
		if (fallbackPath === undefined) {
			// Nothing resolved the definition file either, so there is nowhere
			// to put this. The channel gets it in the shape the manual check
			// already writes, which keeps the finding readable somewhere.
			log(`${SOURCE}: ${findingLine(finding)}`);
			continue;
		}
		add(fallbackPath, findingLine(finding));
	}
	return byPath;
}

/** What CheckDiagnostics needs injected, so its tests spawn nothing and touch no disk. */
export interface CheckDiagnosticsDeps {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly log: (line: string) => void;
	readonly t: Localizer;
	/** vscode.workspace.fs.stat in production, a fixture map under test. */
	readonly statKind: (path: string) => Promise<StatKind>;
	/**
	 * Resolves the file a finding whose own Path is not addressable attaches
	 * to instead, which is what `dinah --workbench <root> path workbench`
	 * answers. Undefined when that call itself failed, which the caller logs
	 * rather than losing the finding silently.
	 */
	readonly resolveFallback: (root: string) => Promise<string | undefined>;
	/**
	 * Writes one workbench's diagnostics, replacing per path whatever was
	 * there before. A path carrying an empty array clears it, and this module
	 * puts those empty arrays in itself, so every clearing is visible in the
	 * call rather than reconstructed by whoever receives it.
	 */
	readonly apply: (byPath: DiagnosticPlan) => void;
}

/** What this module remembers about one workbench between runs. */
interface RootState {
	/** Whether a run has ever completed for this root this session. */
	confirmedOnce: boolean;
	/** Every path the last applied plan wrote to, so the next one can clear them. */
	lastPaths: ReadonlySet<string>;
	/** Whether a run is in flight for this root. */
	running: boolean;
	/** Whether a trigger arrived while one was in flight. */
	queued: boolean;
}

/**
 * Runs the workbench check for the Problems panel and keeps the panel honest
 * between runs.
 *
 * One instance serves every workbench the window watches, keyed by root, and
 * every root's state is independent: a run for one never coalesces with, waits
 * on, or clears anything belonging to another.
 */
export class CheckDiagnostics {
	private readonly roots = new Map<string, RootState>();

	constructor(private readonly deps: CheckDiagnosticsDeps) {}

	/**
	 * Says that this workbench has not been checked yet, before the first run
	 * has answered.
	 *
	 * A reader who opens the Problems panel in that gap sees a row naming the
	 * workbench as unconfirmed instead of seeing nothing, so an empty panel
	 * never has to mean both "clean" and "never asked". The first successful
	 * run replaces it, and this is a no-op from then on.
	 */
	async markPending(root: string, title: string): Promise<void> {
		const state = this.stateOf(root);
		if (state.confirmedOnce) {
			return;
		}
		const fallback = await this.deps.resolveFallback(root);
		if (fallback === undefined) {
			// There is no file to hang the placeholder on. Saying so in the
			// channel is all that is left, and it is better than a panel that
			// quietly says nothing.
			this.deps.log(
				`${SOURCE}: ${root} has not been checked yet, and its definition file did not resolve`,
			);
			return;
		}
		const plan = new Map<string, readonly DiagnosticEntry[]>([
			[
				fallback,
				[
					{
						range: WHOLE_FILE,
						message: this.deps.t(UNCERTAIN_KEY, { workbench: title }),
						severity: "warning",
						source: SOURCE,
					},
				],
			],
		]);
		this.applyPlan(state, plan);
	}

	/**
	 * Runs `dinah check` for one workbench and applies what came back.
	 *
	 * At most one run per root is ever in flight. A trigger arriving during a
	 * run is remembered rather than spawned, and however many arrive they
	 * collapse into one rerun behind the run that was already going, so a busy
	 * workbench costs two structural sweeps rather than a queue of them.
	 */
	async runFor(root: string, title: string): Promise<void> {
		const state = this.stateOf(root);
		if (state.running) {
			state.queued = true;
			return;
		}
		state.running = true;
		try {
			await this.spawnOnce(root, title);
			while (state.queued) {
				// Cleared before the rerun rather than after it, so a trigger
				// arriving during the rerun earns a further one instead of
				// being swallowed by the clearing.
				state.queued = false;
				await this.spawnOnce(root, title);
			}
		} finally {
			state.running = false;
		}
	}

	/**
	 * Applies a report somebody else already paid for, with no spawn of its
	 * own.
	 *
	 * The manual check command has run the binary and read the answer, so this
	 * is how that one invocation reaches the panel as well as the channel.
	 */
	async applyResult(
		root: string,
		title: string,
		outcome: CliOutcome,
	): Promise<void> {
		const state = this.stateOf(root);
		if (outcome.kind !== "ok") {
			// The run did not complete, so nothing here knows anything new.
			// Whatever the panel is showing stays, which is the placeholder
			// before the first success and the last good findings after it, on
			// the terms changes.ts already keeps the tree's last good content
			// through a run of failures. Replacing them with silence would
			// turn a failure into a clean bill of health.
			this.deps.log(
				`${SOURCE}: ${title} did not answer (${outcome.kind}), so its problems are as they were`,
			);
			return;
		}
		const answer = outcome.json as CheckAnswer;
		const fallback = await this.deps.resolveFallback(root);
		const plan = await planFor(
			answer,
			fallback,
			this.deps.statKind,
			this.deps.log,
		);
		this.applyPlan(state, plan);
		state.confirmedOnce = true;
	}

	/** Runs the binary once and applies whatever it answered. */
	private async spawnOnce(root: string, title: string): Promise<void> {
		const outcome = await runCheck(this.deps.spawner, this.deps.exe, root);
		await this.applyResult(root, title, outcome);
	}

	/**
	 * Writes a plan and clears what the previous one left behind.
	 *
	 * A path both plans name is written again rather than cleared and rewritten,
	 * so a finding that is still there does not flicker out of the panel and
	 * back into it.
	 */
	private applyPlan(state: RootState, plan: DiagnosticPlan): void {
		const written = new Map<string, readonly DiagnosticEntry[]>(plan);
		for (const path of state.lastPaths) {
			if (!written.has(path)) {
				written.set(path, []);
			}
		}
		this.deps.apply(written);
		state.lastPaths = new Set(plan.keys());
	}

	/** This root's state, created on first mention. */
	private stateOf(root: string): RootState {
		const found = this.roots.get(root);
		if (found !== undefined) {
			return found;
		}
		const fresh: RootState = {
			confirmedOnce: false,
			lastPaths: new Set<string>(),
			running: false,
			queued: false,
		};
		this.roots.set(root, fresh);
		return fresh;
	}
}
