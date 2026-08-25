// The compatibility gate.
//
// `dinah --json version` reports three fields with three jobs, and this gate
// reads two of them.
//
// `format` is the storage gate. `profile` is the contract gate. `tool` is
// never compared: `verb.ToolRelease` is the literal "0.1.0" in every build
// from source and is overwritten only by a release build's -ldflags, so a
// gate reading it would refuse every contributor's own binary. It is
// displayed and nothing more.

import type { CliOutcome } from "./cli";
import type { VersionReport } from "./api";

/**
 * The storage formats this extension can drive, as a set rather than a floor.
 * A format is a serialisation, not a capability level, so "anything at or
 * above 1" is not a claim this extension can make about a number it has never
 * seen.
 */
export const SUPPORTED_FORMATS: readonly number[] = [1];

/**
 * The conformance claim this extension needs. A different name or major is
 * refused outright. A lower minor is refused, because the binary is older
 * than the fields this extension reads. A higher minor is accepted, because a
 * client that only reads the fields it knows is unharmed by fields it does
 * not.
 */
export const MINIMUM_PROFILE = { name: "dinah-core", major: 0, minor: 4 };

/** Every outcome of asking a candidate binary what it is. */
export type Classification =
	| { readonly kind: "ok"; readonly version: VersionReport }
	/** No candidate existed at this rung at all. */
	| { readonly kind: "no-binary" }
	/**
	 * The spawn failed with ENOENT, which at the PATH rung means only that
	 * this window's environment has no dinah on it. It is a fall-through
	 * rather than an error, and the message says PATH rather than "not
	 * installed", because an extension host does not always inherit the login
	 * shell's PATH.
	 */
	| { readonly kind: "enoent"; readonly detail: string }
	| { readonly kind: "unusable"; readonly detail: string }
	| { readonly kind: "refused"; readonly refusal: string; readonly detail: string }
	| { readonly kind: "unreachable"; readonly detail: string }
	| {
			readonly kind: "format-skew";
			readonly detail: string;
			readonly version: VersionReport;
	  }
	| {
			readonly kind: "profile-skew";
			readonly detail: string;
			readonly version: VersionReport;
	  }
	| {
			readonly kind: "binary-too-old";
			readonly detail: string;
			readonly version: VersionReport;
	  };

/** The three classifications that demote to the carried binary when one exists. */
const DEMOTABLE = new Set(["format-skew", "profile-skew", "binary-too-old"]);

/** Whether this classification lets a lower rung answer instead. */
export function demotes(classification: Classification): boolean {
	return DEMOTABLE.has(classification.kind);
}

/** A one-line description of a binary, for a diagnostic that names two of them. */
export function describeVersion(version: VersionReport): string {
	return `dinah ${version.tool}, ${version.profile}, format ${String(version.format)}`;
}

/** Splits `dinah-core/0.4` into its three parts, or undefined if it will not split. */
export function parseProfile(
	profile: string,
): { name: string; major: number; minor: number } | undefined {
	const slash = profile.lastIndexOf("/");
	if (slash <= 0) {
		return undefined;
	}
	const name = profile.slice(0, slash);
	const numbers = profile.slice(slash + 1).split(".");
	if (numbers.length !== 2) {
		return undefined;
	}
	const major = Number(numbers[0]);
	const minor = Number(numbers[1]);
	if (!Number.isInteger(major) || !Number.isInteger(minor)) {
		return undefined;
	}
	return { name, major, minor };
}

/** Reads the three fields off a parsed `--json version` payload. */
function readReport(json: unknown): VersionReport | undefined {
	const report = json as
		| { tool?: unknown; profile?: unknown; format?: unknown }
		| undefined;
	if (
		!report ||
		typeof report.tool !== "string" ||
		typeof report.profile !== "string" ||
		typeof report.format !== "number"
	) {
		return undefined;
	}
	return { tool: report.tool, profile: report.profile, format: report.format };
}

/**
 * Classifies one candidate binary from the outcome of running
 * `dinah --json version` on it.
 *
 * Every row of the gate's table is its own arm here, so collapsing two of them
 * turns that row's unit test red.
 */
export function classifyVersion(outcome: CliOutcome): Classification {
	switch (outcome.kind) {
		case "spawn-failed":
			if (outcome.errno === "ENOENT") {
				return {
					kind: "enoent",
					detail:
						"no dinah on the PATH this window inherited. A window launched from a desktop launcher does not always inherit the PATH your terminal has; set dinah.path to the binary you want.",
				};
			}
			return { kind: "unusable", detail: outcome.detail };
		case "refused":
			return {
				kind: "refused",
				refusal: outcome.refusal,
				detail: outcome.detail ?? outcome.refusal,
			};
		case "unreachable":
			return { kind: "unreachable", detail: outcome.detail };
		case "stale":
			return { kind: "unusable", detail: outcome.detail };
		case "not-json":
			return { kind: "unusable", detail: outcome.detail };
		case "ok":
			break;
	}

	const version = readReport(outcome.json);
	if (!version) {
		return {
			kind: "unusable",
			detail:
				"this binary is not dinah, or is too old to answer `--json version`",
		};
	}

	if (!SUPPORTED_FORMATS.includes(version.format)) {
		return {
			kind: "format-skew",
			version,
			detail: `this binary writes storage format ${String(version.format)}, and this extension supports ${SUPPORTED_FORMATS.join(", ")}`,
		};
	}

	const profile = parseProfile(version.profile);
	if (!profile) {
		return {
			kind: "profile-skew",
			version,
			detail: `this binary reports the conformance claim "${version.profile}", which is not a claim this extension can read`,
		};
	}
	if (profile.name !== MINIMUM_PROFILE.name || profile.major !== MINIMUM_PROFILE.major) {
		return {
			kind: "profile-skew",
			version,
			detail: `this binary conforms to ${version.profile}, and this extension speaks ${MINIMUM_PROFILE.name}/${String(MINIMUM_PROFILE.major)}.x`,
		};
	}
	if (profile.minor < MINIMUM_PROFILE.minor) {
		return {
			kind: "binary-too-old",
			version,
			detail: `this build of dinah conforms to ${version.profile}, which is older than the ${MINIMUM_PROFILE.name}/${String(MINIMUM_PROFILE.major)}.${String(MINIMUM_PROFILE.minor)} this extension needs`,
		};
	}

	return { kind: "ok", version };
}
