// The served-text scheme: its URI grammar, its rendering, and its refresh
// timer, all driven without an editor.
//
// The loop is asserted on a fake clock for the reason changes.test.ts gives:
// a test that really waited out a ten-second poll would be slow and would
// still not prove which mechanism fired.
//
// dinah-270 AC-2, AC-3, AC-4, AC-5, AC-6 and AC-13.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { Clock } from "../../src/changes";
import {
	KIND_INSTRUCTIONS,
	ServedTextRefreshLoop,
	parseServedTextUri,
	renderInstructionsMarkdown,
	servedTextUriParts,
} from "../../src/servedText";
import type { InstructionChain, ServedAnswer } from "../../src/wire";

// ---------------------------------------------------------------------------
// The wire mirror
// ---------------------------------------------------------------------------

test("a real instructions payload assigns to ServedAnswer with no cast", () => {
	// dinah-270 AC-1. The literal below is the shape `dinah --json
	// instructions <ref>` emits, field for field against verb.Served and
	// verb.Instructions, and the assignment is the assertion: a field name or
	// an optionality that drifts from the Go json tags stops this file
	// compiling rather than failing at run time.
	const payload: ServedAnswer = {
		instructions: {
			global: "Read the card before you claim it.\n",
			standing: "This workbench ships a CLI.\n",
			column: "Where specs become code.\n",
		},
		legal_moves: [
			{ column: "review", ref: "review", title: "Review", direction: "forward" },
		],
		loop: { column: "review", limit: 3, count: 1, at_limit: false },
		column: "doing",
	};
	assert.equal(payload.column, "doing");
	assert.equal(payload.instructions.column, "Where specs become code.\n");
	assert.equal(payload.loop?.at_limit, false);

	// withheld and reread are mirrored too, and the CLI head never populates
	// them under this extension's calling pattern. Naming them here is what
	// keeps the mirror a mirror of the struct rather than of one caller's
	// subset.
	const withheld: ServedAnswer = {
		instructions: { withheld: ["global"], reread: "doing" },
		column: "doing",
	};
	assert.deepEqual(withheld.instructions.withheld, ["global"]);
	assert.equal(withheld.instructions.reread, "doing");
});

// ---------------------------------------------------------------------------
// The URI grammar
// ---------------------------------------------------------------------------

test("a composed served-text URI round-trips back to what it was composed from", () => {
	// dinah-270 AC-2 asks for a Windows root carrying a colon and a space, and
	// the second fixture is that root. The fourth fixture is what arms this
	// test, because a colon, a backslash and a space all survive an unencoded
	// query string and the Windows root alone therefore round-trips through a
	// grammar that concatenates rather than encodes. An ampersand opens a
	// third parameter, an equals sign moves the boundary between a name and
	// its value, and a plus sign decodes back as a space, so a root carrying
	// all three comes back wrong unless the composition really encodes.
	const fixtures = [
		{ kind: KIND_INSTRUCTIONS, root: "/home/paul/bench", ref: "dinah-270" },
		{
			kind: KIND_INSTRUCTIONS,
			root: "C:\\Users\\paul\\My Benches\\dinah",
			ref: "dinah-1",
		},
		{ kind: "guide", root: "C:\\dinah-scratch\\wt", ref: "quick start" },
		{
			kind: KIND_INSTRUCTIONS,
			root: "C:\\Work\\Bench & Vise\\ref=1\\a+b",
			ref: "dinah&ref=other",
		},
	];
	for (const fixture of fixtures) {
		const parts = servedTextUriParts(
			fixture.kind,
			fixture.root,
			fixture.ref,
			"a title nobody parses",
		);
		const parsed = parseServedTextUri(parts.authority, parts.query);
		assert.deepEqual(parsed, fixture, `round trip failed for ${fixture.root}`);
	}
});

test("the tab's title rides the path, which no reader of the URI parses", () => {
	// dinah-270 AC-2's other half: identity and display are separate, so a
	// title in any language leaves the identity untouched.
	const german = servedTextUriParts(
		KIND_INSTRUCTIONS,
		"/bench",
		"dinah-270",
		"dinah-270 (bereitgestellte Anweisungen)",
	);
	assert.equal(german.path, "/dinah-270 (bereitgestellte Anweisungen).md");
	assert.deepEqual(parseServedTextUri(german.authority, german.query), {
		kind: KIND_INSTRUCTIONS,
		root: "/bench",
		ref: "dinah-270",
	});
});

