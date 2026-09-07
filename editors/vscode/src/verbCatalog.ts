// What the command palette can offer, read off the tool table dinah publishes.
//
// Two surfaces could have answered this question and only one of them can.
// The palette's entries are never typed into this extension, because a verb
// added to the CLI has to reach the palette without anybody editing it, so the
// entries come from `tools/list` and the prompt for each argument comes from
// that tool's own input schema (dinah-420).
//
// A schema property carries a type and a description, and dinah publishes
// three further keys beside them: `enum` for a vocabulary fixed in the source,
// `x-dinah-vocabulary-source` for one a head resolves when it runs, and
// `format: "duration"` for a value parsed as a duration. This module turns
// each of those into the prompt a reader answers, and turns anything it does
// not recognise into an exclusion carrying a reason rather than into a guess.
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
 * What building the catalog came to.
 *
 * The failures are separate arms rather than an empty verb list, because the
 * one thing this design must never do is let a failure read as an answer. A
 * caller reaches the picker through the `ok` arm alone, so there is no path
 * on which an empty palette means both that dinah reported no verb and that
 * nobody could ask it.
 */
export type CatalogBuild =
	| {
			readonly kind: "ok";
			readonly verbs: readonly RenderableVerb[];
			readonly excluded: readonly ExcludedVerb[];
	  }
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
 * one of the five prompts this build knows, using only the keys this build
 * knows. Everything else is unrenderable, which is what arrives the day the
 * CLI is newer than the extension and some argument grows a shape written
 * after this classifier was.
 *
 * A property carrying more than one of the three constraining keys is
 * unrenderable too. Nothing publishes such a property today, and guessing
 * which of two constraints wins would be inventing a rule dinah has not
 * declared.
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
	const carried = ["enum", VOCABULARY_SOURCE_KEY, "format"].filter(
		(key) => property[key] !== undefined,
	);
	if (type === "boolean") {
		if (carried.length > 0) {
			return {
				kind: "unrenderable",
				detail: `a boolean carrying ${carried.join(" and ")}`,
			};
		}
		return { kind: "prompt", prompt: { kind: "boolean" } };
	}
	if (carried.length > 1) {
		return { kind: "unrenderable", detail: `${carried.join(" and ")} at once` };
	}
	if (property["enum"] !== undefined) {
		const values = property["enum"];
		if (!isStringArray(values) || values.length === 0) {
			return {
				kind: "unrenderable",
				detail: "an enum that is not a non-empty list of strings",
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
 * An entry with no name is dropped rather than excluded, because an exclusion
 * names the tool it excludes and this one names nothing a reader could look
 * up.
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
	for (const entry of listed) {
		const verdict = classifyTool(entry);
		if (verdict === undefined) {
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
	return { kind: "ok", verbs, excluded };
}

/**
 * The quick-pick entries the verb step opens with.
 *
 * The trailing separator is the whole of how a reader learns that the list is
 * shorter than what dinah reported. A palette that quietly dropped an entry
 * would say the same thing about a tool this build cannot draw as about a tool
 * that does not exist, and the channel line naming each exclusion is no use to
 * somebody who has no reason to open the channel.
 */
export function verbPickItems(
	verbs: readonly RenderableVerb[],
	excluded: readonly ExcludedVerb[],
	t: Localizer,
): PickItem[] {
	const items: PickItem[] = verbs.map((verb) => ({
		label: verb.name,
		value: verb.name,
	}));
	if (excluded.length > 0) {
		items.push({
			label: t("dialog.runVerb.excludedSeparator", { count: excluded.length }),
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
