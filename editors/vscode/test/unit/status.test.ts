import assert from "node:assert/strict";
import { test } from "node:test";

import type { BinaryState, WorkbenchResolution } from "../../src/api";
import { ENGLISH } from "../../src/l10n";
import type {
	HeldCardView,
	HoldingSummary,
	WorkbenchHoldingReport,
} from "../../src/status";
import {
	NOTHING_HELD,
	composeContextKeys,
	composeStatus,
	staleAfterMs,
	summarizeHolding,
} from "../../src/status";
import { AMBIGUOUS_WORKBENCH, NO_WORKBENCH_FOUND } from "../../src/workbench";

const GOOD_BINARY: BinaryState = {
	state: "ok",
	path: "/usr/local/bin/dinah",
	source: "path",
	version: { tool: "v0.1.0-dev.42", profile: "dinah-core/0.4", format: 1 },
};

const INSIDE: WorkbenchResolution = {
	state: "ok",
	root: "/w/.dinah/abc",
	title: "Dinah",
	source: "search",
	profile: "dinah-core/0.4",
	insideWorkspace: true,
};

const OUTSIDE: WorkbenchResolution = { ...INSIDE, insideWorkspace: false };

test("a resolved workbench inside the workspace shows its title and leads with the root", () => {
	const view = composeStatus(GOOD_BINARY, INSIDE, "source");
	assert.equal(view.hidden, false);
	assert.equal(view.text, "$(checklist) Dinah");
	assert.equal(view.tooltip.split("\n")[0], "/w/.dinah/abc");
	assert.ok(view.tooltip.includes("resolved by search"));
	assert.ok(view.tooltip.includes("dinah v0.1.0-dev.42, dinah-core/0.4, format 1"));
	assert.ok(view.tooltip.includes("binary: /usr/local/bin/dinah (path)"));
});

test("a workbench outside the workspace warns and says so first", () => {
	// This is the dinah-241 visibility rule. An extension that showed only the
	// title would pass every other assertion here.
	const view = composeStatus(GOOD_BINARY, OUTSIDE, "source");
	assert.equal(view.text, "$(checklist) Dinah $(warning)");
	assert.equal(
		view.tooltip.split("\n")[0],
		"This workbench is outside your workspace: /w/.dinah/abc",
	);
});

test("an ambiguous refusal warns, names no winner, and lists the candidates", () => {
	const view = composeStatus(
		GOOD_BINARY,
		{
			state: "refused",
			refusal: AMBIGUOUS_WORKBENCH,
			answered: true,
			candidates: [
				{ title: "one", path: "/base/.dinah/aaa" },
				{ title: "two", path: "/base/.dinah/bbb" },
			],
		},
		"source",
	);
	assert.equal(view.text, "$(checklist) Dinah $(warning)");
	assert.ok(view.tooltip.includes("Set dinah.workbench to choose one."));
	assert.ok(view.tooltip.includes("/base/.dinah/aaa"));
	assert.ok(view.tooltip.includes("/base/.dinah/bbb"));
});

test("no workbench found hides the item", () => {
	const view = composeStatus(
		GOOD_BINARY,
		{ state: "refused", refusal: NO_WORKBENCH_FOUND, answered: true },
		"source",
	);
	assert.equal(view.hidden, true);
});

test("no usable binary shows an error and names the two remedies", () => {
	const view = composeStatus({ state: "no-binary" }, undefined, "source");
	assert.equal(view.text, "$(checklist) Dinah $(error)");
	assert.ok(view.tooltip.includes("dinah.path"));
	assert.ok(view.tooltip.includes("github.com/paulmooreparks/dinah"));
});

test("a skewed binary shows an error carrying the gate's own diagnostic", () => {
	const view = composeStatus(
		{
			state: "format-skew",
			path: "dinah",
			detail: "this binary writes storage format 99, and this extension supports 1",
			version: { tool: "x", profile: "dinah-core/0.4", format: 99 },
		},
		INSIDE,
		"source",
	);
	assert.equal(view.text, "$(checklist) Dinah $(error)");
	assert.ok(view.tooltip.includes("storage format 99"));
});

