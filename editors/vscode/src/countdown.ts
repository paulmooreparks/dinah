// The timer that keeps a lease countdown honest between checkpoints.
//
// A lease counts down without anything changing on disk, so `dinah changes`
// reports no change and CheckpointLoop never fires for it. A bar composed
// once at activation therefore shows the same remaining time for as long as
// the window stays open, which is the defect this module answers.
//
// Nothing here spawns a process, and that is the whole point of it. The
// expiry a card carries is an absolute instant, so the time left is a pure
// function of that instant and the wall clock, and a redraw needs no new
// answer from the CLI. Every call the extension makes to dinah still comes
// from the checkpoint loop and from the initial load.
//
// The clock is injected, as CheckpointLoop's already is, so the unit layer
// drives the tick without waiting thirty seconds for it.

import type { Clock } from "./changes";
import { COUNTDOWN_INTERVAL_MS } from "./status";

/**
 * The interval that redraws the status bar from data already held.
 *
 * It is a class rather than a bare setInterval so that the handle has an
 * owner, which is what lets deactivate() clear it the way it already stops
 * the checkpoint loop. An interval nobody holds the handle for outlives the
 * extension that started it.
 */
export class CountdownTicker {
	private timer?: unknown;

	constructor(
		private readonly clock: Clock,
		private readonly render: () => void,
		private readonly intervalMs: number = COUNTDOWN_INTERVAL_MS,
	) {}

	/** Starts ticking, or does nothing when it is already ticking. */
	start(): void {
		if (this.timer !== undefined) {
			return;
		}
		this.timer = this.clock.setInterval(() => {
			this.render();
		}, this.intervalMs);
	}

	/** Stops ticking and drops the handle. */
	stop(): void {
		if (this.timer === undefined) {
			return;
		}
		this.clock.clearInterval(this.timer);
		this.timer = undefined;
	}

	/** Whether a handle is currently held, which the unit layer reads back. */
	get running(): boolean {
		return this.timer !== undefined;
	}
}

/**
 * Wraps the checkpoint loop's refresh callback so that a checkpoint which
 * redraws the tree redraws the bar with it.
 *
 * The loop calls its refresh only when `dinah changes` reported a change, so
 * this is the path a claim, a release or a block anywhere in a watched
 * workbench arrives on. The redraw follows the refresh rather than racing it,
 * because the bar reads what the refresh just stored.
 */
export function redrawAfterRefresh(
	refresh: (folder: string) => Promise<void>,
	render: () => void,
): (folder: string) => Promise<void> {
	return async (folder) => {
		await refresh(folder);
		render();
	};
}
