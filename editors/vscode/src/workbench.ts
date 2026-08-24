// Which workbench a workspace folder resolves to.
//
// Discovery is the binary's, in one `dinah --json status` call per workspace
// folder. `verb.Status` carries `root` and `workbench_source` together, and
// the `dinah.ambiguous-workbench` refusal envelope carries the candidate list,
// so one call answers the whole question.
//
// This extension implements no walk of its own. A second, more cautious
// discovery rule would be exactly the reimplementation the product forbids,
// and it would put the editor and the user's terminal on different
// workbenches, which is a worse failure than the one it would prevent. The
// dinah-241 hazard, where the walk climbs to the drive root and reaches a
// workbench the user did not mean, is answered by showing the resolved
// absolute path and the rung that produced it, not by diverging.

import type { CliOutcome, SpawnOptions, Spawner } from "./cli";
import { runDinah } from "./cli";
import type { Candidate, WorkbenchResolution } from "./api";

/** The three refusals this extension handles by name. */
export const NO_WORKBENCH_FOUND = "dinah.no-workbench-found";
export const AMBIGUOUS_WORKBENCH = "dinah.ambiguous-workbench";
export const NO_CONFIGURED_WORKBENCH = "dinah.no-configured-workbench";

/**
 * The rung name reported when `verb.Status` names none.
 *
 * `WorkbenchSource` is declared `json:"workbench_source,omitempty"`, so the key
 * is absent rather than empty whenever the rung is unnamed. A parser without a
 * default would render `undefined` into a tooltip.
 */
export const UNNAMED_SOURCE = "unknown";

/**
 * Reports whether `candidate` lies inside `folder`, comparing segment by
 * segment so that a sibling directory whose name merely begins with the
 * folder's own name is outside it.
 *
 * The comparison is case-insensitive on Windows, where one directory answers
 * to several spellings, and case-sensitive elsewhere.
 */
export function isInside(
	candidate: string,
	folder: string,
	caseInsensitive: boolean,
): boolean {
	const normalise = (value: string): string[] => {
		let text = value.replace(/\\/g, "/");
		if (caseInsensitive) {
			text = text.toLowerCase();
		}
		return text.split("/").filter((segment) => segment !== "");
	};
	const inner = normalise(candidate);
	const outer = normalise(folder);
	if (inner.length < outer.length) {
		return false;
	}
	return outer.every((segment, index) => inner[index] === segment);
}

/** Reads a successful `--json status` payload into the ok arm. */
export function parseStatus(
	json: unknown,
	folder: string,
	caseInsensitive: boolean,
): WorkbenchResolution | undefined {
	const status = json as
		| {
				workbench?: unknown;
				root?: unknown;
				profile?: unknown;
				workbench_source?: unknown;
		  }
		| undefined;
	if (!status || typeof status.root !== "string") {
		return undefined;
	}
	return {
		state: "ok",
		root: status.root,
		title: typeof status.workbench === "string" ? status.workbench : "",
		source:
			typeof status.workbench_source === "string" && status.workbench_source !== ""
				? status.workbench_source
				: UNNAMED_SOURCE,
		profile: typeof status.profile === "string" ? status.profile : "",
		insideWorkspace: isInside(status.root, folder, caseInsensitive),
	};
}

/** Turns any non-ok outcome into the refused arm. */
export function parseRefusal(outcome: CliOutcome): WorkbenchResolution {
	if (outcome.kind === "refused") {
		return {
			state: "refused",
			refusal: outcome.refusal,
			detail: outcome.detail,
			candidates: outcome.workbenches as readonly Candidate[] | undefined,
		};
	}
	const detail = (outcome as { detail?: string }).detail ?? outcome.kind;
	return { state: "refused", refusal: outcome.kind, detail };
}

/**
 * Resolves one workspace folder's workbench.
 *
 * `pinned` is the dinah.workbench setting for that folder. When it is set it
 * is passed as `--workbench`, which is the same rung `DINAH_WORKBENCH`
 * occupies, and it pins the answer for a user who wants it pinned.
 */
export async function resolveWorkbench(
	spawner: Spawner,
	exe: string,
	folder: string,
	pinned: string,
	caseInsensitive: boolean,
	options: SpawnOptions = {},
): Promise<WorkbenchResolution> {
	const args = pinned.trim() === "" ? ["status"] : ["--workbench", pinned.trim(), "status"];
	const outcome = await runDinah(spawner, exe, args, { ...options, cwd: folder });
	if (outcome.kind !== "ok") {
		return parseRefusal(outcome);
	}
	const parsed = parseStatus(outcome.json, folder, caseInsensitive);
	if (!parsed) {
		return {
			state: "refused",
			refusal: "not-json",
			detail: "dinah answered `status` with something that carries no root",
		};
	}
	return parsed;
}
