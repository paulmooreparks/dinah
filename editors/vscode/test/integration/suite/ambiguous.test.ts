// One .dinah container holding two workbenches.

import * as assert from "node:assert/strict";

import { api, resolution } from "./support";
import { AMBIGUOUS_WORKBENCH } from "../../../src/workbench";

suite("several workbenches reachable from the opened folder", () => {
	test("it chooses nothing and reports both candidates", async () => {
		// Picking the first candidate is the failure this catches, and it
		// would pass any test that only checked a workbench was found.
		const resolved = resolution(await api());
		assert.equal(resolved.state, "refused");
		if (resolved.state !== "refused") {
			return;
		}
		assert.equal(resolved.refusal, AMBIGUOUS_WORKBENCH);
		assert.ok(!("root" in resolved), "a refused resolution must carry no root");

		const candidates = resolved.candidates ?? [];
		assert.equal(candidates.length, 2);
		const slugs = candidates.map((candidate) => candidate.slug).sort();
		assert.deepEqual(slugs, ["alpha", "beta"]);
		for (const candidate of candidates) {
			assert.ok(candidate.path.length > 0);
		}
	});

	test("the status bar warns and names the candidates rather than a winner", async () => {
		const reported = await api();
		assert.ok(reported.statusText.includes("$(warning)"));
		assert.ok(reported.statusTooltip.includes("Set dinah.workbench to choose one."));
		const resolved = resolution(reported);
		if (resolved.state !== "refused") {
			assert.fail("expected a refusal");
		}
		for (const candidate of resolved.candidates ?? []) {
			assert.ok(
				reported.statusTooltip.includes(candidate.path),
				`expected the tooltip to name ${candidate.path}`,
			);
		}
	});
});
