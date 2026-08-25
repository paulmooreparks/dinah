// A workspace folder that is itself a workbench.

import * as assert from "node:assert/strict";

import { api, extension, folder, resolution, until } from "./support";

suite("a workspace containing a workbench", () => {
	test("the extension activates on its own", async () => {
		// The activation event is workspaceContains:**/workbench.md, and
		// `dinah init` puts the anchor at .dinah/<id>/workbench.md. Nothing in
		// this test asks the extension to activate.
		const woke = await until(() => extension().isActive, 30_000);
		assert.equal(woke, true, "the extension did not activate on a folder holding a workbench");
	});

	test("it reports a usable binary and one resolved workbench", async () => {
		const reported = await api();
		assert.equal(reported.binary.state, "ok");
		const resolved = resolution(reported);
		assert.equal(resolved.state, "ok");
		if (resolved.state !== "ok") {
			return;
		}
		assert.ok(resolved.root.length > 0);
		assert.equal(resolved.profile, "dinah-core/0.4");
		assert.equal(resolved.insideWorkspace, true);
		assert.ok(resolved.root.startsWith(folder()));
	});

	test("the status bar shows the workbench and leads with its root", async () => {
		const reported = await api();
		const resolved = resolution(reported);
		if (resolved.state !== "ok") {
			assert.fail("expected a resolved workbench");
		}
		assert.ok(reported.statusText.startsWith("$(checklist) "));
		assert.ok(!reported.statusText.includes("$(warning)"));
		assert.equal(reported.statusTooltip.split("\n")[0], resolved.root);
	});
});
