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

	return new Promise<void>((resolve, reject) => {
		try {
			mocha.run((failures) => {
				if (failures > 0) {
					reject(new Error(`${String(failures)} test(s) failed in suite ${suite}`));
				} else {
					resolve();
				}
			});
		} catch (err) {
			reject(err instanceof Error ? err : new Error(String(err)));
		}
	});
}
