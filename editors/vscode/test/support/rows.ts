// The tree rows, hosts and wiring dinah-490's row-command tests drive.
//
// These fixtures sit in test/support rather than beside one test file because
// three unit files drive the same shapes: the bulk layer's own tests, the
// per-command tests, and the drag round trip. A second copy of a card row
// would be a second thing to keep true.
//
// Nothing here imports vscode, and nothing here starts a process.

import type { CommandHost, PickItem } from "../../src/cardCommands";
import type { CliOutcome, SpawnOutcome, Spawner } from "../../src/cli";
import type { ColumnCommandHost } from "../../src/columnCommands";
import type { Wiring } from "../../src/commandTable";
import { ENGLISH } from "../../src/l10n";
import type { RootRow, TreeElement } from "../../src/tree";
import type { AttachmentView, ColumnView } from "../../src/wire";
import type { WorkbenchCommandHost } from "../../src/workbenchCommands";

export const ROOT = "C:/work/bench";
export const OTHER_ROOT = "C:/work/other";
export const FOLDER = "C:/work";
export const EXE = "C:/tools/dinah.exe";

/** A workbench row, resolved, for the folder and root given. */
export function rootRow(root = ROOT, folder = FOLDER): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder,
		folderName: "work",
		description: "",
		sole: true,
		data: {
			path: root,
			title: "Bench",
			columns: new Map(),
			cards: new Map(),
			holding: [],
		},
	};
}

/** One column of the workbench, drawn as the status join reports it. */
export function columnView(overrides: Partial<ColumnView> = {}): ColumnView {
	return {
		id: "c-doing",
		slug: "doing",
		title: "Doing",
		kind: "flow",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: false,
		count: 1,
		...overrides,
	};
}

/** A card row, whole, standing in the column and workbench given. */
export function cardRow(
	ref: string,
	root = ROOT,
	column: ColumnView = columnView(),
	folder = FOLDER,
): TreeElement {
	return {
		kind: "card",
		row: rootRow(root, folder),
		node: { kind: "card", ref, count: 1 },
		view: { id: ref.replace(/\W/g, ""), ref, title: `Card ${ref}` },
		column,
	};
}

/** A column row, whole, with a column standing after it in the flow. */
export function columnRowFor(
	id: string,
	root = ROOT,
	overrides: Partial<ColumnView> = {},
): TreeElement {
	return {
		kind: "column",
		row: rootRow(root),
		node: { kind: "column", value: id, count: 0 },
		view: columnView({ id, slug: id, title: id, ...overrides }),
		nextColumnRef: "next",
	};
}

/** A workbench root row, which the workbench commands stand on. */
export function workbenchRow(root = ROOT, folder = FOLDER): TreeElement {
	return { kind: "root", row: rootRow(root, folder) };
}

/** An attachment row carrying a readable file. */
export function attachmentRowFor(
	id: string,
	filename = "spec.pdf",
	root = ROOT,
): TreeElement {
	const view: AttachmentView = {
		id,
		ordinal: 1,
		ref: `tr-1/attachments/1`,
		filename,
		provenance: "import",
		path: `${root}/attachments/${filename}`,
	};
	return { kind: "attachment", row: rootRow(root), root, owner: "tr-1", view };
}

/** A row no command on this card can act on. */
export function noteRow(text = "nothing here"): TreeElement {
	return { kind: "note", owner: rootRow(), text, tooltip: text };
}

/** Everything a fake host was asked to do, in call order. */
export interface HostLog {
	readonly errors: string[];
	readonly infos: string[];
	readonly warnings: string[];
	readonly lines: string[];
	readonly opened: string[];
	readonly files: string[];
	readonly served: string[];
	readonly copied: string[];
	readonly prompts: string[];
	readonly placeholders: string[];
	readonly confirmations: { message: string; label: string }[];
	readonly checkpoints: string[];
	readonly logged: string[];
	revealed: number;
	/** What the quick pick answers, and every item it was offered. */
	picked?: PickItem;
	readonly offered: PickItem[];
	/** What the input box answers. */
	typed?: string;
	/** What the confirmation answers. */
	confirmed: boolean;
}

/** A fresh, empty log. */
export function emptyLog(): HostLog {
	return {
		errors: [],
		infos: [],
		warnings: [],
		lines: [],
		opened: [],
		files: [],
		served: [],
		copied: [],
		prompts: [],
		placeholders: [],
		confirmations: [],
		checkpoints: [],
		logged: [],
		revealed: 0,
		offered: [],
		confirmed: false,
	};
}

