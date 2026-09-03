// activate() and deactivate().
//
// This is the only module that imports vscode. Everything it calls is a pure
// function over injected dependencies, which is what lets the unit tests reach
// the whole of the binary ladder, the compatibility gate and the refusal
// parser without a VS Code host.

import * as vscode from "vscode";

import type { DinahApi, WorkbenchResolution } from "./api";
import { resolveBinary } from "./binary";
import type { CommandContext, CommandHost, PickItem } from "./cardCommands";
import {
	blockCard,
	claimCard,
	contextFor,
	copyCardRef,
	moveCard,
	openAttachment,
	openCard,
	releaseCard,
	unblockCard,
} from "./cardCommands";
import type { CheckpointEntry, Watcher } from "./changes";
import { CheckpointLoop, systemClock } from "./changes";
import { runDinah } from "./cli";
import {
	attachFile,
	contextForAttach,
	contextForColumn,
	newCard,
} from "./creationCommands";
import { PAIRED_RELEASE } from "./generated/pairing";
import {
	COMMAND_ATTACH_FILE,
	COMMAND_BLOCK,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_CLAIM,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_MOVE,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_CARD,
	COMMAND_REFRESH,
	COMMAND_RELEASE,
	COMMAND_UNBLOCK,
	SETTING_PATH,
	SETTING_POLL_INTERVAL,
	SETTING_WATCH_FILES,
	SETTING_WORKBENCH,
	VIEW_ID,
} from "./identity";
import {
	ensureExecutable,
	joinPath,
	listDirectory,
	nodeSpawner,
} from "./spawn";
import { composeContextKeys, composeStatus } from "./status";
import type { TreeElement, TreeItemSpec } from "./tree";
import { DinahTreeProvider } from "./tree";
import { classifyVersion, describeVersion } from "./version";
import { NO_WORKBENCH_FOUND, resolveWorkbench } from "./workbench";
import type {
	WorkbenchCommandContext,
	WorkbenchCommandHost,
} from "./workbenchCommands";
import {
	checkWorkbench,
	contextForWorkbench,
	copyWorkbenchPath,
} from "./workbenchCommands";

let statusItem: vscode.StatusBarItem | undefined;
let output: vscode.OutputChannel | undefined;
let loop: CheckpointLoop | undefined;
let treeView: vscode.TreeView<TreeElement> | undefined;

/** Reads a settings value as a string, treating an unset value as empty. */
function setting(key: string, scope?: vscode.Uri): string {
	const dot = key.lastIndexOf(".");
	const section = key.slice(0, dot);
	const name = key.slice(dot + 1);
	return vscode.workspace.getConfiguration(section, scope).get<string>(name) ?? "";
}

/** Reads a settings value of any type, with the caller's own fallback. */
function settingOf<T>(key: string, fallback: T, scope?: vscode.Uri): T {
	const dot = key.lastIndexOf(".");
	const section = key.slice(0, dot);
	const name = key.slice(dot + 1);
	return vscode.workspace.getConfiguration(section, scope).get<T>(name) ?? fallback;
}

/**
 * Turns the provider's plain row description into a real TreeItem.
 *
 * This function is the whole of the boundary tree.ts keeps: everything above
 * it is data a unit test can assert on, and everything it touches is a vscode
 * value that only exists inside an extension host.
 */
function toTreeItem(spec: TreeItemSpec): vscode.TreeItem {
	const collapsible =
		spec.collapsibleState === "expanded"
			? vscode.TreeItemCollapsibleState.Expanded
			: spec.collapsibleState === "collapsed"
				? vscode.TreeItemCollapsibleState.Collapsed
				: vscode.TreeItemCollapsibleState.None;
	const item = new vscode.TreeItem(spec.label, collapsible);
	if (spec.description !== undefined && spec.description !== "") {
		item.description = spec.description;
	}
	if (spec.tooltip !== undefined && spec.tooltip !== "") {
		item.tooltip = spec.tooltip;
	}
	// An absent contextValue is left absent rather than set to an empty
	// string, because a `when` clause comparing against "" would match it.
	if (spec.contextValue !== undefined) {
		item.contextValue = spec.contextValue;
	}
	if (spec.icon !== undefined) {
		item.iconPath =
			spec.icon.color === undefined
				? new vscode.ThemeIcon(spec.icon.id)
				: new vscode.ThemeIcon(spec.icon.id, new vscode.ThemeColor(spec.icon.color));
	}
	if (spec.command !== undefined) {
		item.command = {
			command: spec.command.command,
			title: spec.command.title,
			arguments: [...spec.command.args],
		};
	}
	return item;
}

