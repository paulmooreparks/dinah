// The Run Verb wizard: pick a verb, answer its arguments, run it.
//
// Every prompt in here is generated. The verb list is what dinah's tool table
// reported, each argument's prompt is what that tool's input schema declared,
// and the only English this module owns is the template each prompt is
// rendered from. Nothing names a verb, an argument or a column (dinah-420).
//
// Nothing imports vscode. The wizard runs over the same injected CommandHost
// the tree's own commands take, so the unit layer drives a whole wizard by
// answering the host's prompts in order, and extension.ts binds that host to a
// real window.

import type { CommandHost, PickItem } from "./cardCommands";
import type { SpawnOptions, Spawner } from "./cli";
import { callMcp } from "./mcpClient";
import type {
	CatalogBuild,
	CatalogOk,
	VerbArgument,
	VerbCatalog,
	RenderableVerb,
} from "./verbCatalog";
import { VOCABULARY_RESOLVERS, verbPickItems } from "./verbCatalog";

/**
 * The value the "leave this out" row carries.
 *
 * An optional argument whose schema constrains it still has to be omittable,
 * and the row offering that has to be told apart from every real member. A
 * NUL byte is what separates them: dinah's fixed vocabularies are identifier
 * tokens, and a column's slug and id are drawn from the same alphabet, so no
 * value a picker can offer contains one. Comparing on this string rather than
 * on object identity is deliberate, because extension.ts's `pick` binding
 * hands the array to the editor and gets a copy back.
 */
const OMIT_VALUE = "\u0000dinah.omit";

/** What the wizard needs to ask dinah anything and to report what came back. */
export interface RunVerbContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder whose checkpoint follows a successful run. */
	readonly folder: string;
	/** The workbench every call is pinned to, as the process's directory. */
	readonly root: string;
	readonly catalog: VerbCatalog;
}

/** The value one answered argument contributes, or nothing when it is omitted. */
type Answer =
	| { readonly kind: "value"; readonly value: string | boolean }
	| { readonly kind: "omitted" }
	| { readonly kind: "cancelled" }
	| { readonly kind: "failed" };

/** The three outcomes a contract verb reports as a failure. */
const FAILED_OUTCOMES = new Set(["refused", "stale", "unreachable"]);

/**
 * Reads a tools/call result as success or failure.
 *
 * A tools/call that reached the library always exits cleanly, so the process
 * carries no verdict the way `dinah --json <verb>` does at a terminal, and the
 * outcome rides inside the payload instead. The eight contract verbs marshal
 * verb.Response, which carries `outcome`; every other tool wraps its own
 * answer in an object that carries no `outcome` at all on success and falls
 * back to verb.Response on failure. So one rule covers the whole surface: a
 * payload naming a failed outcome is a failure, and anything else is success.
 */
export function classifyCallResult(
	result: Record<string, unknown>,
): { readonly kind: "ok" } | { readonly kind: "failed"; readonly message: string } {
	const outcome = result["outcome"];
	if (typeof outcome !== "string" || !FAILED_OUTCOMES.has(outcome)) {
		return { kind: "ok" };
	}
	const refusal = typeof result["refusal"] === "string" ? result["refusal"] : outcome;
	const detail = typeof result["detail"] === "string" ? result["detail"] : "";
	return {
		kind: "failed",
		message: detail === "" ? refusal : `${refusal}: ${detail}`,
	};
}

/**
 * Asks for one free-text value, reopening the same box on a blank answer to a
 * required argument.
 *
 * A blank answer to an optional argument omits it rather than sending an empty
 * string, which is what the CLI does with a flag nobody passed.
 */
async function askText(
	host: CommandHost,
	prompt: string,
	required: boolean,
): Promise<Answer> {
	for (;;) {
		const typed = await host.input(prompt);
		if (typed === undefined) {
			return { kind: "cancelled" };
		}
		const trimmed = typed.trim();
		if (trimmed !== "") {
			return { kind: "value", value: trimmed };
		}
		if (!required) {
			return { kind: "omitted" };
		}
	}
}

/**
 * The rows a constrained argument is offered as, with a way to decline when
 * the schema does not require it.
 *
 * Escape is not that way. It cancels the whole wizard, which is what a reader
 * who has changed their mind about running anything wants, and it is not what
 * a reader who wants an unfiltered `list_cards` wants. Without this row nine
 * of the served tools could not be run in their ordinary form from the
 * palette at all, because every one of them constrains an argument it does
 * not require.
 */
function withOmitRow(
	items: readonly PickItem[],
	required: boolean,
	t: CommandHost["t"],
	argument: string,
): PickItem[] {
	if (required) {
		return [...items];
	}
	return [
		{ label: t("dialog.runVerb.omitOptional", { argument }), value: OMIT_VALUE },
		...items,
	];
}

