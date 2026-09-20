// The tree rows, hosts and wiring dinah-490's row-command tests drive.
//
// These fixtures sit in test/support rather than beside one test file because
// three unit files drive the same shapes: the bulk layer's own tests, the
// per-command tests, and the drag round trip. A second copy of a card row
// would be a second thing to keep true.
//
// Nothing here imports vscode, and nothing here starts a process.

import type { CommandHost, PickItem } from "../../src/cardCommands";
import type { CliOutcome, SpawnOptions, SpawnOutcome, Spawner } from "../../src/cli";
import type { ColumnCommandHost } from "../../src/columnCommands";
import type { Wiring } from "../../src/commandTable";
import type { CommentBodyHost, OpenComments } from "../../src/commentBody";
import { ENGLISH } from "../../src/l10n";
import type { RootRow, TreeElement, WorkbenchData } from "../../src/tree";
import type { CatalogBuild } from "../../src/verbCatalog";
import type {
	AttachmentView,
	ColumnView,
	CommentView,
	ItemView,
	TreeNode,
} from "../../src/wire";
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
	return {
		kind: "attachment",
		row: rootRow(root),
		root,
		owner: "tr-1",
		node: { kind: "attachment", ref: view.ref, title: filename, count: 0 },
		view,
	};
}

/** One checklist item, as `show <card> --fields card,checklist` reports it. */
export function itemView(overrides: Partial<ItemView> = {}): ItemView {
	return {
		id: "b00000000001",
		ordinal: 1,
		ref: "tr-1/questions/1",
		kind: "open_question",
		state: "pending",
		text: "Which vendor do we cite for the SLA numbers?",
		...overrides,
	};
}

/**
 * An item row, whole, hanging from the card and workbench given.
 *
 * The node is the `contents` node the row's structure comes from, and its
 * count is what decides the row's arrow. It is set from the view's own
 * comment_count only as a convenience for a caller that names neither, and a
 * test whose subject is which of the two the code reads passes both.
 */
export function itemRow(
	overrides: Partial<ItemView> = {},
	isOperator = false,
	root = ROOT,
	data?: WorkbenchData,
	count?: number,
): TreeElement {
	const row = rootRow(root);
	const view = itemView(overrides);
	return {
		kind: "item",
		row: data === undefined ? row : { ...row, data },
		root,
		card: "tr-1",
		node: {
			kind: "item",
			ref: view.ref,
			title: view.text,
			count: count ?? view.comment_count ?? 0,
		},
		view,
		isOperator,
	};
}

/** One comment, as a structured read reports it. */
export function commentView(
	overrides: Partial<CommentView> = {},
): CommentView {
	return {
		id: "c00000000001",
		ref: "tr-1/comments/1",
		ts: "2026-09-15T09:00:00Z",
		author: "claude",
		body: "## WHAT SHIPPED\n\nThe row family, and the calls behind it.",
		...overrides,
	};
}

/** A comment row, hanging from the holder given. */
export function commentRow(
	overrides: Partial<CommentView> = {},
	count = 0,
	holder = "tr-1",
	root = ROOT,
	joined = true,
): TreeElement {
	const view = commentView(overrides);
	return {
		kind: "comment",
		row: rootRow(root),
		root,
		holder,
		node: { kind: "comment", ref: view.ref, title: view.body, count },
		...(joined ? { view } : {}),
	};
}

/** A collection row, as an entity's own expansion yields one. */
export function collectionRow(
	memberKind = "item",
	members: readonly TreeNode[] = [],
	holder = "tr-1",
	holderKind: TreeElement["kind"] = "card",
	root = ROOT,
): TreeElement {
	return {
		kind: "collection",
		row: rootRow(root),
		root,
		holder,
		holderKind,
		memberKind,
		members,
	};
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
	/**
	 * The options object each call was handed, in the same order as calls.
	 *
	 * It is the object itself rather than a copy of the fields this fixture
	 * knows about, because dinah-506/criteria/30 asks whether the stdin key is
	 * present at all, by Object.hasOwn, and a copy would answer for the copy.
	 */
	readonly options: SpawnOptions[];
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
	const options: SpawnOptions[] = [];
	const state: SpawnerLog = {
		calls,
		options,
		queue,
		fallback,
		spawner: async (_exe, argv, spawnOptions) => {
			calls.push([...argv]);
			options.push(spawnOptions);
			return queue.shift() ?? state.fallback;
		},
	};
	return state;
}

/** Everything a fake CommentBodyHost was asked to do, in call order. */
export interface CommentLog {
	/** Every call, named, in the order they were made. */
	readonly order: string[];
	readonly opened: string[];
	readonly errors: string[];
	readonly infos: string[];
	readonly lines: string[];
	readonly checkpoints: string[];
	readonly logged: string[];
}

/** A fresh, empty comment log. */
export function emptyCommentLog(): CommentLog {
	return {
		order: [],
		opened: [],
		errors: [],
		infos: [],
		lines: [],
		checkpoints: [],
		logged: [],
	};
}

/**
 * A CommentBodyHost that records every call rather than making one.
 *
 * There is no disk here and no index. The comment is an entity in the
 * workbench from the moment it is minted, so what a fixture has to record is
 * which file was opened and what was said about it, and the bytes are the
 * store's rather than this host's.
 */
export function commentHost(log: CommentLog): CommentBodyHost {
	return {
		t: ENGLISH,
		// An anchor carrying a digest, so a session opened over this host
		// remembers one and the save it drives takes the compare-and-swap
		// path rather than quietly falling to the body comparison.
		readFile: async (path) => {
			log.order.push(`readFile ${path}`);
			return [
				"---",
				"ts: 2026-08-01T09:00:00Z",
				"author: ana",
				"ordinal: 1",
				"digest: abc123",
				"---",
				"",
			].join("\n");
		},
		openDocument: async (path) => {
			log.order.push(`openDocument ${path}`);
			log.opened.push(path);
		},
		showError: (message) => {
			log.errors.push(message);
		},
		showInfo: (message) => {
			log.infos.push(message);
		},
		appendLines: (lines) => {
			log.lines.push(...lines);
		},
		checkpoint: async (folder) => {
			log.order.push(`checkpoint ${folder}`);
			log.checkpoints.push(folder);
		},
		log: (line) => {
			log.logged.push(line);
		},
	};
}

/**
 * A catalogue answering one file_item tool whose kind enum carries the members
 * given.
 *
 * The members are a parameter rather than the three dinah publishes today,
 * because dinah-506/criteria/14 drives the form over a fabricated fourth kind
 * and a fixture hard-coding three could not reach it.
 */
export function catalogueWithKinds(
	values: readonly string[] = [
		"acceptance_criterion",
		"open_question",
		"decision",
	],
): CatalogBuild {
	return {
		kind: "ok",
		verbs: [
			{
				name: "file_item",
				args: [
					{ name: "card", required: true, prompt: { kind: "text" } },
					{
						name: "kind",
						required: true,
						prompt: { kind: "choice", values: [...values] },
					},
					{ name: "text", required: true, prompt: { kind: "text" } },
				],
			},
		],
		excluded: [],
		unnamed: 0,
	};
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
	comments: CommentLog = emptyCommentLog(),
	catalogue: CatalogBuild = catalogueWithKinds(),
	openComments: OpenComments = new Map(),
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
		commentHost: commentHost(comments),
		openComments,
		verbCatalog: async () => catalogue,
	};
}
