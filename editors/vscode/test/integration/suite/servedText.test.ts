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

import {
	COMMAND_OPEN_FIRST_SESSION_GUIDE,
	COMMAND_OPEN_INSTRUCTIONS,
	EXTENSION_ID,
	SERVED_TEXT_SCHEME,
	WALKTHROUGH_FIRST_SESSION,
	WALKTHROUGH_STEP_READ_GUIDE,
} from "../../../src/identity";
import { ENGLISH } from "../../../src/l10n";
import {
	GUIDE_TOPIC_FIRST_SESSION,
	KIND_GUIDE,
	renderInstructionsMarkdown,
} from "../../../src/servedText";
import type { ServedAnswer } from "../../../src/wire";
import { api, extension, until } from "./support";

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

/** The open guide document, once one exists. */
function guideDocument(): vscode.TextDocument | undefined {
	return vscode.workspace.textDocuments.find(
		(document) =>
			document.uri.scheme === SERVED_TEXT_SCHEME &&
			document.uri.authority === KIND_GUIDE,
	);
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

suite("the first-session walkthrough", () => {
	// dinah-423 AC-2, and as much of OQ-1 as a running editor can answer
	// without a person looking at the screen.

	teardown(async () => {
		// The suite above counts served-text tabs, and a guide tab left open
		// would be counted there if the order these files run in ever changes.
		await vscode.commands.executeCommand("workbench.action.closeAllEditors");
	});

	test("the command opens a guide tab carrying what the binary prints", async () => {
		await vscode.commands.executeCommand(COMMAND_OPEN_FIRST_SESSION_GUIDE);
		assert.ok(
			await until(() => guideDocument() !== undefined, 20_000),
			"the command opened no guide tab",
		);
		const document = guideDocument();
		assert.ok(document !== undefined);
		assert.equal(document.uri.authority, KIND_GUIDE);
		assert.equal(document.languageId, "markdown");

		// The guide is fetched with no --workbench of its own, so the
		// comparison asks the binary the same way the extension does. Byte
		// equality is the claim: nothing between the binary and the tab
		// re-renders, re-wraps or copies the text.
		const printed = execFileSync(binary(), ["--json", "guide", GUIDE_TOPIC_FIRST_SESSION], {
			encoding: "utf8",
		});
		assert.equal(document.getText(), printed);
		assert.notEqual(document.getText().trim(), "", "the guide tab opened empty");

		// One guide tab, counted by authority rather than by how many
		// served-text tabs the window holds. The suite above leaves an
		// instruction tab open, and the editor may reuse a preview tab for
		// this one, so the total says nothing about what this command did.
		const guideTabs = servedTabs().filter((tab) => {
			const input = tab.input as { uri?: vscode.Uri } | undefined;
			return input?.uri?.authority === KIND_GUIDE;
		});
		assert.equal(
			guideTabs.length,
			1,
			`wanted one guide tab, found ${String(guideTabs.length)}`,
		);
	});

	test("the editor knows the walkthrough the welcome view links to", async () => {
		// The welcome view's link is `<publisher>.<name>#<walkthroughId>`
		// against VS Code's own built-in command, and neither half of that can
		// be checked outside a running editor. What this settles is that the
		// command exists, that the installed manifest really carries the
		// walkthrough the link names, and that invoking the command with the
		// argument the welcome view composes resolves rather than throwing.
		//
		// What it leaves open is whether the walkthrough the editor then
		// selected is this one, and whether the step's image renders, because
		// neither is readable through any documented API. OQ-1 carries that
		// half to the operator's station.
		const known = await vscode.commands.getCommands(true);
		assert.ok(
			known.includes("workbench.action.openWalkthrough"),
			"this editor has no openWalkthrough command, so the welcome view links to nothing",
		);

		const installed = extension().packageJSON as {
			contributes: { walkthroughs: { id: string; steps: { id: string }[] }[] };
		};
		const walkthroughs = installed.contributes.walkthroughs;
		assert.equal(walkthroughs.length, 1);
		assert.equal(walkthroughs[0].id, WALKTHROUGH_FIRST_SESSION);
		assert.deepEqual(
			walkthroughs[0].steps.map((step) => step.id),
			[WALKTHROUGH_STEP_READ_GUIDE],
		);

		await vscode.commands.executeCommand(
			"workbench.action.openWalkthrough",
			`${EXTENSION_ID}#${WALKTHROUGH_FIRST_SESSION}`,
		);
	});
});