/** The rows one runtime vocabulary answered with, as choices. */
export function vocabularyItems(rows: readonly unknown[]): PickItem[] {
	const items: PickItem[] = [];
	for (const row of rows) {
		if (typeof row !== "object" || row === null) {
			continue;
		}
		const record = row as Record<string, unknown>;
		const slug = typeof record["slug"] === "string" ? record["slug"] : "";
		const id = typeof record["id"] === "string" ? record["id"] : "";
		const title = typeof record["title"] === "string" ? record["title"] : "";
		// The slug is sent ahead of the id because no two columns share one,
		// where two could in principle share a title, and bench.ColumnByRef
		// resolves a reference against id, slug and title in that order. A row
		// written before the slug migration ran carries none, and its id
		// resolves just as exactly.
		const value = slug !== "" ? slug : id;
		if (value === "") {
			continue;
		}
		items.push({ label: title !== "" ? title : value, value });
	}
	return items;
}

/** Resolves one runtime-vocabulary argument, or reports why it could not. */
async function askVocabulary(
	context: RunVerbContext,
	verb: RenderableVerb,
	argument: VerbArgument,
	source: string,
	options: SpawnOptions,
): Promise<Answer> {
	const resolver = VOCABULARY_RESOLVERS[source];
	const report = (detail: string): Answer => {
		context.host.showError(
			context.host.t("dialog.runVerb.vocabularyResolutionFailed.toast", {
				argument: argument.name,
			}),
		);
		context.host.log(
			context.host.t("dialog.runVerb.vocabularyResolutionFailed.channel", {
				argument: argument.name,
				verb: verb.name,
				detail,
			}),
		);
		return { kind: "failed" };
	};
	if (resolver === undefined) {
		// Unreachable through the palette, since an unresolvable source makes
		// the whole verb unrenderable, and reported rather than assumed away
		// because a caller reaching this module directly would otherwise get a
		// wizard that stopped without a word.
		return report(`no resolver for the vocabulary source ${source}`);
	}
	const outcome = await callMcp(
		context.spawner,
		context.exe,
		"tools/call",
		{ name: resolver.tool, arguments: {} },
		options,
	);
	if (outcome.kind !== "ok") {
		return report(outcome.detail);
	}
	const classified = classifyCallResult(outcome.result);
	if (classified.kind === "failed") {
		return report(classified.message);
	}
	const rows = outcome.result[resolver.member];
	if (!Array.isArray(rows)) {
		return report(`${resolver.tool} answered with no ${resolver.member} member`);
	}
	// An empty list is a legitimate state rather than a failure, so it opens a
	// picker with nothing in it and the reader escapes out of a wizard that
	// has nothing to offer. A failed call never opens one at all, which is
	// what keeps the two apart.
	const chosen = await context.host.pick(
		withOmitRow(
			vocabularyItems(rows),
			argument.required,
			context.host.t,
			argument.name,
		),
		context.host.t("dialog.runVerb.vocabularyPlaceholder", {
			argument: argument.name,
		}),
	);
	if (chosen === undefined) {
		return { kind: "cancelled" };
	}
	return chosen.value === OMIT_VALUE
		? { kind: "omitted" }
		: { kind: "value", value: chosen.value };
}

