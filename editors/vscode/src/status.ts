// The status bar item's text and tooltip.
//
// Both are composed by a pure function so that an integration test can assert
// on them. A status bar item cannot be read back through the VS Code API at
// all, so the composed strings ride on the object activate() returns and this
// module is what produces them.

import type { BinaryState, WorkbenchResolution } from "./api";
import {
	AMBIGUOUS_WORKBENCH,
	NO_CONFIGURED_WORKBENCH,
	NO_WORKBENCH_FOUND,
} from "./workbench";
import { describeVersion } from "./version";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";

/** What the status bar shows, or that it shows nothing. */
export interface StatusView {
	readonly hidden: boolean;
	readonly text: string;
	readonly tooltip: string;
}

/**
 * Every value the `dinah.binary` context key can hold. It reads "missing" when
 * no usable binary was found and "ok" otherwise.
 *
 * The welcome blocks in package.json partition the product of this set and the
 * one below, and the manifest test enumerates that product from these two
 * arrays rather than from a list somebody typed out again. A value added here
 * with no welcome block to receive it turns that test red, because the state it
 * names would match no block at all.
 */
export const BINARY_KEY_VALUES = ["missing", "ok"] as const;

/** Every value the `dinah.workbench` context key can hold. */
export const WORKBENCH_KEY_VALUES = ["none", "ambiguous", "ok", "unknown"] as const;

export type BinaryKey = (typeof BINARY_KEY_VALUES)[number];
export type WorkbenchKey = (typeof WORKBENCH_KEY_VALUES)[number];

/** The context key values the welcome view's `when` clauses read. */
export interface ContextKeys {
	readonly binary: BinaryKey;
	readonly workbench: WorkbenchKey;
}

const HIDDEN: StatusView = { hidden: true, text: "", tooltip: "" };

/**
 * The lines describing which binary this window is driving.
 *
 * describeVersion's own line is left alone. It names the tool's release, its
 * conformance profile and its storage format, which are machine vocabulary the
 * CLI spells the same way under every language setting, so translating this
 * extension's copy of it would show a reader words the CLI never says.
 */
function binaryLines(
	binary: BinaryState,
	pairedRelease: string,
	t: Localizer,
): string[] {
	const lines: string[] = [];
	if (binary.state === "ok") {
		lines.push(describeVersion(binary.version));
		lines.push(
			t("status.binary.withSource", {
				path: binary.path,
				source: binary.source,
			}),
		);
	} else if (binary.state !== "no-binary") {
		lines.push(binary.detail);
		if (binary.path) {
			lines.push(t("status.binary.plain", { path: binary.path }));
		}
	}
	lines.push(t("status.pairedWith", { release: pairedRelease }));
	return lines;
}

/**
 * Composes the status bar item for this window.
 *
 * `resolution` is the workspace folder's resolution the item speaks for.
 * Multi-root windows resolve every folder and this renders the first; showing
 * more than one entry is the tree card's job.
 */
export function composeStatus(
	binary: BinaryState,
	resolution: WorkbenchResolution | undefined,
	pairedRelease: string,
	t: Localizer = ENGLISH,
): StatusView {
	const trailer = binaryLines(binary, pairedRelease, t);

	if (binary.state === "no-binary") {
		return {
			hidden: false,
			text: "$(checklist) Dinah $(error)",
			tooltip: [
				t("status.noBinary.notFound"),
				t("status.noBinary.install"),
				...trailer,
			].join("\n"),
		};
	}
	if (binary.state !== "ok") {
		return {
			hidden: false,
			text: "$(checklist) Dinah $(error)",
			tooltip: trailer.join("\n"),
		};
	}

	if (!resolution) {
		return HIDDEN;
	}

	if (resolution.state === "refused") {
		if (
			resolution.refusal === NO_WORKBENCH_FOUND ||
			resolution.refusal === NO_CONFIGURED_WORKBENCH
		) {
			return HIDDEN;
		}
		if (resolution.refusal === AMBIGUOUS_WORKBENCH) {
			const candidates = (resolution.candidates ?? []).map(
				(candidate) => `  ${candidate.path}`,
			);
			return {
				hidden: false,
				text: "$(checklist) Dinah $(warning)",
				tooltip: [
					t("status.ambiguous"),
					...candidates,
					...trailer,
				].join("\n"),
			};
		}
		return {
			hidden: false,
			text: "$(checklist) Dinah $(warning)",
			tooltip: [
				t("status.refused", { refusal: resolution.refusal }),
				...(resolution.detail ? [resolution.detail] : []),
				...trailer,
			].join("\n"),
		};
	}

	const title = resolution.title === "" ? "Dinah" : resolution.title;
	const common = [
		t("status.resolvedBy", { source: resolution.source }),
		...trailer,
	];

	if (resolution.insideWorkspace) {
		return {
			hidden: false,
			text: `$(checklist) ${title}`,
			tooltip: [resolution.root, ...common].join("\n"),
		};
	}
	// The dinah-241 visibility rule. The walk climbed past this folder, so the
	// absolute path leads, before the title a reader would otherwise trust.
	return {
		hidden: false,
		text: `$(checklist) ${title} $(warning)`,
		tooltip: [
			t("status.outsideWorkspace", { root: resolution.root }),
			...common,
		].join("\n"),
	};
}

/** The two context keys the welcome view's `when` clauses are driven by. */
export function composeContextKeys(
	binary: BinaryState,
	resolution: WorkbenchResolution | undefined,
): ContextKeys {
	const binaryKey = binary.state === "ok" ? "ok" : "missing";
	if (!resolution) {
		return { binary: binaryKey, workbench: "unknown" };
	}
	if (resolution.state === "ok") {
		return { binary: binaryKey, workbench: "ok" };
	}
	if (resolution.refusal === AMBIGUOUS_WORKBENCH) {
		return { binary: binaryKey, workbench: "ambiguous" };
	}
	if (
		resolution.refusal === NO_WORKBENCH_FOUND ||
		resolution.refusal === NO_CONFIGURED_WORKBENCH
	) {
		return { binary: binaryKey, workbench: "none" };
	}
	return { binary: binaryKey, workbench: "unknown" };
}
