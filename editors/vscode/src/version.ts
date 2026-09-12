// The compatibility gate.
//
// `dinah --json version` reports four fields with four jobs, and this gate
// reads two of them.
//
// `format` is the storage gate. `profile` is the contract gate. `tool` is
// never compared: `verb.ToolRelease` is the literal "0.1.0" in every build
// from source and is overwritten only by a release build's -ldflags, so a
// gate reading it would refuse every contributor's own binary. It is
// displayed and nothing more. `executable` is the binary's own location, which
// no gate compares either: it is carried through for the MCP provider, which
// must hand the editor a command it can run.

import type { CliOutcome } from "./cli";
import type { VersionReport } from "./api";

/**
 * The storage formats this extension can drive, as a set rather than a floor.
 * A format is a serialisation, not a capability level, so "anything at or
 * above 1" is not a claim this extension can make about a number it has never
 * seen.
 *
 * 2 joined the set at dinah-285, which fixed a workbench inside a `.dinah`
 * container and made its directory name a wider identifier. Neither of those
 * reaches this extension: it drives the binary through the JSON verb surface
 * and never reads the tree itself, and that surface did not change.
 *
 * 3 joined the set at dinah-488, which moved the card number out of the card
 * anchor and into a registry at the workbench root. A reference still
 * resolves through the verb surface exactly as it did, and what changed is
 * where the binary keeps the numbers it allocates, which the extension never
 * reads.
 */
export const SUPPORTED_FORMATS: readonly number[] = [1, 2, 3];

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

/**
 * Reads the four fields off a parsed `--json version` payload.
 *
 * This is the only decode of that payload in the extension, and it builds its
 * result field by field, so a field named nowhere here is undefined on every
 * binary however faithfully the CLI sends it.
 *
 * `executable` is optional in both directions. A binary older than the field
 * sends none, and a binary whose own `os.Executable` failed sends none either,
 * so its absence is ordinary rather than a defect in the report. A value of
 * any other type is read as absent for the same reason: the three fields the
 * gate reads are what decide whether this binary is usable at all, and
 * refusing the whole report over a fourth one would refuse a binary this
 * extension can otherwise drive.
 */
function readReport(json: unknown): VersionReport | undefined {
	const report = json as
		| {
				tool?: unknown;
				profile?: unknown;
				format?: unknown;
				executable?: unknown;
		  }
		| undefined;
	if (
		!report ||
		typeof report.tool !== "string" ||
		typeof report.profile !== "string" ||
		typeof report.format !== "number"
	) {
		return undefined;
	}
	const executable =
		typeof report.executable === "string" ? report.executable : undefined;
	return {
		tool: report.tool,
		profile: report.profile,
		format: report.format,
		executable,
	};
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