test("the paired release is displayed", () => {
	const view = composeStatus(GOOD_BINARY, INSIDE, "v0.1.0-dev.42");
	assert.ok(view.tooltip.includes("extension paired with dinah v0.1.0-dev.42"));
});

test("the context keys drive every welcome case", () => {
	assert.deepEqual(composeContextKeys({ state: "no-binary" }, undefined), {
		binary: "missing",
		workbench: "unknown",
	});
	// A window with a usable binary and no folder to resolve from. The
	// manifest gives this state a block of its own, so it is composed here
	// rather than left to fall through to the pre-activation text.
	assert.deepEqual(composeContextKeys(GOOD_BINARY, undefined), {
		binary: "ok",
		workbench: "unknown",
	});
	assert.deepEqual(
		composeContextKeys(GOOD_BINARY, {
			state: "refused",
			refusal: NO_WORKBENCH_FOUND,
			answered: true,
		}),
		{ binary: "ok", workbench: "none" },
	);
	assert.deepEqual(
		composeContextKeys(GOOD_BINARY, {
			state: "refused",
			refusal: AMBIGUOUS_WORKBENCH,
			answered: true,
		}),
		{ binary: "ok", workbench: "ambiguous" },
	);
	assert.deepEqual(composeContextKeys(GOOD_BINARY, INSIDE), {
		binary: "ok",
		workbench: "ok",
	});
});

// ---------------------------------------------------------------------------
// dinah-419: the held card and the time left on its claim
// ---------------------------------------------------------------------------

/** A fixed instant every case below counts from, so no test reads a clock. */
const NOW = Date.parse("2026-09-07T12:00:00Z");

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

/** An RFC3339 stamp the given number of milliseconds from NOW. */
function stampAt(offsetMs: number): string {
	return new Date(NOW + offsetMs).toISOString().replace(/\.\d{3}Z$/, "Z");
}

/** One held card, with the workbench title a fixture rarely varies. */
function held(
	ref: string,
	expiresAt: string,
	workbenchTitle = "Trees",
): HeldCardView {
	return { ref, workbenchTitle, expiresAt };
}

/** A summary carrying the cards given, with nothing stale behind it. */
function holdingOf(...cards: HeldCardView[]): HoldingSummary {
	return { cards, uncertain: false };
}

/** The idle rendering every held-card case is compared against. */
const IDLE_TEXT = "$(checklist) Dinah";
const IDLE_TOOLTIP = [
	"/w/.dinah/abc",
	"resolved by search",
	"dinah v0.1.0-dev.42, dinah-core/0.4, format 1",
	"binary: /usr/local/bin/dinah (path)",
	"extension paired with dinah source",
].join("\n");

test("holding nothing renders exactly what the three-argument call rendered", () => {
	// AC-2. The two call shapes are compared against each other and both are
	// compared against a written-out snapshot, because a comparison of the
	// function with itself would agree however the rendering changed.
	const before = composeStatus(GOOD_BINARY, INSIDE, "source");
	const after = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		{ cards: [], uncertain: false },
		NOW,
		ENGLISH,
	);
	assert.equal(after.text, before.text);
	assert.equal(after.tooltip, before.tooltip);
	assert.equal(after.text, IDLE_TEXT);
	assert.equal(after.tooltip, IDLE_TOOLTIP);
	assert.equal(after.hidden, false);
	// The regression guard for D4: nothing about what is waiting on the
	// reader joined the idle path.
	assert.ok(!after.tooltip.includes("Holding"));
	assert.ok(!/\+\d/.test(after.text));
});

test("one held card names itself and the hours and minutes left", () => {
	// AC-3.
	const view = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(held("dn-7", stampAt(90 * MINUTE))),
		NOW,
		ENGLISH,
	);
	assert.equal(view.text, "$(checklist) Dinah dn-7 (1h 30m left)");
	assert.equal(view.tooltip.split("\n")[0], "Holding dn-7 in Trees: 1h 30m left");
	// The rest of the hover text is untouched, so the held line leads rather
	// than replacing what was already there.
	assert.equal(view.tooltip.split("\n").slice(1).join("\n"), IDLE_TOOLTIP);
});