test("a URI missing any part of its identity is refused rather than guessed at", () => {
	// dinah-270 AC-3. Five ways a URI can arrive unusable, from the back
	// stack, from a restored window, or from a reader who typed one.
	assert.equal(parseServedTextUri("", "root=%2Fbench&ref=dinah-270"), undefined);
	assert.equal(parseServedTextUri(KIND_INSTRUCTIONS, "ref=dinah-270"), undefined);
	assert.equal(parseServedTextUri(KIND_INSTRUCTIONS, "root=%2Fbench"), undefined);
	assert.equal(
		parseServedTextUri(KIND_INSTRUCTIONS, "root=&ref=dinah-270"),
		undefined,
	);
	assert.equal(parseServedTextUri(KIND_INSTRUCTIONS, "root=%2Fbench&ref="), undefined);
});

test("servedText.ts imports no vscode symbol", () => {
	// The same boundary l10n.test.ts holds l10n.ts to. This module is reached
	// by the unit layer, which runs under plain node with no extension host,
	// so an import of vscode here would throw on the first import above.
	const source = readFileSync(
		join(__dirname, "..", "..", "..", "src", "servedText.ts"),
		"utf8",
	);
	const imports = source
		.split("\n")
		.filter((line) => /^import\b/.test(line) && /["']vscode["']/.test(line));
	assert.deepEqual(imports, []);
});

// ---------------------------------------------------------------------------
// The Markdown rendering
// ---------------------------------------------------------------------------

const LABELS = { global: "Global", standing: "Standing", column: "Column" };

test("every present layer gets one heading, in the order the chain is served in", () => {
	// dinah-270 AC-4. The layer text is compared as an exact substring rather
	// than against a normalized rendering: the extension reads the machine
	// surface and never re-renders what the binary rendered, and a comparison
	// that trimmed or collapsed whitespace would pass while this function
	// quietly reformatted a layer.
	const chain: InstructionChain = {
		global: "  keep   my    spacing\n\n\tand my tab\n",
		standing: "standing text",
		column: "column text",
	};
	const rendered = renderInstructionsMarkdown(chain, LABELS);
	assert.equal(
		rendered,
		"## Global\n\n  keep   my    spacing\n\n\tand my tab\n\n\n## Standing\n\nstanding text\n\n## Column\n\ncolumn text",
	);
	assert.ok(rendered.includes(chain.global as string));
	assert.ok(rendered.indexOf("## Global") < rendered.indexOf("## Standing"));
	assert.ok(rendered.indexOf("## Standing") < rendered.indexOf("## Column"));
});

test("a layer that is absent or empty contributes no heading at all", () => {
	// dinah-270 AC-4. Three fixtures: only the column, only the global, and a
	// chain carrying nothing.
	const columnOnly = renderInstructionsMarkdown({ column: "just this" }, LABELS);
	assert.equal(columnOnly, "## Column\n\njust this");
	assert.ok(!columnOnly.includes("## Global"));
	assert.ok(!columnOnly.includes("## Standing"));

	const globalOnly = renderInstructionsMarkdown(
		{ global: "just this", standing: "", column: "" },
		LABELS,
	);
	assert.equal(globalOnly, "## Global\n\njust this");

	assert.equal(renderInstructionsMarkdown({}, LABELS), "");
});

// ---------------------------------------------------------------------------
// The refresh loop
// ---------------------------------------------------------------------------

interface FakeClock extends Clock {
	/** Fires every live interval once. */
	readonly tickIntervals: () => Promise<void>;
	/** How many intervals are running right now. */
	readonly intervalCount: () => number;
}

function fakeClock(): FakeClock {
	let next = 1;
	const intervals = new Map<number, () => void>();
	return {
		now: () => 0,
		setTimeout: (fn) => {
			fn();
			return 0;
		},
		clearTimeout: () => undefined,
		setInterval: (fn) => {
			const handle = next++;
			intervals.set(handle, fn);
			return handle;
		},
		clearInterval: (handle) => {
			intervals.delete(handle as number);
		},
		tickIntervals: async () => {
			for (const fn of [...intervals.values()]) {
				fn();
			}
			// The tick body is async and the interval callback cannot await it,
			// so the microtask queue is drained before the test reads what the
			// tick did.
			await new Promise((resolve) => setImmediate(resolve));
		},
		intervalCount: () => intervals.size,
	};
}

/** A loop over a scripted resolver, with everything it announced recorded. */
function harness(answers: string[] | (() => Promise<string>)) {
	const clock = fakeClock();
	const changed: { uriKey: string; text: string }[] = [];
	const logged: string[] = [];
	const calls: { kind: string; root: string; ref: string }[] = [];
	let index = 0;
	const loop = new ServedTextRefreshLoop({
		clock,
		pollIntervalSeconds: 2,
		resolve: async (kind, root, ref) => {
			calls.push({ kind, root, ref });
			if (typeof answers === "function") {
				return answers();
			}
			const answer = answers[Math.min(index, answers.length - 1)];
			index += 1;
			return answer;
		},
		onChanged: (uriKey, text) => changed.push({ uriKey, text }),
		log: (line) => logged.push(line),
	});
	return { clock, loop, changed, logged, calls };
}

const URI_KEY = "dinah-served://instructions/dinah-270.md?root=%2Fbench&ref=dinah-270";

test("the loop starts no timer until a tab is open", () => {
	// dinah-270 AC-5. No open tab means no reason to spawn anything.
	const { clock, loop } = harness(["text"]);
	assert.equal(clock.intervalCount(), 0);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "text");
	assert.equal(clock.intervalCount(), 1);
	loop.stop();
});

test("the loop stops its timer when the last open tab closes", async () => {
	// dinah-270 AC-5 and AC-13. Closing the tab has to stop the spawning
	// rather than merely stop the announcing, so the resolver's own call count
	// is what this reads.
	const { clock, loop, calls } = harness(["text"]);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "text");
	const second = `${URI_KEY}-other`;
	loop.noteOpened(second, KIND_INSTRUCTIONS, "/bench", "dinah-9", "text");
	loop.noteClosed(URI_KEY);
	assert.equal(clock.intervalCount(), 1, "one tab is still open");
	loop.noteClosed(second);
	assert.equal(clock.intervalCount(), 0);
	assert.equal(loop.openCount, 0);
	const before = calls.length;
	await clock.tickIntervals();
	assert.equal(calls.length, before, "a closed tab is still being fetched");
});

