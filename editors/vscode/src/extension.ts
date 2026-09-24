// activate() and deactivate().
//
// This is the only module that imports vscode. Everything it calls is a pure
// function over injected dependencies, which is what lets the unit tests reach
// the whole of the binary ladder, the compatibility gate and the refusal
// parser without a VS Code host.

import { basename, dirname } from "node:path";

import * as vscode from "vscode";
import { LanguageClient, Trace, TransportKind } from "vscode-languageclient/node";
import type { LanguageClientOptions, ServerOptions } from "vscode-languageclient/node";

import type { DinahApi, WorkbenchResolution } from "./api";
import { resolveBinary } from "./binary";
import type { CommandHost, PickItem } from "./cardCommands";
import { pinnedArgv, refusalMessage } from "./cardCommands";
import type { CheckpointEntry, Watcher } from "./changes";
import { applyDropVerdicts, dragRowsFrom, dropColumnFor, offerDrag } from "./dragAndDrop";
import { CheckpointLoop, systemClock } from "./changes";
import { runDinah, runDinahText } from "./cli";
import type { AnnotationsAnswer, AnnotationsChanged, Chip } from "./lsp";
import {
	ANNOTATIONS_CHANGED,
	ANNOTATIONS_REQUEST,
	LSP_SECTION,
	chipsFrom,
	suppressInlayHints,
} from "./lsp";
import { CountdownTicker, redrawAfterRefresh } from "./countdown";
import type { DiagnosticEntry, DiagnosticPlan, StatKind } from "./diagnostics";
import { CheckDiagnostics } from "./diagnostics";
// Two modules export a function named contextForColumn. dinah-331 and
// dinah-332 each gave the column row an act, and each act composes its own
// context: the creation one declines a row whose ColumnView the status join
// missed, and the editing one falls back to the node's own ref and answers
// anyway. Both are aliased here so that each registration below names the act
// it serves rather than the row the act stands on. dinah-375's Pull is a
// fourth command on the column row and needs no alias, because it declines
// more rows than either of those two and so was given its own name at its own
// module.
import type { ColumnCommandHost } from "./columnCommands";
import { ATTACH_DIALOG_OPTIONS, pickedFilePath } from "./creationCommands";
import { PAIRED_RELEASE } from "./generated/pairing";
import {
	COMMAND_REFRESH,
	COMMAND_REFRESH_VERB_CATALOG,
	COMMAND_RUN_VERB,
	DRAG_MIME_TYPE,
	COMMAND_OPEN_FIRST_SESSION_GUIDE,
	MCP_PROVIDER_ID,
	SERVED_TEXT_SCHEME,
	SETTING_PATH,
	SETTING_POLL_INTERVAL,
	SETTING_LSP_ENABLED,
	SETTING_LSP_TRACE,
	SETTING_REGISTER_MCP,
	SETTING_WATCH_FILES,
	SETTING_WORKBENCH,
	TREE_COMMANDS,
	VIEW_ID,
} from "./identity";
import type { McpServerPlan } from "./mcpServers";
import { mcpPlansDiffer, publishedMcpServers } from "./mcpServers";
import type { Wiring } from "./commandTable";
import { ROW_COMMAND_TABLE } from "./commandTable";
import { targetsFor } from "./selection";
import { assertCommandsFullyRegistered } from "./registrationGuard";
import {
	GUIDE_ROOT,
	GUIDE_TOPIC_FIRST_SESSION,
	KIND_GUIDE,
	KIND_HISTORY,
	KIND_INSTRUCTIONS,
	ServedTextRefreshLoop,
	parseServedTextUri,
	renderHistoryMarkdown,
	renderInstructionsMarkdown,
	servedTextUriParts,
} from "./servedText";
import type { RunVerbContext } from "./runVerbCommand";
import { refreshVerbCatalog, runVerbFromPalette } from "./runVerbCommand";
import { nodeSpawner } from "./spawn";
import { createLocalizer, resolveTag } from "./l10n";
import type { Localizer } from "./l10n";
import {
	NOTHING_HELD,
	composeContextKeys,
	composeStatus,
	staleAfterMs,
	summarizeHolding,
} from "./status";
import type { TreeElement, TreeItemSpec } from "./tree";
import { DinahTreeProvider, elementKey } from "./tree";
import type { CommentBodyHost, OpenComments } from "./commentBody";
import { forgetComment, saveCommentBody } from "./commentBody";
import { classifyVersion, describeVersion } from "./version";
import type { JournalEvent, PathAnswer, ServedAnswer } from "./wire";
import { VerbCatalog } from "./verbCatalog";
import { NO_WORKBENCH_FOUND, resolveWorkbench } from "./workbench";
import type { WorkbenchCommandHost } from "./workbenchCommands";

let statusItem: vscode.StatusBarItem | undefined;
let output: vscode.OutputChannel | undefined;
let loop: CheckpointLoop | undefined;
let ticker: CountdownTicker | undefined;
let treeView: vscode.TreeView<TreeElement> | undefined;
let servedText: ServedTextRefreshLoop | undefined;

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
 *
 * `extensionUri` is the extension context's own, threaded in so an icon
 * carrying `attention` can be resolved to the composed files dinah-599's
 * build step writes under `media/attention/`, which `TreeItem.iconPath`
 * takes as a documented light/dark `Uri` pair. An icon without `attention`
 * draws exactly as it always has, as a theme icon.
 */
