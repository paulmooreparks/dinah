// What this window publishes to the editor's MCP server list.
//
// The whole publish-or-refuse decision lives here rather than in the provider
// callback, because extension.ts is the one module a unit test cannot load:
// it imports vscode as a value, and vscode resolves only inside an editor
// host. A ladder written there would be exercised by nothing. This module
// imports BinaryState as a type alone, which status.ts and diagnostics.ts
// already do, and returns plain data that extension.ts turns into vscode
// values.
//
// Nothing here starts a server. The editor asks what is available, this
// answers, and the editor decides whether to run anything, after asking the
// reader whether they trust it.

import { isAbsolute } from "node:path";

import type { BinaryState } from "./api";
import type { McpTarget } from "./tree";

/** A published server, as plain data extension.ts turns into a vscode value. */
export interface McpServerPlan {
	readonly label: string;
	readonly command: string;
	readonly args: readonly string[];
	readonly cwd: string;
	readonly version: string;
}

/**
 * The key two roots are compared on.
 *
 * Separators are folded because one directory reached two ways is still one
 * directory, and case is folded where the caller says the platform folds it.
 * Nothing else is normalised: no relative segment is resolved, no symbolic
 * link is followed and no trailing separator is trimmed, because every root
 * here is a path `dinah --json status` reported for a workbench it had already
 * resolved, so both sides of any comparison come from that one source.
 *
 * DinahTreeProvider.rootKey applies the same rule for the tree's own purposes
 * and is restated rather than shared, because it is private and reads the
 * provider's own flag rather than an argument.
 */
function key(root: string, caseInsensitive: boolean): string {
	const posix = root.replace(/\\/g, "/");
	return caseInsensitive ? posix.toLowerCase() : posix;
}

/** The label a target carries before any collision is disambiguated. */
function labelFor(target: McpTarget): string {
	const title = target.title.trim();
	if (title !== "") {
		return `Dinah: ${title}`;
	}
	// The basename of the root, and never UNTITLED_WORKBENCH. A substituted
	// title would render `Dinah: Dinah` and could not be told from a workbench
	// genuinely called Dinah.
	const posix = target.root.replace(/\\/g, "/").replace(/\/+$/, "");
	const cut = posix.lastIndexOf("/");
	const base = cut === -1 ? posix : posix.slice(cut + 1);
	return `Dinah: ${base === "" ? target.root : base}`;
}

/**
 * One plan per distinct root, in the order the targets arrived in.
 *
 * `command` is the executable this was given, verbatim. This function composes
 * no path of its own and never substitutes a name for one, and its only caller
 * reaches it past a rung that refuses anything but an absolute path.
 *
 * `args` puts `--workbench` before the command word, because it is a global
 * flag and an argv carrying it after the command word parses differently. No
 * `--root` is passed: that would bound which workbenches the reader's agent
 * may name, where `--workbench` alone gives them this one as a default and
 * fences them out of nothing.
 *
 * Where two plans would carry the same label, every plan carrying it gains its
 * own root in parentheses. Not only the second: a reader cannot tell which of
 * two same-titled workbenches got the bare label.
 */
export function planMcpServers(
	targets: readonly McpTarget[],
	executable: string,
	toolVersion: string,
	caseInsensitive: boolean,
): readonly McpServerPlan[] {
	const seen = new Set<string>();
	const kept: McpTarget[] = [];
	for (const target of targets) {
		const folded = key(target.root, caseInsensitive);
		if (seen.has(folded)) {
			continue;
		}
		seen.add(folded);
		kept.push(target);
	}

	const labels = kept.map(labelFor);
	const collisions = new Set<string>();
	const once = new Set<string>();
	for (const label of labels) {
		if (once.has(label)) {
			collisions.add(label);
		}
		once.add(label);
	}

	return kept.map((target, at) => ({
		label: collisions.has(labels[at])
			? `${labels[at]} (${target.root})`
			: labels[at],
		command: executable,
		args: ["--workbench", target.root, "mcp"],
		cwd: target.root,
		version: toolVersion,
	}));
}

/**
 * Every plan this window publishes, which is often none.
 *
 * This is the publish-or-refuse decision, and it lives here rather than in the
 * callback because no unit test can reach extension.ts.
 *
 * Four rungs, in order. The reader's own switch is read first, so turning it
 * off costs nothing else. Then a binary that did not resolve, since nothing
 * knows where one is. Then a binary that reported no location, which is what
 * an older binary and one whose own os.Executable failed both look like, and
 * both publish nothing. Then a location that is not absolute, which no
 * documented path produces and which is refused rather than replaced by the
 * bare name, because a fallback would reinstate exactly the inference this
 * card exists to remove.
 */
export function publishedMcpServers(
	binary: BinaryState,
	registerMcpServer: boolean,
	targets: readonly McpTarget[],
	caseInsensitive: boolean,
): readonly McpServerPlan[] {
	if (!registerMcpServer) {
		return [];
	}
	if (binary.state !== "ok") {
		return [];
	}
	const executable = binary.version.executable ?? "";
	if (executable === "") {
		return [];
	}
	if (!isAbsolute(executable)) {
		return [];
	}
	return planMcpServers(
		targets,
		executable,
		binary.version.tool,
		caseInsensitive,
	);
}

/**
 * Whether two plan sets differ by value, which is what fires the change event.
 *
 * Object identity decides nothing, because the set is recomputed from fresh
 * objects on every checkpoint and an identity comparison would report a change
 * every time. The editor treats a change as a reason to prompt the reader to
 * refresh their tools, so firing on every poll would be a real cost to them
 * rather than an untidiness.
 */
export function mcpPlansDiffer(
	previous: readonly McpServerPlan[],
	next: readonly McpServerPlan[],
): boolean {
	if (previous.length !== next.length) {
		return true;
	}
	for (let at = 0; at < previous.length; at += 1) {
		const was = previous[at];
		const now = next[at];
		if (
			was.label !== now.label ||
			was.command !== now.command ||
			was.cwd !== now.cwd ||
			was.version !== now.version ||
			was.args.length !== now.args.length
		) {
			return true;
		}
		for (let arg = 0; arg < was.args.length; arg += 1) {
			if (was.args[arg] !== now.args[arg]) {
				return true;
			}
		}
	}
	return false;
}
