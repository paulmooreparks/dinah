// The binary carried inside the extension, arriving without its executable
// bit.
//
// A vsix is a zip, and a zip entry's mode does not survive VS Code's install
// on Linux or macOS. Without the chmod, every carried binary is unusable on
// exactly the platforms it was carried for, and the extension would report
// "no dinah" on a machine that has one inside it.

import * as assert from "node:assert/strict";
import { statSync } from "node:fs";

import { api } from "./support";

suite("a carried binary whose executable bit was stripped", () => {
	test("the resolver chmods it and its version call succeeds", async function () {
		if (process.platform === "win32") {
			// Windows has no executable bit, so there is nothing to strip and
			// nothing to restore. The rung itself is covered by the unit tests
			// on both platforms.
			this.skip();
			return;
		}
		const reported = await api();
		assert.equal(reported.binary.state, "ok");
		if (reported.binary.state !== "ok") {
			return;
		}
		assert.equal(reported.binary.source, "carried");
		assert.equal(reported.binary.version.profile, "dinah-core/0.4");
		assert.equal(reported.binary.version.format, 1);

		const mode = statSync(reported.binary.path).mode;
		assert.equal(
			(mode & 0o111) === 0o111,
			true,
			`the carried binary at ${reported.binary.path} is still not executable`,
		);
	});
});
