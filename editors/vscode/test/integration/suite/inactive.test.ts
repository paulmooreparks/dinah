// A window with nothing for Dinah to do.
//
// This is the only mechanical guard against onStartupFinished or * being
// quietly added to activationEvents later. Adding either turns this red, and
// nothing else would notice: an extension that activates on every window is
// rude in a way no user reports and no other test observes.

import * as assert from "node:assert/strict";

import { extension } from "./support";

suite("a workspace with no Dinah content", () => {
	test("the extension stays inactive after the host settles", async () => {
		// The host is given real time to activate anything it means to, and
		// then asked. Asserting immediately would pass against an extension
		// that was about to wake up.
		await new Promise((resolve) => setTimeout(resolve, 8_000));
		assert.equal(
			extension().isActive,
			false,
			"the extension activated in a window holding no workbench and no Dinah view",
		);
	});
});