test("the hour boundary falls where the clause says it does", () => {
	// AC-3's three sub-cases. Sixty minutes is the boundary itself, and the
	// minute either side of it decides which of the two clauses renders.
	const textAt = (offsetMs: number): string =>
		composeStatus(
			GOOD_BINARY,
			INSIDE,
			"source",
			holdingOf(held("dn-7", stampAt(offsetMs))),
			NOW,
			ENGLISH,
		).text;
	assert.equal(textAt(61 * MINUTE), "$(checklist) Dinah dn-7 (1h 1m left)");
	assert.equal(textAt(60 * MINUTE), "$(checklist) Dinah dn-7 (1h 0m left)");
	assert.equal(textAt(59 * MINUTE), "$(checklist) Dinah dn-7 (59m left)");
	// Ninety minutes is not flattened to ninety minutes, and it is not
	// truncated to the hour either, which are the two wrong renderings.
	assert.equal(textAt(90 * MINUTE), "$(checklist) Dinah dn-7 (1h 30m left)");
});

test("under a minute reads as under a minute rather than as zero", () => {
	const view = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(held("dn-7", stampAt(30_000))),
		NOW,
		ENGLISH,
	);
	assert.equal(view.text, "$(checklist) Dinah dn-7 (less than a minute left)");
});

test("a lapsed lease says so and never counts backwards", () => {
	// AC-4. The boundary is the instant of expiry itself, and the two cases
	// beyond it are a second and an hour past.
	for (const offset of [0, -1000, -HOUR]) {
		const view = composeStatus(
			GOOD_BINARY,
			INSIDE,
			"source",
			holdingOf(held("dn-7", stampAt(offset))),
			NOW,
			ENGLISH,
		);
		assert.equal(
			view.text,
			"$(checklist) Dinah dn-7 (lease expired)",
			`offset ${String(offset)}`,
		);
		assert.equal(
			view.tooltip.split("\n")[0],
			"Holding dn-7 in Trees: lease expired",
		);
		// The clause itself is read out of the parentheses rather than the
		// whole bar, because the card's own reference carries a hyphen and a
		// digit and would satisfy a search for a negative number.
		const clause = /\(([^)]*)\)$/.exec(view.text)?.[1] ?? "";
		assert.equal(clause, "lease expired");
		assert.ok(!clause.includes("-"), "no minus sign reaches the clause");
		assert.ok(!/\d/.test(clause), "no number at all reaches the clause");
		assert.ok(!view.text.includes("left"), "no remaining-time clause renders");
	}
});

test("a claim with no lease says so, whatever the clock reads", () => {
	// AC-5. The same empty stamp is rendered at two clocks a day apart.
	const early = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(held("dn-7", "")),
		NOW,
		ENGLISH,
	);
	const late = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(held("dn-7", "")),
		NOW + 24 * HOUR,
		ENGLISH,
	);
	assert.equal(early.text, "$(checklist) Dinah dn-7 (no lease)");
	assert.equal(late.text, early.text);
	assert.equal(late.tooltip, early.tooltip);
});

test("nothing confirmed warns rather than claiming an empty hand", () => {
	// AC-6. The text differs from the idle baseline by the warning glyph and
	// by nothing else, and the hover says outright that this window could not
	// confirm what the reader holds.
	const view = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		{ cards: [], uncertain: true },
		NOW,
		ENGLISH,
	);
	assert.equal(view.text, `${IDLE_TEXT} $(warning)`);
	assert.equal(
		view.tooltip.split("\n")[0],
		"Dinah could not confirm whether you are holding anything right now",
	);
	assert.equal(view.tooltip.split("\n").slice(1).join("\n"), IDLE_TOOLTIP);
});

