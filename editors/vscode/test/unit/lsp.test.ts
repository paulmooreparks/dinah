// The client half of `dinah lsp`: the inlay-hint suppression, and the chip
// drawn from the structured annotation instead.
//
// dinah-515 criterion 14 is the first test here, and it pins both halves on
// purpose. An empty answer from the server would satisfy the suppression
// clause on its own and prove nothing, so the middleware's own view of what
// the server answered is observed and asserted nonzero beside the zero that
// reaches the editor.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { AnnotationsAnswer, DinahAnnotation } from "../../src/lsp";
import {
	ANNOTATIONS_CHANGED,
	ANNOTATIONS_REQUEST,
	LSP_SECTION,
	chipTextOf,
	chipsFrom,
	suppressInlayHints,
	toneOf,
} from "../../src/lsp";

/// One inlay hint, shaped the way the protocol shapes one. The middleware is
/// generic over the hint type, so the test needs no vscode type for it.
interface Hint {
	label: string;
}

/// annotation builds one annotation of a named kind, with the fields the
/// server publishes for it.
function annotation(fields: Record<string, string>, target: boolean = true): DinahAnnotation {
	return {
		range: { start: { line: 0, character: 0 }, end: { line: 0, character: 12 } },
		label: "A card, in Implement",
		tooltip: "`fx-1`\n\nA card, in Implement\n\non a workbench",
		target: target ? { kind: "card", id: "0dff709e0f3c", ref: "fx-1", card: null } : null,
		fields,
	};
}

test("the middleware asks the server and delivers none of what it answered", async () => {
	// The fixture document's own hints, which a real server answers from a
	// model it has already computed.
	const answered: Hint[] = [{ label: "A card, in Implement" }, { label: "Spec" }];
	let asked = 0;
	let observed = -1;
	const middleware = suppressInlayHints<Hint>((delivered) => {
		observed = delivered;
	});
	const next = async (): Promise<Hint[]> => {
		asked += 1;
		return answered;
	};

	const delivered = await middleware.provideInlayHints(
		{ uri: "file:///fx/card.md" },
		null,
		null,
		next as unknown as Parameters<typeof middleware.provideInlayHints>[3],
	);

	// The accepting case: the server was asked, and it answered hints.
	assert.equal(asked, 1, "the middleware did not ask the server at all");
	assert.ok(answered.length > 0, "the fixture answered no hints, so the suppression proves nothing");
	assert.equal(observed, 2, "the middleware saw a different number of hints than the server answered");
	// The refusing case beside it: none of them reached the editor.
	assert.deepEqual(delivered, [], "a standard hint reached the editor beside the chip");
});

test("a chip is toned from the canonical tokens and never from the label", () => {
	// Every row reads one token out of the fields map. The label is the same
	// sentence in all of them, so a client reading the sentence would tone
	// them alike and this table would collapse.
	const rows: { name: string; fields: Record<string, string>; target: boolean; want: string }[] = [
		{ name: "a blocked card", fields: { state: "blocked" }, target: true, want: "blocked" },
		{ name: "an active card", fields: { state: "active" }, target: true, want: "active" },
		{ name: "a ready card", fields: { state: "ready" }, target: true, want: "plain" },
		{ name: "a column holding on the way out", fields: { hold: "out" }, target: true, want: "holding" },
		{ name: "a column holding both ways", fields: { hold: "both" }, target: true, want: "holding" },
		{ name: "a column holding neither way", fields: {}, target: true, want: "plain" },
		{ name: "a value naming nothing", fields: {}, target: false, want: "unresolved" },
	];
	let swept = 0;
	for (const row of rows) {
		assert.equal(toneOf(annotation(row.fields, row.target)), row.want, row.name);
		swept += 1;
	}
	assert.equal(swept, rows.length);
	assert.ok(swept > 0, "the table swept nothing");
});

test("the chip's brackets are the client's and the label is the server's", () => {
	const drawn = chipTextOf(annotation({ state: "ready" }));
	assert.ok(drawn.includes("A card, in Implement"), "the chip does not carry the server's label");
	assert.ok(drawn.startsWith("⟨") && drawn.endsWith("⟩"), "the chip carries no brackets of its own");
	// The server's own label carries neither bracket, so the rendering is the
	// client's decision rather than a format the server publishes.
	assert.ok(!annotation({}).label.includes("⟨"));
});

test("an answer turns into one chip per annotation, and an empty answer into none", () => {
	const answer: AnnotationsAnswer = {
		annotations: [annotation({ state: "ready" }), annotation({ hold: "out" })],
	};
	assert.equal(chipsFrom(answer).length, 2);
	assert.equal(chipsFrom({ annotations: [] }).length, 0);
	assert.equal(chipsFrom(null).length, 0);
});

test("the namespaced method names and the settings section are spelled once", () => {
	assert.equal(ANNOTATIONS_REQUEST, "dinah/annotations");
	assert.equal(ANNOTATIONS_CHANGED, "dinah/annotationsChanged");
	assert.equal(LSP_SECTION, "dinah.lsp");
});
