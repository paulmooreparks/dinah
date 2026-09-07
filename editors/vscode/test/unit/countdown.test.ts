// What redraws the status bar, how often, and what it costs.
//
// Two things are asserted here and they need two different kinds of check.
// The behaviour of the ticker and of the checkpoint wrapper is driven on a
// fake clock and a fake spawner, which is what proves a tick costs no process
// and that deactivation clears the handle. Where the extension calls them is
// a fact about activate() itself, and activate() imports vscode, so the unit
// layer cannot run it; that half is read off the module's own text.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { Clock } from "../../src/changes";
import { CheckpointLoop } from "../../src/changes";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import { CountdownTicker, redrawAfterRefresh } from "../../src/countdown";
import { COUNTDOWN_INTERVAL_MS } from "../../src/status";

const extensionRoot = join(__dirname, "..", "..", "..");

/** One interval a fake clock is holding, so a test can fire it by hand. */
interface FakeInterval {
	readonly fn: () => void;
	readonly ms: number;
	cleared: boolean;
}

/** A clock that runs nothing on its own and records every handle taken out. */
function fakeClock(): { clock: Clock; intervals: FakeInterval[] } {
	const intervals: FakeInterval[] = [];
	const clock: Clock = {
		now: () => 0,
		setTimeout: (fn, ms) => ({ fn, ms }),
		clearTimeout: () => {},
		setInterval: (fn, ms) => {
			const entry: FakeInterval = { fn, ms, cleared: false };
			intervals.push(entry);
			return entry;
		},
		clearInterval: (handle) => {
			(handle as FakeInterval).cleared = true;
		},
	};
	return { clock, intervals };
}

/** An ok spawn outcome carrying the payload given. */
function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

test("a tick redraws the bar and asks dinah nothing", async () => {
	// AC-10, driven over the same three paths activate() wires: the first
	// paint, the checkpoint loop's refresh callback, and the countdown's own
	// interval. Every call to the spawner is counted, which is what proves
	// that the ticker and the checkpoint wrapper spawn nothing of their own.
	// The render this test passes is a stub, so a renderStatusBar that went
	// to the CLI would not be caught here; what keeps that true is that
	// renderStatusBar reaches only holdingSnapshot, summarizeHolding,
	// composeStatus and the status bar item's own fields, and that neither
	// status.ts nor countdown.ts imports a spawner.
	let renders = 0;
	const render = (): void => {
		renders += 1;
	};
	const calls: string[][] = [];
	let changed = true;
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		return ok({ cursor: "c1", changed });
	};
	let refreshes = 0;
	const refresh = redrawAfterRefresh(async () => {
		refreshes += 1;
	}, render);

	const { clock, intervals } = fakeClock();
	const loop = new CheckpointLoop({
		spawner,
		exe: "dinah",
		clock,
		log: () => {},
		refresh,
		fire: () => {},
		pollIntervalSeconds: 10,
		watchFiles: false,
	});
	const ticker = new CountdownTicker(clock, render);

	// The first paint, which activate() makes once the initial load answered.
	render();
	assert.equal(renders, 1);

	loop.start([{ folder: "C:\\ws", scope: "workbench", path: "C:\\work\\bench" }]);
	ticker.start();

	// A checkpoint that found a change: one `dinah changes` call, one tree
	// refresh, and one redraw behind it.
	await loop.refreshNow();
	assert.equal(refreshes, 1);
	assert.equal(renders, 2);
	const afterCheckpoint = calls.length;
	assert.equal(afterCheckpoint, 1);

	// The countdown's own tick. It redraws, and the tick itself spawns
	// nothing, which is the half of the design's claim a test can drive.
	const countdown = intervals.find((entry) => entry.ms === COUNTDOWN_INTERVAL_MS);
	assert.notEqual(countdown, undefined, "the countdown took out an interval");
	countdown?.fn();
	assert.equal(renders, 3);
	assert.equal(calls.length, afterCheckpoint, "the tick spawned nothing");

	// And a checkpoint that found nothing changed redraws nothing either, so
	// the count above is three renders rather than three plus a poll.
	changed = false;
	await loop.refreshNow();
	assert.equal(renders, 3);
	assert.equal(refreshes, 1);

	loop.stop();
	ticker.stop();
});

test("the countdown's interval is thirty seconds and is cleared on the way out", () => {
	// AC-11. The loop's own timer and the countdown's are two handles, and
	// stopping one must not be mistaken for stopping the other, so both are
	// asserted.
	const { clock, intervals } = fakeClock();
	const loop = new CheckpointLoop({
		spawner: async () => ok({ cursor: "c1", changed: false }),
		exe: "dinah",
		clock,
		log: () => {},
		refresh: async () => {},
		fire: () => {},
		pollIntervalSeconds: 10,
		watchFiles: false,
	});
	const ticker = new CountdownTicker(clock, () => {});

	loop.start([{ folder: "C:\\ws", scope: "workbench", path: "C:\\work\\bench" }]);
	ticker.start();
	assert.equal(ticker.running, true);
	assert.deepEqual(
		intervals.map((entry) => entry.ms),
		[10_000, COUNTDOWN_INTERVAL_MS],
	);
	assert.equal(COUNTDOWN_INTERVAL_MS, 30_000);

	// Starting twice takes out one handle, so a second activation could not
	// leave an orphan ticking behind the first.
	ticker.start();
	assert.equal(intervals.length, 2);

	// What deactivate() does, in the order it does it.
	loop.stop();
	ticker.stop();
	assert.deepEqual(
		intervals.map((entry) => entry.cleared),
		[true, true],
	);
	assert.equal(ticker.running, false);
});

test("activate reaches the bar from three places and deactivate clears the ticker", () => {
	// AC-10's other half. activate() imports vscode, so no test in this layer
	// can run it, and the claim that there is no fourth call site is a claim
	// about the module's text. It is read here rather than reviewed, because
	// a fourth call site added later is exactly what nobody would notice.
	const source = readFileSync(join(extensionRoot, "src", "extension.ts"), "utf8");
	// Comment lines are dropped before the scan, because this module explains
	// the three call sites in prose beside them and a doc comment naming the
	// function is not a call to it.
	const lines = source
		.split("\n")
		.filter((line) => !/^\s*(\/\/|\*|\/\*)/.test(line));
	const invocations = lines.filter((line) => /(^|[^.\w])renderStatusBar\b/.test(line));
	// The declaration, then the three sites: the initial load, the checkpoint
	// loop's refresh callback, and the countdown ticker's own render.
	assert.deepEqual(invocations.map((line) => line.trim()), [
		"function renderStatusBar(): void {",
		"renderStatusBar();",
		"renderStatusBar,",
		"ticker = new CountdownTicker(systemClock, renderStatusBar);",
	]);

	// composeStatus is reached from renderStatusBar and from the pre-load
	// paint beside it, and from nowhere else in the extension.
	const composeCalls = lines.filter((line) => /\bcomposeStatus\(/.test(line));
	assert.equal(composeCalls.length, 2);

	// The teardown, which is what keeps a reloaded window from leaving an
	// interval ticking against a disposed status bar item.
	const deactivate = source.slice(source.indexOf("export function deactivate"));
	assert.ok(deactivate.includes("ticker?.stop();"));
	assert.ok(deactivate.includes("loop?.stop();"));
});
