// Where the dinah binary comes from.
//
// Two rungs, and the answer says which one produced it:
//
//   1. the dinah.path setting               source "setting"
//   2. the bare name `dinah`, resolved by the operating system   source "path"
//
// Neither answering means no binary, and the welcome view says where to get
// one. The extension is a companion to the CLI and carries no copy of it, so
// there is no third rung inside the installed extension to fall back to.
//
// Rung 2 spawns rather than searching PATH itself, because a PATH search
// written here would be a second implementation of something the operating
// system already does, and it would get the Windows PATHEXT rules subtly
// wrong.

import type { BinaryState, BinarySource } from "./api";
import type { Classification } from "./version";

/** The bare name rung 2 spawns and leaves the operating system to resolve. */
export const PATH_NAME = "dinah";

/** Everything the resolver reaches the outside world through. */
export interface ResolveDeps {
	/** The dinah.path setting, empty when unset. */
	readonly setting: string;
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

/**
 * Walks the ladder and reports which rung answered.
 *
 * The setting is terminal: a user who named a binary gets an answer about that
 * binary, including a bad one, rather than a silent fall-through to a
 * different build than the one they asked for.
 *
 * On PATH, only "not on the PATH" means no binary. Every other classification,
 * including a skew and a spawn failure such as a permission denial, is a real
 * answer about a real file and is reported rather than stepped over.
 */
export async function resolveBinary(deps: ResolveDeps): Promise<BinaryState> {
	if (deps.setting.trim() !== "") {
		const path = deps.setting.trim();
		return toState(await deps.probe(path), path, "setting");
	}

	const onPath = await deps.probe(PATH_NAME);
	if (onPath.kind === "enoent") {
		return { state: "no-binary" };
	}
	return toState(onPath, PATH_NAME, "path");
}
