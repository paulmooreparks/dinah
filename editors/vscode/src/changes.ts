// How the tree learns something changed.
//
// The poll is authoritative and the watcher is a latency optimization. That
// split is the whole design, and it is not a belt-and-braces habit. Whether
// vscode.workspace.createFileSystemWatcher observes a path outside every
// workspace folder is undocumented, and a resolved workbench root is often
// exactly such a path, so nothing here may be correct only because the
// watcher fired. The timer is what bounds staleness to one interval on every
// path; the watcher only shortens the wait when it happens to work.
//
// `dinah changes` is what makes a tight interval affordable: a call whose two
// digest terms match the cursor handed back to it answers `changed: false`
// having parsed no journal at all.
//
// Every clock this module uses is injected, so the unit layer drives the
// debounce, the throttle and the poll on a fake clock rather than by waiting.

import type { Spawner } from "./cli";
import { runDinah } from "./cli";
import type { RootChangeSetAnswer, ChangeSetAnswer } from "./wire";

/** How long a burst of file saves is allowed to settle before a check runs. */
export const DEBOUNCE_MS = 400;

/** The floor between two watcher-driven checks of the same entry. */
export const THROTTLE_MS = 1_000;

/** The refusal a cursor the binary will not read comes back as. */
export const MALFORMED = "dinah.malformed";

/** A timer, injected so the unit layer can drive it without waiting. */
export interface Clock {
	readonly now: () => number;
	readonly setTimeout: (fn: () => void, ms: number) => unknown;
	readonly clearTimeout: (handle: unknown) => void;
	readonly setInterval: (fn: () => void, ms: number) => unknown;
	readonly clearInterval: (handle: unknown) => void;
}

/** The real clock, which is what activate() passes. */
export const systemClock: Clock = {
	now: () => Date.now(),
	setTimeout: (fn, ms) => setTimeout(fn, ms),
	clearTimeout: (handle) => {
		clearTimeout(handle as ReturnType<typeof setTimeout>);
	},
	setInterval: (fn, ms) => setInterval(fn, ms),
	clearInterval: (handle) => {
		clearInterval(handle as ReturnType<typeof setInterval>);
	},
};

/** A file watcher, injected so a run with watching off constructs none. */
export interface Watcher {
	readonly dispose: () => void;
}

/** One entry the loop checkpoints: a folder, and how it is asked. */
export interface CheckpointEntry {
	/** The workspace folder, which is the key everything is stored under. */
	readonly folder: string;
	/**
	 * A single-workbench entry names the resolved root and is asked with
	 * `--workbench <root> changes`. A root-scoped entry names the folder and
	 * is asked with `changes --root <folder>`, whose one merged token covers
	 * every workbench beneath it.
	 */
	readonly scope: "workbench" | "root";
	/** The directory the call is pinned to: the root, or the folder. */
	readonly path: string;
}

/** What the loop needs in order to run. */
export interface CheckpointDeps {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly clock: Clock;
	readonly log: (line: string) => void;
	/** Re-reads one folder's rows. Called only when something changed. */
	readonly refresh: (folder: string) => Promise<void>;
	/** Fires onDidChangeTreeData with no element. */
	readonly fire: () => void;
	/** How often the poll runs, in seconds. */
	readonly pollIntervalSeconds: number;
	/** Whether a filesystem watcher is constructed at all. */
	readonly watchFiles: boolean;
	/** Builds a watcher for one entry, called only when watchFiles is true. */
	readonly createWatcher?: (
		entry: CheckpointEntry,
		onEvent: () => void,
	) => Watcher;
}

/** One entry's cursor and its watcher bookkeeping. */
interface EntryState {
	readonly entry: CheckpointEntry;
	/**
	 * The token to hand back next time, stored verbatim.
	 *
	 * A root-scoped entry holds exactly one token for the whole folder, not
	 * one per workbench beneath it. The merged token is a set of per-workbench
	 * cursors keyed by path, and it is opaque on purpose: nothing here decodes
	 * it, and nothing derives a per-workbench token from it.
	 */
	cursor?: string;
	debounce?: unknown;
	lastOffCycle: number;
	watcher?: Watcher;
}

/**
 * The checkpoint loop.
 *
 * One instance covers every entry. The poll timer is a single interval that
 * checks every entry, because a timer per folder buys nothing and costs a
 * handle each.
 */
export class CheckpointLoop {
	private readonly entries = new Map<string, EntryState>();
	private timer?: unknown;
	private visible = true;

	constructor(private readonly deps: CheckpointDeps) {}

	/** Starts watching and polling the entries given. */
	start(entries: readonly CheckpointEntry[]): void {
		this.stop();
		for (const entry of entries) {
			const state: EntryState = { entry, lastOffCycle: 0 };
			if (this.deps.watchFiles && this.deps.createWatcher !== undefined) {
				state.watcher = this.deps.createWatcher(entry, () => {
					this.onFileEvent(entry.folder);
				});
			}
			this.entries.set(entry.folder, state);
		}
		this.startTimer();
	}

	/** Stops every timer and disposes every watcher. */
	stop(): void {
		this.stopTimer();
		for (const state of this.entries.values()) {
			if (state.debounce !== undefined) {
				this.deps.clock.clearTimeout(state.debounce);
			}
			state.watcher?.dispose();
		}
		this.entries.clear();
	}

