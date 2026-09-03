// The manifest is a contract too, and nothing else reads it back.
//
// A manifest cannot import a module, so the extension identifier is spelled in
// package.json and in identity.ts and these tests are what keep the two from
// drifting. The identifier is permanent from the first publish, and the
// integration tests reach the extension through it, so a drift would be found
// by a test failing for a reason that looks unrelated.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import {
	COMMAND_CHECK_WORKBENCH,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_OPEN_ATTACHMENT,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_CARD_BLOCKED,
	CONTEXT_CARD_READY_CLAIM,
	CONTEXT_CARD_READY_NONE,
	CONTEXT_COLUMN,
	CONTEXT_STATE_GROUP,
	CONTEXT_WORKBENCH_CANDIDATE,
	CONTEXT_WORKBENCH_FOREST,
	CONTEXT_WORKBENCH_ROOT,
	EXTENSION_ID,
	EXTENSION_NAME,
	GLOBAL_COMMANDS,
	PUBLISHER,
	ROW_COMMANDS,
	SETTING_PATH,
	SETTING_POLL_INTERVAL,
	SETTING_WATCH_FILES,
	SETTING_WORKBENCH,
	TREE_COMMANDS,
	VIEW_CONTAINER_ID,
	VIEW_ID,
} from "../../src/identity";
import { BINARY_KEY_VALUES, WORKBENCH_KEY_VALUES } from "../../src/status";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const manifest = JSON.parse(
	readFileSync(join(extensionRoot, "package.json"), "utf8"),
) as Record<string, unknown>;

const contributes = manifest.contributes as Record<string, unknown>;

interface WelcomeBlock {
	readonly view: string;
	readonly when?: string;
	readonly contents: string;
}

/** The context key values a window can be in, with undefined meaning unset. */
interface KeyState {
	readonly binary?: string;
	readonly workbench?: string;
}

function welcomeBlocks(): WelcomeBlock[] {
	return (contributes.viewsWelcome as WelcomeBlock[]).filter(
		(block) => block.view === VIEW_ID,
	);
}

// The `when` grammar these blocks are held to. Restricting it is what makes
// the exclusivity proof below a proof rather than a sampling: every clause is
// an equality or an inequality against a literal, joined by `and`, so a state
// decides every clause with no evaluation order and no truthiness of an unset
// key involved.
const TERM = /^(\S+) (==|!=) '([^']*)'$/;

function evaluate(when: string | undefined, state: KeyState): boolean {
	if (when === undefined) {
		return true;
	}
	return when.split("&&").every((raw) => {
		const term = TERM.exec(raw.trim());
		assert.ok(term, `welcome clause outside the supported grammar: ${raw.trim()}`);
		const [, key, operator, literal] = term;
		const held =
			key === "dinah.binary"
				? state.binary
				: key === "dinah.workbench"
					? state.workbench
					: assert.fail(`welcome clause reads an unknown context key: ${key}`);
		return operator === "==" ? held === literal : held !== literal;
	});
}

/**
 * The one state no welcome block covers, and the reason it does not.
 *
 * DinahTreeProvider.getChildren always returns at least one row once a
 * workbench resolves, so VS Code renders the tree's own content for this state
 * and never consults a welcome view at all. A block written for it would be
 * text no reader can reach. Every other state still has to match exactly one
 * block, so this is a single named exclusion rather than a relaxation of the
 * loop below.
 */
const TREE_RENDERS_INSTEAD: KeyState = { binary: "ok", workbench: "ok" };

/** Whether a state is the one the tree renders for. */
function rendersTree(state: KeyState): boolean {
	return (
		state.binary === TREE_RENDERS_INSTEAD.binary &&
		state.workbench === TREE_RENDERS_INSTEAD.workbench
	);
}

/**
 * Every state a window can be in, unset keys first.
 *
 * The half-set states are the point of this function rather than an edge of
 * it. activate() writes the two keys in two separately awaited commands, so a
 * window holds one key and not the other across an extension-host round trip
 * on every healthy startup, and that interval is where a state with no block
 * of its own hides. The product alone cannot express it, which is how the
 * first repair passed a proof it did not satisfy.
 *
 * The one state named in TREE_RENDERS_INSTEAD is excluded, and nothing else
 * is. A second exclusion added here without its own reason written beside it
 * would hide exactly the defect this function exists to find.
 */
