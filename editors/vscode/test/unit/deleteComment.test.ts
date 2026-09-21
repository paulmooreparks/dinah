// dinah-550: a comment row deletes its comment, and a comment that is an
// item's answer of record is deleted only after a second confirmation.
//
// The spawner is a fake answering from a script, so no test here starts the
// real binary and none can inherit an ambient Dinah identity. The host records
// every call it receives on one timeline, because the design under test is
// about what the reader is shown and how many times: a refusal the command is
// about to recover from must not appear before the second confirmation, and
// every path reports at most once and checkpoints exactly once.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { CommandHost } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import { ROW_COMMAND_TABLE } from "../../src/commandTable";
import {
	COMMAND_DELETE_COMMENT,
	COMMAND_OPEN_COMMENT,
	ROW_COMMANDS,
	TREE_COMMANDS,
} from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import { SELECTION_POLICIES } from "../../src/selection";
import type { TreeElement } from "../../src/tree";
import {
	FOLDER,
	ROOT,
	cardHost,
	commentRow,
	emptyLog,
	noteRow,
	ok,
	wiringFor,
} from "../support/rows";

const NOT_DESIGNATABLE = "dinah.not-designatable";
const ITEM = "tr-1/questions/1";
const ANSWER = "tr-1/questions/1/comments/1";

/** The argv every verb reaches dinah as, once --json and the pin are in front. */
function pinned(...args: string[]): string[] {
	return ["--json", "--workbench", ROOT, ...args];
}

/**
 * The refusal `dinah delete` answers for a comment that is an item's answer of
 * record, in the shape a reproduction against the real binary produced. The
 * context member is left out when item is undefined, which is the case the
 * second confirmation must not be raised for.
 */
function notDesignatable(ref: string, item: string | undefined): SpawnOutcome {
	return {
		code: 2,
		stdout: JSON.stringify({
			outcome: "refused",
			refusal: NOT_DESIGNATABLE,
			detail: ref,
			...(item === undefined ? {} : { context: { item } }),
		}),
		stderr: "",
	};
}

/** The message that refusal shows, as refusalMessage composes it. */
function notDesignatableMessage(ref: string): string {
	return `${NOT_DESIGNATABLE}: ${ref}`;
}

interface Run {
	/** Every host call and every spawn, in the order they happened. */
	readonly timeline: string[];
	readonly calls: string[][];
	readonly errors: string[];
	readonly confirmations: { message: string; label: string }[];
	readonly checkpoints: string[];
	readonly lines: string[];
	readonly warnings: string[];
}

/**
 * Drives the entry the editor registers for Delete Comment.
 *
 * answers holds what each confirmation returns, in order, so a run can confirm
 * the first question and decline the second. A confirmation asked with no
 * answer left declines, which is what a dismissed modal answers.
 */
async function invoke(
	elements: readonly TreeElement[],
	answers: readonly boolean[],
	answer: (argv: readonly string[]) => SpawnOutcome = () => ok(),
): Promise<Run> {
	const entry = ROW_COMMAND_TABLE.find((row) => row.id === COMMAND_DELETE_COMMENT);
	assert.notEqual(entry, undefined, "no table entry carries Delete Comment");
	const timeline: string[] = [];
	const calls: string[][] = [];
	const errors: string[] = [];
	const confirmations: { message: string; label: string }[] = [];
	const checkpoints: string[] = [];
	const warnings: string[] = [];
	const lines: string[] = [];
	const scripted = [...answers];
	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		timeline.push(`spawn ${argv.slice(3).join(" ")}`);
		return answer(argv);
	};
	const log = emptyLog();
	const host: CommandHost = {
		...cardHost(log),
		showError: (message) => {
			errors.push(message);
			timeline.push(`error ${message}`);
		},
		showWarning: async (message) => {
			warnings.push(message);
			timeline.push(`warning ${message}`);
			return undefined;
		},
		appendLines: (added) => {
			lines.push(...added);
		},
		confirmDestructive: async (message, label) => {
			confirmations.push({ message, label });
			timeline.push(`confirm ${message}`);
			return scripted.shift() ?? false;
		},
		checkpoint: async (folder) => {
			checkpoints.push(folder);
			timeline.push(`checkpoint ${folder}`);
		},
	};
	await (entry as (typeof ROW_COMMAND_TABLE)[number]).invoke(elements, {
		...wiringFor(log, spawner),
		cardHost: host,
	});
	return { timeline, calls, errors, confirmations, checkpoints, lines, warnings };
}

