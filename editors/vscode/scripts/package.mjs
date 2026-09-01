// Packages the vsix artifacts.
//
// With no --binaries directory this packages the universal artifact alone,
// which is what a local build and a pull-request CI run want. With one, it
// stages exactly one binary per platform target and packages all seven, which
// is what a release wants.
//
// Staging is a wipe and a copy rather than a copy, because a bin/ directory
// left over from the previous target would ship two binaries in one vsix and
// the archive check would be the only thing that noticed.
//
// The version inside the archives depends on where they are going. Pass
// --published, which only the marketplace publish script does, and they carry
// the number package.json commits to. Otherwise they carry an unpublished
// ordinal that sorts below every release, which scripts/version.mjs computes
// and explains.

import { execFileSync } from "node:child_process";
import {
	copyFileSync,
	existsSync,
	mkdirSync,
	readFileSync,
	rmSync,
	writeFileSync,
} from "node:fs";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { PLATFORM_TARGETS, UNIVERSAL } from "./targets.mjs";
import { unpublishedVersion } from "./version.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");
const repoRoot = join(extensionRoot, "..", "..");
const binDir = join(extensionRoot, "bin");
const outDir = join(extensionRoot, "vsix");
const manifestPath = join(extensionRoot, "package.json");

function argValue(name) {
	const at = process.argv.indexOf(name);
	if (at < 0 || at + 1 >= process.argv.length) {
		return undefined;
	}
	return process.argv[at + 1];
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

function stage(binaryPath) {
	rmSync(binDir, { recursive: true, force: true });
	if (binaryPath === undefined) {
		return;
	}
	mkdirSync(binDir, { recursive: true });
	copyFileSync(binaryPath, join(binDir, binaryPath.split(/[\\/]/).pop()));
	// A .gitignore inside the staging directory would be packaged too. The
	// directory is ignored from the repository root instead, so nothing but
	// the binary is ever in here.
}

function packageOne(target, binaryPath) {
	stage(binaryPath);
	const name = target === UNIVERSAL ? "dinah-universal.vsix" : `dinah-${target}.vsix`;
	const args = ["package", "--pre-release", "--out", join(outDir, name)];
	if (target !== UNIVERSAL) {
		args.push("--target", target);
	}
	vsce(args);
	return join(outDir, name);
}

const binariesArg = argValue("--binaries");
const binaries = binariesArg
	? isAbsolute(binariesArg)
		? binariesArg
		: resolve(process.cwd(), binariesArg)
	: undefined;

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });

// Which version goes inside the archives. --published means these archives are
// the ones going to the marketplace, so they carry the number package.json
// commits to. Everything else is an unpublished archive, from a local build or
// from CI, and carries an ordinal on the 0.0.x line so that installing one by
// hand can never look newer than a release. See scripts/version.mjs.
const published = process.argv.includes("--published");
const manifestBefore = readFileSync(manifestPath, "utf8");
const version = published
	? JSON.parse(manifestBefore).version
	: unpublishedVersion({ repoRoot });

const produced = [];

try {
	if (!published) {
		const manifest = JSON.parse(manifestBefore);
		manifest.version = version;
		writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
	}

	if (binaries) {
		for (const { target, binary } of PLATFORM_TARGETS) {
			const path = join(binaries, binary);
			if (!existsSync(path)) {
				throw new Error(
					`the release binary ${binary} is not in ${binaries}; a vsix for ${target} would carry nothing and would be worse than no vsix at all`,
				);
			}
			produced.push({ target, vsix: packageOne(target, path) });
		}
	}

	produced.push({ target: UNIVERSAL, vsix: packageOne(UNIVERSAL, undefined) });
} finally {
	// The committed manifest is put back whether the packaging worked or not,
	// so a failed run never leaves a checkout claiming a version it does not
	// have.
	if (!published) {
		writeFileSync(manifestPath, manifestBefore, "utf8");
	}
}

// Leave nothing staged. A local `npm run package` that left a binary in bin/
// would put it into the next artifact somebody built by hand.
rmSync(binDir, { recursive: true, force: true });

writeFileSync(join(outDir, "manifest.json"), `${JSON.stringify(produced, null, 2)}\n`, "utf8");
process.stdout.write(`packaged version ${version}\n`);
for (const { target, vsix } of produced) {
	process.stdout.write(`${target}: ${vsix}\n`);
}
