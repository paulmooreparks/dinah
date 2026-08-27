// The workbench is the parent of the opened folder.
//
// This is the dinah-241 shape: the walk climbs past the folder the user opened
// and lands on a workbench above it. The extension does not try to be safer
// than the CLI, because a second discovery rule would put the editor and the
// terminal on different workbenches. It makes the answer visible instead.

import * as assert from "node:assert/strict";
import { dirname } from "node:path";

import { api, folder, resolution } from "./support";
import { isInside } from "../../../src/workbench";

suite("a workbench above the opened folder", () => {
	test("it resolves to the parent's workbench through the search rung", async () => {
		// An extension that reimplemented the climb and bounded it at the
		// workspace folder reports nothing here and fails on this assertion.
		const resolved = resolution(await api());
		assert.equal(resolved.state, "ok");
		if (resolved.state !== "ok") {
			return;
		}
		assert.equal(resolved.source, "search");

		const opened = folder();
		const parent = dirname(opened);
		assert.ok(
			isInside(resolved.root, parent, process.platform === "win32"),
			`expected ${resolved.root} to be under the parent ${parent}`,
		);
		assert.equal(
			isInside(resolved.root, opened, process.platform === "win32"),
			false,
			"the resolved workbench should not be inside the opened folder",
		);
		assert.equal(resolved.insideWorkspace, false);
	});

	test("the tooltip leads with the absolute resolved path", async () => {
		// An extension that showed only the workbench title passes everything
		// above and fails here, which is the visibility half of the rule.
		const reported = await api();
		const resolved = resolution(reported);
		if (resolved.state !== "ok") {
			assert.fail("expected a resolved workbench");
		}
		const first = reported.statusTooltip.split("\n")[0];
		assert.ok(
			first.includes(resolved.root),
			`expected the tooltip to lead with ${resolved.root}, got: ${first}`,
		);
		assert.ok(reported.statusText.includes("$(warning)"));
	});
});