function everyState(): KeyState[] {
	const states: KeyState[] = [{}];
	for (const binary of BINARY_KEY_VALUES) {
		states.push({ binary });
	}
	for (const workbench of WORKBENCH_KEY_VALUES) {
		states.push({ workbench });
	}
	for (const binary of BINARY_KEY_VALUES) {
		for (const workbench of WORKBENCH_KEY_VALUES) {
			states.push({ binary, workbench });
		}
	}
	return states.filter((state) => !rendersTree(state));
}

function describeState(state: KeyState): string {
	return `binary=${state.binary ?? "(unset)"} workbench=${state.workbench ?? "(unset)"}`;
}

test("the manifest and identity.ts spell the same extension identifier", () => {
	assert.equal(manifest.publisher, PUBLISHER);
	assert.equal(manifest.name, EXTENSION_NAME);
	assert.equal(`${String(manifest.publisher)}.${String(manifest.name)}`, EXTENSION_ID);
});

test("activation is the two declared events and nothing else", () => {
	// onStartupFinished and * are both forbidden: a window with no Dinah
	// content must cost nothing, and this is the only mechanical guard against
	// either being added later for convenience.
	assert.deepEqual(manifest.activationEvents, [
		"workspaceContains:**/workbench.md",
		`onView:${VIEW_ID}`,
	]);
});

test("the view container and its single view carry the declared ids", () => {
	const containers = contributes.viewsContainers as {
		activitybar: { id: string }[];
	};
	assert.equal(containers.activitybar.length, 1);
	assert.equal(containers.activitybar[0].id, VIEW_CONTAINER_ID);

	const views = contributes.views as Record<string, { id: string }[]>;
	assert.deepEqual(Object.keys(views), [VIEW_CONTAINER_ID]);
	assert.equal(views[VIEW_CONTAINER_ID].length, 1);
	assert.equal(views[VIEW_CONTAINER_ID][0].id, VIEW_ID);
});

test("exactly one welcome block matches in every state a window can be in", () => {
	// The defect this pins is a state with no block of its own. A window that
	// resolved a binary and a workbench matched none of the case-specific
	// blocks, fell through to an unconditional block, and told a reader whose
	// extension was working perfectly that it was still looking. Nothing
	// noticed, because three of the four states were specified and the fourth
	// was the one that shipped.
	//
	// Exactly one, rather than at least one, is the load-bearing half. The
	// contribution-points documentation does not say whether VS Code renders
	// the first matching block or concatenates every match, so a repair that
	// overlapped two blocks would be correct only by undocumented behaviour.
	// Blocks that cannot overlap never ask the question.
	const blocks = welcomeBlocks();
	for (const state of everyState()) {
		const matched = blocks.filter((block) => evaluate(block.when, state));
		assert.equal(
			matched.length,
			1,
			`${matched.length} welcome blocks match ${describeState(state)}, wanted 1`,
		);
	}
});

test("no welcome block is reachable in the state the tree renders for", () => {
	// The companion to the exclusion above, and the half that does the work.
	// everyState() skips this state, so the exactly-one loop cannot see a
	// block written for it, and a block put back here would be text no reader
	// reaches sitting in the manifest with nothing to notice it. Asserting
	// zero matches is what notices.
	//
	// This also pins the delta the removal was specified as. Every other state
	// must still match exactly one block, so a block added for another state,
	// or one removed from another state, is caught by the loop above; a block
	// added back for this one is caught here. Between them the array cannot
	// change by anything but the one entry this card took out, and neither
	// assertion goes stale when an unrelated card edits the manifest.
	const blocks = welcomeBlocks();
	// A view whose blocks all moved or were renamed matches nothing here for a
	// reason that has nothing to do with the removal this pins, so the roster
	// is checked before the match count is read.
	assert.ok(
		blocks.length > 0,
		"the view contributes no welcome blocks at all, so this check proved nothing",
	);
	const matched = blocks.filter((block) =>
		evaluate(block.when, TREE_RENDERS_INSTEAD),
	);
	assert.deepEqual(
		matched.map((block) => block.when),
		[],
		"a welcome block matches the state DinahTreeProvider always draws rows in",
	);
});

