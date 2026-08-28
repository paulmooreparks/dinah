// The checkpoint loop, driven on a fake clock.
//
// Every timing assertion below is about a mechanism rather than about a
// duration somebody waited out, which is why the clock is injected: a test
// that really slept 400ms to prove a 400ms debounce would be slow and would
// still not prove the debounce is what fired.
//
// The debounce and the throttle are asserted separately, because they answer
// two different questions and a single mechanism doing both would pass a test
// written against either one alone.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import type { CheckpointEntry, Clock } from "../../src/changes";
import { CheckpointLoop, DEBOUNCE_MS, MALFORMED, THROTTLE_MS } from "../../src/changes";

/** A clock whose hands only move when a test moves them. */
interface FakeClock extends Clock {
	/** Runs every timer due within `ms` of now, advancing the clock as it goes. */
	readonly advance: (ms: number) => void;
	/** Fires every interval once, without advancing the clock. */
	readonly tickIntervals: () => void;
}

function fakeClock(): FakeClock {
	let current = 0;
	let next = 1;
	const timeouts = new Map<number, { at: number; fn: () => void }>();
	const intervals = new Map<number, { every: number; fn: () => void }>();
	return {
		now: () => current,
		setTimeout: (fn, ms) => {
			const handle = next++;
			timeouts.set(handle, { at: current + ms, fn });
			return handle;
		},
		clearTimeout: (handle) => {
			timeouts.delete(handle as number);
		},
		setInterval: (fn, every) => {
			const handle = next++;
			intervals.set(handle, { every, fn });
			return handle;
		},
		clearInterval: (handle) => {
			intervals.delete(handle as number);
		},
		advance: (ms) => {
			const target = current + ms;
			for (;;) {
				const due = [...timeouts.entries()]
					.filter(([, entry]) => entry.at <= target)
					.sort((a, b) => a[1].at - b[1].at)[0];
				if (due === undefined) {
					break;
				}
				timeouts.delete(due[0]);
				current = due[1].at;
				due[1].fn();
			}
			current = target;
		},
		tickIntervals: () => {
			for (const entry of [...intervals.values()]) {
				entry.fn();
			}
		},
	};
}

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

/** A spawner counting `changes` calls and answering as the test dictates. */
function counting(answers: () => unknown): {
	spawner: Spawner;
	calls: string[][];
} {
	const calls: string[][] = [];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return ok(answers());
	};
	return { spawner, calls };
}

const ENTRY: CheckpointEntry = {
	folder: "C:\\work\\bench",
	scope: "workbench",
	path: "C:\\work\\bench",
};

const ROOT_ENTRY: CheckpointEntry = {
	folder: "C:\\customers",
	scope: "root",
	path: "C:\\customers",
};

interface Harness {
	readonly loop: CheckpointLoop;
	readonly clock: FakeClock;
	readonly calls: string[][];
	readonly refreshed: string[];
	readonly fired: number[];
	/** Set by a test to change what the next changes call answers. */
	answer: unknown;
	/** Set by a test to hand the loop a watcher event. */
	save?: () => void;
}

function harness(
	over: {
		watchFiles?: boolean;
		entry?: CheckpointEntry;
	} = {},
): Harness {
	const clock = fakeClock();
	const refreshed: string[] = [];
	const fired: number[] = [];
	const state: Harness = {
		loop: undefined as unknown as CheckpointLoop,
		clock,
		calls: [],
		refreshed,
		fired,
		answer: { cursor: "c1", changed: false },
	};
	const { spawner, calls } = counting(() => state.answer);
	(state as { calls: string[][] }).calls = calls;
	const loop = new CheckpointLoop({
		spawner,
		exe: "dinah",
		clock,
		log: () => undefined,
		refresh: async (folder) => {
			refreshed.push(folder);
		},
		fire: () => fired.push(clock.now()),
		pollIntervalSeconds: 10,
		watchFiles: over.watchFiles ?? true,
		createWatcher: (_entry, onEvent) => {
			state.save = onEvent;
			return { dispose: () => undefined };
		},
	});
	(state as { loop: CheckpointLoop }).loop = loop;
	loop.start([over.entry ?? ENTRY]);
	return state;
}