test("a tick whose text has moved announces it exactly once", async () => {
	// dinah-270 AC-5.
	const { clock, loop, changed } = harness(["moved on"]);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "as opened");
	await clock.tickIntervals();
	assert.deepEqual(changed, [{ uriKey: URI_KEY, text: "moved on" }]);
	// The second tick reads the same text the first one cached, so the tab is
	// not told twice about one change.
	await clock.tickIntervals();
	assert.equal(changed.length, 1);
	loop.stop();
});

test("a tick whose text is unchanged announces nothing", async () => {
	// dinah-270 AC-5. This is the half that keeps a poll from repainting a tab
	// every interval, which would lose the reader's scroll position.
	const { clock, loop, changed } = harness(["as opened"]);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "as opened");
	await clock.tickIntervals();
	await clock.tickIntervals();
	assert.deepEqual(changed, []);
	loop.stop();
});

test("a failed tick is logged, swallowed, and leaves the cached text alone", async () => {
	// dinah-270 AC-6. A tab goes on showing its last good text through a
	// refusal, and the cache is not corrupted by one: the following tick reads
	// the original text back and still counts it as unchanged.
	let fail = true;
	const { clock, loop, changed, logged } = harness(async () => {
		if (fail) {
			throw new Error("dinah.unreadable");
		}
		return "as opened";
	});
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "as opened");
	await clock.tickIntervals();
	assert.equal(changed.length, 0);
	assert.equal(logged.length, 1);
	assert.ok(logged[0].includes(URI_KEY), `the log names no tab: ${logged[0]}`);
	assert.ok(logged[0].includes("dinah.unreadable"));

	fail = false;
	await clock.tickIntervals();
	assert.deepEqual(changed, [], "the failed tick corrupted the cached text");
	loop.stop();
});

test("recordFetched keeps the provider's own fetch from reading as a change", async () => {
	// The content provider fetches on open, and the document then holds text
	// the loop never saw. Without this the first tick would announce a change
	// nobody made.
	const { clock, loop, changed } = harness(["fetched by the provider"]);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "");
	loop.recordFetched(URI_KEY, "fetched by the provider");
	await clock.tickIntervals();
	assert.deepEqual(changed, []);
	loop.stop();
});

test("stop clears every open tab, so deactivation leaves nothing running", async () => {
	const { clock, loop, calls } = harness(["text"]);
	loop.noteOpened(URI_KEY, KIND_INSTRUCTIONS, "/bench", "dinah-270", "text");
	loop.stop();
	assert.equal(clock.intervalCount(), 0);
	assert.equal(loop.openCount, 0);
	await clock.tickIntervals();
	assert.deepEqual(calls, []);
});
