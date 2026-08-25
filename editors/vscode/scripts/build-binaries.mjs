// Cross-compiles the six release binaries into a directory, so the packaging
// check has six real archives to read.
//
// This is not how a release is built. release.yml builds the six with the
// release tag injected through -ldflags and publishes them; this builds the
// same six without a tag, purely so that CI can package all seven vsix
// artifacts on an ordinary pull request and assert that each one carries
// exactly the binary it should.

import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { PLATFORM_TARGETS } from "./targets.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..", "..");

const requested = process.argv[2];
if (!requested) {
	throw new Error("usage: node scripts/build-binaries.mjs <output directory>");
}
const outDir = isAbsolute(requested) ? requested : resolve(process.cwd(), requested);

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });

for (const { binary, goos, goarch } of PLATFORM_TARGETS) {
	execFileSync(
		"go",
		["build", "-trimpath", "-o", join(outDir, binary), "./cmd/dinah"],
		{
			cwd: repoRoot,
			stdio: "inherit",
			env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" },
		},
	);
	process.stdout.write(`built ${binary}\n`);
}
