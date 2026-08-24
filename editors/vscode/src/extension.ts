// activate() and deactivate().
//
// This is the only module that imports vscode. Everything it calls is a pure
// function over injected dependencies, which is what lets the unit tests reach
// the whole of the binary ladder, the compatibility gate and the refusal
// parser without a VS Code host.

import * as vscode from "vscode";

import type { DinahApi, WorkbenchResolution } from "./api";
import { resolveBinary } from "./binary";
import { runDinah } from "./cli";
import { PAIRED_RELEASE } from "./generated/pairing";
import { SETTING_PATH, SETTING_WORKBENCH, VIEW_ID } from "./identity";
import {
	ensureExecutable,
	joinPath,
	listDirectory,
	nodeSpawner,
} from "./spawn";
import { composeContextKeys, composeStatus } from "./status";
import { classifyVersion } from "./version";
import { resolveWorkbench } from "./workbench";

let statusItem: vscode.StatusBarItem | undefined;
let output: vscode.OutputChannel | undefined;

/** Reads a settings value as a string, treating an unset value as empty. */
function setting(key: string, scope?: vscode.Uri): string {
	const dot = key.lastIndexOf(".");
	const section = key.slice(0, dot);
	const name = key.slice(dot + 1);
	return vscode.workspace.getConfiguration(section, scope).get<string>(name) ?? "";
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

	return {
		binary,
		workbenches,
		statusTooltip: view.tooltip,
		statusText: view.text,
		pairedRelease: PAIRED_RELEASE,
	};
}

export function deactivate(): void {
	statusItem?.dispose();
	statusItem = undefined;
	output = undefined;
}
