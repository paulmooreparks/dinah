// The runtime half of the extension's localisation, as a plain module with no
// VS Code in it.
//
// The manifest half is not here. Command titles, the view container's title,
// the view's name, the welcome blocks and the settings descriptions are
// resolved by the editor itself before this extension activates, out of
// package.nls.json and its per-locale siblings, and no code of ours runs for
// them.
//
// The runtime half deliberately does not use vscode.l10n.t(). tree.ts and the
// command-host modules import no vscode symbol, because the unit layer runs
// them under plain `node --test` with no extension host, and esbuild.mjs marks
// vscode external so it resolves only inside a real host. A module reaching
// for vscode.l10n would throw the moment a unit test imported it. So a
// Localizer is injected exactly as showError, showInfo, pick and input already
// are: extension.ts is the one file that reads the editor's display language,
// and it builds one Localizer here and threads it into every host and every
// render call it makes.
//
// The catalogues are imported rather than read off disk. The vsix ignores
// src/, so a file read at run time would find nothing in a packaged
// extension, while an import is inlined into dist/extension.js by esbuild and
// copied beside the compiled module by tsc for the unit layer.

import afCatalog from "./locales/af.json";
import csCatalog from "./locales/cs.json";
import deCatalog from "./locales/de.json";
import enCatalog from "./locales/en.json";
import esCatalog from "./locales/es.json";
import filCatalog from "./locales/fil.json";
import hiCatalog from "./locales/hi.json";
import idCatalog from "./locales/id.json";

/** The eight languages the extension carries a catalogue for. */
export type SupportedTag = "af" | "cs" | "de" | "en" | "es" | "fil" | "hi" | "id";

/**
 * The eight tags, written by hand here and nowhere else.
 *
 * test/unit/l10n.test.ts reads the package.nls.<tag>.json filenames beside
 * package.json and the <tag>.json filenames in src/locales, and holds both
 * sets against this one, so a ninth language added to one half and forgotten
 * in the other fails rather than shipping a translated sidebar beside an
 * English command palette.
 */
export const SUPPORTED_TAGS: readonly SupportedTag[] = [
	"af",
	"cs",
	"de",
	"en",
	"es",
	"fil",
	"hi",
	"id",
];

/** The language every other catalogue is written against and falls back to. */
export const BASE_TAG: SupportedTag = "en";

/** One message: the text a reader sees, and what a translator needs to render it. */
export interface LocaleEntry {
	/** The message, carrying named placeholders in braces. */
	readonly text: string;
	/** What the message is for, and what each placeholder holds. */
	readonly context?: string;
	/** Marks an entry carrying the English text unchanged, awaiting translation. */
	readonly skeleton?: boolean;
	/** Marks a translation whose answer really is the English text, letter for letter. */
	readonly verbatim?: boolean;
	/** A fingerprint of the English this entry was translated from. */
	readonly source?: string;
}

/** One language's messages. */
export interface LocaleCatalog {
	readonly tag: string;
	readonly entries: Readonly<Record<string, LocaleEntry>>;
}

/** Renders one message in one language, with its placeholders filled. */
export type Localizer = (
	key: string,
	params?: Readonly<Record<string, string | number>>,
) => string;

/**
 * The shipped catalogues, keyed by tag.
 *
 * The cast is over the shape JSON import infers, which is a literal type
 * naming every key rather than the map this module reads them as.
 */
export const CATALOGS: Readonly<Record<SupportedTag, LocaleCatalog>> = {
	af: afCatalog as unknown as LocaleCatalog,
	cs: csCatalog as unknown as LocaleCatalog,
	de: deCatalog as unknown as LocaleCatalog,
	en: enCatalog as unknown as LocaleCatalog,
	es: esCatalog as unknown as LocaleCatalog,
	fil: filCatalog as unknown as LocaleCatalog,
	hi: hiCatalog as unknown as LocaleCatalog,
	id: idCatalog as unknown as LocaleCatalog,
};

/** Whether a lowercased string is one of the eight tags. */
function isSupported(candidate: string): candidate is SupportedTag {
	return (SUPPORTED_TAGS as readonly string[]).includes(candidate);
}

/**
 * Matches the editor's own display language against the eight tags.
 *
 * VS Code spells a display language as a BCP 47 tag, so a regional spelling
 * such as `de-ch` or `pt-br` arrives here rather than a bare language. The
 * primary subtag is what a catalogue answers to, so it is tried after the
 * whole string, and a language the extension carries no catalogue for reads
 * as English.
 */
export function resolveTag(envLanguage: string): SupportedTag {
	const lowered = envLanguage.trim().toLowerCase();
	if (isSupported(lowered)) {
		return lowered;
	}
	const primary = lowered.split("-")[0];
	if (primary !== undefined && isSupported(primary)) {
		return primary;
	}
	return BASE_TAG;
}

/** Fills `{name}` placeholders from params, leaving a name nobody passed alone. */
function fill(
	text: string,
	params: Readonly<Record<string, string | number>> | undefined,
): string {
	if (params === undefined) {
		return text;
	}
	return text.replace(/\{([A-Za-z0-9_]+)\}/g, (whole, name: string) => {
		const value = params[name];
		return value === undefined ? whole : String(value);
	});
}

/**
 * Builds a Localizer reading one catalogue with another beneath it.
 *
 * Separate from createLocalizer so a test can drive the two answers below
 * against fixture catalogues rather than against a shipped file, which is
 * what lets it exercise a key one catalogue is missing without a shipped
 * catalogue ever being allowed to miss one.
 *
 * A key the primary catalogue does not carry is answered from the base, which
 * degrades one string rather than the window. That path is unreachable in a
 * shipped build: test/unit/l10n.test.ts holds every catalogue's key set equal
 * to the base's, so a locale missing a key fails the suite long before a
 * reader could meet it.
 *
 * A key neither catalogue carries throws, naming the key and the tag. There is
 * no English text to fall back to there, so the alternatives are a throw and
 * printing the key itself at a reader, and a key on screen is a defect that
 * looks like a translation.
 */
export function localizerOver(
	primary: LocaleCatalog,
	base: LocaleCatalog,
): Localizer {
	return (key, params) => {
		const entry = primary.entries[key] ?? base.entries[key];
		if (entry === undefined) {
			throw new Error(
				`no catalogue entry for ${key} in ${primary.tag} or ${base.tag}`,
			);
		}
		return fill(entry.text, params);
	};
}

/** Builds a Localizer bound to one shipped tag's catalogue. */
export function createLocalizer(tag: SupportedTag): Localizer {
	return localizerOver(CATALOGS[tag], CATALOGS[BASE_TAG]);
}

/**
 * The English Localizer, which is what a render function falls back to when a
 * caller passes none.
 *
 * Every such default is a test call site rather than a production one:
 * extension.ts builds the window's own Localizer from the editor's display
 * language and passes it down every path a reader's text travels, and the
 * parameter exists defaulted so the unit layer's several hundred existing call
 * sites go on asserting on English without each one having to name a
 * localizer.
 */
export const ENGLISH: Localizer = createLocalizer(BASE_TAG);