test("each resolved state renders its own text, and none renders the still-looking text", () => {
	// The four states the spec names, plus the interval before activate() has
	// set either key. Each of the five reaches different contents, so no
	// resolved window can be shown the text written for a window that has not
	// answered yet. The half-set state where a binary resolved and the
	// workbench key is not written yet is named here too: it is still looking,
	// but it is looking for less than a window that has answered nothing, and
	// telling a reader otherwise is the same lie one degree smaller.
	const blocks = welcomeBlocks();
	const contentsFor = (state: KeyState): string => {
		const matched = blocks.filter((block) => evaluate(block.when, state));
		assert.equal(matched.length, 1, `no single block for ${describeState(state)}`);
		return matched[0].contents;
	};

	const stillLooking = contentsFor({});
	// The workbench-resolved state is absent from this list because it now
	// reaches no welcome view at all: the tree draws its own rows there. The
	// test below asserts that absence directly rather than leaving it to be
	// inferred from a list somebody shortened.
	const named: [string, KeyState][] = [
		["no usable binary", { binary: "missing", workbench: "unknown" }],
		["no workbench found", { binary: "ok", workbench: "none" }],
		["several workbenches", { binary: "ok", workbench: "ambiguous" }],
		["binary resolved, workbench not written yet", { binary: "ok" }],
	];

	const seen = new Map<string, string>();
	for (const [label, state] of named) {
		const contents = contentsFor(state);
		assert.notEqual(
			contents,
			stillLooking,
			`the ${label} state still renders the pre-activation text`,
		);
		const earlier = seen.get(contents);
		assert.equal(
			earlier,
			undefined,
			`the ${label} state renders the same text as the ${String(earlier)} state`,
		);
		seen.set(contents, label);
	}
});

test("every welcome clause reads only the two keys the extension sets", () => {
	// evaluate() fails on a clause outside the grammar or on an unknown key,
	// so running one state through every block is what checks them.
	const blocks = welcomeBlocks();
	for (const block of blocks) {
		evaluate(block.when, { binary: "ok", workbench: "ok" });
	}
	assert.ok(blocks.length > 0);
});

test("the settings are contributed with the scopes their subjects need", () => {
	const configuration = contributes.configuration as {
		properties: Record<
			string,
			{ scope?: string; default?: unknown; type?: string; minimum?: number }
		>;
	};
	assert.deepEqual(Object.keys(configuration.properties), [
		SETTING_PATH,
		SETTING_WORKBENCH,
		SETTING_POLL_INTERVAL,
		SETTING_WATCH_FILES,
	]);
	// A binary path is a property of the machine and must not travel through
	// settings sync to a different one.
	assert.equal(configuration.properties[SETTING_PATH].scope, "machine-overridable");
	// Which workbench a folder uses is a property of the folder.
	assert.equal(configuration.properties[SETTING_WORKBENCH].scope, "resource");
	// So is how often that folder is checked, and whether it is watched.
	const poll = configuration.properties[SETTING_POLL_INTERVAL];
	assert.equal(poll.scope, "resource");
	assert.equal(poll.type, "integer");
	assert.equal(poll.default, 10);
	// The floor keeps a mistyped 0 or 1 from turning the poll into a spin.
	assert.equal(poll.minimum, 2);
	const watch = configuration.properties[SETTING_WATCH_FILES];
	assert.equal(watch.scope, "resource");
	assert.equal(watch.default, true);
});

test("the manifest declares exactly the commands identity.ts names", () => {
	const commands = contributes.commands as { command: string; title: string }[];
	assert.deepEqual(
		commands.map((entry) => entry.command),
		[...TREE_COMMANDS],
	);
});

