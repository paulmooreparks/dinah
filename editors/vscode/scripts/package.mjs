// Packages the vsix artifact.
//
// There is one archive and it carries no binary. The extension is a companion
// to the dinah command-line tool, so a local build, a pull-request CI run and
// a marketplace publish all produce the same universal artifact.
//
// The version inside the archive depends on where it is going. Pass
// --published, which only the marketplace publish script does, and it carries
// the number package.json commits to. Otherwise it carries an unpublished
// ordinal that sorts below every release, which scripts/version.mjs computes
// and explains.

import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { UNIVERSAL } from "./targets.mjs";
import { unpublishedVersion } from "./version.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");
const repoRoot = join(extensionRoot, "..", "..");
const binDir = join(extensionRoot, "bin");
const outDir = join(extensionRoot, "vsix");
const manifestPath = join(extensionRoot, "package.json");

// vsce's own entry point, run through this Node rather than through npx.
// Node 22 and later refuse to spawn a .cmd shim without a shell, and a shell
// would leave the arguments unescaped, so the module is named directly.
const VSCE = join(extensionRoot, "node_modules", "@vscode", "vsce", "vsce");

function vsce(args) {
	execFileSync(process.execPath, [VSCE, ...args], {
		cwd: extensionRoot,
		stdio: "inherit",
	});
}

// Nothing stages a binary here any more. The wipe stays because a bin/
// directory left by a local build predating this change would be packaged
// like any other file, and wiping a directory this step must never populate
// costs nothing.
rmSync(binDir, { recursive: true, force: true });

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });

// Which version goes inside the archive. --published means this archive is the
// one going to the marketplace, so it carries the number package.json commits
// to. Everything else is an unpublished archive, from a local build or from
// CI, and carries an ordinal on the 0.0.x line so that installing one by hand
// can never look newer than a release. See scripts/version.mjs.
const published = process.argv.includes("--published");
const manifestBefore = readFileSync(manifestPath, "utf8");
const version = published
	? JSON.parse(manifestBefore).version
	: unpublishedVersion({ repoRoot });

const vsix = join(outDir, "dinah-universal.vsix");

try {
	if (!published) {
		const manifest = JSON.parse(manifestBefore);
		manifest.version = version;
		writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
	}

	vsce(["package", "--pre-release", "--out", vsix]);
} finally {
	// The committed manifest is put back whether the packaging worked or not,
	// so a failed run never leaves a checkout claiming a version it does not
	// have.
	if (!published) {
		writeFileSync(manifestPath, manifestBefore, "utf8");
	}
}

writeFileSync(
	join(outDir, "manifest.json"),
	`${JSON.stringify([{ target: UNIVERSAL, vsix }], null, 2)}\n`,
	"utf8",
);
process.stdout.write(`packaged version ${version}\n`);
process.stdout.write(`${UNIVERSAL}: ${vsix}\n`);
