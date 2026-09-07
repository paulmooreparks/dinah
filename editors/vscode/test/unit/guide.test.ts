// The guide the first-session walkthrough opens, driven without an editor.
//
// dinah-423 AC-3 and AC-4. Two claims live here. A guide's successful stdout
// is Markdown rather than JSON, so the text-returning path has to hand those
// bytes back untouched instead of reporting a binary that is not dinah; and a
// guide that cannot be fetched has to reach the reader as a sentence, because
// the walkthrough is the first thing a new reader touches and a blank step
// tells them nothing.
//
// The second claim is composed here out of the same three pieces the content
// provider's catch composes it from: the outcome, refusalMessage, and the
// servedText.refused entry. Nothing in the unit layer can open a tab, so this
// asserts what the provider would put in one.

import assert from "node:assert/strict";
import { test } from "node:test";

import { refusalMessage } from "../../src/cardCommands";
import { runDinahText } from "../../src/cli";
import type { SpawnOptions, SpawnOutcome, Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import { GUIDE_TOPIC_FIRST_SESSION, KIND_GUIDE } from "../../src/servedText";

/** A spawner that records what it was asked to run and replays one outcome. */
function stub(outcome: SpawnOutcome): {
	spawner: Spawner;
	calls: { exe: string; argv: string[]; options: SpawnOptions }[];
} {
	const calls: { exe: string; argv: string[]; options: SpawnOptions }[] = [];
	const spawner: Spawner = async (exe, argv, options) => {
		calls.push({ exe, argv: [...argv], options });
		return outcome;
	};
	return { spawner, calls };
}

/** The refusal envelope `dinah --json guide <unknown topic>` writes on exit 2. */
const UNKNOWN_GUIDE = JSON.stringify({
	outcome: "refused",
	refusal: "dinah.unknown-guide",
	detail: "no guide is embedded under that name",
	context: { topic: "bogus" },
});

test("the guide is asked for by topic alone, with no workbench and no cwd", async () => {
	// AC-3's first half. `dinah guide` opens no workbench, so pinning one
	// would be an argument the verb ignores, and pinning a cwd would tie a
	// document that lives inside the binary to a directory on disk. The
	// window this walkthrough is written for has neither to offer.
	const { spawner, calls } = stub({ code: 0, stdout: "# Guide\n", stderr: "" });
	await runDinahText(spawner, "dinah", ["guide", GUIDE_TOPIC_FIRST_SESSION]);
	assert.deepEqual(calls[0].argv, ["--json", "guide", "first-session"]);
	assert.ok(
		!calls[0].argv.includes("--workbench"),
		"the guide was asked for against a workbench",
	);
	assert.equal(calls[0].options.cwd, undefined);
});

test("exit 0 comes back as the bytes dinah wrote, whatever they are", async () => {
	// AC-3's second half, and the reason this path exists at all. runDinah
	// would parse this stdout, fail, and report that the binary is not dinah.
	// The prose below is deliberately not JSON: it is what a guide looks like.
	const markdown = "# First session\n\nRun `dinah init` here.\n\n- a bullet\n";
	const { spawner } = stub({ code: 0, stdout: markdown, stderr: "" });
	const outcome = await runDinahText(spawner, "dinah", ["guide", "first-session"]);
	assert.equal(outcome.kind, "ok");
	if (outcome.kind !== "ok") {
		return;
	}
	assert.equal(outcome.text, markdown);
});

test("a refused guide and a missing binary both reach the reader as a sentence", async () => {
	// AC-4. Both failures are composed the way the content provider's catch
	// composes them, and neither may come out blank. A tab that opened empty
	// would tell a reader the guide is empty, which is the failure shape this
	// card was told to avoid.
	const cases: { name: string; outcome: SpawnOutcome }[] = [
		{
			name: "an unknown topic",
			outcome: { code: 2, stdout: UNKNOWN_GUIDE, stderr: "" },
		},
		{
			name: "no binary at all",
			outcome: {
				code: null,
				stdout: "",
				stderr: "",
				spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
			},
		},
	];
	for (const one of cases) {
		const { spawner } = stub(one.outcome);
		const outcome = await runDinahText(spawner, "dinah", ["guide", "first-session"]);
		assert.notEqual(outcome.kind, "ok", `${one.name} came back as a success`);
		if (outcome.kind === "ok") {
			return;
		}
		const detail = refusalMessage(outcome);
		assert.notEqual(detail.trim(), "", `${one.name} produced an empty message`);
		const body = ENGLISH("servedText.refused", { detail });
		assert.notEqual(body.trim(), "", `${one.name} rendered an empty tab body`);
		assert.ok(
			body.includes(detail),
			`${one.name}: the tab body drops what dinah said: ${body}`,
		);
	}
});

test("the kind the guide is served under is its own, and not the instruction chain's", () => {
	// The provider dispatches on this string, so a guide URI colliding with
	// the instruction chain's kind would fetch the wrong document entirely.
	assert.equal(KIND_GUIDE, "guide");
	assert.notEqual(KIND_GUIDE, "instructions");
});