/** The window calls the flow commands make, bound to the real window. */
function commandHost(
	channel: vscode.OutputChannel,
	checkpoint: (folder: string) => Promise<void>,
): CommandHost {
	return {
		showError: (message) => {
			void vscode.window.showErrorMessage(message);
		},
		showInfo: (message) => {
			void vscode.window.showInformationMessage(message);
		},
		copyToClipboard: async (text) => vscode.env.clipboard.writeText(text),
		pick: async (items, placeholder) => {
			const chosen = await vscode.window.showQuickPick(
				items.map((item) => ({ ...item })),
				{ placeHolder: placeholder },
			);
			return chosen as PickItem | undefined;
		},
		input: async (prompt) => vscode.window.showInputBox({ prompt }),
		openDocument: async (path) => {
			const document = await vscode.workspace.openTextDocument(
				vscode.Uri.file(path),
			);
			await vscode.window.showTextDocument(document);
		},
		// vscode.open rather than openTextDocument, because an attachment is
		// arbitrary bytes and the editor decides how to render it (dinah-335
		// Decision 3).
		openFile: async (path) => {
			await vscode.commands.executeCommand("vscode.open", vscode.Uri.file(path));
		},
		// One file, and a file rather than a folder, because `dinah attach`
		// takes one payload path. The label names the act rather than the
		// dialog's default "Open", which would say the wrong verb.
		pickFile: async () => {
			const picked = await vscode.window.showOpenDialog({
				canSelectMany: false,
				canSelectFiles: true,
				canSelectFolders: false,
				openLabel: "Attach",
			});
			return picked?.[0]?.fsPath;
		},
		checkpoint,
		log: (line) => channel.appendLine(line),
	};
}

/** The window calls the workbench-row commands make, bound to the real window. */
function workbenchCommandHost(
	channel: vscode.OutputChannel,
): WorkbenchCommandHost {
	return {
		showInfo: (message) => {
			void vscode.window.showInformationMessage(message);
		},
		showWarning: async (message, actions) =>
			vscode.window.showWarningMessage(message, ...actions),
		appendLines: (lines) => {
			for (const line of lines) {
				channel.appendLine(line);
			}
		},
		revealOutput: () => channel.show(),
		copyToClipboard: async (text) => vscode.env.clipboard.writeText(text),
		log: (line) => channel.appendLine(line),
	};
}

