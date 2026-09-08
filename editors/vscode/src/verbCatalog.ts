// What the command palette can offer, read off the tool table dinah publishes.
//
// Two surfaces could have answered this question and only one of them can.
// The palette's entries are never typed into this extension, because a verb
// added to the CLI has to reach the palette without anybody editing it, so the
// entries come from `tools/list` and the prompt for each argument comes from
// that tool's own input schema (dinah-420).
//
// A schema property carries a type and a description, and dinah publishes
// four further keys beside them: `enum` for a vocabulary fixed in the source
// whose one member is the whole value, `x-dinah-vocabulary-members` for a
// fixed vocabulary whose members are combined into a comma-separated list,
// `x-dinah-vocabulary-source` for a vocabulary a head resolves when it runs,
// and `format: "duration"` for a value parsed as a duration. A list-valued
// property carries `x-dinah-value-list: true` beside its members, and a
// property the head fills in itself carries `x-dinah-injected: true` and is
// never asked about. This module
// turns each of those into the prompt a reader answers, and turns anything it
// does not recognise into an exclusion carrying a reason rather than into a
// guess.
//
// Nothing here imports vscode. The build runs under `node --test` against
// fabricated tool tables and against the binary this commit builds, and
// extension.ts binds the result to a real quick pick.

import type { PickItem } from "./cardCommands";
import type { SpawnOptions, Spawner } from "./cli";
import type { Localizer } from "./l10n";
import { callMcp } from "./mcpClient";

/** The vendor key dinah publishes a runtime-resolved vocabulary's source under. */
export const VOCABULARY_SOURCE_KEY = "x-dinah-vocabulary-source";

/**
 * The vendor key carrying the members of a list-valued fixed vocabulary.
 *
 * A single-valued argument publishes its set as `enum`, because the whole
 * value has to be one member. `show`'s `fields` argument is split on commas
 * before any member is checked, so `card,body` is legal and is not a member,
 * and an enum would have made it invalid on a surface that accepts it. dinah
 * publishes the members under this key instead, which describes the argument
 * without narrowing it.
 */
export const VOCABULARY_MEMBERS_KEY = "x-dinah-vocabulary-members";

/** The vendor key marking a property whose value is a comma-separated list. */
export const VALUE_LIST_KEY = "x-dinah-value-list";

/**
 * The vendor key marking a property the head supplies rather than the caller.
 *
 * `dinah mcp` fills in the acting name, the claim basis and the workbench on
 * every call it serves, and publishes those as ordinary schema properties so a
 * client that wants to override one can. A wizard walking a reader through a
 * verb's arguments must not ask for any of them: the transport already knows
 * all three, and a reader asked for a claim basis has been handed a question
 * about this extension's plumbing.
 *
 * The alternative was a list of the three names inside this extension, and the
 * operator ruled against it on dinah-420. A name list is right until the head
 * injects a fourth property, and then it is wrong in every installed copy of
 * the extension at once, while the marker is right on the day the fourth one
 * lands. Every other MCP client gets the same fact for the same reason.
 */
export const INJECTED_KEY = "x-dinah-injected";

/** The one `format` this build knows how to prompt for. */
export const DURATION_FORMAT = "duration";

/** How one runtime-resolved vocabulary is turned into a list of choices. */
export interface VocabularyResolver {
	/** The tool called to resolve it, with no arguments. */
	readonly tool: string;
	/** The member of that tool's result carrying the rows. */
	readonly member: string;
}

/**
 * The vocabulary sources this build can resolve, keyed by the source's own
 * name as dinah publishes it.
 *
 * The key is plural because the source is. `internal/verb/definition.go`
 * declares the vocabulary named `column` with `Source: "columns"`, and the
 * schema publishes the source rather than the vocabulary's own name, so a
 * table keyed on the singular matches nothing. It fails silently too: an
 * unmatched source is an unrenderable argument, which would take move,
 * list_cards, next_card and pull out of the palette with a count and no
 * further explanation.
 *
 * A Go test (TestEveryVocabularySourceAServedToolPublishesIsTheOneColumnsSource)
 * pins the sources a served tool can reach to exactly this set, so a card that
 * gives a served tool an argument with a new source fails there and is told to
 * extend this table in the same diff.
 */
export const VOCABULARY_RESOLVERS: Readonly<Record<string, VocabularyResolver>> =
	{
		columns: { tool: "columns", member: "columns" },
	};

/** What a reader is asked for one argument, once its schema has classified. */
export type ArgumentPrompt =
	| { readonly kind: "text" }
	| { readonly kind: "boolean" }
	| { readonly kind: "choice"; readonly values: readonly string[] }
	| { readonly kind: "list"; readonly values: readonly string[] }
	| { readonly kind: "vocabulary"; readonly source: string }
	| { readonly kind: "duration" };

