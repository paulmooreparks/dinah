// What every suite inside the extension host needs.

import { execFileSync } from "node:child_process";

import * as vscode from "vscode";

import type { DinahApi } from "../../../src/api";
import { EXTENSION_ID } from "../../../src/identity";
import { parseProfile } from "../../../src/version";

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

/**
 * The conformance profile the binary under test declares about itself.
 *
 * A suite that spells the profile out instead goes red on every amendment to
 * dinah-core, which is a schedule this extension does not control and did not
 * participate in. Asking the binary turns the assertion into the question
 * worth asking, which is whether the extension reports what the binary said
 * rather than whether the profile is any particular number.
 */
export function declaredProfile(): string {
	const binary = process.env.DINAH_FIXTURE_BINARY;
	if (binary === undefined || binary.trim() === "") {
		throw new Error(
			"DINAH_FIXTURE_BINARY is unset, so this suite cannot ask the binary what it conforms to",
		);
	}
	const stdout = execFileSync(binary, ["--json", "version"], { encoding: "utf8" });
	const report = JSON.parse(stdout) as { profile?: unknown };
	if (typeof report.profile !== "string") {
		throw new Error(`\`dinah --json version\` from ${binary} reported no profile`);
	}
	if (!parseProfile(report.profile)) {
		throw new Error(
			`${binary} reports "${report.profile}", which is not a conformance claim this extension can read`,
		);
	}
	return report.profile;
}

/**
 * The storage format the binary under test writes, asked of that binary for
 * the reason declaredProfile asks it for the conformance claim: the number
 * moves on Dinah's own schedule, and a suite that spelled it out would fail on
 * the day it moved while telling nobody anything about the extension. What is
 * worth asserting is that the extension reports what the binary said.
 */
export function declaredFormat(): number {
	const binary = process.env.DINAH_FIXTURE_BINARY;
	if (binary === undefined || binary.trim() === "") {
		throw new Error(
			"DINAH_FIXTURE_BINARY is unset, so this suite cannot ask the binary what it writes",
		);
	}
	const stdout = execFileSync(binary, ["--json", "version"], { encoding: "utf8" });
	const report = JSON.parse(stdout) as { format?: unknown };
	if (typeof report.format !== "number") {
		throw new Error(`\`dinah --json version\` from ${binary} reported no storage format`);
	}
	return report.format;
}
