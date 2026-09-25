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
 *
 * 4 joined the set at dinah-498, which retired a heading a card body carried
 * a value under into a field the workbench declares. The value moved from the
 * body to the front matter of the same anchor, and the extension reads
 * neither: it asks the verb surface for a card and is answered a card, with
 * one more member on it that an older reader ignores.
 *
 * 6 joined the set at dinah-525, which made a checklist item's answer a
 * designated comment rather than a free-text note and gave a comment's anchor
 * a digest. Both are anchor keys, and this extension reads no anchor: what it
 * sees is one member renamed on the item view a read answers with, and a
 * comment's own body, which it now writes through `dinah set` rather than
 * leaving where the editor put it.
 *
 * 7 joined the set at dinah-472, which keys a checklist item's answer on the
 * designated comment's own identifier rather than on that comment's position.
 * That is an anchor key too, and this extension reads no anchor: the item view
 * a read answers with goes on carrying the position, composed by the binary at
 * the moment of the read, and gains `resolution_id` beside it, which an older
 * reader ignores.
 *
 * 8 joined the set at dinah-590, which let a declaration carry a condition
 * saying which cards it applies to and gave a level axis a mapping form. Both
 * are members of the workbench anchor, which this extension never reads: the
 * card view a read answers with gains `inapplicable` beside `fields`, which
 * an older reader ignores, and a write the condition refuses is refused by
 * name through the same verb surface as every other refusal.
 *
 * 9 joined the set at dinah-593, which made a quoted scalar on a workbench or
 * column anchor read as text and rewrote the raw JSON lines an earlier import
 * had quoted. Both are facts about the anchor files, which this extension
 * never reads, and the verb surface it drives is unchanged.
 *
 * 10 joined the set at dinah-605, which gave a card three scheduling dates
 * that an older build would ignore and so hand a dated card out early. The
 * dates are keys of the card anchor, which this extension never reads: the
 * card view a read answers with gains `start_after`, `start_by`, `due` and
 * `schedule`, which an older reader ignores.
 *
 * 11 joined the set at dinah-608, which let a link of a kind the workbench
 * declares under `dinah.holds` hold a card back from selection, where an older
 * build would ignore the hold and hand the card out early. The layer is a key
 * of the workbench anchor and links are keys of the card anchor, neither of
 * which this extension reads: the card view gains `waits_on` and the offer
 * gains `waiting` and `waiting_on`, which an older reader ignores.
 */
export const SUPPORTED_FORMATS: readonly number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];

/**
 * The conformance claim this extension needs. A different name or major is
 * refused outright. A lower minor is refused, because the binary is older
 * than the fields this extension reads. A higher minor is accepted, because a
 * client that only reads the fields it knows is unharmed by fields it does
 * not.
 */
export const MINIMUM_PROFILE = { name: "dinah-core", major: 0, minor: 18 };

/*
 * The minor moved from 4 to 18 at dinah-597. Unblock now asks why the block
 * is lifted and writes the answer as the verb's reason, which is a slot a
 * binary claiming an earlier revision refuses with `dinah.usage` before the
 * verb runs. An updated extension against such a binary would ask the
 * operator for a paragraph and then refuse it row by row, so the gate refuses
 * that binary whole at activation instead, as `binary-too-old` with both
 * numbers in the detail. The remedy is the update the message names, and the
 * extension ships from the same repository as the binary. A conditional send
 * keyed on the reported minor was declined for the reason the paragraph below
 * declines a negotiated payload: it is a shim carried for a transient skew.
 */

/**
 * The skew this gate does not catch, written down because no gate can catch it.
 *
 * dinah-536 gave a card's checklist three published branches in the
 * containment payload: below a card, the questions, criteria and decisions
 * that used to arrive as one flat run of `item` nodes now arrive as three
 * nodes of kind `collection`, each holding its own members. An extension from
 * before that card partitions a card's children by kind and labels each group
 * from a map keyed by member kind, so it collects all three into one group and
 * labels it with the literal token `collection`, whose children are the three
 * branches rather than the items. Nothing is lost and nothing is wrong on
 * disk; one row reads oddly until the extension is updated, which is the
 * remedy, and the extension ships from the same repository as the binary.
 *
 * No version bump refuses it. A higher minor is accepted here on purpose, for
 * the reason given above, so bumping the minor would refuse nothing; and a
 * major bump asserts that everything this gate names has changed, which is far
 * more than that card changed. The alternatives, a negotiated payload shape or
 * an opt-in flag, are a permanent surface carried for a transient skew, which
 * is the compatibility shim the operator ruled against on 2026-09-15. So the
 * break is accepted and recorded rather than prevented.
 */

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