/** One argument of a verb the palette can run. */
export interface VerbArgument {
	readonly name: string;
	readonly required: boolean;
	readonly prompt: ArgumentPrompt;
}

/** One verb the palette can run, with every argument it will ask about. */
export interface RenderableVerb {
	readonly name: string;
	readonly args: readonly VerbArgument[];
}

/** One verb the palette will not offer, and what stopped it. */
export interface ExcludedVerb {
	readonly name: string;
	readonly argument: string;
	readonly detail: string;
}

/**
 * A build that reached a verb list, with everything the reader is owed about
 * what did not reach it.
 *
 * `unnamed` counts the tool-table entries this build could not name: an entry
 * that is not an object, one carrying no `name` member, and one whose name is
 * the empty string. Such an entry earns no `ExcludedVerb`, because an
 * exclusion names the tool it excludes and there is no name here to print.
 * It is still counted, and it still writes a line to the channel, because the
 * palette's whole claim is that its entries are what the tool reported, and a
 * silently dropped entry breaks that claim exactly where nobody is looking.
 */
export interface CatalogOk {
	readonly kind: "ok";
	readonly verbs: readonly RenderableVerb[];
	readonly excluded: readonly ExcludedVerb[];
	readonly unnamed: number;
}

/**
 * What building the catalog came to.
 *
 * The failures are separate arms rather than an empty verb list, because the
 * one thing this design must never do is let a failure read as an answer. A
 * caller reaches the picker through the `ok` arm alone, so there is no path
 * on which an empty palette means both that dinah reported no verb and that
 * nobody could ask it.
 */
export type CatalogBuild =
	| CatalogOk
	| { readonly kind: "spawn-failed"; readonly detail: string }
	| { readonly kind: "transport-error"; readonly detail: string }
	| { readonly kind: "no-renderable-verbs"; readonly detail: string };

/** What classifying one property came to. */
type Classification =
	| { readonly kind: "prompt"; readonly prompt: ArgumentPrompt }
	| { readonly kind: "unrenderable"; readonly detail: string };

/** Whether a value is a JSON object rather than an array, a null or a scalar. */
function isObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Whether a value is an array of strings, which is what an enum must be. */
function isStringArray(value: unknown): value is string[] {
	return Array.isArray(value) && value.every((item) => typeof item === "string");
}

/**
 * Decides what one schema property becomes, or why it cannot become anything.
 *
 * The rule is a whitelist: a property is renderable when it classifies under
 * one of the six prompts this build knows, using only the keys this build
 * knows. Everything else is unrenderable, which is what arrives the day the
 * CLI is newer than the extension and some argument grows a shape written
 * after this classifier was.
 *
 * A property carrying more than one of the four constraining keys is
 * unrenderable too. Nothing publishes such a property today, and guessing
 * which of two constraints wins would be inventing a rule dinah has not
 * declared.
 *
 * The list marker is read as a modifier rather than as one of those four,
 * because it says how the value is composed and not what the value may be.
 * A marker spelled as anything but `true`, and a members key on a property
 * carrying no marker, are both shapes this build has no rule for, so both are
 * unrenderable rather than guessed at.
 */