// ---------------------------------------------------------------------------
// AC-7: a checkpoint redraws only when the answer says something changed
// ---------------------------------------------------------------------------

test("an unchanged answer re-fetches nothing and fires nothing", async () => {
	const h = harness();
	h.answer = { cursor: "c2", changed: false };
	await h.loop.refreshNow();
	assert.equal(h.calls.length, 1, "the checkpoint itself still runs");
	assert.deepEqual(h.refreshed, [], "an unchanged answer must not re-read the board");
	assert.deepEqual(h.fired, [], "an unchanged answer must not repaint the tree");
});

test("a changed answer fetches once and fires once", async () => {
	const h = harness();
	h.answer = { cursor: "c2", changed: true };
	await h.loop.refreshNow();
	assert.deepEqual(h.refreshed, [ENTRY.folder]);
	assert.equal(h.fired.length, 1);
});

test("the cursor handed back is the one the next call carries", async () => {
	const h = harness();
	h.answer = { cursor: "minted", changed: false };
	await h.loop.refreshNow();
	assert.equal(h.loop.cursorFor(ENTRY.folder), "minted");
	await h.loop.refreshNow();
	assert.deepEqual(h.calls[1].slice(-2), ["--since", "minted"]);
	// The first call carries none, because there is nothing yet to be since.
	assert.ok(!h.calls[0].includes("--since"));
});

test("a cursor the binary will not read is dropped rather than retried", async () => {
	const clock = fakeClock();
	let refusing = false;
	const spawner: Spawner = async () =>
		refusing
			? { code: 2, stdout: JSON.stringify({ refusal: MALFORMED }), stderr: "" }
			: ok({ cursor: "c1", changed: false });
	const loop = new CheckpointLoop({
		spawner,
		exe: "dinah",
		clock,
		log: () => undefined,
		refresh: async () => undefined,
		fire: () => undefined,
		pollIntervalSeconds: 10,
		watchFiles: false,
	});
	loop.start([ENTRY]);
	await loop.refreshNow();
	assert.equal(loop.cursorFor(ENTRY.folder), "c1");
	refusing = true;
	await loop.refreshNow();
	assert.equal(loop.cursorFor(ENTRY.folder), undefined, "a bad token must be let go");
});

// ---------------------------------------------------------------------------
// AC-22: one merged cursor per folder, handed back verbatim
// ---------------------------------------------------------------------------

test("a root-scoped folder holds one merged token, not one per member", async () => {
	const h = harness({ entry: ROOT_ENTRY });
	h.answer = {
		root: "C:\\customers",
		cursor: "bWVyZ2Vk",
		changed: true,
		workbenches: [
			{ title: "Acme Co", path: "C:\\customers\\acme\\board", changes: { cursor: "acme-own", changed: true } },
			{ title: "Bell", path: "C:\\customers\\bell\\tracker", changes: { cursor: "bell-own", changed: false } },
		],
	};
	await h.loop.refreshNow();
	// One token for the folder. Not two, and not either member's own.
	assert.equal(h.loop.cursorCount(ROOT_ENTRY.folder), 1);
	assert.equal(h.loop.cursorFor(ROOT_ENTRY.folder), "bWVyZ2Vk");

	h.answer = { root: "C:\\customers", cursor: "bWVyZ2Vk", changed: false, workbenches: [] };
	await h.loop.refreshNow();
	// Handed back exactly as it arrived: not decoded, and no per-member token
	// derived from it.
	assert.deepEqual(h.calls[1].slice(-2), ["--since", "bWVyZ2Vk"]);
	assert.ok(h.calls[1].includes("--root"));
});