test("the openAttachment entry is declared last, under the title a click names", () => {
	// The drift test above holds the two rosters together by position, which
	// already pins this command's place. This one pins the entry's other
	// half: a command whose title read like a menu item would show up in the
	// Command Palette as an action with no target, which is the one surface
	// the tree itself never renders.
	const commands = contributes.commands as { command: string; title: string }[];
	assert.equal(commands[commands.length - 1].command, COMMAND_OPEN_ATTACHMENT);
	assert.equal(commands[commands.length - 1].title, "Dinah: Open Attachment");
});

/** The commandPalette entries, which are absent from a manifest declaring none. */
function paletteEntries(): { command: string; when?: string }[] {
	const menus = contributes.menus as Record<
		string,
		{ command: string; when?: string }[]
	>;
	return menus.commandPalette ?? [];
}

test("every tree command is classified as either a row command or a global one", () => {
	// This is the check that fails on a command nobody classified, which is
	// the whole point of the two arrays. Six commands shipped reading their
	// element argument before anyone noticed that the Command Palette passes
	// none, and dinah-330 and dinah-335 each add more; a command added to
	// TREE_COMMANDS and to neither array goes red here rather than reaching a
	// reader as a palette entry that throws.
	const row = new Set(ROW_COMMANDS);
	const global = new Set(GLOBAL_COMMANDS);
	const classified = [...ROW_COMMANDS, ...GLOBAL_COMMANDS];
	assert.deepEqual(
		[...new Set(classified)].sort(),
		[...TREE_COMMANDS].sort(),
		"ROW_COMMANDS and GLOBAL_COMMANDS together are not the declared commands",
	);
	const both = [...row].filter((command) => global.has(command));
	assert.deepEqual(both, [], "a command is classified as both row-scoped and global");
	assert.equal(
		classified.length,
		TREE_COMMANDS.length,
		"a command is named twice across the two classification arrays",
	);
});

test("every row command is hidden from the Command Palette", () => {
	// VS Code hands a palette invocation no argument, so a command that needs
	// a row cannot act on one there. Hiding it is the documented mechanism and
	// it is what the repository already does with the context menus: an
	// illegal action is absent rather than offered and then refused.
	const entries = paletteEntries();
	for (const command of ROW_COMMANDS) {
		const matched = entries.filter((entry) => entry.command === command);
		assert.equal(
			matched.length,
			1,
			`${command} has ${matched.length} commandPalette entries, wanted 1`,
		);
		assert.equal(
			matched[0].when,
			"false",
			`${command} is not hidden from the palette: when is ${String(matched[0].when)}`,
		);
	}
});

test("no global command is named in the commandPalette block at all", () => {
	// Undeclared is VS Code's own default, and it means visible. An explicit
	// entry for Refresh would add manifest text with no effect, and a hiding
	// one would take away the only tree command a palette can actually run.
	const named = paletteEntries().map((entry) => entry.command);
	for (const command of GLOBAL_COMMANDS) {
		assert.ok(
			!named.includes(command),
			`${command} needs no palette entry and has one`,
		);
	}
});

test("every commandPalette entry names a command identity.ts knows", () => {
	// A stray or misspelled entry hides nothing and reports nothing, the same
	// way an undeclared menu command shows an item that does nothing.
	const entries = paletteEntries();
	assert.ok(
		entries.length > 0,
		"the manifest contributes no commandPalette entries, so this check proved nothing",
	);
	for (const entry of entries) {
		assert.ok(
			TREE_COMMANDS.includes(entry.command),
			`commandPalette names ${entry.command}, which is not a tree command`,
		);
	}
});

test("every menu entry names a command the manifest declares", () => {
	// A menu whose command is undeclared shows an item that does nothing when
	// it is clicked, which VS Code reports nowhere.
	const declared = new Set(
		(contributes.commands as { command: string }[]).map((entry) => entry.command),
	);
	const menus = contributes.menus as Record<string, { command: string }[]>;
	let checked = 0;
	for (const [where, entries] of Object.entries(menus)) {
		for (const entry of entries) {
			checked += 1;
			assert.ok(
				declared.has(entry.command),
				`${where} names the undeclared command ${entry.command}`,
			);
		}
	}
	// A manifest contributing no menu entries satisfies the loop above without
	// the loop having read anything, so the count is what separates a clean
	// set of menus from an absent one.
	assert.ok(checked > 0, "no menu entry was scanned at all, so this check proved nothing");
});

