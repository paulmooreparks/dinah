// The classifier held against the binary this commit builds.
//
// Every other check of the catalog runs against a tool table written in this
// repository, which proves the classifier's branches and proves nothing about
// what dinah actually publishes. This one builds the CLI from the working
// tree, starts its MCP head, reads the real tool table over the real
// transport, and requires the palette to carry every tool in it (dinah-420
// AC12).
//
// That is what keeps the two halves of this card from drifting apart. A schema
// key added on the Go side that the extension has no rule for, and a
// classifier rule tightened past what the head publishes, both fail here on
// the commit that caused them rather than shipping a palette that has quietly
// lost an entry.
//
// This is the one unit file that starts a process, and test/unit/layers.test.ts
// carries the exemption with the reason. The cost is one `go build` and two
// short-lived spawns; the alternative is an integration test, and this
// column's own instructions rule those out for this card.

import assert from "node:assert/strict";
import { rmSync } from "node:fs";
import { join } from "node:path";
import { after, test } from "node:test";

import { REQUEST_ID, callMcp } from "../../src/mcpClient";
import { nodeSpawner } from "../../src/spawn";
import { INJECTED_KEY, buildCatalog } from "../../src/verbCatalog";
import type { FixtureRoot } from "../support/fixtures";
import { buildBinary, fixtureEnv } from "../support/fixtures";

const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");

/**
 * The binary both tests read, built once.
 *
 * `go build` is the whole cost of this file, so it is paid by the first test
 * that needs it and shared rather than paid twice.
 */
let built: FixtureRoot | undefined;

function binary(): FixtureRoot {
	built ??= buildBinary(repoRoot);
	return built;
}

after(() => {
	if (built !== undefined) {
		rmSync(built.tempRoot, { recursive: true, force: true });
	}
});

/**
 * Where a spawn runs and what it inherits.
 *
 * Inside the run's own temp root with DINAH_HOME under it, so neither the
 * discovery walk nor a configuration write can reach the operator's own
 * workbenches. The MCP head serves its tool table whether or not it resolved a
 * workbench, which is what lets this run in a directory that holds none.
 */
function options(root: FixtureRoot): { cwd: string; env: NodeJS.ProcessEnv } {
	return { cwd: root.tempRoot, env: fixtureEnv(root) };
}

test("the palette can render every tool this commit's binary publishes", async () => {
	const root = binary();
	const listed = await callMcp(
		nodeSpawner,
		root.binary,
		"tools/list",
		{},
		options(root),
	);
	assert.equal(
		listed.kind,
		"ok",
		`the binary answered tools/list with ${listed.kind === "ok" ? "ok" : listed.detail}`,
	);
	if (listed.kind !== "ok") {
		return;
	}
	const tools = listed.result["tools"];
	assert.ok(Array.isArray(tools), "tools/list answered with no tools array");
	assert.ok(
		tools.length > 0,
		"the binary published no tool at all, so this test would prove nothing about the classifier",
	);

	const lines: string[] = [];
	const catalog = await buildCatalog({
		spawner: nodeSpawner,
		exe: root.binary,
		options: options(root),
		log: (line) => lines.push(line),
	});
	assert.equal(
		catalog.kind,
		"ok",
		`the catalog build answered ${catalog.kind}: ${catalog.kind === "ok" ? "" : catalog.detail}`,
	);
	if (catalog.kind !== "ok") {
		return;
	}
	// Four assertions rather than one, because "excluded nothing" is true of a
	// build that read nothing at all. The palette has to carry a verb for every
	// tool the binary published, by name, with the exclusion list empty, no
	// entry dropped for want of a name, and its own log silent.
	assert.deepEqual(
		catalog.excluded,
		[],
		`this build cannot render every published tool:\n${lines.join("\n")}`,
	);
	assert.equal(
		catalog.unnamed,
		0,
		`the binary published an entry this build could not name:\n${lines.join("\n")}`,
	);
	assert.deepEqual(lines, [], "an exclusion was logged with nothing excluded");
	assert.deepEqual(
		catalog.verbs.map((verb) => verb.name).sort(),
		(tools as { name: string }[]).map((tool) => tool.name).sort(),
		"the palette's verbs are not the tools the binary published",
	);

	// The head marks every property it fills in itself, and the palette holds
	// those back by reading the mark rather than by naming actor, basis and
	// workbench in its own source. Two assertions, because the halves fail in
	// opposite directions: a head that published the mark nowhere would leave
	// the extension's rule reading nothing, and an extension that ignored the
	// mark would prompt a reader for plumbing while every fixture still passed.
	const marked = new Map<string, string[]>();
	for (const tool of tools as LiveTool[]) {
		const properties = tool.inputSchema?.properties ?? {};
		const names = Object.keys(properties).filter(
			(name) => properties[name]?.[INJECTED_KEY] === true,
		);
		if (names.length > 0) {
			marked.set(tool.name, names);
		}
	}
	assert.ok(
		marked.size > 0,
		`no tool this binary publishes carries ${INJECTED_KEY}, so the rule that holds the head's plumbing back reads nothing`,
	);
	for (const verb of catalog.verbs) {
		const held = marked.get(verb.name) ?? [];
		const asked = verb.args.map((arg) => arg.name).filter((name) => held.includes(name));
		assert.deepEqual(
			asked,
			[],
			`the palette would prompt for ${verb.name}'s ${asked.join(", ")}, which the binary marked as ${INJECTED_KEY}`,
		);
	}
});

/** One entry of the live tool table, read for the marks it publishes. */
interface LiveTool {
	readonly name: string;
	readonly inputSchema?: {
		readonly properties?: Record<string, Record<string, unknown>>;
	};
}

test("the live transport answers the request it was sent and not the handshake", async () => {
	// The reader of the answer picks the response by id, so a transport that
	// answered the handshake's id would look like a server with no tools. This
	// asserts against the running binary what the fabricated fixtures assert
	// against a string.
	const root = binary();
	const outcome = await callMcp(
		nodeSpawner,
		root.binary,
		"tools/list",
		{},
		options(root),
	);
	assert.equal(outcome.kind, "ok");
	assert.ok(
		outcome.kind === "ok" && Array.isArray(outcome.result["tools"]),
		`id ${String(REQUEST_ID)} answered with something other than the tool table`,
	);
});