function toTreeItem(spec: TreeItemSpec, extensionUri: vscode.Uri): vscode.TreeItem {
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
			spec.icon.attention === true
				? {
						light: vscode.Uri.joinPath(
							extensionUri,
							"media",
							"attention",
							`${spec.icon.id}-light.svg`,
						),
						dark: vscode.Uri.joinPath(
							extensionUri,
							"media",
							"attention",
							`${spec.icon.id}-dark.svg`,
						),
					}
				: spec.icon.color === undefined
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
	t: Localizer,
): CommandHost {
	return {
		t,
		showError: (message) => {
			void vscode.window.showErrorMessage(message);
		},
		showInfo: (message) => {
			void vscode.window.showInformationMessage(message);
		},
		// The three members CommandHost gained when every host came to extend
		// ReporterHost (dinah-490 D-25). The warning is the documented
		// three-argument overload the other two factories already write, and
		// the channel pair binds to the output channel rather than to the
		// window, which is why neither counts as a message call.
		showWarning: async (message, actions) =>
			vscode.window.showWarningMessage(message, ...actions),
		appendLines: (lines) => {
			for (const line of lines) {
				channel.appendLine(line);
			}
		},
		revealOutput: () => channel.show(),
		copyToClipboard: async (text) => vscode.env.clipboard.writeText(text),
		// A row marked as a separator becomes the editor's own label-only
		// divider, which a reader cannot select and which showQuickPick
		// therefore never answers with. The modules composing these arrays
		// import no vscode symbol, so the marker is a string there and becomes
		// a QuickPickItemKind here (dinah-420 D6).
		pick: async (items, placeholder) => {
			const chosen = await vscode.window.showQuickPick(
				items.map((item) =>
					item.kind === "separator"
						? { ...item, kind: vscode.QuickPickItemKind.Separator }
						: { ...item, kind: vscode.QuickPickItemKind.Default },
				),
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
		// setTextDocumentLanguage rather than trusting the `.md` suffix on the
		// path: the suffix is display, and the language association is a
		// documented call that says what it means.
		openServedText: async (kind, root, ref, title) => {
			const parts = servedTextUriParts(kind, root, ref, title);
			const uri = vscode.Uri.from({
				scheme: SERVED_TEXT_SCHEME,
				authority: parts.authority,
				path: parts.path,
				query: parts.query,
			});
			const document = await vscode.workspace.openTextDocument(uri);
			await vscode.languages.setTextDocumentLanguage(document, "markdown");
			await vscode.window.showTextDocument(document);
		},
		// The documented three-argument overload, whose MessageOptions.modal
		// makes the dialog block and whose items become its buttons. VS Code
		// supplies Cancel itself, and a dismissal answers undefined, which is
		// read here as declined (dinah-451 D-3).
		confirmDestructive: async (message, confirmLabel) => {
			const picked = await vscode.window.showWarningMessage(
				message,
				{ modal: true },
				confirmLabel,
			);
			return picked === confirmLabel;
		},
		checkpoint,
		log: (line) => channel.appendLine(line),
	};
}

/** The window calls the comment commands make, bound to the real window. */
function commentBodyHost(
	channel: vscode.OutputChannel,
	t: Localizer,
	checkpoint: (folder: string) => Promise<void>,
): CommentBodyHost {
	const decoder = new TextDecoder();
	return {
		t,
		// A read that throws is read as no file rather than propagated,
		// because the one question this call answers is what the anchor's
		// header says, and "it does not say" is an answer the caller handles.
		readFile: async (path) => {
			try {
				return decoder.decode(
					await vscode.workspace.fs.readFile(vscode.Uri.file(path)),
				);
			} catch {
				return undefined;
			}
		},
		openDocument: async (path) => {
			const document = await vscode.workspace.openTextDocument(
				vscode.Uri.file(path),
			);
			await vscode.window.showTextDocument(document);
		},
		showError: (message) => {
			void vscode.window.showErrorMessage(message);
		},
		showInfo: (message) => {
			void vscode.window.showInformationMessage(message);
		},
		appendLines: (lines) => {
			for (const line of lines) {
				channel.appendLine(line);
			}
		},
		checkpoint,
		log: (line) => channel.appendLine(line),
	};
}

/** The window calls the workbench-row commands make, bound to the real window. */
function workbenchCommandHost(
	channel: vscode.OutputChannel,
	t: Localizer,
): WorkbenchCommandHost {
	return {
		t,
		// showError arrives with ReporterHost (dinah-490 D-25), so a workbench
		// command's refusals are collectable like every other host's.
		showError: (message) => {
			void vscode.window.showErrorMessage(message);
		},
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
function columnCommandHost(
	channel: vscode.OutputChannel,
	t: Localizer,
): ColumnCommandHost {
	return {
		t,
		// showError and showInfo arrive with ReporterHost (dinah-490 D-25).
		showError: (message) => {
			void vscode.window.showErrorMessage(message);
		},
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

	// The one read of the editor's display language in the whole extension.
	// Everything a reader sees at run time is rendered through this Localizer,
	// which is threaded into every host and every render call below; the
	// manifest half is resolved by the editor itself out of package.nls.json
	// before any of this runs. Nothing here reaches the CLI: the operator
	// ruled on 2026-09-06 that the extension imposes no language on a spawned
	// dinah, so no DINAH_LANG is set and no --lang is passed, and a German
	// editor over a Czech environment shows German chrome around Czech card
	// text.
	const t = createLocalizer(resolveTag(vscode.env.language));

	const binary = await resolveBinary({
		setting: setting(SETTING_PATH),
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
	// The pre-load paint. No provider exists yet, so this one carries no held
	// card at all; renderStatusBar() below replaces it as soon as the initial
	// load has answered, and again on every checkpoint and every tick.
	let view = composeStatus(
		binary,
		primary,
		PAIRED_RELEASE,
		NOTHING_HELD,
		Date.now(),
		t,
	);
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
		t,
		// The same call that resolved every folder at activation, offered
		// back to the provider so that a folder resolving to no workbench is
		// asked again at each checkpoint. Its vacancy expires like any other
		// answer, so somebody has to renew it, and this is the only call that
		// can. The pinned setting is read per folder here exactly as it is
		// above, because a reader who pins one folder's workbench expects the
		// pin to hold on every later read of it.
		resolve: (folder) =>
			resolveWorkbench(
				nodeSpawner,
				binary.state === "ok" ? binary.path : "",
				folder,
				setting(SETTING_WORKBENCH, vscode.Uri.file(folder)),
				process.platform === "win32",
			),
		deadEndSentence: (refusal) =>
			refusal === NO_WORKBENCH_FOUND
				? t("tree.root.deadEnd.noWorkbenchSentence")
				: t("status.refused", { refusal }),
	});

	// The Problems panel's half of the check, and the one collection every
	// workbench's findings are written into.
	const diagnosticCollection =
		vscode.languages.createDiagnosticCollection("Dinah");
	context.subscriptions.push(diagnosticCollection);

	/**
	 * Every severity a projected entry can carry, mapped once.
	 *
	 * A record rather than a conditional, so a second severity added to the
	 * union fails to compile here instead of quietly rendering as this one.
	 */
	const severities: Record<
		DiagnosticEntry["severity"],
		vscode.DiagnosticSeverity
	> = { warning: vscode.DiagnosticSeverity.Warning };

	/**
	 * The definition file a finding attaches to when its own path is not a
	 * document, asked of the CLI once per workbench.
	 *
	 * `path workbench` rather than joining workbench.md onto the root, for the
	 * reason editWorkbenchDefinition already gives: a workbench whose
	 * definition file is relocated or malformed is exactly the case a
	 * hardcoded join gets wrong, and this is the surface built to answer it.
	 *
	 * A failed call is not remembered, so the next run asks again rather than
	 * carrying one bad moment for the rest of the session.
	 */
	const definitionFiles = new Map<string, string>();
	const resolveFallback = async (
		root: string,
	): Promise<string | undefined> => {
		const known = definitionFiles.get(root);
		if (known !== undefined) {
			return known;
		}
		const outcome = await runDinah(
			nodeSpawner,
			binary.state === "ok" ? binary.path : "",
			["--workbench", root, "path", "workbench"],
			{ cwd: root },
		);
		if (outcome.kind !== "ok") {
			channel.appendLine(
				`dinah check: ${root} names no readable definition file (${outcome.kind})`,
			);
			return undefined;
		}
		const resolved = (outcome.json as PathAnswer).path;
		if (resolved === undefined || resolved === "") {
			channel.appendLine(`dinah check: ${root} answered path with no path`);
			return undefined;
		}
		definitionFiles.set(root, resolved);
		return resolved;
	};

	const diagnostics = new CheckDiagnostics({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		log: (line) => channel.appendLine(line),
		t,
		statKind: async (path): Promise<StatKind> => {
			try {
				const stat = await vscode.workspace.fs.stat(vscode.Uri.file(path));
				// The type is a bit set rather than an enumeration, so a
				// symbolic link to a directory carries the directory bit
				// alongside the link bit and is read as the directory it
				// points at.
				if ((stat.type & vscode.FileType.Directory) !== 0) {
					return "directory";
				}
				if ((stat.type & vscode.FileType.File) !== 0) {
					return "file";
				}
				// Something is there and it is neither, which the API reports
				// as Unknown. It is not a document a diagnostic can be opened
				// on, so it takes the same route a directory takes.
				return "missing";
			} catch {
				// A path that is not there throws, which is what the API
				// documents for stat and the only signal it gives.
				return "missing";
			}
		},
		resolveFallback,
		apply: (byPath: DiagnosticPlan) => {
			for (const [path, entries] of byPath) {
				diagnosticCollection.set(
					vscode.Uri.file(path),
					entries.map((entry) => {
						const diagnostic = new vscode.Diagnostic(
							new vscode.Range(
								entry.range[0],
								entry.range[1],
								entry.range[2],
								entry.range[3],
							),
							entry.message,
							severities[entry.severity],
						);
						diagnostic.source = entry.source;
						return diagnostic;
					}),
				);
			}
		},
	});

	/**
	 * Recomposes the status bar from data the provider is already holding.
	 *
	 * Nothing here spawns a process. The held cards and their expiry stamps
	 * come off the `status` answers the tree's own reads already made, and the
	 * time left is arithmetic over those stamps and the wall clock, so a
	 * redraw costs a string and no CLI call.
	 *
	 * Three call sites reach it and there is no fourth: the initial load
	 * below, the checkpoint loop's refresh callback, and the countdown's own
	 * thirty-second tick.
	 */
	function renderStatusBar(): void {
		const now = Date.now();
		view = composeStatus(
			binary,
			primary,
			PAIRED_RELEASE,
			summarizeHolding(
				provider.holdingSnapshot(),
				now,
				staleAfterMs(settingOf<number>(SETTING_POLL_INTERVAL, 10)),
			),
			now,
			t,
		);
		if (statusItem === undefined) {
			return;
		}
		statusItem.text = view.text;
		statusItem.tooltip = view.tooltip;
		if (view.hidden) {
			statusItem.hide();
		} else {
			statusItem.show();
		}
	}

	if (binary.state === "ok") {
		await provider.load(
			(vscode.workspace.workspaceFolders ?? []).map((folder) => ({
				folder: folder.uri.fsPath,
				name: folder.name,
				resolution:
					workbenches.get(folder.uri.fsPath) ??
					({
						state: "refused",
						refusal: NO_WORKBENCH_FOUND,
						// Nobody asked dinah anything about this folder, so
						// this resolution is a placeholder rather than an
						// answer. Saying otherwise would offer the vacancy
						// predicate a synthetic refusal to act on, and
						// `answered` is the field standing between the two.
						answered: false,
					} as WorkbenchResolution),
			})),
		);
		renderStatusBar();
		// The first check of the session, one per workbench the load resolved.
		// runFor is deliberately not awaited: a structural sweep of several
		// workbenches would otherwise hold up the rest of activation, and each
		// root's own result reaches the panel as it lands.
		//
		// Nothing here puts the "not checked yet" row up, and that is the
		// point. CheckDiagnostics marks the root itself before it spawns, so a
		// reader opening the Problems panel while the sweep is still running
		// sees the workbench named as unconfirmed either way. This loop skips
		// every workbench the load has not heard from, and those are reached
		// later through the checkpoint callback below, which is why the
		// obligation cannot live at a call site.
		for (const report of provider.holdingSnapshot()) {
			if (report.state !== "answered") {
				continue;
			}
			void diagnostics.runFor(report.source, report.title);
		}
	}

	// createTreeView rather than registerTreeDataProvider, because the
	// checkpoint loop suspends while the view is hidden and TreeView.visible is
	// the only way to know that it is.
	//
	// The drag controller is composed here rather than declared in
	// package.json, because TreeViewOptions takes it at construction time and
	// nothing under contributes.views names drag support at all. Both handlers
	// reach the command host built further down this same function, which
	// exists by the time a reader can drag a row and would be the only thing
	// this object needed activation to finish first.
	const dragAndDropController: vscode.TreeDragAndDropController<TreeElement> = {
		dragMimeTypes: [DRAG_MIME_TYPE],
		dropMimeTypes: [DRAG_MIME_TYPE],
		handleDrag: (source, dataTransfer) => {
			offerDrag(
				source,
				DRAG_MIME_TYPE,
				dataTransfer,
				(rows) => new vscode.DataTransferItem(rows),
				t,
			);
		},
		handleDrop: async (target, dataTransfer) => {
			// An entry this controller did not put there, a value of a shape
			// it could not have set, and a drag that started on rows carrying
			// no card at all, all arrive as undefined here, and none of them
			// is this controller's to act on. The read is a function rather
			// than a cast because the first thing the drop path does with the
			// value is iterate it.
			const rows = dragRowsFrom(dataTransfer.get(DRAG_MIME_TYPE)?.value);
			if (rows === undefined) {
				return;
			}
			await applyDropVerdicts(
				rows,
				dropColumnFor(target),
				binary.state === "ok" ? binary.path : "",
				host,
				nodeSpawner,
			);
		},
	};
	treeView = vscode.window.createTreeView<TreeElement>(VIEW_ID, {
		treeDataProvider: {
			onDidChangeTreeData: emitter.event,
			getTreeItem: (element) => toTreeItem(provider.getTreeItem(element), context.extensionUri),
			getChildren: (element) => provider.getChildren(element),
		},
		dragAndDropController,
		// TreeViewOptions.canSelectMany documents that with it set "the first
		// argument to the command is the tree item that the command was
		// executed on and the second argument is an array containing all
		// selected tree items". Every row command is registered to read both.
		canSelectMany: true,
	});
	context.subscriptions.push(treeView);

	// The editor's MCP server list, one entry per workbench this window has
	// resolved. Registration is unconditional, because a reader who installs
	// the binary after the window opened would otherwise get no provider at
	// all and nothing would tell them so; the provider answers with an empty
	// array until there is something to publish. Nothing here starts a
	// server. The editor asks, this answers, and the editor asks the reader
	// whether they trust a server before it runs one.
	const mcpChanged = new vscode.EventEmitter<void>();
	context.subscriptions.push(mcpChanged);

	let published: readonly McpServerPlan[] = [];
	const currentPlans = (): readonly McpServerPlan[] =>
		publishedMcpServers(
			binary,
			settingOf<boolean>(SETTING_REGISTER_MCP, true),
			provider.mcpTargets(),
			process.platform === "win32",
		);

	// The one impure step: plain data becomes the editor's own value. The
	// installed .d.ts takes cwd as a settable property rather than as a
	// constructor argument, which is why it is assigned rather than passed.
	const toDefinition = (plan: McpServerPlan): vscode.McpStdioServerDefinition => {
		const definition = new vscode.McpStdioServerDefinition(
			plan.label,
			plan.command,
			[...plan.args],
			{},
			plan.version,
		);
		definition.cwd = vscode.Uri.file(plan.cwd);
		return definition;
	};

	context.subscriptions.push(
		vscode.lm.registerMcpServerDefinitionProvider(MCP_PROVIDER_ID, {
			onDidChangeMcpServerDefinitions: mcpChanged.event,
			provideMcpServerDefinitions: () => {
				published = currentPlans();
				return published.map(toDefinition);
			},
		}),
	);

	// The reader changed the switch, so the set is answered for again without
	// waiting for a window reload.
	context.subscriptions.push(
		vscode.workspace.onDidChangeConfiguration((event) => {
			if (event.affectsConfiguration(SETTING_REGISTER_MCP)) {
				mcpChanged.fire();
			}
		}),
	);

	loop = new CheckpointLoop({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		clock: systemClock,
		log: (line) => channel.appendLine(line),
		// The one hook a check re-runs on. CheckpointLoop calls this only when
		// `dinah changes` answered that something moved, so the panel is
		// refreshed by a change on disk rather than by a timer of its own and
		// never by a keystroke. Scoping to the folder's own workbenches keeps
		// a change in one from re-sweeping every other workbench the window
		// has open.
		refresh: redrawAfterRefresh(
			async (folder) => {
				await provider.refresh(folder);
				for (const { root, title } of provider.rootsFor(folder)) {
					await diagnostics.runFor(root, title);
				}
				// Only where the set actually moved. Firing every checkpoint
				// would prompt the reader to refresh their tools every poll
				// interval, which the version field's documented behaviour
				// makes a real cost to them.
				const next = currentPlans();
				if (mcpPlansDiffer(published, next)) {
					published = next;
					mcpChanged.fire();
				}
			},
			renderStatusBar,
		),
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

	// The countdown's own timer, which redraws the remaining time between
	// checkpoints and asks dinah nothing. It runs only where there is a
	// binary to have produced a held card in the first place.
	ticker = new CountdownTicker(systemClock, renderStatusBar);
	if (binary.state === "ok") {
		ticker.start();
	}
	const counting = ticker;
	context.subscriptions.push({ dispose: () => counting.stop() });

	// dinah-515's head. It reads and never writes, so it is started wherever
	// there is a binary to run it and the reader has left it on, and it is
	// stopped with the extension.
	if (binary.state === "ok" && settingOf<boolean>(SETTING_LSP_ENABLED, true)) {
		context.subscriptions.push(startLanguageServer(binary.path));
	}

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
		handler: (
			element: TreeElement | undefined,
			selection?: readonly TreeElement[],
		) => Promise<unknown>,
	): void {
		registeredIds.push(id);
		context.subscriptions.push(vscode.commands.registerCommand(id, handler));
	}

	// The served-text scheme, its one content provider, and the timer that
	// keeps an open tab current.
	//
	// The provider dispatches on a resolver table keyed by the URI's authority,
	// which is the kind of text being served. dinah-270 puts one entry in that
	// table. A later document type adds a second entry and touches neither the
	// provider, the URI grammar, nor the refresh loop, which is the whole point
	// of keying it this way.
	const resolvers: Record<
		string,
		(root: string, ref: string) => Promise<string>
	> = {
		[KIND_INSTRUCTIONS]: async (root, ref) => {
			const outcome = await runDinah(
				nodeSpawner,
				binary.state === "ok" ? binary.path : "",
				pinnedArgv(root, ["instructions", ref]),
				{ cwd: root },
			);
			if (outcome.kind !== "ok") {
				throw new Error(refusalMessage(outcome));
			}
			const served = outcome.json as ServedAnswer;
			return renderInstructionsMarkdown(served.instructions, {
				global: t("servedText.heading.global"),
				standing: t("servedText.heading.standing"),
				column: t("servedText.heading.column"),
				columnAttachments: t("servedText.heading.columnAttachments"),
			});
		},
		// The guide is fetched with no --workbench and no cwd, because
		// `dinah guide` opens no workbench and ignores the flag when it is
		// given one. That is what lets this kind answer in a window that found
		// no workbench, which is the window the first-session walkthrough is
		// written for. The root the table's shape passes is a fixed
		// placeholder that only the tab bookkeeping reads.
		// The card's own journal, fetched through the same pinned argv every
		// other card-scoped call composes. The --json flag is not spelled here
		// and is not pinnedArgv's either: cli.ts's composeArgv puts it in front
		// of whatever argv reaches runDinah, and it refuses an argv that already
		// carries it.
		[KIND_HISTORY]: async (root, ref) => {
			const outcome = await runDinah(
				nodeSpawner,
				binary.state === "ok" ? binary.path : "",
				pinnedArgv(root, ["list", `${ref}/journal`]),
				{ cwd: root },
			);
			if (outcome.kind !== "ok") {
				throw new Error(refusalMessage(outcome));
			}
			// The cast admits null, because a card whose journal is absent or
			// carries no lines is answered with the JSON literal null rather
			// than with an empty array. renderHistoryMarkdown takes both.
			return renderHistoryMarkdown(outcome.json as JournalEvent[] | null, t);
		},
		[KIND_GUIDE]: async (_root, ref) => {
			const outcome = await runDinahText(
				nodeSpawner,
				binary.state === "ok" ? binary.path : "",
				["guide", ref],
			);
			if (outcome.kind !== "ok") {
				throw new Error(refusalMessage(outcome));
			}
			return outcome.text;
		},
	};
	// The Uri a tab opened under, kept so that a change can be announced for
	// the same value the editor holds. onDidChange takes a Uri and the loop
	// deals in the string form, which is the only key a Map can compare.
	const servedTextUris = new Map<string, vscode.Uri>();
	const servedTextEmitter = new vscode.EventEmitter<vscode.Uri>();
	context.subscriptions.push(servedTextEmitter);
	const servedTextRefreshLoop = new ServedTextRefreshLoop({
		clock: systemClock,
		// The tree's own setting, read a second time rather than shared as one
		// value, because the two loops start and stop on unrelated conditions:
		// the tree's on whether its view is visible, this one on whether any
		// tab is open.
		pollIntervalSeconds: settingOf<number>(SETTING_POLL_INTERVAL, 10),
		resolve: async (kind, root, ref) => {
			const resolve = resolvers[kind];
			if (resolve === undefined) {
				throw new Error(`no servedText resolver for kind ${kind}`);
			}
			return resolve(root, ref);
		},
		onChanged: (uriKey) => {
			const uri = servedTextUris.get(uriKey);
			if (uri !== undefined) {
				servedTextEmitter.fire(uri);
			}
		},
		log: (line) => channel.appendLine(line),
	});
	servedText = servedTextRefreshLoop;
	context.subscriptions.push(
		vscode.workspace.onDidOpenTextDocument((document) => {
			if (document.uri.scheme !== SERVED_TEXT_SCHEME) {
				return;
			}
			const parsed = parseServedTextUri(document.uri.authority, document.uri.query);
			if (parsed === undefined) {
				return;
			}
			servedTextUris.set(document.uri.toString(), document.uri);
			servedTextRefreshLoop.noteOpened(
				document.uri.toString(),
				parsed.kind,
				parsed.root,
				parsed.ref,
				document.getText(),
			);
		}),
		vscode.workspace.onDidCloseTextDocument((document) => {
			if (document.uri.scheme !== SERVED_TEXT_SCHEME) {
				return;
			}
			servedTextUris.delete(document.uri.toString());
			servedTextRefreshLoop.noteClosed(document.uri.toString());
		}),
		{ dispose: () => servedTextRefreshLoop.stop() },
		vscode.workspace.registerTextDocumentContentProvider(SERVED_TEXT_SCHEME, {
			onDidChange: servedTextEmitter.event,
			provideTextDocumentContent: async (uri) => {
				const parsed = parseServedTextUri(uri.authority, uri.query);
				if (parsed === undefined) {
					return t("servedText.malformedUri");
				}
				const resolve = resolvers[parsed.kind];
				if (resolve === undefined) {
					return t("servedText.unknownKind", { kind: parsed.kind });
				}
				try {
					const text = await resolve(parsed.root, parsed.ref);
					servedTextRefreshLoop.recordFetched(uri.toString(), text);
					return text;
				} catch (err) {
					return t("servedText.refused", {
						detail: err instanceof Error ? err.message : String(err),
					});
				}
			},
		}),
	);

	const host = commandHost(channel, (folder) => checkpointing.checkNow(folder), t);
	const workbenchHost = workbenchCommandHost(channel, t);
	const columnHost = columnCommandHost(channel, t);
	const commentHost = commentBodyHost(channel, t, (folder) =>
		checkpointing.checkNow(folder),
	);
	// The comment files this window has opened. It is here rather than inside
	// a module because two things read it: the commands that open a comment,
	// which add to it, and the save listener below, which is what writes a
	// body through the verb rather than leaving the editor's own bytes on
	// disk.
	const openComments: OpenComments = new Map();
	// The command palette's own catalogue, built here rather than below it
	// because the filing form reads its kind choices from the same object and
	// the wiring has to carry it. The build is still lazy, so a window that
	// never opens either surface never spawns for one.
	//
	// The wizard is pinned to the primary folder's own workbench, because a
	// verb has to run against one and this is the same workbench the status
	// bar and the first tree root already name. A window whose folder resolved
	// to nothing falls back to the folder itself, which is what the binary
	// would walk from at a terminal opened there.
	const verbRoot =
		primary?.state === "ok" ? primary.root : (first?.uri.fsPath ?? "");
	const verbCatalog = new VerbCatalog({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		options: { cwd: verbRoot === "" ? undefined : verbRoot },
		log: (line) => channel.appendLine(line),
	});
	const wiring: Wiring = {
		exe: binary.state === "ok" ? binary.path : "",
		// describeVersion rather than a second spelling of the same line: the
		// status tooltip and the demotion diagnostic already describe a binary
		// this way, and this is display, the only thing version.ts's header
		// allows the release tag inside it to be used for.
		binaryLabel: binary.state === "ok" ? describeVersion(binary.version) : "",
		spawner: nodeSpawner,
		t,
		cardHost: host,
		workbenchHost,
		columnHost,
		// Check Workbench needed a closure over activate() before dinah-490,
		// because the diagnostics object is not in any command's context. It
		// travels as this one structural member instead, so commandTable.ts
		// names no type from a module owning a DiagnosticCollection and every
		// entry there can be a value rather than a closure.
		applyCheckResult: async (path, label, outcome) => {
			await diagnostics.applyResult(path, label, outcome);
		},
		commentHost,
		openComments,
		verbCatalog: () => verbCatalog.get(),
	};

	// One loop registers every command that reads rows, and it is the only
	// place a selection is resolved. targetsFor answers the rows the reader
	// aimed at, the entry's invoke is handed that list whole, and nothing
	// between here and runBulk filters it (dinah-490 D-16). The four commands
	// that read no row keep their own registrations below.
	for (const { id, invoke } of ROW_COMMAND_TABLE) {
		register(id, async (element, selection) =>
			invoke(targetsFor(element, selection, elementKey), wiring),
		);
	}

	// The walkthrough's button, and a Command Palette entry beside it. The
	// root passed here is a fixed word rather than a directory: a guide has no
	// directory to be pinned to, the guide resolver never reads it, and
	// parseServedTextUri refuses an empty one, so it has to be some non-empty
	// word (dinah-423).
	register(COMMAND_OPEN_FIRST_SESSION_GUIDE, async () => {
		await host.openServedText(
			KIND_GUIDE,
			GUIDE_ROOT,
			GUIDE_TOPIC_FIRST_SESSION,
			t("servedText.title.guide", { topic: GUIDE_TOPIC_FIRST_SESSION }),
		);
	});
	register(COMMAND_REFRESH, async () => {
		await checkpointing.refreshNow();
		emitter.fire(undefined);
	});

	// Saving a comment's own file writes its body through the verb.
	//
	// The editor has already put its bytes on disk by the time this runs, and
	// that is the state the call corrects rather than prevents: the write
	// re-renders the anchor from the body it is given and records the digest
	// over it, so the record and the file agree again and the journal says who
	// changed it. Without it every save of a comment would leave one `dinah
	// check` reports as diverged.
	//
	// A file this window did not open a comment into is left alone, which
	// saveCommentBody answers for by reading the map rather than the path.
	context.subscriptions.push(
		vscode.workspace.onDidSaveTextDocument((document) => {
			void saveCommentBody(
				commentHost,
				nodeSpawner,
				binary.state === "ok" ? binary.path : "",
				openComments,
				document.uri.fsPath,
				document.getText(),
			).catch((err: unknown) => {
				channel.appendLine(
					`comment save: ${err instanceof Error ? err.message : String(err)}`,
				);
			});
		}),
		vscode.workspace.onDidCloseTextDocument((document) => {
			forgetComment(openComments, document.uri.fsPath);
		}),
	);

	// The command palette's two commands, over the catalogue built above.
	const verbContext = (): RunVerbContext => ({
		spawner: nodeSpawner,
		exe: binary.state === "ok" ? binary.path : "",
		host,
		folder: first?.uri.fsPath ?? verbRoot,
		root: verbRoot,
		catalog: verbCatalog,
	});
	register(COMMAND_RUN_VERB, async () => {
		await runVerbFromPalette(verbContext());
	});
	register(COMMAND_REFRESH_VERB_CATALOG, async () => {
		await refreshVerbCatalog(verbContext());
	});
	if (binary.state === "ok") {
		// Built once at activation and held, so opening the palette costs no
		// spawn. The build is not awaited: it is one round trip against a
		// process that has to start, and nothing else in activation depends on
		// it, so awaiting it would delay the tree for a list nobody has asked
		// for yet. A build that failed is held as its own failure arm and
		// reported when a reader runs the command.
		//
		// The rejection handler is not decoration. nodeSpawner resolves on
		// every path it has today, so nothing here can reject, and a future
		// spawner that throws would otherwise surface as an unhandled
		// rejection in the extension host's own log rather than in Dinah's.
		verbCatalog.get().catch((reason: unknown) => {
			channel.appendLine(
				`Command palette: the catalog build at activation threw. ${String(reason)}`,
			);
		});
		// A watcher on the binary's own path rather than a glob over its
		// directory, which is the shape the checkpoint loop's watcher already
		// uses, narrowed to one file. An upgrade that replaces the binary in
		// place invalidates what is held, and the next invocation rebuilds it.
		// The path is watched for creation as well as change, because an
		// installer that writes a new file beside the old one and renames it
		// reaches this path as a create.
		const binaryWatcher = vscode.workspace.createFileSystemWatcher(
			new vscode.RelativePattern(
				vscode.Uri.file(dirname(binary.path)),
				basename(binary.path),
			),
		);
		binaryWatcher.onDidChange(() => verbCatalog.invalidate());
		binaryWatcher.onDidCreate(() => verbCatalog.invalidate());
		context.subscriptions.push(binaryWatcher);
	}

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
	ticker?.stop();
	ticker = undefined;
	servedText?.stop();
	servedText = undefined;
	treeView = undefined;
	output = undefined;
}

/**
 * The chip a card reference is drawn with, one decoration type per tone.
 *
 * A tone is decided from the canonical tokens the server publishes rather
 * than from the sentence it publishes beside them, which is what lets this
 * client change how a chip looks without the server changing at all.
 */
const CHIP_COLOURS: Record<Chip["tone"], string> = {
	blocked: "editorError.foreground",
	active: "editorInfo.foreground",
	holding: "editorWarning.foreground",
	unresolved: "descriptionForeground",
	plain: "editorCodeLens.foreground",
};

/**
 * Starts the language server and draws its annotations.
 *
 * The standard inlay hints are suppressed by a middleware that asks the
 * server anyway and delivers none of what it answered, so a chip and a hint
 * never draw together. The chip itself is built from the namespaced answer,
 * because no member of a protocol inlay hint carries a payload an extension
 * can read.
 */
function startLanguageServer(exe: string): vscode.Disposable {
	const server: ServerOptions = {
		run: { command: exe, args: ["lsp"], transport: TransportKind.stdio },
		debug: { command: exe, args: ["lsp"], transport: TransportKind.stdio },
	};
	const decorations = new Map<Chip["tone"], vscode.TextEditorDecorationType>();
	for (const [tone, colour] of Object.entries(CHIP_COLOURS)) {
		decorations.set(
			tone as Chip["tone"],
			vscode.window.createTextEditorDecorationType({
				after: { color: new vscode.ThemeColor(colour), margin: "0 0 0 0.6em" },
			}),
		);
	}
	// The two settings the server itself reads, dinah.lsp.annotateProse and
	// dinah.lsp.pollIntervalSeconds, are not passed here. The client declares
	// the configuration capability, so the server pulls the section at
	// startup and is sent it again on every change, which is the route
	// section 5.4 of the card's contract specifies.
	const options: LanguageClientOptions = {
		documentSelector: [
			{ scheme: "file", language: "markdown" },
			{ scheme: "file", language: "plaintext" },
		],
		synchronize: { configurationSection: LSP_SECTION },
		outputChannel: output,
		middleware: suppressInlayHints<vscode.InlayHint>(),
	};
	const client = new LanguageClient("dinah", "Dinah", server, options);
	const trace = settingOf<string>(SETTING_LSP_TRACE, "off");
	if (trace !== "off") {
		void client.setTrace(trace === "verbose" ? Trace.Verbose : Trace.Messages);
	}

	const draw = async (uri: string): Promise<void> => {
		const editor = vscode.window.visibleTextEditors.find(
			(open) => open.document.uri.toString() === uri,
		);
		if (editor === undefined) {
			return;
		}
		const answer = await client.sendRequest<AnnotationsAnswer>(ANNOTATIONS_REQUEST, {
			textDocument: { uri },
		});
		const drawn = new Map<Chip["tone"], vscode.DecorationOptions[]>();
		for (const tone of decorations.keys()) {
			drawn.set(tone, []);
		}
		for (const chip of chipsFrom(answer)) {
			drawn.get(chip.tone)?.push({
				range: new vscode.Range(
					chip.range.start.line,
					chip.range.start.character,
					chip.range.end.line,
					chip.range.end.character,
				),
				renderOptions: { after: { contentText: chip.text } },
				hoverMessage: new vscode.MarkdownString(chip.tooltip),
			});
		}
		for (const [tone, type] of decorations) {
			editor.setDecorations(type, drawn.get(tone) ?? []);
		}
	};

	void client.start().then(() => {
		client.onNotification(ANNOTATIONS_CHANGED, (params: AnnotationsChanged) => {
			for (const uri of params.uris) {
				void draw(uri);
			}
		});
		for (const editor of vscode.window.visibleTextEditors) {
			void draw(editor.document.uri.toString());
		}
	});
	const opened = vscode.window.onDidChangeVisibleTextEditors((editors) => {
		for (const editor of editors) {
			void draw(editor.document.uri.toString());
		}
	});
	return {
		dispose: () => {
			opened.dispose();
			for (const type of decorations.values()) {
				type.dispose();
			}
			void client.stop();
		},
	};
}