test("nothing in the manifest offers a Pull", () => {
	// dinah's pull verb takes a destination column and picks its own card from
	// that column's upstream, so no card-scoped Pull can be aimed at the row
	// that was clicked, and no read-only call publishes the destination-side
	// fact a column-scoped one would need. The item is specified on dinah-265
	// and built by whichever card lands dinah-280. This is the guard that
	// keeps it from arriving early and mistargeting somebody's board.
	const serialized = JSON.stringify(contributes);
	// The two absence assertions below read one string, and a contributions
	// block that had lost its commands or its menus would satisfy both while
	// declaring nothing at all. A pull would arrive as a command, a menu entry
	// or both, so both arrays being populated is what makes their silence mean
	// something.
	const commands = contributes.commands as unknown[];
	const menuEntries = Object.values(
		contributes.menus as Record<string, unknown[]>,
	).flat();
	assert.ok(
		commands.length > 0 && menuEntries.length > 0,
		`the contributions declare ${commands.length} commands and ${menuEntries.length} menu entries, so a missing pull proves nothing`,
	);
	assert.ok(
		!serialized.includes("pullInto"),
		"the manifest declares a pull command dinah cannot aim safely",
	);
	assert.ok(
		!serialized.includes("dinah.column.pull"),
		"a menu is registered against a column contextValue nothing composes",
	);
});

test("the card menus are registered against the four contextValues actionsFor composes", () => {
	// The manifest's `when` clauses and actionsFor's return values are two
	// spellings of one set. A contextValue composed in code but named in no
	// clause is a menu that never opens, and the reverse is a clause that
	// never matches; neither shows up at run time as anything but silence.
	const menus = contributes.menus as Record<string, { when: string }[]>;
	const clauses = menus["view/item/context"].map((entry) => entry.when).join(" ");
	assert.ok(clauses.includes(CONTEXT_CARD_READY_CLAIM));
	assert.ok(clauses.includes(CONTEXT_CARD_ACTIVE));
	assert.ok(clauses.includes(CONTEXT_CARD_BLOCKED));
	// The one contextValue with no Claim item of its own is still reached by
	// the Move and Block clauses, which match the ready prefix.
	assert.ok(
		clauses.includes("^dinah\\.card\\.(ready|active)"),
		`the ready/active prefix clause is not in the menus: ${clauses}`,
	);
	assert.ok(!clauses.includes(CONTEXT_CARD_READY_NONE));
});

/**
 * The contextValues a workbench-row menu item is offered on, as one regex.
 *
 * The manifest spells this same pattern inside a `when` clause, and the test
 * below composes that clause from this constant rather than from a second
 * copy of the pattern. Two hand-written spellings would drift, and the drift
 * would show up as a menu that silently stopped opening.
 */
const WORKBENCH_ROW_PATTERN = /^dinah\.workbench(Root|Candidate|Forest)$/;

/** The one `when` clause both workbench-row items are registered under. */
const WORKBENCH_ROW_CLAUSE = `view == dinah.workbenchView && viewItem =~ /${WORKBENCH_ROW_PATTERN.source}/`;

test("the two workbench-row commands are declared with bare titles", () => {
	// AC-1. Bare rather than prefixed, matching Claim, Release and the rest:
	// both are row commands the palette never shows, so a "Dinah: " prefix
	// would be read by nobody and would make the context menu say the product
	// name twice. The prefix question for palette-visible commands is
	// dinah-348's, and this assertion is what keeps these two out of it.
	const commands = contributes.commands as { command: string; title: string }[];
	const titles = new Map(commands.map((entry) => [entry.command, entry.title]));
	assert.equal(titles.get(COMMAND_CHECK_WORKBENCH), "Check");
	assert.equal(titles.get(COMMAND_COPY_WORKBENCH_PATH), "Copy Path");
});

