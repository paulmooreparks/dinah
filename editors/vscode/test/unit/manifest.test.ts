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
	COMMAND_ATTACH_FILE,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_REFRESH,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_CARD_BLOCKED,
	CONTEXT_CARD_READY_CLAIM,
	CONTEXT_CARD_READY_NONE,
	CONTEXT_COLUMN,
	CONTEXT_COLUMN_FULL,
	CONTEXT_COLUMN_OPEN,
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
import { assertCommandsFullyRegistered } from "../../src/registrationGuard";
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

// The vocabulary an affirmative claim reaches for. This set is deliberately
// wider than the wording that was in the tree: a guard armed only against the
// sentence already fixed misses the next one, written a different way, in
// exactly the way the phrase-by-phrase sweep that found the first one did.
//
// `has` and `contains` are here in their third-person singular forms only.
// "The extension has a dinah binary" is the claim this guard exists to refuse,
// while "set `dinah.path` to a binary you already have" is honest copy about
// the reader's own machine, so `have` and `contain` stay out.
const POSSESSION_WORD = String.raw`(?:carr(?:y|ies|ied|ying)|bundl(?:e|es|ed|ing)|includ(?:e|es|ed|ing)|ship(?:s|ped|ping)?|suppl(?:y|ies|ied|ying)|packag(?:e|es|ed|ing)|embed(?:s|ded|ding)?|provid(?:e|es|ed|ing)|has|contains|built[- ]in|comes? with|came with|own copy)`;
const POSSESSION = new RegExp(String.raw`\b${POSSESSION_WORD}\b`, "gi");

// Who is said to do the possessing, and what is said to be possessed. Frame
// one below requires both to be named, because a possession word beside the
// extension alone says nothing about dinah: "this extension provides a tree
// view of your workbench" is true, honest, and none of this guard's business.
const SUBJECT = /\b(?:extensions?|vsix)\b/i;
const PAYLOAD = /\b(?:dinah|binar(?:y|ies)|cop(?:y|ies)|cli|executable)\b/i;

// A possession word attached straight to the payload, which names no
// extension at all: "the built-in dinah binary".
const QUALIFIED_PAYLOAD =
	/\b(built[- ]in|bundled|embedded|packaged|included|carried|shipped|supplied|provided)\s+(copy|dinah|binary|cli|executable)\b/gi;

// The payload written as "the one", whose antecedent is a dinah binary earlier
// in the same copy: "the one carried inside this extension". The bare word is
// deliberately absent from PAYLOAD, since a sentence that counts things ("one
// view per workbench") would then satisfy frame one; it counts here only where
// a possession word governs it directly.
const ANAPHORIC_PAYLOAD = new RegExp(
	String.raw`\bthe one\b\s+(?:that\s+|which\s+)?(?:is\s+|was\s+)?${POSSESSION_WORD}\b`,
	"gi",
);

