// The served-text tab, against a real editor and a real workbench.
//
// The unit layer drives the URI grammar, the Markdown rendering and the
// refresh loop against fixtures, which is fast and covers what a real bench
// cannot easily be pushed into. Two claims are left over that only a running
// editor can settle: that invoking the command really opens one Markdown tab
// carrying the text the binary served, and that an edit to a column's own
// instructions file reaches that tab. The second is the case the whole poll
// exists for, because `dinah changes` never reports it.
//
// CI-only. Never run the integration layer locally.

import { execFileSync } from "node:child_process";
import * as assert from "node:assert/strict";
import { appendFileSync } from "node:fs";

import * as vscode from "vscode";

import { COMMAND_OPEN_INSTRUCTIONS, SERVED_TEXT_SCHEME } from "../../../src/identity";
import { ENGLISH } from "../../../src/l10n";
import { renderInstructionsMarkdown } from "../../../src/servedText";
import type { ServedAnswer } from "../../../src/wire";
import { api, until } from "./support";

/** The binary this run built, which the fixtures were made with. */
function binary(): string {
	const found = process.env.DINAH_FIXTURE_BINARY;
	if (found === undefined || found.trim() === "") {
		throw new Error("DINAH_FIXTURE_BINARY is unset, so this suite has no binary to ask");
	}
	return found;
}

/** Asks the binary a question in its machine form, pinned to one workbench. */
function ask<T>(root: string, args: readonly string[]): T {
	const stdout = execFileSync(binary(), ["--json", "--workbench", root, ...args], {
		encoding: "utf8",
	});
	return JSON.parse(stdout) as T;
}

/** The first card row the tree drew, with the workbench it stands in. */
async function firstCard(): Promise<{ element: unknown; ref: string; root: string }> {
	const reported = await api();
	const roots = await reported.tree.getChildren();
	const columns = await reported.tree.getChildren(roots[0]);
	const found: unknown[] = [];
	const walk = async (element: unknown): Promise<void> => {
		for (const child of await reported.tree.getChildren(element)) {
			const item = reported.tree.getTreeItem(child) as { contextValue?: string };
			if (String(item.contextValue).startsWith("dinah.card.")) {
				found.push(child);
			}
			await walk(child);
		}
	};
	for (const element of columns) {
		await walk(element);
	}
	assert.ok(found.length > 0, "the fixture's card reached no row in the tree");
	const element = found[0] as {
		view?: { ref?: string };
		node: { ref: string };
		row: { data?: { path?: string } };
	};
	const ref = element.view?.ref ?? element.node.ref;
	const root = element.row.data?.path;
	assert.ok(root !== undefined && root !== "", "the card's row names no workbench");
	return { element: found[0], ref, root };
}

/** Every open tab standing on the served-text scheme. */
function servedTabs(): vscode.Tab[] {
	const tabs: vscode.Tab[] = [];
	for (const group of vscode.window.tabGroups.all) {
		for (const tab of group.tabs) {
			const input = tab.input as { uri?: vscode.Uri } | undefined;
			if (input?.uri?.scheme === SERVED_TEXT_SCHEME) {
				tabs.push(tab);
			}
		}
	}
	return tabs;
}

/** The open served-text document, once one exists. */
function servedDocument(): vscode.TextDocument | undefined {
	return vscode.workspace.textDocuments.find(
		(document) => document.uri.scheme === SERVED_TEXT_SCHEME,
	);
}

suite("served text opens as an editor tab", () => {
	test("the command opens one Markdown tab carrying what the binary served", async () => {
		// dinah-270 AC-11.
		const card = await firstCard();
		await vscode.commands.executeCommand(COMMAND_OPEN_INSTRUCTIONS, card.element);
		assert.ok(
			await until(() => servedDocument() !== undefined, 20_000),
			"the command opened no served-text document",
		);
		const document = servedDocument();
		assert.ok(document !== undefined);
		assert.equal(document.languageId, "markdown");

		// The expected text is composed from a fresh read of the same card
		// through the same binary, so a rendering that reformatted a layer, or
		// a chain the extension read out of the wrong field, fails here.
		const served = ask<ServedAnswer>(card.root, ["instructions", card.ref]);
		const expected = renderInstructionsMarkdown(served.instructions, {
			global: ENGLISH("servedText.heading.global"),
			standing: ENGLISH("servedText.heading.standing"),
			column: ENGLISH("servedText.heading.column"),
		});
		assert.equal(document.getText(), expected);

		const tabs = servedTabs();
		assert.equal(tabs.length, 1, `wanted one served-text tab, found ${String(tabs.length)}`);
		assert.ok(
			tabs[0].label.includes(card.ref),
			`the tab is labelled "${tabs[0].label}", which does not name ${card.ref}`,
		);

		// A second invocation reaches the same URI, so the editor shows the tab
		// that is already open rather than opening a duplicate.
		await vscode.commands.executeCommand(COMMAND_OPEN_INSTRUCTIONS, card.element);
		assert.equal(servedTabs().length, 1, "a second invocation opened a second tab");
	});

	test("an edit to the column's own instructions reaches the open tab", async () => {
		// dinah-270 AC-12, and the reason the tab polls rather than listening.
		// A column's column.md is not among the things `dinah changes` watches,
		// so nothing about this edit ever reports as a change; the tab is
		// current only because it asked again and compared.
		const card = await firstCard();
		await vscode.commands.executeCommand(COMMAND_OPEN_INSTRUCTIONS, card.element);
		assert.ok(
			await until(() => servedDocument() !== undefined, 20_000),
			"the command opened no served-text document",
		);
		const document = servedDocument();
		assert.ok(document !== undefined);

		const served = ask<ServedAnswer>(card.root, ["instructions", card.ref]);
		const path = ask<{ path?: string }>(card.root, ["path", served.column]).path;
		assert.ok(path !== undefined && path !== "", "dinah reported no path for the column");

		const planted = "A sentence planted by the served-text suite.";
		assert.ok(
			!document.getText().includes(planted),
			"the tab already showed the sentence this test plants",
		);
		appendFileSync(path, `\n${planted}\n`, "utf8");

		// The fixture pins dinah.pollIntervalSeconds to its floor of two, so a
		// generous wait here is still several intervals rather than one.
		assert.ok(
			await until(() => (servedDocument()?.getText() ?? "").includes(planted), 30_000),
			"the tab did not pick up an edit to the column's own instructions",
		);
	});
});