/** A CommandHost that records every call rather than making one. */
export function cardHost(log: HostLog): CommandHost {
	return {
		t: ENGLISH,
		showError: (message) => {
			log.errors.push(message);
		},
		showInfo: (message) => {
			log.infos.push(message);
		},
		showWarning: async (message) => {
			log.warnings.push(message);
			return undefined;
		},
		appendLines: (lines) => {
			log.lines.push(...lines);
		},
		revealOutput: () => {
			log.revealed += 1;
		},
		copyToClipboard: async (text) => {
			log.copied.push(text);
		},
		pick: async (items, placeholder) => {
			log.offered.push(...items);
			log.placeholders.push(placeholder);
			return log.picked;
		},
		input: async (prompt) => {
			log.prompts.push(prompt);
			return log.typed;
		},
		openDocument: async (path) => {
			log.opened.push(path);
		},
		openFile: async (path) => {
			log.files.push(path);
		},
		pickFile: async () => undefined,
		openServedText: async (kind, _root, ref) => {
			log.served.push(`${kind}:${ref}`);
		},
		confirmDestructive: async (message, label) => {
			log.confirmations.push({ message, label });
			return log.confirmed;
		},
		checkpoint: async (folder) => {
			log.checkpoints.push(folder);
		},
		log: (line) => {
			log.logged.push(line);
		},
	};
}

/** A WorkbenchCommandHost recording into the same log. */
export function workbenchHost(log: HostLog): WorkbenchCommandHost {
	return {
		t: ENGLISH,
		showError: (message) => {
			log.errors.push(message);
		},
		showInfo: (message) => {
			log.infos.push(message);
		},
		showWarning: async (message) => {
			log.warnings.push(message);
			return undefined;
		},
		appendLines: (lines) => {
			log.lines.push(...lines);
		},
		revealOutput: () => {
			log.revealed += 1;
		},
		copyToClipboard: async (text) => {
			log.copied.push(text);
		},
		openDocument: async (path) => {
			log.opened.push(path);
		},
		log: (line) => {
			log.logged.push(line);
		},
	};
}

/** A ColumnCommandHost recording into the same log. */
export function columnHost(log: HostLog): ColumnCommandHost {
	return {
		t: ENGLISH,
		showError: (message) => {
			log.errors.push(message);
		},
		showInfo: (message) => {
			log.infos.push(message);
		},
		showWarning: async (message) => {
			log.warnings.push(message);
			return undefined;
		},
		appendLines: (lines) => {
			log.lines.push(...lines);
		},
		revealOutput: () => {
			log.revealed += 1;
		},
		openDocument: async (path) => {
			log.opened.push(path);
		},
		log: (line) => {
			log.logged.push(line);
		},
	};
}

/** A spawner that records every argv and answers from a queue or a default. */
export interface SpawnerLog {
	readonly spawner: Spawner;
	readonly calls: string[][];
	/** Answers consumed one per call; the default answers once they run out. */
	readonly queue: SpawnOutcome[];
	fallback: SpawnOutcome;
}

/** An ok answer carrying the payload given. */
export function ok(payload: unknown = {}): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

/** A refusal the extension reads as one. */
export function refused(refusal: string, detail?: string): SpawnOutcome {
	return { code: 2, stdout: JSON.stringify({ refusal, detail }), stderr: "" };
}

/** A recording spawner. */
export function spawnerLog(fallback: SpawnOutcome = ok()): SpawnerLog {
	const calls: string[][] = [];
	const queue: SpawnOutcome[] = [];
	const state: SpawnerLog = {
		calls,
		queue,
		fallback,
		spawner: async (_exe, argv) => {
			calls.push([...argv]);
			return queue.shift() ?? state.fallback;
		},
	};
	return state;
}

/** What a check answered, so the Problems panel can be updated from it. */
export interface CheckResults {
	readonly applied: { path: string; label: string; outcome: CliOutcome }[];
}

/** The wiring a row command's invoke is handed. */
export function wiringFor(
	log: HostLog,
	spawner: Spawner,
	results: CheckResults = { applied: [] },
): Wiring {
	return {
		exe: EXE,
		binaryLabel: "dinah 1.0.0",
		spawner,
		t: ENGLISH,
		cardHost: cardHost(log),
		workbenchHost: workbenchHost(log),
		columnHost: columnHost(log),
		applyCheckResult: async (path, label, outcome) => {
			results.applied.push({ path, label, outcome });
		},
	};
}