export function classifyProperty(property: unknown): Classification {
	if (!isObject(property)) {
		return { kind: "unrenderable", detail: "a schema that is not an object" };
	}
	const type = property["type"];
	if (type !== "string" && type !== "boolean") {
		return {
			kind: "unrenderable",
			detail:
				type === undefined
					? "no type at all, which JSON Schema leaves unconstrained"
					: `a type of ${JSON.stringify(type)}`,
		};
	}
	const carried = [
		"enum",
		VOCABULARY_MEMBERS_KEY,
		VOCABULARY_SOURCE_KEY,
		"format",
	].filter((key) => property[key] !== undefined);
	const marker = property[VALUE_LIST_KEY];
	if (marker !== undefined && marker !== true) {
		return {
			kind: "unrenderable",
			detail: `${VALUE_LIST_KEY} spelled as ${JSON.stringify(marker)} rather than true`,
		};
	}
	const isList = marker === true;
	if (type === "boolean") {
		if (carried.length > 0 || isList) {
			return {
				kind: "unrenderable",
				detail: `a boolean carrying ${[...carried, ...(isList ? [VALUE_LIST_KEY] : [])].join(" and ")}`,
			};
		}
		return { kind: "prompt", prompt: { kind: "boolean" } };
	}
	if (carried.length > 1) {
		return { kind: "unrenderable", detail: `${carried.join(" and ")} at once` };
	}
	if (property[VOCABULARY_MEMBERS_KEY] !== undefined) {
		const values = property[VOCABULARY_MEMBERS_KEY];
		if (!isStringArray(values) || values.length === 0) {
			return {
				kind: "unrenderable",
				detail: `a ${VOCABULARY_MEMBERS_KEY} that is not a non-empty list of strings`,
			};
		}
		if (!isList) {
			return {
				kind: "unrenderable",
				detail: `${VOCABULARY_MEMBERS_KEY} on a property carrying no ${VALUE_LIST_KEY}, so what the members bound is undeclared`,
			};
		}
		return { kind: "prompt", prompt: { kind: "list", values } };
	}
	if (property["enum"] !== undefined) {
		const values = property["enum"];
		if (!isStringArray(values) || values.length === 0) {
			return {
				kind: "unrenderable",
				detail: "an enum that is not a non-empty list of strings",
			};
		}
		if (isList) {
			return {
				kind: "unrenderable",
				detail: `an enum on a property carrying ${VALUE_LIST_KEY}, which would refuse a legal answer naming two members`,
			};
		}
		return { kind: "prompt", prompt: { kind: "choice", values } };
	}
	const source = property[VOCABULARY_SOURCE_KEY];
	if (source !== undefined) {
		if (
			typeof source !== "string" ||
			VOCABULARY_RESOLVERS[source] === undefined
		) {
			return {
				kind: "unrenderable",
				detail: `the vocabulary source ${JSON.stringify(source)}, which this extension cannot resolve`,
			};
		}
		return { kind: "prompt", prompt: { kind: "vocabulary", source } };
	}
	const format = property["format"];
	if (format !== undefined) {
		if (format !== DURATION_FORMAT) {
			return { kind: "unrenderable", detail: `the format ${JSON.stringify(format)}` };
		}
		return { kind: "prompt", prompt: { kind: "duration" } };
	}
	return { kind: "prompt", prompt: { kind: "text" } };
}

/** What reading one property's injected marker came to. */
type InjectedVerdict =
	| { readonly kind: "injected" }
	| { readonly kind: "argument" }
	| { readonly kind: "unrenderable"; readonly detail: string };

/**
 * Reads the injected marker, which decides whether a property is an argument
 * at all before anything asks what shape of prompt it would take.
 *
 * A marked property is skipped whatever else it carries, so a head that starts
 * injecting a property of some shape written after this classifier was does
 * not cost the reader the verb. That is the point of the order: an exclusion
 * is what this build says about an argument it cannot draw, and a property the
 * reader is never asked for is not an argument.
 *
 * A marker spelled as anything but `true` is a shape this build has no rule
 * for, and it is read the way the list marker's is, as an exclusion rather
 * than as a guess in either direction. Reading it as absent would prompt for
 * plumbing; reading it as present would drop an argument the verb needs and
 * refuse the call for a value nobody was asked for.
 *
 * A property that is not an object answers `argument`, so classifyProperty
 * stays the one place that names that defect.
 */
export function readInjectedMarker(property: unknown): InjectedVerdict {
	if (!isObject(property)) {
		return { kind: "argument" };
	}
	const marker = property[INJECTED_KEY];
	if (marker === undefined) {
		return { kind: "argument" };
	}
	if (marker !== true) {
		return {
			kind: "unrenderable",
			detail: `${INJECTED_KEY} spelled as ${JSON.stringify(marker)} rather than true`,
		};
	}
	return { kind: "injected" };
}

/** What classifying one whole tool came to. */
export type ToolVerdict =
	| { readonly kind: "renderable"; readonly verb: RenderableVerb }
	| { readonly kind: "unrenderable"; readonly excluded: ExcludedVerb };

/**
 * Orders a verb's arguments: the required ones first, then the rest by name.
 *
 * Neither half depends on the order a JSON object happened to decode in. A
 * reader answering a wizard should meet what the verb cannot run without
 * before what it can, and the remainder should not move about between two
 * runs against the same binary.
 */
function orderArguments(
	properties: Record<string, unknown>,
	required: readonly string[],
): string[] {
	const names = Object.keys(properties);
	const first = required.filter((name) => names.includes(name));
	const held = new Set(first);
	const rest = names.filter((name) => !held.has(name)).sort();
	return [...first, ...rest];
}

/**
 * Classifies one entry of the tool table, and answers undefined for an entry
 * that is not a tool at all.
 *
 * An entry with no name answers undefined rather than an exclusion, because
 * an exclusion names the tool it excludes and this one names nothing a reader
 * could look up. buildCatalog counts what it drops and says so on the
 * channel, so answering undefined here is not the same as the entry vanishing.
 */