test("a single-workbench folder pins its call and a root-scoped one walks", async () => {
	const single = harness();
	await single.loop.refreshNow();
	assert.deepEqual(single.calls[0], ["--json", "--workbench", "C:\\work\\bench", "changes"]);

	const walking = harness({ entry: ROOT_ENTRY });
	await walking.loop.refreshNow();
	assert.deepEqual(walking.calls[0], ["--json", "changes", "--root", "C:\\customers"]);
});

// ---------------------------------------------------------------------------
// AC-8: the debounce and the throttle, as two mechanisms
// ---------------------------------------------------------------------------

test("a burst of five saves collapses into one check, after the debounce window", async () => {
	const h = harness();
	assert.ok(h.save !== undefined, "no watcher was wired");
	let lastSaveAt = 0;
	for (let i = 0; i < 5; i += 1) {
		lastSaveAt = h.clock.now();
		h.save();
		h.clock.advance(50);
	}
	// Two hundred milliseconds of saves have gone by and nothing has run: the
	// window keeps resetting while the writing continues.
	assert.equal(h.calls.length, 0);
	// Up to one tick short of the window closing on the LAST save, which is
	// what the debounce measures from.
	h.clock.advance(lastSaveAt + DEBOUNCE_MS - h.clock.now() - 1);
	assert.equal(h.calls.length, 0, "the check ran before the window closed");
	h.clock.advance(2);
	await Promise.resolve();
	assert.equal(h.calls.length, 1, "the burst produced other than one check");
});

test("a second burst is held to the throttle floor since the first check", async () => {
	const h = harness();
	assert.ok(h.save !== undefined);
	h.save();
	h.clock.advance(DEBOUNCE_MS + 1);
	await Promise.resolve();
	assert.equal(h.calls.length, 1);
	const firstAt = h.clock.now();

	// The second burst starts 200ms after the first check fired.
	h.clock.advance(200);
	h.save();
	h.clock.advance(DEBOUNCE_MS + 1);
	await Promise.resolve();
	// The debounce has closed, but the floor has not lifted, so nothing ran.
	assert.equal(h.calls.length, 1, "the throttle floor was bypassed");
	h.clock.advance(THROTTLE_MS);
	await Promise.resolve();
	assert.equal(h.calls.length, 2);
	assert.ok(
		h.clock.now() - firstAt >= THROTTLE_MS,
		"the second check ran inside the floor",
	);
});

// ---------------------------------------------------------------------------
// AC-9: the watcher is optional and the poll is not
// ---------------------------------------------------------------------------

test("with watching off no watcher is built and the poll still drives the loop", async () => {
	const h = harness({ watchFiles: false });
	assert.equal(h.loop.hasWatcher(ENTRY.folder), false);
	assert.equal(h.save, undefined, "a watcher was constructed anyway");
	h.clock.tickIntervals();
	await Promise.resolve();
	assert.equal(h.calls.length, 1, "turning the watcher off stopped the poll too");
});

test("with watching on a watcher is built", () => {
	const h = harness({ watchFiles: true });
	assert.equal(h.loop.hasWatcher(ENTRY.folder), true);
});

// ---------------------------------------------------------------------------
// AC-10: the poll follows the view's visibility
// ---------------------------------------------------------------------------

test("the poll stops while the view is hidden and checks at once when it returns", async () => {
	const h = harness();
	h.loop.setVisible(false);
	h.clock.tickIntervals();
	await Promise.resolve();
	assert.equal(h.calls.length, 0, "the poll went on running behind a hidden view");

	h.loop.setVisible(true);
	// Within one turn of the event loop, and not at the next scheduled tick.
	await Promise.resolve();
	await Promise.resolve();
	assert.equal(h.calls.length, 1, "becoming visible waited for the next tick");

	h.clock.tickIntervals();
	await Promise.resolve();
	assert.equal(h.calls.length, 2, "the timer did not restart with the view");
});