// Negation is what separates the honest copy from the claim, and it sits on
// either side of the word: "does not carry a copy", "carries no copy".
const NEGATED_BEFORE = /\b(not|never|no|without|n't|neither|nor)\s+(\w+\s+)?$/i;
const NEGATED_AFTER = /^\s*(no|none|nothing|neither)\b/i;

/** Sentence-sized pieces, so a negation in one clause cannot excuse another. */
function sentencesOf(text: string): string[] {
	return text.split(/(?<=[.!?])\s+|\n+/);
}

function negated(sentence: string, at: number, length: number): boolean {
	return (
		NEGATED_BEFORE.test(sentence.slice(0, at)) ||
		NEGATED_AFTER.test(sentence.slice(at + length))
	);
}

/**
 * The sentences in `text` that affirmatively say a dinah binary lives inside
 * the extension. Three frames, because the claim gets written several ways
 * round: a sentence naming both the extension and the payload with a
 * possession word somewhere in it, in either order and at any distance; a
 * possession word attached straight to the payload; and the payload written as
 * "the one".
 *
 * What these three frames do not reach, written down so that the case list
 * below is not read as an exhaustive one. A claim split across a sentence
 * boundary escapes, because the sentence carrying it names neither party ("The
 * extension is self-contained. It carries dinah."), and sentence-sized
 * judgement is what makes the negation check trustworthy, so the two go
 * together. A verb outside the set escapes, whether passive or indirect ("A
 * dinah binary is distributed with this extension", "This extension installs
 * dinah for you", "dinah is vendored into this extension"). So does the
 * payload named as something other than a binary ("the bundled command-line
 * tool"). A reviewer's own sweep, not this pattern, is the first line against
 * all three.
 */
function claimsToCarryDinah(text: string): string[] {
	const found = new Set<string>();
	for (const sentence of sentencesOf(text)) {
		// Frame one needs both parties named; the other two carry the payload
		// in the match itself, so QUALIFIED_PAYLOAD needs no subject at all.
		const patterns = [QUALIFIED_PAYLOAD];
		if (SUBJECT.test(sentence)) {
			patterns.push(ANAPHORIC_PAYLOAD);
			if (PAYLOAD.test(sentence)) {
				patterns.push(POSSESSION);
			}
		}
		for (const pattern of patterns) {
			pattern.lastIndex = 0;
			for (let m = pattern.exec(sentence); m !== null; m = pattern.exec(sentence)) {
				if (!negated(sentence, m.index, m[0].length)) {
					found.add(sentence.trim());
					break;
				}
			}
		}
	}
	return [...found];
}

// The sentences this guard exists to refuse, written the several ways somebody
// would actually write them rather than the one way the tree happened to use.
// They live here as a case rather than in a comment, so that loosening the
// pattern fails at once and names the frame it stopped catching.
const CLAIMS_A_CARRIED_BINARY = [
	"Leave empty to use the one on your PATH, or the one carried inside this extension.",
	"The dinah binary bundled in this extension is used when nothing is on your PATH.",
	"A dinah binary is included in this extension.",
	"This extension includes a dinah binary.",
	"The extension carries a dinah binary.",
	"This extension ships with a copy of dinah.",
	"A dinah binary is packaged with this extension.",
	"Leave empty to use the one that comes with this extension.",
	"Leave empty to use the built-in dinah binary.",
	"Leave empty to use the extension's own copy of dinah.",
	"If dinah is not on your PATH the extension will fall back to its own copy.",
	"The extension provides a dinah binary for your platform.",
	"A copy of dinah is embedded in the extension.",
	"The extension supplies dinah when it is missing.",
	"The extension has a dinah binary for every platform.",
	"The .vsix contains a dinah binary.",
];

// Copy that says the opposite, or says nothing about where dinah comes from,
// and has to keep passing. The first four are live strings this extension
// publishes today, two of them from `src/status.ts` rather than the manifest,
// and the first of the four is the welcome block without its trailing
// settings-link line, which carries no possession vocabulary and is walked
// whole by the corpus test below. A widened pattern is as likely to start
// refusing honest copy as it is to start catching a claim, and the second
// group is what proves it did not: seven ordinary sentences about the
// extension's own features, all of them refused by the first widening and
// measured at Operator Code Review, plus two that pin the two exclusions this
// pattern makes on purpose (`have` as against `has`, and a counted "one" as
// against an anaphoric one).
const HONEST_COPY = [
	"Dinah is not installed on this machine. This extension is a companion to the `dinah` command-line tool, and it does not carry a copy of it. Install dinah from [github.com/paulmooreparks/dinah#install](https://github.com/paulmooreparks/dinah#install), or set `dinah.path` to a binary you already have.",
	"Absolute path to the `dinah` binary. Leave empty to use the one on your PATH. A binary path is a property of the machine, so this setting does not travel through settings sync.",
	"No dinah binary was found. This extension is a companion to the dinah command-line tool and carries no copy of it.",
	"Install it from https://github.com/paulmooreparks/dinah#install, or set dinah.path to a binary you already have.",
	"This extension never carries a dinah binary.",
	"This extension does not include a copy of dinah.",
	"This extension has no built-in copy of dinah.",
	"This text is replaced as soon as the extension has an answer.",
	"Dinah has a binary for this window but no workbench to go with it.",
	"This extension provides a tree view of your workbench.",
	"The extension includes a status bar item showing which workbench this folder resolves to.",
	"This extension ships with commands for claiming, moving and releasing cards.",
	"This extension supplies the editor with a view of the cards in your workbench.",
	"The extension comes with a settings page.",
	"The extension bundles its own webview assets.",
	"This extension includes telemetry: none.",
	"This extension will use a binary you already have.",
	"The extension provides one view per workbench.",
];

test("the carried-binary guard catches the spellings a writer would reach for", () => {
	for (const sentence of CLAIMS_A_CARRIED_BINARY) {
		assert.deepEqual(
			claimsToCarryDinah(sentence),
			[sentence],
			`the guard no longer catches: ${sentence}`,
		);
	}
	for (const sentence of HONEST_COPY) {
		assert.deepEqual(
			claimsToCarryDinah(sentence),
			[],
			`the guard now refuses honest copy: ${sentence}`,
		);
	}
});

test("no manifest string offers the extension itself as a place dinah comes from", () => {
	// The extension is a companion to the CLI and installs nothing, so every
	// user-facing string the manifest publishes has to agree with that. The
	// dinah.path description was the one that did not: it survived the card that
	// removed the carried binary because that sweep searched for the phrases the
	// welcome view used, and this sentence shared no word with them.
	//
	// In scope: the extension description, both description forms of every
	// setting, and every welcome block for this view. Out of scope, and pinned
	// elsewhere: `enumDescriptions` and `markdownEnumDescriptions`, which no
	// setting here declares, and the status-bar tooltips, which live in
	// `src/status.ts` and are held by `test/unit/status.test.ts`.
	const configuration = contributes.configuration as {
		properties: Record<string, { markdownDescription?: string; description?: string }>;
	};
	const strings: { where: string; text: string }[] = [
		{ where: "the manifest description", text: manifest.description as string },
	];
	for (const [key, property] of Object.entries(configuration.properties)) {
		// Both forms are read. Collecting one or the other would skip the plain
		// description of any setting that grew a markdown one beside it.
		if (property.markdownDescription !== undefined) {
			strings.push({
				where: `the ${key} setting's markdownDescription`,
				text: property.markdownDescription,
			});
		}
		if (property.description !== undefined) {
			strings.push({ where: `the ${key} setting's description`, text: property.description });
		}
	}
	for (const block of welcomeBlocks()) {
		strings.push({ where: `the welcome block for ${block.when}`, text: block.contents });
	}
	// A vacuous pass is the failure mode here, so the corpus is counted twice
	// over. The derived count fails when the collector above stops reading one
	// of the manifest's three sources, and the literal count fails when the
	// published surface shrinks under a guard that would otherwise report a
	// smaller sweep as a clean one.
	const declared =
		1 +
		Object.values(configuration.properties).reduce(
			(n, property) =>
				n +
				(property.markdownDescription === undefined ? 0 : 1) +
				(property.description === undefined ? 0 : 1),
			0,
		) +
		welcomeBlocks().length;
	assert.equal(strings.length, declared);
	assert.equal(strings.length, 11);
	for (const { where, text } of strings) {
		assert.deepEqual(
			claimsToCarryDinah(text),
			[],
			`${where} says a dinah binary lives inside the extension`,
		);
	}
});

test("the manifest declares exactly the commands identity.ts names", () => {
	const commands = contributes.commands as { command: string; title: string }[];
	assert.deepEqual(
		commands.map((entry) => entry.command),
		[...TREE_COMMANDS],
	);
});

test("the openAttachment entry is declared where identity.ts puts it, under the title a click names", () => {
	// The drift test above holds the two rosters together by position, which
	// already pins this command's place. This one pins the entry's other
	// half: a command whose title read like a menu item would show up in the
	// Command Palette as an action with no target, which is the one surface
	// the tree itself never renders.
	//
	// The position was asserted as last until dinah-331 appended two commands
	// after it. Reading the position out of TREE_COMMANDS keeps what the
	// assertion was for, which is that the two rosters agree about where this
	// entry sits, and stops it going stale every time a card adds a command.
	const commands = contributes.commands as { command: string; title: string }[];
	const declared = commands.findIndex(
		(entry) => entry.command === COMMAND_OPEN_ATTACHMENT,
	);
	assert.equal(declared, TREE_COMMANDS.indexOf(COMMAND_OPEN_ATTACHMENT));
	assert.equal(commands[declared].title, "Dinah: Open Attachment");
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

test("no workbench menu item is offered on a column row, and a state group carries none at all", () => {
	// dinah-330 AC-2's exclusion, narrowed twice. The state-group half is
	// unchanged: that row is a heading over cards and names no entity, so no
	// menu of any kind belongs on it.
	//
	// The column half was written as the same absence assertion, because a
	// column row then carried no menu either. dinah-332 gave it Edit
	// Instructions and dinah-331 gave it two more acts, so the claim worth
	// keeping is the one the heading always named: a workbench act must not be
	// offered on a row that names no workbench. That is asserted against the
	// clause the workbench items are registered under rather than by substring,
	// since dinah.column.open contains dinah.column and every substring reading
	// of these values now passes on text that means the opposite of what it
	// says.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	const items = menus["view/item/context"];
	for (const value of [CONTEXT_COLUMN, CONTEXT_COLUMN_OPEN, CONTEXT_COLUMN_FULL]) {
		assert.doesNotMatch(value, WORKBENCH_ROW_PATTERN);
	}
	const workbenchItems = items.filter((entry) =>
		[
			COMMAND_CHECK_WORKBENCH,
			COMMAND_COPY_WORKBENCH_PATH,
			COMMAND_EDIT_WORKBENCH_DEFINITION,
		].includes(entry.command),
	);
	// Three items are expected, and reading the count is what stops a filter
	// that matched nothing from satisfying the loop beneath it. Attach File is
	// not counted here: it reaches a workbench row under the same clause, but
	// it also reaches columns and cards, so it is not a workbench act.
	assert.equal(
		workbenchItems.length,
		3,
		`the three workbench-row items produced ${String(workbenchItems.length)} entries`,
	);
	for (const entry of workbenchItems) {
		assert.equal(entry.when, WORKBENCH_ROW_CLAUSE, entry.command);
	}
	const allClauses = items.map((entry) => entry.when).join(" ");
	assert.ok(
		!allClauses.includes(CONTEXT_STATE_GROUP),
		`a context menu is registered against a state group row: ${allClauses}`,
	);
});

/**
 * The contextValues a column row carries, as one regex.
 *
 * dinah-332 registered its item against the single value dinah.column, which
 * was then the only value a column row could carry. dinah-331 gave that row
 * two suffixed values by capacity and left the bare one for the row whose
 * ColumnView the status join missed, so an equality against the bare value
 * would now reach only that miss. The pattern covers all three, which is the
 * reach the item shipped with.
 */
const COLUMN_ROW_PATTERN = /^dinah\.column(\.open|\.full)?$/;

/** The one `when` clause the column row's editing item is registered under. */
const COLUMN_ROW_CLAUSE = `view == dinah.workbenchView && viewItem =~ /${COLUMN_ROW_PATTERN.source}/`;

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

test("the instructions item matches every column row and nothing else", () => {
	// dinah-332 AC-2, the column half, held to the three contextValues a column
	// row can carry since dinah-331. The clause is composed from the pattern
	// rather than spelled a second time, so a rename that misses the manifest
	// goes red here instead of shipping a menu that never opens.
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
	// Read back through the clause the manifest carries rather than through the
	// pattern composed above, so a manifest that drifted from this file is what
	// goes red. A column at capacity and a column the join missed both keep the
	// item, because editing a column's instructions is not gated on anything the
	// status join reports.
	for (const value of [CONTEXT_COLUMN, CONTEXT_COLUMN_OPEN, CONTEXT_COLUMN_FULL]) {
		assert.equal(opensOn(matched[0].when, value), true, value);
	}
	// A state group, every card state and the three workbench row kinds stay
	// outside it, which is what the anchors buy over a bare prefix.
	for (const value of [
		...FORBIDDEN_FOR_EDIT_ITEMS,
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_WORKBENCH_CANDIDATE,
		CONTEXT_WORKBENCH_FOREST,
	]) {
		assert.equal(opensOn(matched[0].when, value), false, value);
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

// ---------------------------------------------------------------------------
// dinah-331: the two creation commands, and the four rows they are offered on
// ---------------------------------------------------------------------------

/**
 * The contextValues each creation item is offered on, as one regex apiece.
 *
 * The manifest spells these same patterns inside its `when` clauses, and the
 * tests below compose the clauses from these constants rather than from a
 * second copy of each pattern, on the terms WORKBENCH_ROW_PATTERN already set.
 * Two hand-written spellings drift, and the drift shows up only as a menu that
 * silently stopped opening.
 */
const COLUMN_ATTACH_PATTERN = /^dinah\.column\./;
const CARD_ATTACH_PATTERN = /^dinah\.card\./;

/** The clause New Card is registered under, which matches one value exactly. */
const NEW_CARD_CLAUSE = `view == dinah.workbenchView && viewItem == ${CONTEXT_COLUMN_OPEN}`;

/**
 * Whether a `when` clause taken from the manifest opens on a given viewItem.
 *
 * The clause is read rather than restated. A table asserting that
 * CONTEXT_COLUMN_OPEN equals CONTEXT_COLUMN_OPEN would pass whatever the
 * manifest said, which is a check every value satisfies and therefore no check
 * at all; this reads the operand out of the clause the manifest actually
 * carries, so editing that clause is what turns the table red.
 *
 * Both spellings VS Code allows here are handled, because the two creation
 * items use one each: New Card is an equality, and the three Attach File items
 * are regexes.
 */
function opensOn(clause: string, viewItem: string): boolean {
	const regex = /viewItem =~ \/(.+?)\/(?:\s|$)/.exec(clause);
	if (regex !== null) {
		return new RegExp(regex[1]).test(viewItem);
	}
	const equality = /viewItem == (\S+)/.exec(clause);
	assert.notEqual(equality, null, `no viewItem operand in the clause: ${clause}`);
	return equality?.[1] === viewItem;
}

/**
 * The `when` clause of the one Attach File item sitting in the menu group given.
 *
 * The group is the key because it is the one thing about an item that is
 * decided independently of the clause: it says where in the context menu the
 * entry appears. Selecting the entry by the pattern the table is about to
 * check would ask the manifest to confirm what the test already assumed, so
 * the two clauses could be swapped between groups and every table would go on
 * passing.
 */
function attachClauseFor(group: string): string {
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string; group: string }[]
	>;
	const matched = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_ATTACH_FILE && entry.group === group,
	);
	assert.equal(matched.length, 1, `${group} holds ${matched.length} Attach File items`);
	return matched[0].when;
}

test("the clause reader answers both spellings, so the tables below are not vacuous", () => {
	// The matcher is the thing every table on this card leans on, so it is
	// driven against a clause of each shape with a value that must match and a
	// value that must not. A matcher that answered true always, or false
	// always, would make every table below pass or fail as one.
	assert.equal(opensOn("view == v && viewItem == dinah.column.open", "dinah.column.open"), true);
	assert.equal(opensOn("view == v && viewItem == dinah.column.open", "dinah.column.full"), false);
	assert.equal(opensOn("view == v && viewItem =~ /^dinah\\.card\\./", "dinah.card.active"), true);
	assert.equal(opensOn("view == v && viewItem =~ /^dinah\\.card\\./", "dinah.column"), false);
});

test("both creation commands are declared with bare titles that name a further prompt", () => {
	// dinah-331 AC-9. Bare rather than "Dinah: " prefixed, matching Move and
	// Block: both are row commands the palette never shows, so the prefix
	// would be read by nobody. The trailing ellipsis is the other half of that
	// convention, and it is honest here, since each one asks before it acts.
	const commands = contributes.commands as { command: string; title: string }[];
	const titles = new Map(commands.map((entry) => [entry.command, entry.title]));
	assert.equal(titles.get(COMMAND_NEW_CARD), "New Card...");
	assert.equal(titles.get(COMMAND_ATTACH_FILE), "Attach File...");
	for (const command of [COMMAND_NEW_CARD, COMMAND_ATTACH_FILE]) {
		assert.ok(
			!String(titles.get(command)).startsWith("Dinah: "),
			`${command} carries the palette prefix on a row command`,
		);
	}
});

test("neither creation command is reachable from the Command Palette", () => {
	// dinah-331 AC-9's other half, and dinah-342's defect. Each command acts on
	// the row it was invoked on, and the palette passes no row, so a palette
	// entry would be an action with no target.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	for (const command of [COMMAND_NEW_CARD, COMMAND_ATTACH_FILE]) {
		const entries = menus.commandPalette.filter(
			(entry) => entry.command === command,
		);
		assert.equal(entries.length, 1, `${command} has ${entries.length} palette entries`);
		assert.equal(entries[0].when, "false");
		assert.ok(ROW_COMMANDS.includes(command), `${command} is not a row command`);
		assert.ok(
			!GLOBAL_COMMANDS.includes(command),
			`${command} is classified as global, and it cannot act without a row`,
		);
	}
});

test("New Card is offered on an open column and on no other row in the tree", () => {
	// dinah-331 AC-10. The clause matches one value exactly rather than by
	// prefix, which is the whole of Decision 2 expressed in the manifest: a
	// full column and a column the join missed both stop offering the act.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string }[]
	>;
	const entries = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_NEW_CARD,
	);
	assert.equal(entries.length, 1, `New Card has ${entries.length} menu entries`);
	assert.equal(entries[0].when, NEW_CARD_CLAUSE);
	// Read back through the clause the manifest carries. A prefix clause
	// written here by mistake would match all three column values and would
	// read, in the manifest, exactly like one that worked.
	const cases: [string, boolean][] = [
		[CONTEXT_COLUMN_OPEN, true],
		[CONTEXT_COLUMN_FULL, false],
		[CONTEXT_COLUMN, false],
		[CONTEXT_CARD_ACTIVE, false],
		[CONTEXT_WORKBENCH_ROOT, false],
	];
	for (const [value, matches] of cases) {
		assert.equal(opensOn(entries[0].when, value), matches, value);
	}
});

test("Attach File is offered on the workbench, on either column, and on every card", () => {
	// dinah-331 AC-11. Three entries and not one, because the three levels sit
	// in different menu groups: the workbench row's acts belong together, the
	// column row's do, and a card's creation act sits after its flow verbs.
	const menus = contributes.menus as Record<
		string,
		{ command: string; when: string; group: string }[]
	>;
	const entries = menus["view/item/context"].filter(
		(entry) => entry.command === COMMAND_ATTACH_FILE,
	);
	assert.equal(entries.length, 3, `Attach File has ${entries.length} menu entries`);
	const byClause = new Map(entries.map((entry) => [entry.when, entry.group]));
	assert.ok(
		byClause.has(
			`view == dinah.workbenchView && viewItem =~ /${COLUMN_ATTACH_PATTERN.source}/`,
		),
		`no column clause among: ${[...byClause.keys()].join(" | ")}`,
	);
	assert.ok(byClause.has(WORKBENCH_ROW_CLAUSE));
	assert.ok(
		byClause.has(
			`view == dinah.workbenchView && viewItem =~ /${CARD_ATTACH_PATTERN.source}/`,
		),
		`no card clause among: ${[...byClause.keys()].join(" | ")}`,
	);
});

test("the column clause for Attach File reaches both capacities and not the join miss", () => {
	// dinah-331 AC-11. Capacity never gates attaching (Decision 3), so a full
	// column still offers it. A column the join missed offers nothing, which is
	// what the trailing dot in the pattern buys: dinah.column has no dot after
	// column and so does not match.
	const clause = attachClauseFor("1_column@3");
	const cases: [string, boolean][] = [
		[CONTEXT_COLUMN_OPEN, true],
		[CONTEXT_COLUMN_FULL, true],
		[CONTEXT_COLUMN, false],
	];
	for (const [value, matches] of cases) {
		assert.equal(opensOn(clause, value), matches, value);
	}
});

test("the workbench clause for Attach File reaches all three resolved workbench rows", () => {
	// dinah-331 AC-11. The same pattern the Check and Copy Path items already
	// use, reused rather than respelled, so a row kind renamed on one side is
	// caught for all three commands at once.
	const clause = attachClauseFor("1_workbench@4");
	for (const value of [
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_WORKBENCH_CANDIDATE,
		CONTEXT_WORKBENCH_FOREST,
	]) {
		assert.equal(opensOn(clause, value), true, value);
	}
	for (const value of [CONTEXT_COLUMN_OPEN, CONTEXT_CARD_ACTIVE, CONTEXT_STATE_GROUP]) {
		assert.equal(opensOn(clause, value), false, value);
	}
});

test("the card clause for Attach File reaches every state a card row can stand in", () => {
	// dinah-331 AC-11. attach refuses nothing about a card's state (Decision
	// 3), so all four values actionsFor composes match. A blocked card can be
	// given the file that explains why it is blocked, which is the case that
	// makes a state gate here plainly wrong.
	const clause = attachClauseFor("2_attachment@1");
	for (const value of [
		CONTEXT_CARD_READY_CLAIM,
		CONTEXT_CARD_READY_NONE,
		CONTEXT_CARD_ACTIVE,
		CONTEXT_CARD_BLOCKED,
	]) {
		assert.equal(opensOn(clause, value), true, value);
	}
	for (const value of [CONTEXT_COLUMN_OPEN, CONTEXT_WORKBENCH_ROOT, CONTEXT_STATE_GROUP]) {
		assert.equal(opensOn(clause, value), false, value);
	}
});

// The registration guard's own comparison, driven directly (dinah-369).
//
// These four rows call assertCommandsFullyRegistered with lists this file
// builds, which is the only way any unit test can reach it. The one place the
// function runs against a real registration set is activate(), and no unit
// test may import extension.ts, so these prove that the comparison is correct
// and prove nothing about activate() calling it.
//
// Nothing standing proves that half either, and a reader should not hand the
// job to the integration suite. That suite activates the extension in a real
// editor host, but activation succeeds just as readily with activate()'s call
// to the guard deleted, so the suite reddens only when a registration is
// genuinely missing at the same time. Nothing has yet dropped a registration
// and watched that suite go red. The arming pass is staged at this card's Test
// stage (dinah-369 AC-6). Until it runs, no standing check reddens if the call
// is removed. registrationGuard.ts's header carries the same account.
//
// They sit here rather than in a file of their own because the sibling rows
// above hold identity.ts's roster against the manifest, and this holds the
// same roster against what activation did with it.

test("assertCommandsFullyRegistered does not throw when every declared command is registered, in any order", () => {
	assert.ok(
		TREE_COMMANDS.length > 0,
		"identity.ts declares no commands, so every row below would pass on an empty roster",
	);
	assert.doesNotThrow(() =>
		assertCommandsFullyRegistered(TREE_COMMANDS, [...TREE_COMMANDS]),
	);
	// Which order registeredIds arrives in depends on which of activate()'s
	// two loops and five single calls ran first, and that is incidental to the
	// guarantee, so the reversal has to pass as well.
	assert.doesNotThrow(() =>
		assertCommandsFullyRegistered(TREE_COMMANDS, [...TREE_COMMANDS].reverse()),
	);
});

test("assertCommandsFullyRegistered throws naming a command dropped from registration", () => {
	// The defect the guard exists to catch: one register() call site deleted
	// from activate() while every other check stays green.
	const registered = TREE_COMMANDS.filter((id) => id !== COMMAND_NEW_CARD);
	assert.equal(
		registered.length,
		TREE_COMMANDS.length - 1,
		"the filter dropped no command, so the throw below would be about something else",
	);
	assert.throws(
		() => assertCommandsFullyRegistered(TREE_COMMANDS, registered),
		(error: unknown) => {
			assert.ok(error instanceof Error);
			assert.ok(
				error.message.includes(COMMAND_NEW_CARD),
				`the message does not name the dropped command: ${error.message}`,
			);
			assert.ok(
				error.message.includes("missing"),
				`the message does not report the command as missing: ${error.message}`,
			);
			return true;
		},
	);
});

test("assertCommandsFullyRegistered throws naming a command registered that identity.ts does not declare", () => {
	// The other direction, which is a command registered in activate() and
	// declared in neither identity.ts nor the manifest. Nothing draws it in
	// the tree, so it reaches a reader only through the Command Palette.
	const stray = "dinah.tree.notReal";
	assert.ok(
		!TREE_COMMANDS.includes(stray),
		"identity.ts now declares the id this row uses as its undeclared one",
	);
	assert.throws(
		() => assertCommandsFullyRegistered(TREE_COMMANDS, [...TREE_COMMANDS, stray]),
		(error: unknown) => {
			assert.ok(error instanceof Error);
			assert.ok(
				error.message.includes(stray),
				`the message does not name the undeclared command: ${error.message}`,
			);
			assert.ok(
				error.message.includes("unexpected"),
				`the message does not report the command as unexpected: ${error.message}`,
			);
			return true;
		},
	);
});

test("assertCommandsFullyRegistered does not report a duplicate registration as unexpected", () => {
	// A repeated id does not make the roster incomplete, since every declared
	// command is still covered. Pinning the count or the positions would fail
	// activation over a duplicate or over a legitimate reordering, so the
	// comparison reads both sides as sets (dinah-369 D-3). What VS Code does
	// with a second registration of the same id is the editor's business, and
	// nothing here rests on it.
	const registered = [...TREE_COMMANDS, COMMAND_REFRESH];
	assert.equal(
		registered.length,
		TREE_COMMANDS.length + 1,
		"the duplicate was not appended, so this row proves nothing about duplicates",
	);
	assert.doesNotThrow(() =>
		assertCommandsFullyRegistered(TREE_COMMANDS, registered),
	);
});

// The licence the extension declares, and the licence text it carries.
//
// A manifest field is what the marketplace displays and what a consumer reads
// to decide what they may do with the code, so it is a statement about terms
// rather than a note. It shipped as "MIT" on an Apache-2.0 project until
// dinah-371, and nothing read it back. The repository root is the authority
// for both halves: LICENSE holds the text, and these rows hold the extension
// to it.

const repoRoot = join(extensionRoot, "..", "..");

test("the manifest declares the licence the project is under", () => {
	assert.equal(
		manifest.license,
		"Apache-2.0",
		"the extension manifest declares a licence the project is not under",
	);
});

test("the extension carries the repository's own licence text", () => {
	// Copied rather than linked, because a symbolic link needs privilege on
	// Windows, which is where this is built. A copy can drift, so the bytes
	// are compared instead of the file merely being required to exist.
	const carried = readFileSync(join(extensionRoot, "LICENSE"), "utf8");
	const authority = readFileSync(join(repoRoot, "LICENSE"), "utf8");
	assert.equal(
		carried,
		authority,
		"editors/vscode/LICENSE has drifted from the repository root LICENSE",
	);
});
