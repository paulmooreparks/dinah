// Packages the vsix artifact.
//
// There is one archive and it carries no binary. The extension is a companion
// to the dinah command-line tool, so a local build, a pull-request CI run and
// a marketplace publish all produce the same universal artifact.
//
// The version inside the archive is told to this script rather than chosen by
// it. `--version 1.4.7` is the release path: the release workflow computes the
// number from the release history before anything is packaged, and
// publish-extension.ps1 reads the newest release's own number, so both hand it
// over explicitly. With no --version the archive carries whatever package.json
// commits to, untouched, which is what the CI sanity build wants: it proves
// packaging works and the archive goes nowhere.
//
// The manifest is rewritten and put back around the packaging call on the
// release path, so nothing downstream may read package.json's version back out
// afterwards to learn what was packaged. By then the file says the floor again.

import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { UNIVERSAL } from "./targets.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");
const binDir = join(extensionRoot, "bin");
const outDir = join(extensionRoot, "vsix");
const manifestPath = join(extensionRoot, "package.json");

/** The shape a version has to have before it goes into an archive. */
const VERSION = /^\d+\.\d+\.\d+$/;

/**
 * The version this run packages.
 *
 * Returns the manifest's own committed version, untouched, when argv carries no
 * --version. Any other flag argv still carries is not read here at all, so an
 * old caller passing --published selects nothing and gets the committed version
 * exactly as a caller passing no flags does.
 *
 * Throws, naming the offending value, when --version is present and its value
 * is not a major.minor.patch string. A bad number reaches the marketplace as a
 * refused upload several steps later, so it is refused here instead.
 *
 * @param {{argv: string[], manifestVersion: string}} options
 * @returns {string}
 */
export function resolvePackageVersion({ argv, manifestVersion }) {
	let given;
	let seen = false;
	for (let i = 0; i < argv.length; i += 1) {
		if (argv[i] === "--version") {
			seen = true;
			given = argv[i + 1];
			break;
		}
		if (argv[i].startsWith("--version=")) {
			seen = true;
			given = argv[i].slice("--version=".length);
			break;
		}
	}
	if (!seen) {
		return manifestVersion;
	}
	if (given === undefined) {
		throw new Error("--version was given no value, so nothing was packaged");
	}
	if (!VERSION.test(given)) {
		throw new Error(
			`--version was given ${JSON.stringify(given)}, which is not a major.minor.patch version, so nothing was packaged`,
		);
	}
	return given;
}

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

function main() {
	// Nothing stages a binary here any more. The wipe stays because a bin/
	// directory left by a local build predating this change would be packaged
	// like any other file, and wiping a directory this step must never populate
	// costs nothing.
	rmSync(binDir, { recursive: true, force: true });

	rmSync(outDir, { recursive: true, force: true });
	mkdirSync(outDir, { recursive: true });

	const manifestBefore = readFileSync(manifestPath, "utf8");
	const committed = JSON.parse(manifestBefore).version;
	const version = resolvePackageVersion({ argv: process.argv.slice(2), manifestVersion: committed });
	const rewritten = version !== committed;

	const vsix = join(outDir, "dinah-universal.vsix");

	try {
		if (rewritten) {
			const manifest = JSON.parse(manifestBefore);
			manifest.version = version;
			writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
		}

		vsce(["package", "--pre-release", "--out", vsix]);
	} finally {
		// The committed manifest is put back whether the packaging worked or not,
		// so a failed run never leaves a checkout claiming a version it does not
		// have.
		if (rewritten) {
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
}

// The packaging runs only when this file is the program, so a test importing
// resolvePackageVersion does not package anything. Node documents process.argv[1]
// as the path of the JavaScript file being executed, and pathToFileURL turns it
// into the same spelling import.meta.url carries.
if (process.argv[1] !== undefined && pathToFileURL(process.argv[1]).href === import.meta.url) {
	main();
}