export async function activate(
	context: vscode.ExtensionContext,
): Promise<DinahApi> {
	output = vscode.window.createOutputChannel("Dinah");
	context.subscriptions.push(output);

	const carriedDir = joinPath(context.extensionUri.fsPath, "bin");
	const binary = await resolveBinary({
		setting: setting(SETTING_PATH),
		carriedDir,
		listCarried: listDirectory,
		join: joinPath,
		ensureExecutable,
		probe: async (exe) =>
			classifyVersion(await runDinah(nodeSpawner, exe, ["version"])),
	});

	const workbenches = new Map<string, WorkbenchResolution>();
	if (binary.state === "ok") {
		const caseInsensitive = process.platform === "win32";
		for (const folder of vscode.workspace.workspaceFolders ?? []) {
			const resolution = await resolveWorkbench(
				nodeSpawner,
				binary.path,
				folder.uri.fsPath,
				setting(SETTING_WORKBENCH, folder.uri),
				caseInsensitive,
			);
			workbenches.set(folder.uri.fsPath, resolution);
		}
	}

	const first = vscode.workspace.workspaceFolders?.[0];
	const primary = first ? workbenches.get(first.uri.fsPath) : undefined;
	const view = composeStatus(binary, primary, PAIRED_RELEASE);
	const keys = composeContextKeys(binary, primary);

	await vscode.commands.executeCommand("setContext", "dinah.binary", keys.binary);
	await vscode.commands.executeCommand(
		"setContext",
		"dinah.workbench",
		keys.workbench,
	);

	statusItem = vscode.window.createStatusBarItem(
		"dinah.status",
		vscode.StatusBarAlignment.Left,
	);
	statusItem.name = "Dinah";
	statusItem.text = view.text;
	statusItem.tooltip = view.tooltip;
	statusItem.command = {
		title: "Open the Dinah view",
		command: `${VIEW_ID}.focus`,
	};
	context.subscriptions.push(statusItem);
	if (view.hidden) {
		statusItem.hide();
	} else {
		statusItem.show();
	}

	// Reported once, into a channel a reader opens on purpose. A window whose
	// binary is missing or skewed says so in the status bar and in this
	// channel, and never as a notification that returns every time the window
	// is reopened.
	output.appendLine(view.tooltip);

	const channel = output;
	const emitter = new vscode.EventEmitter<TreeElement | undefined>();
	context.subscriptions.push(emitter);

	const provider = new DinahTreeProvider({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		log: (line) => channel.appendLine(line),
		caseInsensitive: process.platform === "win32",
		deadEndSentence: (refusal) =>
			refusal === NO_WORKBENCH_FOUND
				? "No workbench was found from this folder. Run `dinah init` in a terminal to create one, or set `dinah.workbench` to the directory of one you already have."
				: `dinah refused: ${refusal}`,
	});

	if (binary.state === "ok") {
		await provider.load(
			(vscode.workspace.workspaceFolders ?? []).map((folder) => ({
				folder: folder.uri.fsPath,
				name: folder.name,
				resolution:
					workbenches.get(folder.uri.fsPath) ??
					({ state: "refused", refusal: NO_WORKBENCH_FOUND } as WorkbenchResolution),
			})),
		);
	}

	// createTreeView rather than registerTreeDataProvider, because the
	// checkpoint loop suspends while the view is hidden and TreeView.visible is
	// the only way to know that it is.
	treeView = vscode.window.createTreeView<TreeElement>(VIEW_ID, {
		treeDataProvider: {
			onDidChangeTreeData: emitter.event,
			getTreeItem: (element) => toTreeItem(provider.getTreeItem(element)),
			getChildren: (element) => provider.getChildren(element),
		},
	});
	context.subscriptions.push(treeView);

	loop = new CheckpointLoop({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		clock: systemClock,
		log: (line) => channel.appendLine(line),
		refresh: (folder) => provider.refresh(folder),
		fire: () => emitter.fire(undefined),
		pollIntervalSeconds: settingOf<number>(SETTING_POLL_INTERVAL, 10),
		watchFiles: settingOf<boolean>(SETTING_WATCH_FILES, true),
		createWatcher: (entry, onEvent): Watcher => {
			const watcher = vscode.workspace.createFileSystemWatcher(
				new vscode.RelativePattern(vscode.Uri.file(entry.path), "**/*.md"),
			);
			watcher.onDidChange(onEvent);
			watcher.onDidCreate(onEvent);
			watcher.onDidDelete(onEvent);
			return { dispose: () => watcher.dispose() };
		},
	});
	const checkpoints: CheckpointEntry[] = provider.states.map((state) => ({
		folder: state.folder,
		// A folder that resolved to one workbench is asked about that
		// workbench; a folder whose workbenches were found by walking down into
		// it is asked about the folder, and one merged token covers them all.
		scope: state.mode === "forest" ? "root" : "workbench",
		path: state.mode === "forest" ? state.folder : (state.root ?? state.folder),
	}));
	if (binary.state === "ok" && checkpoints.length > 0) {
		loop.start(checkpoints);
	}
	const checkpointing = loop;
	treeView.onDidChangeVisibility((event) => {
		checkpointing.setVisible(event.visible);
	});
	context.subscriptions.push({ dispose: () => checkpointing.stop() });

	const host = commandHost(channel, (folder) => checkpointing.checkNow(folder));
	const flowCommands: [string, (c: CommandContext) => Promise<unknown>][] = [
		[COMMAND_CLAIM, claimCard],
		[COMMAND_MOVE, moveCard],
		[COMMAND_RELEASE, releaseCard],
		[COMMAND_BLOCK, blockCard],
		[COMMAND_UNBLOCK, unblockCard],
		[COMMAND_COPY_CARD_REF, copyCardRef],
		[COMMAND_OPEN_CARD, openCard],
	];
	for (const [id, run] of flowCommands) {
		context.subscriptions.push(
			vscode.commands.registerCommand(
				id,
				async (element: TreeElement | undefined) => {
					const target = contextFor(
						element,
						binary.state === "ok" ? binary.path : "",
						host,
					);
					if (target === undefined) {
						channel.appendLine(`${id} was invoked on a row that names no card`);
						return;
					}
					await run(target);
				},
			),
		);
	}
	// The workbench-row commands get a loop of their own rather than joining the
	// one above. The two families take different contexts and different hosts,
	// and a single loop over both would have to widen each of those to a union
	// that neither handler can use without narrowing it again.
	const workbenchHost = workbenchCommandHost(channel);
	const workbenchCommands: [
		string,
		(c: WorkbenchCommandContext) => Promise<unknown>,
	][] = [
		[COMMAND_CHECK_WORKBENCH, checkWorkbench],
		[COMMAND_COPY_WORKBENCH_PATH, copyWorkbenchPath],
	];
	for (const [id, run] of workbenchCommands) {
		context.subscriptions.push(
			vscode.commands.registerCommand(
				id,
				async (element: TreeElement | undefined) => {
					const target = contextForWorkbench(
						element,
						binary.state === "ok" ? binary.path : "",
						// describeVersion rather than a second spelling of the
						// same line: the status tooltip and the demotion
						// diagnostic already describe a binary this way, and
						// this is display, the only thing version.ts's header
						// allows the release tag inside it to be used for.
						binary.state === "ok" ? describeVersion(binary.version) : "",
						workbenchHost,
						nodeSpawner,
					);
					if (target === undefined) {
						channel.appendLine(
							`${id} was invoked on a row that names no workbench`,
						);
						return;
					}
					await run(target);
				},
			),
		);
	}
	// An attachment row is registered on its own rather than through the loop
	// above, because it is not a card and carries no CommandContext: the path
	// it was drawn from is the whole of what opening it needs.
	context.subscriptions.push(
		vscode.commands.registerCommand(
			COMMAND_OPEN_ATTACHMENT,
			async (element: TreeElement | undefined) => {
				await openAttachment(element, host, (line) => channel.appendLine(line));
			},
		),
	);
	// The two creation commands are registered on their own for the reason the
	// loops above are separate from each other: New Card takes a column context
	// and Attach File takes an entity context, and neither fits CommandContext,
	// which names a card. Both need the checkpoint the flow host carries, so
	// both take that host rather than the workbench one.
	context.subscriptions.push(
		vscode.commands.registerCommand(
			COMMAND_NEW_CARD,
			async (element: TreeElement | undefined) => {
				const target = contextForColumn(
					element,
					binary.state === "ok" ? binary.path : "",
					host,
					nodeSpawner,
				);
				if (target === undefined) {
					channel.appendLine(
						`${COMMAND_NEW_CARD} was invoked on a row that names no column`,
					);
					return;
				}
				await newCard(target);
			},
		),
	);
	context.subscriptions.push(
		vscode.commands.registerCommand(
			COMMAND_ATTACH_FILE,
			async (element: TreeElement | undefined) => {
				const target = contextForAttach(
					element,
					binary.state === "ok" ? binary.path : "",
					host,
					nodeSpawner,
				);
				if (target === undefined) {
					channel.appendLine(
						`${COMMAND_ATTACH_FILE} was invoked on a row that names no attachable entity`,
					);
					return;
				}
				await attachFile(target, host.pickFile);
			},
		),
	);
	context.subscriptions.push(
		vscode.commands.registerCommand(COMMAND_REFRESH, async () => {
			await checkpointing.refreshNow();
			emitter.fire(undefined);
		}),
	);

	return {
		binary,
		workbenches,
		tree: {
			getChildren: (element) => provider.getChildren(element as TreeElement),
			getTreeItem: (element) => provider.getTreeItem(element as TreeElement),
		},
		statusTooltip: view.tooltip,
		statusText: view.text,
		pairedRelease: PAIRED_RELEASE,
	};
}

export function deactivate(): void {
	statusItem?.dispose();
	statusItem = undefined;
	loop?.stop();
	loop = undefined;
	treeView = undefined;
	output = undefined;
}