	/** The cursor held for one folder, which the unit layer reads back. */
	cursorFor(folder: string): string | undefined {
		return this.entries.get(folder)?.cursor;
	}

	/** How many cursors are held for one folder: always one, or none. */
	cursorCount(folder: string): number {
		return this.entries.get(folder)?.cursor === undefined ? 0 : 1;
	}

	/** Whether a watcher was constructed for one folder. */
	hasWatcher(folder: string): boolean {
		return this.entries.get(folder)?.watcher !== undefined;
	}

	/**
	 * Reports the view's visibility.
	 *
	 * Polling stops while the view is hidden, because a checkpoint nobody can
	 * see is a spawn nobody asked for, and resumes with an immediate check
	 * rather than waiting out the interval, so a reader who opens the view
	 * sees the current board rather than the board as of ten seconds ago.
	 */
	setVisible(visible: boolean): void {
		if (visible === this.visible) {
			return;
		}
		this.visible = visible;
		if (visible) {
			this.startTimer();
			void this.checkAll();
		} else {
			this.stopTimer();
		}
	}

	/** Checks every entry now, ignoring the debounce and the throttle. */
	async refreshNow(): Promise<void> {
		await this.checkAll();
	}

	/** Checks one entry now, ignoring the debounce and the throttle. */
	async checkNow(folder: string): Promise<void> {
		const state = this.entries.get(folder);
		if (state !== undefined) {
			await this.check(state);
		}
	}

	private startTimer(): void {
		if (this.timer !== undefined || !this.visible) {
			return;
		}
		const ms = Math.max(2, this.deps.pollIntervalSeconds) * 1000;
		this.timer = this.deps.clock.setInterval(() => {
			void this.checkAll();
		}, ms);
	}

	private stopTimer(): void {
		if (this.timer !== undefined) {
			this.deps.clock.clearInterval(this.timer);
			this.timer = undefined;
		}
	}

	/**
	 * A file save arrived.
	 *
	 * The debounce collapses a burst into one check, and the throttle keeps
	 * two bursts a moment apart from becoming two checks a moment apart. They
	 * are two mechanisms rather than one because they answer two different
	 * questions: the debounce asks whether the writing has stopped, and the
	 * throttle asks whether enough time has passed since the last check.
	 */
	private onFileEvent(folder: string): void {
		const state = this.entries.get(folder);
		if (state === undefined) {
			return;
		}
		if (state.debounce !== undefined) {
			this.deps.clock.clearTimeout(state.debounce);
		}
		state.debounce = this.deps.clock.setTimeout(() => {
			state.debounce = undefined;
			this.afterDebounce(state);
		}, DEBOUNCE_MS);
	}

	private afterDebounce(state: EntryState): void {
		const since = this.deps.clock.now() - state.lastOffCycle;
		if (since >= THROTTLE_MS) {
			void this.check(state);
			return;
		}
		// Inside the floor. The check is deferred to the moment the floor
		// lifts rather than dropped, because the save that triggered it is
		// exactly the change the reader wants to see.
		state.debounce = this.deps.clock.setTimeout(() => {
			state.debounce = undefined;
			void this.check(state);
		}, THROTTLE_MS - since);
	}

	private async checkAll(): Promise<void> {
		for (const state of this.entries.values()) {
			await this.check(state);
		}
	}

	/** One checkpoint: ask, and redraw only if the answer says to. */
	private async check(state: EntryState): Promise<void> {
		state.lastOffCycle = this.deps.clock.now();
		const args =
			state.entry.scope === "root"
				? ["changes", "--root", state.entry.path]
				: ["--workbench", state.entry.path, "changes"];
		const argv =
			state.cursor === undefined ? args : [...args, "--since", state.cursor];

		const outcome = await runDinah(this.deps.spawner, this.deps.exe, argv, {
			cwd: state.entry.path,
		});

		if (outcome.kind !== "ok") {
			if (outcome.kind === "refused" && outcome.refusal === MALFORMED) {
				// The binary will not read the token we are holding, single or
				// merged. Start over rather than retry it: the next call mints
				// a fresh one and reports no change against it.
				this.deps.log(
					`dinah changes at ${state.entry.path} refused the cursor; starting over`,
				);
				state.cursor = undefined;
				return;
			}
			// Logged and otherwise ignored. The tree keeps its last good
			// content and the next tick tries again with the same cursor.
			this.deps.log(
				`dinah changes at ${state.entry.path}: ${outcome.kind}`,
			);
			return;
		}

		const answer = outcome.json as RootChangeSetAnswer & ChangeSetAnswer;
		if (typeof answer.cursor === "string") {
			state.cursor = answer.cursor;
		}
		for (const member of answer.workbenches ?? []) {
			if (member.unanswered !== undefined && member.unanswered !== "") {
				this.deps.log(
					`workbench at ${member.path} did not answer this checkpoint: ${member.unanswered}`,
				);
			}
		}
		if (answer.changed !== true) {
			return;
		}
		await this.deps.refresh(state.entry.folder);
		this.deps.fire();
	}
}
