// What every suite inside the extension host needs.

import * as vscode from "vscode";

import type { DinahApi } from "../../../src/api";
import { EXTENSION_ID } from "../../../src/identity";

/** The extension under test, as VS Code knows it. */
export function extension(): vscode.Extension<DinahApi> {
	const found = vscode.extensions.getExtension<DinahApi>(EXTENSION_ID);
	if (!found) {
		throw new Error(
			`no extension is registered as ${EXTENSION_ID}; the manifest and identity.ts have drifted apart`,
		);
	}
	return found;
}

/** Activates the extension and returns the object activate() produced. */
export async function api(): Promise<DinahApi> {
	return extension().activate();
}

/** The workspace folder this window was opened on. */
export function folder(): string {
	const folders = vscode.workspace.workspaceFolders ?? [];
	if (folders.length !== 1) {
		throw new Error(
			`this suite expects exactly one workspace folder, and the window has ${String(folders.length)}`,
		);
	}
	return folders[0].uri.fsPath;
}

/** Waits until `condition` holds, or gives up. */
export async function until(
	condition: () => boolean,
	ms: number,
): Promise<boolean> {
	const deadline = Date.now() + ms;
	while (Date.now() < deadline) {
		if (condition()) {
			return true;
		}
		await new Promise((resolve) => setTimeout(resolve, 100));
	}
	return condition();
}

/** The resolution for the folder this window opened on. */
export function resolution(reported: DinahApi) {
	const found = reported.workbenches.get(folder());
	if (!found) {
		throw new Error(`no resolution was reported for ${folder()}`);
	}
	return found;
}