/** The second confirmation's message for the comment and item given. */
function designatedPrompt(ref: string, item: string): string {
	return ENGLISH("dialog.comment.delete.designated.confirm", { ref, item });
}

test("Delete Comment is a oneInput command", () => {
	assert.deepEqual(SELECTION_POLICIES[COMMAND_DELETE_COMMENT], { policy: "oneInput" });
});

test("Delete Comment stands immediately after Open Comment in both rosters", () => {
	for (const roster of [TREE_COMMANDS, ROW_COMMANDS]) {
		const open = roster.indexOf(COMMAND_OPEN_COMMENT);
		assert.notEqual(open, -1);
		assert.equal(roster[open + 1], COMMAND_DELETE_COMMENT);
	}
});

test("Delete Comment is declared after Open Comment, hidden from the palette and offered in the destructive group", () => {
	const manifest = JSON.parse(
		readFileSync(join(__dirname, "..", "..", "..", "package.json"), "utf8"),
	) as {
		contributes: {
			commands: { command: string; title: string }[];
			menus: Record<string, { command: string; when: string; group?: string }[]>;
		};
	};
	const declared = manifest.contributes.commands.map((command) => command.command);
	assert.equal(
		declared[declared.indexOf(COMMAND_OPEN_COMMENT) + 1],
		COMMAND_DELETE_COMMENT,
	);
	assert.deepEqual(
		manifest.contributes.commands.find((command) => command.command === COMMAND_DELETE_COMMENT),
		{
			command: COMMAND_DELETE_COMMENT,
			title: "%manifest.command.dinah.tree.deleteComment.title%",
		},
	);
	assert.deepEqual(
		manifest.contributes.menus["view/item/context"].filter(
			(item) => item.command === COMMAND_DELETE_COMMENT,
		),
		[
			{
				command: COMMAND_DELETE_COMMENT,
				when: "view == dinah.workbenchView && viewItem == dinah.comment",
				group: "9_destructive@1",
			},
		],
	);
	assert.deepEqual(
		manifest.contributes.menus.commandPalette.filter(
			(item) => item.command === COMMAND_DELETE_COMMENT,
		),
		[{ command: COMMAND_DELETE_COMMENT, when: "false" }],
	);
});

test("one confirmed comment is deleted with delete <ref> --yes and shows no error", async () => {
	const ref = "tr-1/comments/3";
	const run = await invoke([commentRow({ ref })], [true]);
	assert.deepEqual(run.calls, [pinned("delete", ref, "--yes")]);
	assert.deepEqual(run.confirmations, [
		{
			message: ENGLISH("dialog.comment.delete.confirm", { ref }),
			label: ENGLISH("dialog.comment.delete.action"),
		},
	]);
	assert.deepEqual(run.errors, []);
	assert.deepEqual(run.warnings, []);
	assert.deepEqual(run.checkpoints, [FOLDER]);
});

test("a declined first confirmation spawns nothing", async () => {
	const run = await invoke([commentRow({ ref: "tr-1/comments/3" })], [false]);
	assert.equal(run.confirmations.length, 1);
	assert.deepEqual(run.calls, []);
	assert.deepEqual(run.errors, []);
	assert.deepEqual(run.checkpoints, []);
});

test("several selected comments are confirmed once, with the count, and each is deleted", async () => {
	const refs = ["tr-1/comments/1", "tr-1/comments/2", "tr-2/comments/1"];
	const run = await invoke(
		refs.map((ref) => commentRow({ ref })),
		[true],
	);
	assert.deepEqual(run.confirmations, [
		{
			message: ENGLISH("dialog.comment.delete.confirm.many", { count: "3" }),
			label: ENGLISH("dialog.comment.delete.action"),
		},
	]);
	assert.deepEqual(
		run.calls,
		refs.map((ref) => pinned("delete", ref, "--yes")),
	);
	assert.deepEqual(run.errors, []);
	assert.deepEqual(run.warnings, []);
	// The bulk runner collects the per-row checkpoints and runs one per folder.
	assert.deepEqual(run.checkpoints, [FOLDER]);
});

test("a row naming no comment is skipped with the NO_COMMENT reason", async () => {
	const ref = "tr-1/comments/1";
	const run = await invoke([commentRow({ ref }), noteRow("a note")], [true]);
	assert.deepEqual(run.calls, [pinned("delete", ref, "--yes")]);
	assert.ok(
		run.lines.some((line) => line.endsWith(": names no comment")),
		`the channel carries no NO_COMMENT line: ${run.lines.join(" | ")}`,
	);
});