test("both workbench-row commands are classified as row-scoped and never as global", () => {
	// AC-1's other half. The partition test above catches a command in neither
	// array; this catches one put in the wrong one, which would leave a palette
	// entry that throws on an argument VS Code never passes.
	for (const command of [COMMAND_CHECK_WORKBENCH, COMMAND_COPY_WORKBENCH_PATH]) {
		assert.ok(ROW_COMMANDS.includes(command), `${command} is not a row command`);
		assert.ok(
			!GLOBAL_COMMANDS.includes(command),
			`${command} is classified as global, and it cannot act without a row`,
		);
	}
});

test("the workbench menu items match every resolved workbench row and nothing else", () => {
	// AC-2. The clause is asserted whole rather than by substring, because the
	// anchors are what keep it off a contextValue a later card composes with
	// the same prefix.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	const items = menus["view/item/context"];
	for (const command of [COMMAND_CHECK_WORKBENCH, COMMAND_COPY_WORKBENCH_PATH]) {
		const matched = items.filter((entry) => entry.command === command);
		assert.equal(matched.length, 1, `${command} has ${matched.length} menu entries`);
		assert.equal(matched[0].when, WORKBENCH_ROW_CLAUSE);
	}
	// The clause and the three contextValues rootItem composes are two
	// spellings of one set, exactly as the card clauses and actionsFor are. A
	// row kind renamed on one side and not the other is a menu that never
	// opens, and nothing at run time reports it.
	for (const value of [
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_WORKBENCH_CANDIDATE,
		CONTEXT_WORKBENCH_FOREST,
	]) {
		assert.match(value, WORKBENCH_ROW_PATTERN);
	}
	// A card contextValue must not reach these items, which is the half the
	// anchors do the work for.
	for (const value of [CONTEXT_CARD_ACTIVE, CONTEXT_COLUMN, CONTEXT_STATE_GROUP]) {
		assert.doesNotMatch(value, WORKBENCH_ROW_PATTERN);
	}
});

test("no workbench menu item is offered on a column row, and no item at all on a state group", () => {
	// AC-2's exclusion. The column half is now narrower than it was: dinah-332
	// gives the column row its own Edit Instructions item, so a blanket search
	// for CONTEXT_COLUMN across every clause would fail on the very item that
	// card added. What must still hold is that no WORKBENCH act is offered
	// there, because a column row names no workbench of its own.
	//
	// The state group keeps the blanket form, because it still has no item of
	// any kind and its one plausible act is the queue pull dinah-280 has not
	// published.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	const items = menus["view/item/context"];
	const workbenchClauses = items
		.filter((entry) =>
			[
				COMMAND_CHECK_WORKBENCH,
				COMMAND_COPY_WORKBENCH_PATH,
				COMMAND_EDIT_WORKBENCH_DEFINITION,
			].includes(entry.command),
		)
		.map((entry) => entry.when);
	// Three items are expected, and reading the count is what stops a filter
	// that matched nothing from satisfying the loop beneath it.
	assert.equal(
		workbenchClauses.length,
		3,
		`the three workbench-row items produced ${String(workbenchClauses.length)} clauses`,
	);
	for (const clause of workbenchClauses) {
		assert.ok(
			!clause.includes(CONTEXT_COLUMN),
			`a workbench act is registered against a column row: ${clause}`,
		);
	}
	const allClauses = items.map((entry) => entry.when).join(" ");
	assert.ok(
		!allClauses.includes(CONTEXT_STATE_GROUP),
		`a context menu is registered against a state group row: ${allClauses}`,
	);
});

/** The one `when` clause the column row's own item is registered under. */
const COLUMN_ROW_CLAUSE = `view == dinah.workbenchView && viewItem == ${CONTEXT_COLUMN}`;

/** Every contextValue neither new item may be offered on. */
const FORBIDDEN_FOR_EDIT_ITEMS = [
	CONTEXT_STATE_GROUP,
	CONTEXT_CARD_READY_CLAIM,
	CONTEXT_CARD_READY_NONE,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_CARD_BLOCKED,
];