export function classifyTool(entry: unknown): ToolVerdict | undefined {
	if (
		!isObject(entry) ||
		typeof entry["name"] !== "string" ||
		entry["name"] === ""
	) {
		return undefined;
	}
	const name = entry["name"];
	const schema = entry["inputSchema"];
	if (!isObject(schema)) {
		return {
			kind: "unrenderable",
			excluded: {
				name,
				argument: "(the whole schema)",
				detail: "an inputSchema that is not an object",
			},
		};
	}
	const properties = isObject(schema["properties"]) ? schema["properties"] : {};
	const required = isStringArray(schema["required"]) ? schema["required"] : [];
	const args: VerbArgument[] = [];
	for (const argument of orderArguments(properties, required)) {
		const injected = readInjectedMarker(properties[argument]);
		if (injected.kind === "unrenderable") {
			return {
				kind: "unrenderable",
				excluded: { name, argument, detail: injected.detail },
			};
		}
		if (injected.kind === "injected") {
			continue;
		}
		const classified = classifyProperty(properties[argument]);
		if (classified.kind === "unrenderable") {
			return {
				kind: "unrenderable",
				excluded: { name, argument, detail: classified.detail },
			};
		}
		args.push({
			name: argument,
			required: required.includes(argument),
			prompt: classified.prompt,
		});
	}
	return { kind: "renderable", verb: { name, args } };
}

/** What buildCatalog needs to ask dinah anything. */
export interface CatalogDeps {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly options?: SpawnOptions;
	readonly log: (line: string) => void;
}

/** Reads the tool table and classifies every entry of it. */
export async function buildCatalog(deps: CatalogDeps): Promise<CatalogBuild> {
	const outcome = await callMcp(
		deps.spawner,
		deps.exe,
		"tools/list",
		{},
		deps.options,
	);
	if (outcome.kind !== "ok") {
		return { kind: outcome.kind, detail: outcome.detail };
	}
	const listed = outcome.result["tools"];
	if (!Array.isArray(listed)) {
		return {
			kind: "transport-error",
			detail: "dinah mcp answered tools/list with no tools array",
		};
	}
	const verbs: RenderableVerb[] = [];
	const excluded: ExcludedVerb[] = [];
	let unnamed = 0;
	for (const [position, entry] of listed.entries()) {
		const verdict = classifyTool(entry);
		if (verdict === undefined) {
			unnamed += 1;
			deps.log(
				`Command palette: dropping tool table entry ${String(position + 1)} ` +
					`of ${String(listed.length)}, which carries no usable name`,
			);
			continue;
		}
		if (verdict.kind === "renderable") {
			verbs.push(verdict.verb);
			continue;
		}
		excluded.push(verdict.excluded);
		deps.log(
			`Command palette: excluding "${verdict.excluded.name}", whose parameter ` +
				`"${verdict.excluded.argument}" declares ${verdict.excluded.detail}`,
		);
	}
	if (verbs.length === 0) {
		return {
			kind: "no-renderable-verbs",
			detail: `dinah mcp reported ${String(listed.length)} tool(s) and this build can render none of them`,
		};
	}
	return { kind: "ok", verbs, excluded, unnamed };
}

/**
 * The quick-pick entries the verb step opens with.
 *
 * The trailing separator is the whole of how a reader learns that the list is
 * shorter than what dinah reported. A palette that quietly dropped an entry
 * would say the same thing about a tool this build cannot draw as about a tool
 * that does not exist, and the channel line naming each exclusion is no use to
 * somebody who has no reason to open the channel.
 *
 * The count is every entry the build did not turn into a row, which means the
 * unnameable entries as well as the excluded ones. Their names cannot reach
 * the channel and their existence can, so leaving them out of the count would
 * put the palette back in the position of being quietly short.
 */
export function verbPickItems(build: CatalogOk, t: Localizer): PickItem[] {
	const items: PickItem[] = build.verbs.map((verb) => ({
		label: verb.name,
		value: verb.name,
	}));
	const missing = build.excluded.length + build.unnamed;
	if (missing > 0) {
		items.push({
			label: t("dialog.runVerb.excludedSeparator", { count: missing }),
			value: "",
			kind: "separator",
		});
	}
	return items;
}

/**
 * Holds one build of the catalog, and forgets it when the binary changes.
 *
 * The catalog is never re-read on a keystroke: a quick pick filters an array
 * it was handed before it opened, so the only reads are the one at activation,
 * the one after an invalidation, and the one the refresh command forces.
 */
export class VerbCatalog {
	private held: CatalogBuild | undefined;

	constructor(private readonly deps: CatalogDeps) {}

	/** Drops what is held, so the next read asks dinah again. */
	invalidate(): void {
		this.held = undefined;
	}

	/** What is held, reading it first when nothing is. */
	async get(): Promise<CatalogBuild> {
		if (this.held === undefined) {
			this.held = await buildCatalog(this.deps);
		}
		return this.held;
	}

	/** Drops what is held and reads it again now. */
	async rebuild(): Promise<CatalogBuild> {
		this.invalidate();
		return this.get();
	}
}