test("an item's answer raises the second confirmation, with no error shown before it", async () => {
	const run = await invoke([commentRow({ ref: ANSWER })], [true, false], (argv) =>
		argv.includes("--force") ? ok() : notDesignatable(ANSWER, ITEM),
	);
	assert.equal(run.confirmations.length, 2);
	assert.deepEqual(run.confirmations[1], {
		message: designatedPrompt(ANSWER, ITEM),
		label: ENGLISH("dialog.comment.delete.designated.action"),
	});
	const second = run.timeline.indexOf(`confirm ${designatedPrompt(ANSWER, ITEM)}`);
	assert.notEqual(second, -1);
	assert.deepEqual(
		run.timeline.slice(0, second).filter((event) => event.startsWith("error ")),
		[],
		`an error was shown before the second confirmation: ${run.timeline.join(" | ")}`,
	);
});

test("confirming the second confirmation forces the delete, and shows nothing when it succeeds", async () => {
	const run = await invoke([commentRow({ ref: ANSWER })], [true, true], (argv) =>
		argv.includes("--force") ? ok() : notDesignatable(ANSWER, ITEM),
	);
	assert.deepEqual(run.calls, [
		pinned("delete", ANSWER, "--yes"),
		pinned("delete", ANSWER, "--yes", "--force"),
	]);
	assert.deepEqual(run.errors, []);
	assert.deepEqual(run.warnings, []);
	assert.deepEqual(run.checkpoints, [FOLDER]);
});

test("a refused forced delete shows that refusal once and nothing about the first", async () => {
	const run = await invoke([commentRow({ ref: ANSWER })], [true, true], (argv) =>
		argv.includes("--force")
			? { code: 2, stdout: JSON.stringify({ refusal: "not-operator", detail: ITEM }), stderr: "" }
			: notDesignatable(ANSWER, ITEM),
	);
	assert.deepEqual(run.errors, [`not-operator: ${ITEM}`]);
	assert.deepEqual(run.checkpoints, [FOLDER]);
});

test("declining the second confirmation spawns no forced delete, shows the refusal once and records the row as failed", async () => {
	const run = await invoke([commentRow({ ref: ANSWER })], [true, false], () =>
		notDesignatable(ANSWER, ITEM),
	);
	assert.deepEqual(run.calls, [pinned("delete", ANSWER, "--yes")]);
	assert.deepEqual(run.errors, [notDesignatableMessage(ANSWER)]);
	assert.deepEqual(run.checkpoints, [FOLDER]);
	// A single-row run shows no summary of its own, so the row's failure is
	// read off the channel line runBulk writes for every row that did not
	// finish.
	assert.ok(
		run.lines.some((line) => line.endsWith(`: ${notDesignatableMessage(ANSWER)}`)),
		`the row was not recorded as failed: ${run.lines.join(" | ")}`,
	);
});

test("a refusal carrying no item raises no second confirmation and shows the refusal once", async () => {
	const run = await invoke([commentRow({ ref: ANSWER })], [true, true], () =>
		notDesignatable(ANSWER, undefined),
	);
	assert.equal(run.confirmations.length, 1);
	assert.deepEqual(run.calls, [pinned("delete", ANSWER, "--yes")]);
	assert.deepEqual(run.errors, [notDesignatableMessage(ANSWER)]);
	assert.deepEqual(run.checkpoints, [FOLDER]);
});

test("in a run over several comments the second confirmation is raised for the one answer alone", async () => {
	const plain = ["tr-1/comments/1", "tr-2/comments/4"];
	const run = await invoke(
		[commentRow({ ref: plain[0] }), commentRow({ ref: ANSWER }), commentRow({ ref: plain[1] })],
		[true, true],
		(argv) =>
			argv.includes(ANSWER) && !argv.includes("--force")
				? notDesignatable(ANSWER, ITEM)
				: ok(),
	);
	assert.deepEqual(
		run.confirmations.map((confirmation) => confirmation.message),
		[
			ENGLISH("dialog.comment.delete.confirm.many", { count: "3" }),
			designatedPrompt(ANSWER, ITEM),
		],
	);
	assert.deepEqual(run.calls, [
		pinned("delete", plain[0], "--yes"),
		pinned("delete", ANSWER, "--yes"),
		pinned("delete", ANSWER, "--yes", "--force"),
		pinned("delete", plain[1], "--yes"),
	]);
	assert.deepEqual(run.errors, []);
	assert.deepEqual(run.warnings, []);
});