test("the two edit commands are declared with bare titles", () => {
	// dinah-332 AC-3. Bare rather than prefixed, matching Check, Copy Path,
	// Claim and Release: both are row commands the palette never shows, so a
	// "Dinah: " prefix would be read by nobody and would make the context menu
	// say the product name twice.
	const commands = contributes.commands as { command: string; title: string }[];
	const titles = new Map(commands.map((entry) => [entry.command, entry.title]));
	assert.equal(titles.get(COMMAND_EDIT_WORKBENCH_DEFINITION), "Edit Definition");
	assert.equal(titles.get(COMMAND_EDIT_COLUMN_INSTRUCTIONS), "Edit Instructions");
});

test("both edit commands are classified as row-scoped and never as global", () => {
	// dinah-332 AC-1. The partition test above catches a command in neither
	// array; this catches one put in the wrong one, which would leave a palette
	// entry that throws on an argument VS Code never passes.
	for (const command of [
		COMMAND_EDIT_WORKBENCH_DEFINITION,
		COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	]) {
		assert.ok(ROW_COMMANDS.includes(command), `${command} is not a row command`);
		assert.ok(
			!GLOBAL_COMMANDS.includes(command),
			`${command} is classified as global, and it cannot act without a row`,
		);
	}
});

test("the definition item matches every resolved workbench row and nothing else", () => {
	// dinah-332 AC-2, the workbench half. The clause is asserted whole rather
	// than by substring, because the anchors are what keep the item off a
	// contextValue a later card composes with the same prefix, and the group
	// pins it after Check and Copy Path rather than among them.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string; group?: string }[]
	>;
	const matched = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_EDIT_WORKBENCH_DEFINITION,
	);
	assert.equal(
		matched.length,
		1,
		`${COMMAND_EDIT_WORKBENCH_DEFINITION} has ${String(matched.length)} menu entries, wanted 1`,
	);
	assert.equal(matched[0].when, WORKBENCH_ROW_CLAUSE);
	assert.equal(matched[0].group, "1_workbench@3");
	// The three row kinds the clause is meant to reach, held to the pattern
	// rather than the pattern being trusted to cover them.
	for (const value of [
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_WORKBENCH_CANDIDATE,
		CONTEXT_WORKBENCH_FOREST,
	]) {
		assert.match(value, WORKBENCH_ROW_PATTERN);
	}
	// A column row, a state group and every card state must not reach it.
	for (const value of [CONTEXT_COLUMN, ...FORBIDDEN_FOR_EDIT_ITEMS]) {
		assert.doesNotMatch(value, WORKBENCH_ROW_PATTERN);
	}
});

test("the instructions item matches a column row and nothing else", () => {
	// dinah-332 AC-2, the column half. The clause is composed from
	// CONTEXT_COLUMN rather than spelled a second time, so a rename that misses
	// the manifest goes red here instead of shipping a menu that never opens.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string; group?: string }[]
	>;
	const matched = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	);
	assert.equal(
		matched.length,
		1,
		`${COMMAND_EDIT_COLUMN_INSTRUCTIONS} has ${String(matched.length)} menu entries, wanted 1`,
	);
	assert.equal(matched[0].when, COLUMN_ROW_CLAUSE);
	assert.equal(matched[0].group, "1_column@1");
	// The clause is an equality against one literal, so what it excludes is
	// every other contextValue the tree composes. A state group and every card
	// state are named here because AC-2 names them.
	for (const value of FORBIDDEN_FOR_EDIT_ITEMS) {
		assert.notEqual(
			value,
			CONTEXT_COLUMN,
			`${value} is the same string as the column contextValue, so the clause reaches it`,
		);
	}
	// And the three workbench row kinds, which the equality also excludes.
	for (const value of [
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_WORKBENCH_CANDIDATE,
		CONTEXT_WORKBENCH_FOREST,
	]) {
		assert.notEqual(value, CONTEXT_COLUMN);
	}
});

/** The contextValues a card row's copy item is offered on, as one regex. */
const CARD_ROW_PATTERN = /^dinah\.card\./;

/** The one `when` clause the copy item is registered under. */
const CARD_ROW_CLAUSE = `view == dinah.workbenchView && viewItem =~ /${CARD_ROW_PATTERN.source}/`;

