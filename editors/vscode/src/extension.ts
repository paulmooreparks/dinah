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
// Two modules export a function named contextForColumn. dinah-331 and
// dinah-332 each gave the column row an act, and each act composes its own
// context: the creation one declines a row whose ColumnView the status join
// missed, and the editing one falls back to the node's own ref and answers
// anyway. Both are aliased here so that each registration below names the act
// it serves rather than the row the act stands on. dinah-375's Pull is a
// fourth command on the column row and needs no alias, because it declines
// more rows than either of those two and so was given its own name at its own
// module.
import type {
	ColumnCommandContext,
	ColumnCommandHost,
} from "./columnCommands";
import {
	contextForColumn as contextForInstructions,
	editColumnInstructions,
} from "./columnCommands";
import {
	ATTACH_DIALOG_OPTIONS,
	attachFile,
	contextForAttach,
	contextForColumn as contextForNewCard,
	newCard,
	pickedFilePath,
} from "./creationCommands";
import { PAIRED_RELEASE } from "./generated/pairing";
import {
	COMMAND_ATTACH_FILE,
	COMMAND_BLOCK,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_CLAIM,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_MOVE,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_CARD,
	COMMAND_PULL,
	COMMAND_REFRESH,
	COMMAND_RELEASE,
	COMMAND_UNBLOCK,
	SETTING_PATH,
	SETTING_POLL_INTERVAL,
	SETTING_WATCH_FILES,
	SETTING_WORKBENCH,
	TREE_COMMANDS,
	VIEW_ID,
} from "./identity";
import { contextForPull, pullFromColumn } from "./pullCommands";
import { assertCommandsFullyRegistered } from "./registrationGuard";
import { joinPath, nodeSpawner } from "./spawn";
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
	editWorkbenchDefinition,
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
		// The options and the read of what came back both live in
		// creationCommands.ts, where a unit test drives them. What is left
		// here is the call itself, which no test in this layer can reach
		// (dinah-331 AC-12).
		pickFile: async () =>
			pickedFilePath(await vscode.window.showOpenDialog(ATTACH_DIALOG_OPTIONS)),
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
		openDocument: async (path) => {
			const document = await vscode.workspace.openTextDocument(
				vscode.Uri.file(path),
			);
			await vscode.window.showTextDocument(document);
		},
		log: (line) => channel.appendLine(line),
	};
}

/** The window calls the column-row command makes, bound to the real window. */
function columnCommandHost(channel: vscode.OutputChannel): ColumnCommandHost {
	return {
		showWarning: async (message, actions) =>
			vscode.window.showWarningMessage(message, ...actions),
		appendLines: (lines) => {
			for (const line of lines) {
				channel.appendLine(line);
			}
		},
		revealOutput: () => channel.show(),
		openDocument: async (path) => {
			const document = await vscode.workspace.openTextDocument(
				vscode.Uri.file(path),
			);
			await vscode.window.showTextDocument(document);
		},
		log: (line) => channel.appendLine(line),
	};
}

