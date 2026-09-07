// The served-text scheme: its URI grammar, its rendering, and its refresh
// timer, all driven without an editor.
//
// The loop is asserted on a fake clock for the reason changes.test.ts gives:
// a test that really waited out a ten-second poll would be slow and would
// still not prove which mechanism fired.
//
// dinah-270 AC-2, AC-3, AC-4, AC-5, AC-6 and AC-13, and dinah-422 AC-3,
// AC-6, AC-7, AC-8 and AC-9.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { pinnedArgv, refusalMessage } from "../../src/cardCommands";
import type { Clock } from "../../src/changes";
import { runDinah } from "../../src/cli";
import type { SpawnOptions, SpawnOutcome, Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import {
	HISTORY_ROWS,
	KIND_HISTORY,
	KIND_INSTRUCTIONS,
	ServedTextRefreshLoop,
	parseServedTextUri,
	renderHistoryMarkdown,
	renderInstructionsMarkdown,
	servedTextUriParts,
} from "../../src/servedText";
import type { InstructionChain, JournalEvent, ServedAnswer } from "../../src/wire";

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

// ---------------------------------------------------------------------------
// The history rendering (dinah-422)
// ---------------------------------------------------------------------------

/**
 * The twenty-one event names internal/contract/contract.go's Events slice
 * declares, in the order the constants above it are declared and the slice
 * lists them.
 *
 * Quoted here rather than derived, because the point of the assertion below is
 * that two independently maintained lists agree. A twenty-second name added to
 * the Go slice and not to this file fails when somebody next reconciles the
 * two; a row added to HISTORY_ROWS and not to the contract, or a row dropped
 * from it, fails immediately.
 */
const CONTRACT_EVENTS: readonly string[] = [
	"created",
	"claimed",
	"moved",
	"released",
	"blocked",
	"unblocked",
	"expired",
	"commented",
	"attached",
	"attachment_replaced",
	"attachment_removed",
	"attachment_renamed",
	"archived",
	"restored",
	"deleted",
	"manual_correction",
	"workstream_joined",
	"workstream_left",
	"card_updated",
	"tier_overridden",
	"tier_override_dropped",
];

/** Fills the skeleton every journal line carries, so a fixture names only its own fields. */
function event(fields: Partial<JournalEvent> & { event: string }): JournalEvent {
	return { ts: "2026-09-08T10:00:00Z", actor: "paul", ...fields };
}

test("the render table names exactly the events the contract declares", () => {
	// dinah-422 AC-6. Sorted on both sides, because the table is written for a
	// reader in the contract's own order and a Record's key order is not a
	// promise this assertion should rest on.
	assert.deepEqual(
		Object.keys(HISTORY_ROWS).sort(),
		[...CONTRACT_EVENTS].sort(),
		"HISTORY_ROWS and internal/contract's Events slice have drifted apart",
	);
	assert.equal(CONTRACT_EVENTS.length, 21);
});

test("an empty journal renders the catalogue's own sentence and nothing else", () => {
	// dinah-422 AC-6. The wording is asserted whole, because "is recorded" is
	// the half that keeps this from reading as a claim that nothing happened.
	//
	// Both wire readings are driven, and null is the one the binary actually
	// prints. `dinah --json log` answers a card whose journal is absent or
	// carries no lines with the JSON literal null, because bench.ReadJournal
	// returns a nil slice and encoding/json writes a nil slice as null.
	// Captured by running the built binary against a card whose journal file
	// was emptied and then removed: both runs printed `null` and exited 0.
	for (const events of [null, []] as (readonly JournalEvent[] | null)[]) {
		assert.equal(
			renderHistoryMarkdown(events, ENGLISH),
			"No history is recorded for this card.",
			`the empty sentence did not render for ${JSON.stringify(events)}`,
		);
	}
});

test("an event name that collides with Object.prototype falls through to unknown", () => {
	// dinah-422. HISTORY_ROWS is a plain object literal, so indexing it with
	// toString, valueOf or __proto__ resolves to the inherited member and
	// walks past the unknown-name fallback into a throw that takes the whole
	// tab to servedText.refused. No contract event has such a name; the guard
	// is here because a newer binary chooses its own names and the fallback
	// exists precisely for names this catalogue has never seen.
	for (const name of ["toString", "valueOf", "hasOwnProperty", "constructor", "__proto__"]) {
		assert.equal(
			renderHistoryMarkdown([event({ event: name })], ENGLISH),
			`paul recorded an event of kind ${name}.`,
			`${name} did not reach the unknown-name row`,
		);
	}
});

test("events render one line each, in the order they arrive", () => {
	// dinah-422 AC-6. The second fixture is older than the first, and the
	// rendering keeps it second, so a renderer that sorted by timestamp would
	// fail here rather than quietly reordering a journal.
	const rendered = renderHistoryMarkdown(
		[
			event({ event: "claimed", ts: "2026-09-08T12:00:00Z" }),
			event({ event: "released", ts: "2026-09-08T09:00:00Z", actor: "ana" }),
		],
		ENGLISH,
	);
	assert.equal(rendered, "paul claimed the card.\nana released the card.");
});

/**
 * One fixture per event name, with the sentence each one must render to.
 *
 * Each case is its own test, so a wording change to one template reddens only
 * its own line and a reviewer reading a failure sees which event moved.
 */
const HISTORY_CASES: readonly {
	name: string;
	fixture: JournalEvent;
	want: string;
}[] = [
	{
		name: "created",
		fixture: event({ event: "created", title: "Ship the tab", to_title: "Intake" }),
		want: 'paul created the card "Ship the tab" in Intake.',
	},
	{
		name: "claimed",
		fixture: event({ event: "claimed" }),
		want: "paul claimed the card.",
	},
	{
		name: "moved",
		fixture: event({
			event: "moved",
			from: "spec",
			to: "build",
			from_title: "Spec",
			to_title: "Implement",
		}),
		want: "paul moved the card from Spec to Implement.",
	},
	{
		name: "released",
		fixture: event({ event: "released" }),
		want: "paul released the card.",
	},
	{
		name: "blocked",
		fixture: event({
			event: "blocked",
			reason: "the spec names no destination",
			kind: "ambiguous_spec",
		}),
		want: "paul blocked the card: the spec names no destination",
	},
	{
		name: "unblocked",
		fixture: event({ event: "unblocked" }),
		want: "paul unblocked the card.",
	},
	{
		name: "expired",
		fixture: event({ event: "expired", expires: "2026-09-08T09:00:00Z" }),
		want: "paul's claim on the card expired.",
	},
	{
		name: "commented",
		fixture: event({ event: "commented", comment: "c-7" }),
		want: "paul added a comment.",
	},
	{
		name: "attached",
		fixture: event({ event: "attached", attachment: "a-1", filename: "trace.log" }),
		want: "paul attached trace.log.",
	},
	{
		name: "attachment_replaced",
		fixture: event({
			event: "attachment_replaced",
			attachment: "a-1",
			filename: "trace.log",
		}),
		want: "paul replaced the attachment with trace.log.",
	},
	{
		name: "attachment_removed",
		fixture: event({
			event: "attachment_removed",
			attachment: "a-1",
			filename: "trace.log",
		}),
		want: "paul removed the attachment trace.log.",
	},
	{
		name: "attachment_renamed",
		fixture: event({
			event: "attachment_renamed",
			attachment: "a-1",
			from: "trace.log",
			filename: "run.log",
		}),
		want: "paul renamed an attachment from trace.log to run.log.",
	},
	{
		name: "archived",
		fixture: event({ event: "archived", note: "card-9f2c" }),
		want: "paul archived something recorded here.",
	},
	{
		name: "restored",
		fixture: event({ event: "restored" }),
		want: "paul restored something recorded here.",
	},
	{
		name: "deleted",
		fixture: event({ event: "deleted" }),
		want: "paul deleted something recorded here.",
	},
	{
		name: "manual_correction",
		fixture: event({
			event: "manual_correction",
			from_title: "Spec",
			to_title: "Implement",
		}),
		want: "paul corrected the card's recorded position from Spec to Implement.",
	},
	{
		name: "workstream_joined",
		fixture: event({ event: "workstream_joined", workstream: "ws-editor" }),
		want: "paul added the card to workstream ws-editor.",
	},
	{
		name: "workstream_left",
		fixture: event({ event: "workstream_left", workstream: "ws-editor" }),
		want: "paul removed the card from workstream ws-editor.",
	},
	{
		name: "card_updated",
		fixture: event({ event: "card_updated", field: "priority", from: "soon", to: "now" }),
		want: "paul changed priority from soon to now.",
	},
	{
		name: "tier_overridden",
		fixture: event({
			event: "tier_overridden",
			column: "col-3",
			column_title: "Implement",
			from: "workhorse",
			to: "frontier",
			expr: "+1",
			against: "workhorse",
			reason: "the diff reaches the contract",
		}),
		want: "paul set the card's tier in Implement to frontier (+1).",
	},
	{
		name: "tier_override_dropped",
		fixture: event({
			event: "tier_override_dropped",
			column: "col-3",
			from: "frontier",
		}),
		want:
			"paul's tier override for the card in col-3 (frontier) was dropped because the column was retired.",
	},
	{
		name: "an unrecognised name",
		fixture: event({ event: "teleported" }),
		want: "paul recorded an event of kind teleported.",
	},
];

for (const testCase of HISTORY_CASES) {
	test(`the ${testCase.name} row reads as the catalogue's English`, () => {
		// dinah-422 AC-7. Byte for byte against the filled English template, so
		// a placeholder left unfilled or filled from the wrong wire field fails
		// here rather than reaching a reader.
		assert.equal(renderHistoryMarkdown([testCase.fixture], ENGLISH), testCase.want);
	});
}

test("every contract event has a case above, and the unknown fallback has one too", () => {
	// dinah-422 AC-7's own completeness. Without this, dropping a case from
	// HISTORY_CASES would silently stop testing an event.
	const covered = HISTORY_CASES.map((testCase) => testCase.fixture.event);
	for (const name of CONTRACT_EVENTS) {
		assert.ok(covered.includes(name), `no fixture renders the ${name} row`);
	}
	assert.equal(HISTORY_CASES.length, CONTRACT_EVENTS.length + 1);
});

test("an absent from or to on a field change names none rather than nothing", () => {
	// dinah-422 AC-8. A first write to a field that carried nothing, and a
	// write that cleared one. Both are things that happened, and neither may
	// reach a reader as an empty span or an unfilled token.
	const firstWrite = renderHistoryMarkdown(
		[event({ event: "card_updated", field: "severity", to: "major" })],
		ENGLISH,
	);
	assert.equal(firstWrite, "paul changed severity from none to major.");

	const cleared = renderHistoryMarkdown(
		[event({ event: "card_updated", field: "severity", from: "major" })],
		ENGLISH,
	);
	assert.equal(cleared, "paul changed severity from major to none.");

	for (const rendered of [firstWrite, cleared]) {
		assert.doesNotMatch(rendered, /\{from\}|\{to\}/, "a placeholder survived");
		assert.doesNotMatch(rendered, /from to |to \.$/, "a value rendered as nothing");
	}
});

test("a tier override with no captured column title falls back to the identifier", () => {
	// dinah-422 AC-9's sibling case. journal.go says only a raise captures a
	// column title, so the ordinary per-column write must still read as a
	// sentence rather than as a gap.
	assert.equal(
		renderHistoryMarkdown(
			[event({ event: "tier_overridden", column: "col-3", to: "frontier", expr: "frontier" })],
			ENGLISH,
		),
		"paul set the card's tier in col-3 to frontier (frontier).",
	);
});

// ---------------------------------------------------------------------------
// The history resolver, composed here out of the pieces extension.ts composes
// it from, since nothing in the unit layer can register a content provider.
// ---------------------------------------------------------------------------

/** A spawner that records what it was asked to run and replays one outcome. */
function historySpawner(outcome: SpawnOutcome): {
	spawner: Spawner;
	calls: { exe: string; argv: string[]; options: SpawnOptions }[];
} {
	const calls: { exe: string; argv: string[]; options: SpawnOptions }[] = [];
	const spawner: Spawner = async (exe, argv, options) => {
		calls.push({ exe, argv: [...argv], options });
		return outcome;
	};
	return { spawner, calls };
}

/** What extension.ts's KIND_HISTORY entry does, with its dependencies injected. */
async function resolveHistory(spawner: Spawner, root: string, ref: string): Promise<string> {
	const outcome = await runDinah(spawner, "dinah", pinnedArgv(root, ["log", ref]), {
		cwd: root,
	});
	if (outcome.kind !== "ok") {
		throw new Error(refusalMessage(outcome));
	}
	return renderHistoryMarkdown(outcome.json as JournalEvent[], ENGLISH);
}

test("the journal is asked for with the pinned argv and no hand-built flag", () => {
	// dinah-422 AC-3. --json is composed by cli.ts and refused if a caller
	// spells it, so the argv this resolver hands over carries the workbench
	// and the verb alone.
	// stdout is the literal null the binary prints for a card with no journal
	// lines, captured from a run of the built binary rather than imagined. An
	// earlier form of this fixture printed "[]", a shape dinah never emits,
	// and it agreed with a renderer that threw on the real answer.
	const { spawner, calls } = historySpawner({
		code: 0,
		stdout: "null",
		stderr: "",
	});
	return resolveHistory(spawner, "/bench", "dinah-422").then((text) => {
		assert.equal(text, "No history is recorded for this card.");
		assert.deepEqual(calls[0].argv, ["--json", "--workbench", "/bench", "log", "dinah-422"]);
		assert.equal(calls[0].options.cwd, "/bench");
	});
});

test("a refused log throws what the existing refusal path already renders", async () => {
	// dinah-422 AC-3's other half. The kind adds no failure string of its own:
	// the provider's catch turns this into servedText.refused, which is the
	// same entry the instructions kind has used since dinah-270.
	// Captured from `dinah --json log` against a reference no card carries:
	// the refusal name is bare rather than dotted, and detail is the
	// reference itself rather than a sentence about it. Exit code 2.
	const refusal = JSON.stringify({
		outcome: "refused",
		refusal: "unknown-card",
		detail: "dinah-999",
	});
	const { spawner } = historySpawner({ code: 2, stdout: refusal, stderr: "" });
	await assert.rejects(
		() => resolveHistory(spawner, "/bench", "dinah-999"),
		/^Error: unknown-card: dinah-999$/,
	);

	// The catalogue carries no history-specific refusal entry, which is what
	// "the existing path, unchanged" means in practice.
	const catalogue = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "src", "locales", "en.json"), "utf8"),
	) as { entries: Record<string, unknown> };
	const refusalKeys = Object.keys(catalogue.entries).filter(
		(key) => key.startsWith("history.") && key.includes("refus"),
	);
	assert.deepEqual(refusalKeys, [], "history added a refusal string of its own");
	assert.equal(
		ENGLISH("servedText.refused", { detail: "x" }),
		"dinah refused: x",
		"the shared refusal entry moved",
	);
});