/** Asks for one argument, whatever kind of prompt its schema earned. */
async function askArgument(
	context: RunVerbContext,
	verb: RenderableVerb,
	argument: VerbArgument,
	options: SpawnOptions,
): Promise<Answer> {
	const t = context.host.t;
	switch (argument.prompt.kind) {
		case "boolean": {
			const chosen = await context.host.pick(
				[
					{ label: t("dialog.runVerb.booleanYes"), value: "yes" },
					{ label: t("dialog.runVerb.booleanNo"), value: "no" },
				],
				t("dialog.runVerb.booleanPlaceholder", { argument: argument.name }),
			);
			if (chosen === undefined) {
				return { kind: "cancelled" };
			}
			// A marker the reader said no to is left out of the call, which is
			// what the CLI does with a flag nobody typed. Sending false would
			// be sending an argument the reader declined.
			return chosen.value === "yes"
				? { kind: "value", value: true }
				: { kind: "omitted" };
		}
		case "choice": {
			// The members are canonical tokens, so they are shown as they are
			// spelled on the wire and are never translated.
			const chosen = await context.host.pick(
				withOmitRow(
					argument.prompt.values.map((value) => ({ label: value, value })),
					argument.required,
					t,
					argument.name,
				),
				t("dialog.runVerb.vocabularyPlaceholder", { argument: argument.name }),
			);
			if (chosen === undefined) {
				return { kind: "cancelled" };
			}
			return chosen.value === OMIT_VALUE
				? { kind: "omitted" }
				: { kind: "value", value: chosen.value };
		}
		case "list":
			// A list-valued argument is typed rather than picked, because its
			// legal answers are the subsets of the vocabulary rather than its
			// members, and a quick pick offers one row at a time. The members
			// ride in the prompt so a reader never has to know them already,
			// and a blank answer to an optional one omits it, which is the
			// same omit path every text argument has.
			return askText(
				context.host,
				t(
					argument.required
						? "dialog.runVerb.requiredListPrompt"
						: "dialog.runVerb.optionalListPrompt",
					{
						argument: argument.name,
						values: argument.prompt.values.join(", "),
					},
				),
				argument.required,
			);
		case "vocabulary":
			return askVocabulary(context, verb, argument, argument.prompt.source, options);
		case "duration":
			// The answer travels unexamined. verb.ParseDuration is the one
			// parser for this grammar, and a malformed answer comes back as an
			// ordinary refusal rendered the way every other refusal is.
			return askText(
				context.host,
				t("dialog.runVerb.durationPlaceholder", { argument: argument.name }),
				argument.required,
			);
		default:
			return askText(
				context.host,
				argument.required
					? t("dialog.runVerb.requiredValuePrompt", { argument: argument.name })
					: t("dialog.runVerb.optionalValuePrompt", { argument: argument.name }),
				argument.required,
			);
	}
}

/**
 * Runs the whole wizard: which verb, then one prompt per argument, then the
 * call.
 *
 * The picker opens on the `ok` arm of the catalog build alone. Every other arm
 * shows the reader a message and writes the detail to the channel, so an empty
 * palette never stands in for a failure to ask.
 */
export async function runVerbFromPalette(context: RunVerbContext): Promise<void> {
	const t = context.host.t;
	const options: SpawnOptions = { cwd: context.root };
	const build = await context.catalog.get();
	if (build.kind !== "ok") {
		reportFailedBuild(context.host, build);
		return;
	}
	const picked = await context.host.pick(
		verbPickItems(build, t),
		t("dialog.runVerb.pickVerbPlaceholder"),
	);
	if (picked === undefined) {
		return;
	}
	const verb = build.verbs.find((candidate) => candidate.name === picked.value);
	if (verb === undefined) {
		// The separator row carries no verb and cannot be chosen in a real
		// quick pick. Returning quietly is what a reader who somehow lands on
		// one should get, rather than a call composed against nothing.
		return;
	}
	const args: Record<string, string | boolean> = {};
	for (const argument of verb.args) {
		const answer = await askArgument(context, verb, argument, options);
		if (answer.kind === "cancelled" || answer.kind === "failed") {
			return;
		}
		if (answer.kind === "value") {
			args[argument.name] = answer.value;
		}
	}
	const outcome = await callMcp(
		context.spawner,
		context.exe,
		"tools/call",
		{ name: verb.name, arguments: args },
		options,
	);
	if (outcome.kind !== "ok") {
		context.host.showError(outcome.detail);
		await context.host.checkpoint(context.folder);
		return;
	}
	const classified = classifyCallResult(outcome.result);
	if (classified.kind === "failed") {
		context.host.showError(classified.message);
	}
	// The checkpoint runs whichever way the call went, exactly as runVerb's
	// does: a refusal often means the board moved under the reader, and the
	// read that follows is what shows them.
	await context.host.checkpoint(context.folder);
}

/**
 * Says on both surfaces that the catalog could not be built, and says the
 * same thing on each.
 *
 * The two failing arms are told apart rather than sharing one line. A build
 * that reached the tool table and could draw nothing in it is not a build
 * that could not read the table, and the channel line telling a reader the
 * listing failed when the listing succeeded sends them looking for the wrong
 * fault.
 */
function reportFailedBuild(
	host: CommandHost,
	build: Exclude<CatalogBuild, CatalogOk>,
): void {
	const renderable = build.kind === "no-renderable-verbs";
	host.showError(
		host.t(
			renderable
				? "dialog.runVerb.noRenderableVerbs.toast"
				: "dialog.runVerb.enumerationFailed.toast",
		),
	);
	host.log(
		host.t(
			renderable
				? "dialog.runVerb.noRenderableVerbs.channel"
				: "dialog.runVerb.enumerationFailed.channel",
			{ detail: build.detail },
		),
	);
}

/** Rebuilds the catalog now, and says so when the rebuild failed. */
export async function refreshVerbCatalog(context: RunVerbContext): Promise<void> {
	const build = await context.catalog.rebuild();
	if (build.kind === "ok") {
		return;
	}
	reportFailedBuild(context.host, build);
}