export async function activate(
	context: vscode.ExtensionContext,
): Promise<DinahApi> {
	output = vscode.window.createOutputChannel("Dinah");
	context.subscriptions.push(output);

	const binary = await resolveBinary({
		setting: setting(SETTING_PATH),
		join: joinPath,
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

	// Every command this extension contributes is registered through this one
	// helper, so that registeredIds is a record of what activation actually
	// did rather than a second hand-maintained roster. The completeness check
	// below reads it, and a registration that went around the helper would be
	// invisible to that check (dinah-369).
	//
	// The id is recorded before the registration call rather than after it, and
	// swapping the two would buy nothing. Neither statement depends on the
	// other, so stopping the helper from registering leaves the push running in
	// either order and the check below still reads a full roster. That was
	// watched rather than assumed: a helper that records every id and registers
	// nothing leaves the unit suite green at all 299 rows. Deleting the
	// registration line by itself does break the build, because it leaves the
	// handler parameter unread, but one further token restores the build with
	// the commands still unregistered, so the compiler is not standing in for
	// the check that is missing here.
	//
	// Recording the id from the call's own result is not available either,
	// because registerCommand returns a Disposable and a Disposable names no
	// command. What would actually close the gap is asking the editor through
	// vscode.commands.getCommands, which resolves to the ids the editor knows
	// about. That is a different guard rather than a reordering of this one,
	// because it is asynchronous, it answers only inside a running editor, and
	// it puts the comparison back inside the module no unit test can import.
	// The order therefore stays, and registrationGuard.ts's header says plainly
	// that the check reads this list rather than the registration itself.
	const registeredIds: string[] = [];
	function register(
		id: string,
		handler: (element: TreeElement | undefined) => Promise<unknown>,
	): void {
		registeredIds.push(id);
		context.subscriptions.push(vscode.commands.registerCommand(id, handler));
	}

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
		register(id, async (element: TreeElement | undefined) => {
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
		});
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
		[COMMAND_EDIT_WORKBENCH_DEFINITION, editWorkbenchDefinition],
	];
	for (const [id, run] of workbenchCommands) {
		register(id, async (element: TreeElement | undefined) => {
			const target = contextForWorkbench(
				element,
				binary.state === "ok" ? binary.path : "",
				// describeVersion rather than a second spelling of the same
				// line: the status tooltip and the demotion diagnostic already
				// describe a binary this way, and this is display, the only
				// thing version.ts's header allows the release tag inside it to
				// be used for.
				binary.state === "ok" ? describeVersion(binary.version) : "",
				workbenchHost,
				nodeSpawner,
			);
			if (target === undefined) {
				channel.appendLine(`${id} was invoked on a row that names no workbench`);
				return;
			}
			await run(target);
		});
	}
	// Four commands stand on the column row, and each is registered on its own
	// rather than gathered into a third loop: they agree on nothing a loop
	// could hold, since Edit Instructions, Pull, New Card and Attach File each
	// compose a different context and the first two do not even share a host.
	// Edit Instructions is the one that takes the column host, which opens a
	// file and carries no checkpoint.
	const columnHost = columnCommandHost(channel);
	register(
		COMMAND_EDIT_COLUMN_INSTRUCTIONS,
		async (element: TreeElement | undefined) => {
			const target: ColumnCommandContext | undefined = contextForInstructions(
				element,
				binary.state === "ok" ? binary.path : "",
				columnHost,
				nodeSpawner,
			);
			if (target === undefined) {
				channel.appendLine(
					`${COMMAND_EDIT_COLUMN_INSTRUCTIONS} was invoked on a row that names no column`,
				);
				return;
			}
			await editColumnInstructions(target);
		},
	);
	// Pull takes the flow host rather than the column one, because a pull
	// mutates the board and columnHost carries no checkpoint (dinah-375).
	register(COMMAND_PULL, async (element: TreeElement | undefined) => {
		const target = contextForPull(
			element,
			binary.state === "ok" ? binary.path : "",
			host,
			nodeSpawner,
		);
		if (target === undefined) {
			channel.appendLine(
				`${COMMAND_PULL} was invoked on a row that cannot be pulled from`,
			);
			return;
		}
		await pullFromColumn(target);
	});
	// An attachment row is registered on its own rather than through the loop
	// above, because it is not a card and carries no CommandContext: the path
	// it was drawn from is the whole of what opening it needs.
	register(COMMAND_OPEN_ATTACHMENT, async (element: TreeElement | undefined) => {
		await openAttachment(element, host, (line) => channel.appendLine(line));
	});
	// The two creation commands are registered on their own for the reason the
	// loops above are separate from each other: New Card takes a column context
	// and Attach File takes an entity context, and neither fits CommandContext,
	// which names a card. Both need the checkpoint the flow host carries, so
	// both take that host rather than the workbench one.
	register(COMMAND_NEW_CARD, async (element: TreeElement | undefined) => {
		const target = contextForNewCard(
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
	});
	register(COMMAND_ATTACH_FILE, async (element: TreeElement | undefined) => {
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
	});
	register(COMMAND_REFRESH, async () => {
		await checkpointing.refreshNow();
		emitter.fire(undefined);
	});

	// Every registration above has run by now, so this is where a dropped one
	// becomes visible. Activation fails with a message naming the id rather
	// than shipping a command as a menu item that reports itself missing on
	// the first click (dinah-369).
	assertCommandsFullyRegistered(TREE_COMMANDS, registeredIds);

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
