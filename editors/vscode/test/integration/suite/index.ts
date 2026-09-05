// The entry point VS Code loads inside the extension host.
//
// Which suite runs is decided by DINAH_SUITE, because each one needs its own
// window opened on its own fixture workspace.

import { join } from "node:path";

import Mocha from "mocha";

export function run(): Promise<void> {
	const suite = process.env.DINAH_SUITE;
	if (!suite) {
		return Promise.reject(
			new Error("DINAH_SUITE is not set, so no suite knows it is the one to run"),
		);
	}

	const mocha = new Mocha({ ui: "tdd", color: true, timeout: 60_000 });
	mocha.addFile(join(__dirname, `${suite}.test.js`));

	let runner: Mocha.Runner | undefined;

	return new Promise<void>((resolve, reject) => {
		try {
			runner = mocha.run((failures) => {
				if (failures > 0) {
					reject(new Error(`${String(failures)} test(s) failed in suite ${suite}`));
					return;
				}
				// Zero failures out of zero tests is what a suite reports when
				// its file loaded and registered nothing, and every launch in
				// runTest.ts would then be green while asserting nothing. The
				// unit layer had this same hole and CI found it the expensive
				// way, so the count is checked on both sides. A declared skip
				// still counts as a test here, which is deliberate: a suite
				// that says what it is not running is not the failure this
				// catches.
				const executed = runner?.stats?.tests ?? 0;
				if (executed === 0) {
					reject(
						new Error(
							`suite ${suite} ran 0 tests, so it asserted nothing and cannot be reported as passing`,
						),
					);
					return;
				}
				resolve();
			});
		} catch (err) {
			reject(err instanceof Error ? err : new Error(String(err)));
		}
	});
}