test("a stale workbench beside a confirmed held card does not undermine it", () => {
	// AC-7. One workbench answered and reports a card; a second has not been
	// heard from. Something confirmed is on screen, so nothing warns.
	const stale = staleAfterMs(10);
	const summary = summarizeHolding(
		[
			{
				state: "answered",
				source: "C:\\trees",
				title: "Trees",
				holding: [{ id: "aaa", ref: "dn-7", expires: stampAt(90 * MINUTE) }],
				fetchedAt: NOW - 1000,
			},
			{
				state: "answered",
				source: "C:\\maps",
				title: "Maps",
				holding: [],
				fetchedAt: NOW - stale - 1,
			},
		],
		NOW,
		stale,
	);
	assert.equal(summary.uncertain, false);
	assert.equal(summary.cards.length, 1);

	const view = composeStatus(GOOD_BINARY, INSIDE, "source", summary, NOW, ENGLISH);
	const single = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(held("dn-7", stampAt(90 * MINUTE))),
		NOW,
		ENGLISH,
	);
	assert.equal(view.text, single.text);
	assert.equal(view.tooltip, single.tooltip);
	assert.ok(!view.text.includes("$(warning)"));
	assert.ok(!view.tooltip.includes("could not confirm"));
});

test("several held cards name the soonest and count the rest", () => {
	// AC-8. Three workbenches and three distinct expiries, then the same
	// again with a leaseless claim among them to prove where it sorts.
	const view = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(
			held("dn-9", stampAt(3 * HOUR), "Maps"),
			held("dn-7", stampAt(20 * MINUTE), "Trees"),
			held("dn-8", stampAt(2 * HOUR), "Ledgers"),
		),
		NOW,
		ENGLISH,
	);
	assert.equal(view.text, "$(checklist) Dinah dn-7 (20m left) +2");
	assert.deepEqual(view.tooltip.split("\n").slice(0, 3), [
		"Holding 3 cards; soonest is dn-7 in Trees, 20m left",
		"dn-8 in Ledgers: 2h 0m left",
		"dn-9 in Maps: 3h 0m left",
	]);

	const withLeaseless = composeStatus(
		GOOD_BINARY,
		INSIDE,
		"source",
		holdingOf(
			held("dn-0", "", "Atlas"),
			held("dn-9", stampAt(3 * HOUR), "Maps"),
			held("dn-7", stampAt(20 * MINUTE), "Trees"),
		),
		NOW,
		ENGLISH,
	);
	assert.equal(withLeaseless.text, "$(checklist) Dinah dn-7 (20m left) +2");
	assert.deepEqual(withLeaseless.tooltip.split("\n").slice(0, 3), [
		"Holding 3 cards; soonest is dn-7 in Trees, 20m left",
		"dn-9 in Maps: 3h 0m left",
		"dn-0 in Atlas: no lease",
	]);
});

test("a workbench nobody has heard from is stale rather than fresh", () => {
	// The freshness rule summarizeHolding applies, and the window it applies
	// it over, which tracks the configured poll interval rather than a second
	// number of its own.
	assert.equal(staleAfterMs(10), 30_000);
	assert.equal(staleAfterMs(20), 60_000);
	// The floor CheckpointLoop.startTimer applies to its own timer.
	assert.equal(staleAfterMs(0), 6_000);

	const never: WorkbenchHoldingReport = {
		state: "unheard",
		source: "C:\\trees",
	};
	assert.deepEqual(summarizeHolding([never], NOW, 30_000), {
		cards: [],
		uncertain: true,
	});
	// Just inside the window is fresh, and one millisecond beyond it is not.
	const at = (age: number): WorkbenchHoldingReport => ({
		state: "answered",
		source: "C:\\trees",
		title: "Trees",
		holding: [],
		fetchedAt: NOW - age,
	});
	assert.deepEqual(summarizeHolding([at(30_000)], NOW, 30_000), NOTHING_HELD);
	assert.deepEqual(summarizeHolding([at(30_001)], NOW, 30_000), {
		cards: [],
		uncertain: true,
	});
	// A card with no ref of its own falls back to its id, so a row is never
	// drawn with an empty name.
	assert.deepEqual(
		summarizeHolding(
			[
				{
					state: "answered",
					source: "C:\\trees",
					title: "Trees",
					holding: [{ id: "aaa" }],
					fetchedAt: NOW,
				},
			],
			NOW,
			30_000,
		),
		{
			cards: [{ ref: "aaa", workbenchTitle: "Trees", expiresAt: "" }],
			uncertain: false,
		},
	);
});