test("the card copy command is declared with a bare title", () => {
	// dinah-337 AC-2. Bare rather than prefixed, matching Claim, Release and
	// the workbench-row items: this is a row command the palette never shows,
	// so a "Dinah: " prefix would be read by nobody and would make the context
	// menu say the product name twice.
	const commands = contributes.commands as { command: string; title: string }[];
	const titles = new Map(commands.map((entry) => [entry.command, entry.title]));
	assert.equal(titles.get(COMMAND_COPY_CARD_REF), "Copy Reference");
});

test("the card copy item is offered on every card row, whatever state it stands in", () => {
	// dinah-337 AC-2. A reference is copyable from a ready, an active and a
	// blocked card alike, unlike Claim and Release, which are state-gated. The
	// clause is asserted whole rather than by substring, because the prefix
	// anchor is what keeps the item off a contextValue a later card composes.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	const matched = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_COPY_CARD_REF,
	);
	assert.equal(
		matched.length,
		1,
		`${COMMAND_COPY_CARD_REF} has ${String(matched.length)} menu entries, wanted 1`,
	);
	assert.equal(matched[0].when, CARD_ROW_CLAUSE);
	// The clause and the four contextValues actionsFor composes are two
	// spellings of one set, so each is held to the pattern rather than the
	// pattern being trusted to cover them.
	for (const value of [
		CONTEXT_CARD_READY_CLAIM,
		CONTEXT_CARD_READY_NONE,
		CONTEXT_CARD_ACTIVE,
		CONTEXT_CARD_BLOCKED,
	]) {
		assert.match(value, CARD_ROW_PATTERN);
	}
	// A row that is not a card must not reach the item, which is the half the
	// prefix anchor does the work for.
	for (const value of [
		CONTEXT_COLUMN,
		CONTEXT_STATE_GROUP,
		CONTEXT_WORKBENCH_ROOT,
	]) {
		assert.doesNotMatch(value, CARD_ROW_PATTERN);
	}
});

test("the extension version is major.minor.patch, which is all the marketplace accepts", () => {
	assert.match(String(manifest.version), /^\d+\.\d+\.\d+$/);
});

test("the unit layer runs through the guard that refuses an empty run", () => {
	// The script this replaced handed a quoted glob to `node --test`, which
	// the pinned CI version could not expand, so the whole layer matched no
	// files on a runner. The guard enumerates the compiled files itself and
	// reads the run's own count back. A revert to any form that describes the
	// files by pattern brings the silent-empty-run failure back with it, and
	// this is the only thing that would notice.
	const scripts = manifest.scripts as Record<string, string>;
	assert.ok(
		scripts["test:unit"].includes("scripts/run-unit-tests.mjs"),
		`test:unit no longer runs through the guard: ${scripts["test:unit"]}`,
	);
	assert.ok(
		!scripts["test:unit"].includes("*"),
		`test:unit describes its files with a pattern again: ${scripts["test:unit"]}`,
	);
});

test("the extension's version is its own and is read from no CLI file", () => {
	// This used to assert the opposite, that the version began with whatever
	// the repository's VERSION file said. That coupling published a dev build
	// as 0.1.42 and then published the stable release v0.1.0 as 0.1.0, which
	// the marketplace reads as older and never offers as an update. The two
	// numbers count different things on separate cadences now, so what is
	// guarded is that nothing computes one from the other.
	assert.ok(
		!/^0\.0\.\d+$/.test(String(manifest.version)),
		`package.json carries ${String(manifest.version)}, which is on the 0.0.x line reserved for unpublished archives`,
	);
	const packagingFiles = [
		join(extensionRoot, "esbuild.mjs"),
		join(extensionRoot, "scripts", "package.mjs"),
		join(extensionRoot, "scripts", "version.mjs"),
		join(extensionRoot, "scripts", "publish-extension.ps1"),
	];
	for (const file of packagingFiles) {
		const text = readFileSync(file, "utf8");
		assert.ok(
			!/["']VERSION["']/.test(text),
			`${file} reads the CLI's VERSION file, so the extension's version is a projection of the CLI's again`,
		);
	}
});
