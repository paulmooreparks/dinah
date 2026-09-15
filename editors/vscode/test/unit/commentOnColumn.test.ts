// Comment on Column: which rows offer it, and what the draft is handed.
//
// The population of column-row context values is derived from
// columnActionsFor, which is the only producer of one, rather than read off
// the clauses already in the manifest. A rule tested against the shapes
// whoever wrote the rule had in mind agrees with itself by construction: the
// four suffixed values pass both the anchored pattern and the prefix pattern,
// and only the bare dinah.column tells them apart. That value is the one an
// earlier draft of this card's contract missed, so it is asserted by name.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import type { ColumnCommandHost } from "../../src/columnCommands";
import { invokeCommentOnColumn } from "../../src/columnCommands";
import {
	COMMAND_COMMENT_ON_COLUMN,
	CONTEXT_CARD_ACTIVE,
	CONTEXT_STATE_GROUP,
	CONTEXT_WORKBENCH_ROOT,
} from "../../src/identity";
import { ENGLISH } from "../../src/l10n";
import type { RootRow, TreeElement } from "../../src/tree";
import { columnActionsFor } from "../../src/tree";
import type { ColumnView } from "../../src/wire";

const extensionRoot = join(__dirname, "..", "..", "..");
const manifest = JSON.parse(
	readFileSync(join(extensionRoot, "package.json"), "utf8"),
) as {
	contributes: {
		menus: Record<string, { command: string; when: string }[]>;
	};
};

const BENCH = "C:/work/board";
const FOLDER = "C:/work";
const COLUMN_ID = "c0100000000a";
const COLUMN_SLUG = "spec";

/**
 * Every context value columnActionsFor can return, derived from the function
 * rather than written out, so a suffix the extension mints later joins this
 * population without anybody remembering to add it here.
 *
 * The five inputs below are the five branches of that function: no view at
 * all, and then each combination of full and pullable.
 */
function everyColumnContextValue(): string[] {
	const view = (overrides: Partial<ColumnView>): ColumnView => ({
		id: COLUMN_ID,
		slug: COLUMN_SLUG,
		title: "Spec",
		kind: "flow",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: true,
		count: 0,
		...overrides,
	});
	const values = [
		columnActionsFor(undefined),
		columnActionsFor(view({ count: 1, capacity: 0 })),
		columnActionsFor(view({ count: 2, capacity: 2 })),
		columnActionsFor(view({ count: 1, capacity: 0, takes_work_up: false }), "doing"),
		columnActionsFor(view({ count: 2, capacity: 2, takes_work_up: false }), "doing"),
	];
	return [...new Set(values)];
}

/** The clause the manifest registers Comment on Column under. */
function commentOnColumnClause(): string {
	const items = manifest.contributes.menus["view/item/context"];
	const rows = items.filter((item) => item.command === COMMAND_COMMENT_ON_COLUMN);
	assert.equal(
		rows.length,
		1,
		`the manifest registers Comment on Column ${String(rows.length)} times in the context menu`,
	);
	return rows[0].when;
}

/** Whether a clause admits a contextValue, reading the operand out of it. */
function admits(clause: string, viewItem: string): boolean {
	const found = /viewItem =~ \/(.+?)\/(?:\s|$)/.exec(clause);
	assert.notEqual(found, null, `no viewItem operand in the clause: ${clause}`);
	return new RegExp(found?.[1] ?? "").test(viewItem);
}

test("Comment on Column is offered on every column row, including the one the status join missed", () => {
	const values = everyColumnContextValue();
	assert.equal(
		values.length,
		5,
		`columnActionsFor produced ${String(values.length)} distinct values, and this run is written against five: ${values.join(", ")}`,
	);
	assert.ok(
		values.includes("dinah.column"),
		`the bare column value is absent from the population: ${values.join(", ")}`,
	);
	const clause = commentOnColumnClause();
	for (const value of values) {
		assert.ok(
			admits(clause, value),
			`the clause ${clause} refuses the column row value ${value}`,
		);
	}
});

test("Comment on Column is offered on no row that is not a column", () => {
	const clause = commentOnColumnClause();
	for (const value of [
		CONTEXT_WORKBENCH_ROOT,
		CONTEXT_CARD_ACTIVE,
		CONTEXT_STATE_GROUP,
		"dinah.attachment",
		"dinah.comment",
	]) {
		assert.equal(
			admits(clause, value),
			false,
			`the clause ${clause} admits ${value}, which is not a column row`,
		);
	}
});

function rootRow(): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: FOLDER,
		folderName: "work",
		description: "",
		sole: true,
		data: {
			path: BENCH,
			title: "Work",
			columns: new Map(),
			cards: new Map(),
			holding: [],
		},
	};
}

function columnElement(view: ColumnView | undefined, value: string): TreeElement {
	return {
		kind: "column",
		row: rootRow(),
		node: { kind: "column", axis: "column", value, title: "Spec", count: 2 },
		view,
	};
}

const silentHost: ColumnCommandHost = {
	t: ENGLISH,
	showError: () => undefined,
	showInfo: () => undefined,
	showWarning: async () => undefined,
	appendLines: () => undefined,
	revealOutput: () => undefined,
	openDocument: async () => undefined,
	log: () => undefined,
};

const silentSpawner: Spawner = async (): Promise<SpawnOutcome> => ({
	code: 0,
	stdout: "",
	stderr: "",
});

test("the draft Comment on Column opens posts through the comment verb", async () => {
	// The row carries no ColumnView, which is the case the manifest clause
	// exists to admit, and the whole act still resolves: a build resolving
	// through the creation commands' own column context answers undefined
	// here and reports a skip instead of writing a draft.
	const drafts: { root: string; folder: string; ref: string }[] = [];
	const entries: { argv: string[]; target: string }[] = [];
	const draftHost = {
		t: ENGLISH,
		storageRoot: "C:/storage",
		ensureDirectory: async () => undefined,
		readDraft: async () => undefined,
		writeDraft: async (path: string, _text: string) => {
			void path;
		},
		readIndex: () => ({}),
		writeIndex: async (index: Record<string, { argv: string[]; target: string }>) => {
			for (const entry of Object.values(index)) {
				entries.push({ argv: [...entry.argv], target: entry.target });
			}
		},
		openDocument: async () => undefined,
		showError: () => undefined,
		showInfo: (message: string) => {
			void message;
		},
	};
	const wiring = {
		exe: "dinah",
		spawner: silentSpawner,
		columnHost: silentHost,
		t: ENGLISH,
		draftHost,
	};

	const report = await invokeCommentOnColumn(
		[columnElement(undefined, COLUMN_SLUG)],
		wiring as unknown as Parameters<typeof invokeCommentOnColumn>[1],
	);
	const skipped = report.entries.filter((entry) => entry.outcome.kind === "skipped");
	assert.equal(skipped.length, 0, "the column row was skipped rather than acted on");
	assert.equal(entries.length, 1, "the draft index gained no entry");
	assert.deepEqual(entries[0].argv, ["comment", COLUMN_SLUG, "-"]);
	assert.equal(entries[0].target, COLUMN_SLUG);
	void drafts;
});