test("a bare identifier renders as itself, with no second call to resolve it", async () => {
	// dinah-422 AC-9. The three events carrying an identifier and no captured
	// title all render in one pass, and the spawner sees the log call and
	// nothing else. A lookup added later to prettify one of them fails here.
	const journal: JournalEvent[] = [
		event({ event: "archived", note: "card-9f2c" }),
		event({ event: "workstream_joined", workstream: "ws-editor" }),
		event({ event: "workstream_left", workstream: "ws-editor" }),
		event({ event: "tier_override_dropped", column: "col-3", from: "frontier" }),
	];
	const { spawner, calls } = historySpawner({
		code: 0,
		stdout: JSON.stringify(journal),
		stderr: "",
	});
	const text = await resolveHistory(spawner, "/bench", "dinah-422");
	assert.equal(calls.length, 1, "rendering a history spawned dinah more than once");
	assert.deepEqual(calls[0].argv.slice(-2), ["log", "dinah-422"]);
	assert.ok(text.includes("ws-editor"), "the workstream identifier was not printed");
	assert.ok(text.includes("col-3"), "the column identifier was not printed");
	assert.equal(text.split("\n").length, 4);
});

test("extension.ts's own history entry asks for log through the pinned argv", () => {
	// dinah-422 AC-3. The resolver above is this test file's reconstruction of
	// the table entry, and a reconstruction guards its own copy rather than the
	// shipped one, so the shipped entry is read here as source. The same reason
	// spawn-sites.test.ts reads source: nothing in the unit layer can register
	// a content provider, and the claim is about a call site rather than about
	// a value a function returns.
	const source = readFileSync(
		join(__dirname, "..", "..", "..", "src", "extension.ts"),
		"utf8",
	);
	const at = source.indexOf("[KIND_HISTORY]:");
	assert.notEqual(at, -1, "extension.ts declares no KIND_HISTORY resolver");
	const entry = source.slice(at, source.indexOf("[KIND_GUIDE]:", at));
	assert.match(
		entry,
		/pinnedArgv\(root, \["log", ref\]\)/,
		"the history resolver no longer composes the log verb through pinnedArgv",
	);
	assert.doesNotMatch(entry, /--json/, "the history resolver spells --json by hand");
	assert.match(entry, /renderHistoryMarkdown\(/);
	assert.match(entry, /throw new Error\(refusalMessage\(outcome\)\)/);
	// One spawn per rendered history, asserted on the shipped entry rather than
	// on the reconstruction, which is the other half of AC-9: a lookup added
	// later to resolve one of the bare identifiers would show up here.
	assert.equal(
		(entry.match(/runDinah\(/g) ?? []).length,
		1,
		"the history resolver spawns dinah more than once",
	);
	// The renderer itself cannot spawn at all, because its module reaches
	// neither the CLI wrapper nor the spawner.
	const renderer = readFileSync(
		join(__dirname, "..", "..", "..", "src", "servedText.ts"),
		"utf8",
	);
	assert.doesNotMatch(renderer, /from "\.\/(cli|spawn)"/);
});

test("the history kind is spelled once and travels through the URI grammar", () => {
	// dinah-422 AC-3. The kind is a table key and a URI authority, and the
	// round trip is what proves the second kind needed no change to the
	// grammar the first one uses.
	assert.equal(KIND_HISTORY, "history");
	const parts = servedTextUriParts(KIND_HISTORY, "/bench", "dinah-422", "dinah-422 (history)");
	assert.deepEqual(parseServedTextUri(parts.authority, parts.query), {
		kind: KIND_HISTORY,
		root: "/bench",
		ref: "dinah-422",
	});
});
