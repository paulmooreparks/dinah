// Where the dinah binary comes from.
//
// Four rungs, and the answer says which one produced it:
//
//   1. the dinah.path setting               source "setting"
//   2. the bare name `dinah`, resolved by the operating system   source "path"
//   3. the binary carried inside this extension                  source "carried"
//   4. nothing, and the welcome view says so
//
// Rung 2 spawns rather than searching PATH itself, because a PATH search
// written here would be a second implementation of something the operating
// system already does, and it would get the Windows PATHEXT rules subtly
// wrong.
//
// PATH beats the carried copy deliberately: a user who has installed dinah has
// a binary their terminal uses, and an extension driving a different build is a
// split brain. The single exception is announced rather than silent, and it is
// the demotion below.

import type { BinaryState, BinarySource, VersionReport } from "./api";
import { demotes, describeVersion } from "./version";
import type { Classification } from "./version";

/** The bare name rung 2 spawns and leaves the operating system to resolve. */
export const PATH_NAME = "dinah";

/** Everything the resolver reaches the outside world through. */
export interface ResolveDeps {
	/** The dinah.path setting, empty when unset. */
	readonly setting: string;
	/** The bin/ directory inside the installed extension. */
	readonly carriedDir: string;
	/**
	 * Lists the carried directory. It must reject with a code of "ENOENT"
	 * when the directory does not exist: `editors/vscode/bin/` is gitignored
	 * and git tracks no empty directory, so on the universal vsix, which
	 * carries no binary at all, the directory is absent rather than empty.
	 * That is rung 4, not an error, and every universal install reaches it.
	 */
	readonly listCarried: (dir: string) => Promise<string[]>;
	/** Joins a directory and a file name the way the host platform spells paths. */
	readonly join: (dir: string, name: string) => string;
	/**
	 * Makes a carried binary executable. A zip entry's mode does not survive
	 * VS Code's install on Linux and macOS, so without this every carried
	 * binary is unusable on exactly the platforms it was carried for.
	 */
	readonly ensureExecutable: (path: string) => Promise<void>;
	/** Runs `--json version` against one candidate and classifies the answer. */
	readonly probe: (exe: string) => Promise<Classification>;
}

/** Turns a classification into the API's binary arm for a named candidate. */
function toState(
	classification: Classification,
	path: string,
	source: BinarySource,
): BinaryState {
	switch (classification.kind) {
		case "ok":
			return {
				state: "ok",
				path,
				source,
				version: classification.version,
			};
		case "format-skew":
		case "profile-skew":
		case "binary-too-old":
			return {
				state: classification.kind,
				path,
				detail: classification.detail,
				version: classification.version,
			};
		case "no-binary":
			return { state: "no-binary" };
		case "refused":
			return {
				state: "unusable",
				path,
				detail: `${path} refused to answer: ${classification.refusal} ${classification.detail}`,
			};
		case "unreachable":
			return {
				state: "unusable",
				path,
				detail: `${path} could not answer: ${classification.detail}`,
			};
		case "enoent":
		case "unusable":
			return { state: "unusable", path, detail: classification.detail };
	}
}

/** A one-line description of whatever a classification found, skewed or not. */
function describeClassification(classification: Classification): string {
	const version = (classification as { version?: VersionReport }).version;
	if (version) {
		return describeVersion(version);
	}
	return (classification as { detail?: string }).detail ?? classification.kind;
}

/**
 * Composes the diagnostic for the one announced demotion, naming both paths
 * and both versions so a user can see which build they lost and which they
 * got.
 */
export function composeDemotion(
	pathClassification: Classification,
	carriedPath: string,
	carriedVersion: VersionReport,
): string {
	return (
		`The dinah on PATH (${PATH_NAME}) reports ${describeClassification(pathClassification)}, ` +
		`which this extension will not drive. Using the binary carried with the extension instead: ` +
		`${carriedPath}, ${describeVersion(carriedVersion)}.`
	);
}

/** Reads the sole carried binary's path, or undefined when none is carried. */
export async function findCarried(
	deps: ResolveDeps,
): Promise<string | undefined> {
	let entries: string[];
	try {
		entries = await deps.listCarried(deps.carriedDir);
	} catch (err) {
		// The universal vsix carries no bin/ directory at all. That is rung 4.
		if ((err as NodeJS.ErrnoException).code === "ENOENT") {
			return undefined;
		}
		throw err;
	}
	// The staged file keeps the name release.yml published it under, so
	// `dinah-linux-amd64` rather than `dinah`. Reading the directory rather
	// than composing a name from process.platform means a rename in
	// release.yml cannot silently produce a vsix whose binary this extension
	// fails to find.
	const carried = entries.filter((name) => name.startsWith("dinah"));
	if (carried.length !== 1) {
		return undefined;
	}
	return deps.join(deps.carriedDir, carried[0]);
}

/** Probes the carried binary, chmodding it first. */
async function probeCarried(
	deps: ResolveDeps,
): Promise<{ path: string; classification: Classification } | undefined> {
	const path = await findCarried(deps);
	if (path === undefined) {
		return undefined;
	}
	await deps.ensureExecutable(path);
	return { path, classification: await deps.probe(path) };
}

/**
 * Walks the ladder and reports which rung answered.
 *
 * The setting is terminal: a user who named a binary gets an answer about that
 * binary, including a bad one, rather than a silent fall-through to a
 * different build than the one they asked for.
 */
export async function resolveBinary(deps: ResolveDeps): Promise<BinaryState> {
	if (deps.setting.trim() !== "") {
		const path = deps.setting.trim();
		return toState(await deps.probe(path), path, "setting");
	}

	const onPath = await deps.probe(PATH_NAME);
	if (onPath.kind === "ok") {
		return toState(onPath, PATH_NAME, "path");
	}

	if (demotes(onPath)) {
		const carried = await probeCarried(deps);
		if (carried && carried.classification.kind === "ok") {
			return {
				state: "ok",
				path: carried.path,
				source: "carried",
				version: carried.classification.version,
				demotedFrom: composeDemotion(
					onPath,
					carried.path,
					carried.classification.version,
				),
			};
		}
		return toState(onPath, PATH_NAME, "path");
	}

	if (onPath.kind !== "enoent") {
		// A spawn that failed for a reason other than "not on the PATH", such
		// as a permission denial or a binary built for another architecture,
		// is a real answer about a real file and is reported rather than
		// stepped over.
		return toState(onPath, PATH_NAME, "path");
	}

	const carried = await probeCarried(deps);
	if (!carried) {
		return { state: "no-binary" };
	}
	return toState(carried.classification, carried.path, "carried");
}
