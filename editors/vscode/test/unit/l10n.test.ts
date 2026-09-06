// The guards over the two catalogues: that they carry the same keys, that an
// entry reading English says so, that both languages use the words the CLI's
// own glossary declares, and that the tag set is written down in one place.
//
// The two catalogues are two files with two schemas, because VS Code fixes the
// manifest side to a flat string map and the runtime side needs somewhere to
// put the skeleton and verbatim flags. src/locales/flags.json is the sidecar
// that lets the manifest side carry those flags anyway, so the honesty rule
// below is one rule applied twice rather than two rules that can drift.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { fingerprint } from "../../src/fingerprint";
import {
	BASE_TAG,
	CATALOGS,
	SUPPORTED_TAGS,
	createLocalizer,
	localizerOver,
	resolveTag,
} from "../../src/l10n";
import type { LocaleCatalog, SupportedTag } from "../../src/l10n";

// This file is compiled to out/test/unit/, so the extension root is three up
// and the repository root two above that.
const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");
const localesDir = join(extensionRoot, "src", "locales");

/** The five languages that ship as declared placeholders rather than translations. */
const SKELETON_TAGS: readonly SupportedTag[] = ["af", "cs", "es", "fil", "id"];

/** The two languages that ship translated, matching what internal/msg carries. */
const TRANSLATED_TAGS: readonly SupportedTag[] = ["de", "hi"];

function readJson<T>(...parts: string[]): T {
	return JSON.parse(readFileSync(join(...parts), "utf8")) as T;
}

/** One tag's manifest catalogue, read off the file that actually ships. */
function manifestCatalog(tag: SupportedTag): Record<string, string> {
	const name = tag === BASE_TAG ? "package.nls.json" : `package.nls.${tag}.json`;
	return readJson<Record<string, string>>(extensionRoot, name);
}

/** One tag's runtime catalogue, read off the file rather than off the import. */
function runtimeCatalog(tag: SupportedTag): LocaleCatalog {
	return readJson<LocaleCatalog>(localesDir, `${tag}.json`);
}

interface Flags {
	readonly skeleton: Record<string, string[]>;
	readonly verbatim: Record<string, string[]>;
	readonly glossary: GlossaryTerm[];
	readonly source: Record<string, Record<string, string>>;
}

interface GlossaryTerm {
	readonly en: string;
	readonly forms: Record<string, string[]>;
}

/**
 * The manifest namespace's honesty flags, plus the glossary both namespaces
 * read.
 *
 * The runtime namespace carries its own flags on each entry, because its
 * schema has room for them. This file exists for the manifest namespace, whose
 * values are bare strings by VS Code's own rule.
 */
const flags = readJson<Flags>(localesDir, "flags.json");

const sortedKeys = (record: Record<string, unknown>): string[] =>
	Object.keys(record).sort();

// ---------------------------------------------------------------------------
// The manifest namespace
// ---------------------------------------------------------------------------

test("every %key% the manifest spells resolves to a key in package.nls.json", () => {
	// AC-1. The walk is over the parsed tree rather than over the file's text,
	// so a placeholder inside a `when` clause or a menu group would be found
	// too, and a comment could not hide one.
	const base = manifestCatalog(BASE_TAG);
	const found: string[] = [];
	const walk = (value: unknown): void => {
		if (typeof value === "string") {
			for (const match of value.matchAll(/%[A-Za-z0-9_.-]+%/g)) {
				found.push(match[0]);
			}
			return;
		}
		if (Array.isArray(value)) {
			value.forEach(walk);
			return;
		}
		if (value !== null && typeof value === "object") {
			Object.values(value).forEach(walk);
		}
	};
	walk(readJson<unknown>(extensionRoot, "package.json"));

	// A vacuous pass is the failure mode here: a manifest that lost every
	// placeholder would satisfy the loop below and say nothing.
	assert.equal(
		found.length,
		Object.keys(base).length,
		"every catalogue key is spelled exactly once in the manifest",
	);
	for (const placeholder of found) {
		const key = placeholder.slice(1, -1);
		assert.ok(
			Object.prototype.hasOwnProperty.call(base, key),
			`package.json spells ${placeholder} and package.nls.json carries no ${key}`,
		);
	}
});

test("every manifest sibling carries exactly the base catalogue's keys", () => {
	// AC-2. Set equality in both directions, and the failure names the keys
	// rather than the count, because a count sends a reader back to a diff.
	const base = new Set(Object.keys(manifestCatalog(BASE_TAG)));
	for (const tag of SUPPORTED_TAGS) {
		if (tag === BASE_TAG) {
			continue;
		}
		const sibling = new Set(Object.keys(manifestCatalog(tag)));
		const missing = [...base].filter((key) => !sibling.has(key)).sort();
		const extra = [...sibling].filter((key) => !base.has(key)).sort();
		assert.deepEqual(missing, [], `package.nls.${tag}.json is missing keys`);
		assert.deepEqual(extra, [], `package.nls.${tag}.json carries extra keys`);
	}
});

test("a manifest entry that reads as English says so, and one that says so reads as English", () => {
	// AC-3. Both directions. An entry equal to the English with no flag is
	// English left standing where a translation should be, and an entry
	// flagged as a placeholder or as deliberately identical while differing
	// from the English is a flag that has outlived its claim.
	const base = manifestCatalog(BASE_TAG);
	const unflagged: string[] = [];
	const lying: string[] = [];
	for (const tag of SUPPORTED_TAGS) {
		if (tag === BASE_TAG) {
			continue;
		}
		const sibling = manifestCatalog(tag);
		const skeleton = new Set(flags.skeleton[tag] ?? []);
		const verbatim = new Set(flags.verbatim[tag] ?? []);
		for (const [key, english] of Object.entries(base)) {
			const declared = skeleton.has(key) || verbatim.has(key);
			if (sibling[key] === english && !declared) {
				unflagged.push(`${tag}/${key}`);
			}
			if (sibling[key] !== english && declared) {
				lying.push(`${tag}/${key}`);
			}
		}
	}
	assert.deepEqual(unflagged, [], "English left standing under another tag");
	assert.deepEqual(lying, [], "flagged as English while differing from it");
});

test("the manifest's two translated languages carry no skeleton and its five skeletons carry every key", () => {
	// AC-4.
	const keys = sortedKeys(manifestCatalog(BASE_TAG));
	for (const tag of TRANSLATED_TAGS) {
		assert.deepEqual(flags.skeleton[tag], [], `${tag} ships translated`);
	}
	for (const tag of SKELETON_TAGS) {
		assert.deepEqual(
			[...(flags.skeleton[tag] ?? [])].sort(),
			keys,
			`${tag} ships as a fully flagged skeleton`,
		);
	}
});

// ---------------------------------------------------------------------------
// The runtime namespace
// ---------------------------------------------------------------------------

test("every runtime sibling carries exactly the base catalogue's keys", () => {
	// AC-5, checked the same way as AC-2 and reporting the same way.
	const base = new Set(Object.keys(runtimeCatalog(BASE_TAG).entries));
	for (const tag of SUPPORTED_TAGS) {
		if (tag === BASE_TAG) {
			continue;
		}
		const catalog = runtimeCatalog(tag);
		assert.equal(catalog.tag, tag, `${tag}.json answers to its own filename`);
		const sibling = new Set(Object.keys(catalog.entries));
		const missing = [...base].filter((key) => !sibling.has(key)).sort();
		const extra = [...sibling].filter((key) => !base.has(key)).sort();
		assert.deepEqual(missing, [], `src/locales/${tag}.json is missing keys`);
		assert.deepEqual(extra, [], `src/locales/${tag}.json carries extra keys`);
	}
});

test("a runtime entry that reads as English says so, and the five skeletons say it everywhere", () => {
	// AC-6, the runtime equivalent of AC-3 and AC-4. The flags live on the
	// entry here rather than in the sidecar, because this schema has room for
	// them.
	const base = runtimeCatalog(BASE_TAG).entries;
	const unflagged: string[] = [];
	const lying: string[] = [];
	for (const tag of SUPPORTED_TAGS) {
		if (tag === BASE_TAG) {
			continue;
		}
		const entries = runtimeCatalog(tag).entries;
		for (const [key, english] of Object.entries(base)) {
			const entry = entries[key];
			const declared = entry.skeleton === true || entry.verbatim === true;
			if (entry.text === english.text && !declared) {
				unflagged.push(`${tag}/${key}`);
			}
			if (entry.text !== english.text && declared) {
				lying.push(`${tag}/${key}`);
			}
			assert.ok(
				!(entry.skeleton === true && entry.verbatim === true),
				`${tag}/${key} claims to be a placeholder and a translation at once`,
			);
		}
	}
	assert.deepEqual(unflagged, [], "English left standing under another tag");
	assert.deepEqual(lying, [], "flagged as English while differing from it");

	const keys = sortedKeys(base);
	for (const tag of TRANSLATED_TAGS) {
		const entries = runtimeCatalog(tag).entries;
		const skeletons = keys.filter((key) => entries[key].skeleton === true);
		assert.deepEqual(skeletons, [], `${tag} ships translated`);
	}
	for (const tag of SKELETON_TAGS) {
		const entries = runtimeCatalog(tag).entries;
		const skeletons = keys.filter((key) => entries[key].skeleton === true);
		assert.deepEqual(skeletons, keys, `${tag} ships as a fully flagged skeleton`);
	}
});

// ---------------------------------------------------------------------------
// The glossary, borrowed from the CLI rather than invented a second time
// ---------------------------------------------------------------------------

/** The four span shapes that name a thing rather than saying a word about it. */
const MACHINE_SPANS = [/\{[^}]*\}/g, /`[^`]*`/g, /<[^>]*>/g, /--[A-Za-z0-9-]+/g];

/** Returns text with every machine-vocabulary span removed. */
function prose(text: string): string {
	let plain = text;
	for (const span of MACHINE_SPANS) {
		plain = plain.replace(span, "");
	}
	return plain;
}

test("flags.json's glossary is the CLI's own, byte for byte", () => {
	// The extension does not invent a second term list. Holding the copy to
	// the original is what keeps the two from drifting, since nothing else
	// would notice a term added to internal/msg and not here.
	const original = readJson<GlossaryTerm[]>(
		repoRoot,
		"internal",
		"msg",
		"glossary.json",
	);
	assert.deepEqual(flags.glossary, original);
});

test("a translated entry renders each declared term with a declared word", () => {
	// AC-7, mirroring internal/msg's TestATranslationUsesTheDeclaredWord. The
	// trigger is the English phrase on word boundaries against the base text
	// with its machine spans removed, and the match is containment of a
	// declared form in the translation, because a language inflects its own
	// words and spells them inside compounds.
	//
	// Only the two languages the CLI's glossary declares forms for are
	// checked. The five skeleton languages carry no translation to hold to a
	// word, and the glossary declares nothing for them either.
	assert.ok(flags.glossary.length > 0, "the glossary carries no terms");
	const base = runtimeCatalog(BASE_TAG).entries;
	const baseManifest = manifestCatalog(BASE_TAG);
	let checked = 0;
	for (const tag of SUPPORTED_TAGS) {
		if (tag === BASE_TAG) {
			continue;
		}
		const runtime = runtimeCatalog(tag).entries;
		const manifest = manifestCatalog(tag);
		const skeleton = new Set(flags.skeleton[tag] ?? []);
		const pairs: { key: string; english: string; text: string }[] = [];
		for (const [key, entry] of Object.entries(base)) {
			if (runtime[key].skeleton === true) {
				continue;
			}
			pairs.push({ key, english: entry.text, text: runtime[key].text });
		}
		for (const [key, english] of Object.entries(baseManifest)) {
			if (skeleton.has(key)) {
				continue;
			}
			pairs.push({ key, english, text: manifest[key] });
		}
		for (const term of flags.glossary) {
			const forms = term.forms[tag];
			if (forms === undefined) {
				continue;
			}
			const trigger = new RegExp(
				`\\b${term.en.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}\\b`,
				"i",
			);
			for (const pair of pairs) {
				if (!trigger.test(prose(pair.english))) {
					continue;
				}
				checked += 1;
				assert.ok(
					forms.some((form) => pair.text.includes(form)),
					`${tag}/${pair.key}: wanted the glossary word for "${term.en}" (one of ${forms.join(", ")}), got "${pair.text}"`,
				);
			}
		}
	}
	assert.ok(checked > 0, "no entry triggered a glossary term");
});

// ---------------------------------------------------------------------------
// The module itself
// ---------------------------------------------------------------------------

test("l10n.ts imports no vscode symbol and writes the eight tags down once", () => {
	// AC-8. The module is imported by tree.ts and by every command host, all
	// of which the unit layer runs under plain node with no extension host, so
	// a vscode import here would take the whole layer down.
	const source = readFileSync(join(extensionRoot, "src", "l10n.ts"), "utf8");
	const importsVscode = source
		.split("\n")
		.filter((line) => /^import\b/.test(line) && /["']vscode["']/.test(line));
	assert.deepEqual(importsVscode, []);
	assert.deepEqual(
		[...SUPPORTED_TAGS],
		["af", "cs", "de", "en", "es", "fil", "hi", "id"],
	);
});

test("resolveTag answers each supported tag with itself and anything else with English", () => {
	// AC-9, nine cases asserted one at a time so a failure names the tag.
	for (const tag of SUPPORTED_TAGS) {
		assert.equal(resolveTag(tag), tag);
	}
	assert.equal(resolveTag("fr"), "en");
	// VS Code spells a display language as a BCP 47 tag, so the regional
	// spellings are what a reader in Switzerland or Brazil actually arrives
	// with. Neither is one of the nine cases above; both are why the primary
	// subtag is tried.
	assert.equal(resolveTag("de-ch"), "de");
	assert.equal(resolveTag("pt-br"), "en");
});

test("a localizer answers a key its own catalogue is missing out of the base, and refuses one neither carries", () => {
	// AC-10. The missing-key arm is driven from fixtures rather than from a
	// shipped file, because the parity tests above mean no shipped catalogue
	// is allowed to miss a key, so the arm has no shipped case to exercise.
	const base: LocaleCatalog = {
		tag: "en",
		entries: {
			"fixture.kept": { text: "kept {who}" },
			"fixture.dropped": { text: "dropped" },
		},
	};
	const primary: LocaleCatalog = {
		tag: "de",
		entries: { "fixture.kept": { text: "behalten {who}" } },
	};
	const t = localizerOver(primary, base);
	assert.equal(t("fixture.kept", { who: "alka" }), "behalten alka");
	assert.equal(t("fixture.dropped"), "dropped");
	assert.throws(
		() => t("fixture.absent"),
		/fixture\.absent.*de.*en/,
		"the throw names the key and both tags",
	);

	// The same refusal on a real catalogue, which is the arm a shipped
	// catalogue can actually reach.
	assert.throws(() => createLocalizer("de")("no.such.key"), /no\.such\.key/);
	// A placeholder nobody passed a value for is left alone rather than
	// rendered as the word undefined.
	assert.equal(createLocalizer("en")("dialog.card.copiedRef"), "Copied {ref}");
});

test("the tag list, the manifest siblings and the runtime catalogues all name the same eight languages", () => {
	// AC-11, and the reason it exists: both halves read vscode.env.language
	// independently, VS Code's own loader for the manifest and resolveTag for
	// the runtime, so a ninth language added to one half and forgotten in the
	// other would show a reader a translated sidebar beside an English command
	// palette. All three sets are read off the directory rather than typed out
	// again here.
	const expected = [...SUPPORTED_TAGS].sort();

	const manifestTags = readdirSync(extensionRoot)
		.map((name) => /^package\.nls(?:\.([A-Za-z-]+))?\.json$/.exec(name))
		.filter((match): match is RegExpExecArray => match !== null)
		.map((match) => match[1] ?? BASE_TAG)
		.sort();
	assert.deepEqual(manifestTags, expected);

	const runtimeTags = readdirSync(localesDir)
		.filter((name) => name.endsWith(".json") && name !== "flags.json")
		.map((name) => name.slice(0, -".json".length))
		.sort();
	assert.deepEqual(runtimeTags, expected);

	// The imported map is the third place the eight are written, and it is
	// what the extension actually reads at run time.
	assert.deepEqual(sortedKeys(CATALOGS), expected);
});

test("fingerprint is FNV-1a 64-bit over the UTF-8 bytes, formatted as lowercase hex", () => {
	// AC-17's half about the function rather than about the catalogues. The
	// empty string is the offset basis untouched, which pins the constant; the
	// two-character case pins the multiply and the mask; and the non-ASCII
	// case pins the encoding, since hashing UTF-16 code units would answer
	// differently.
	assert.equal(fingerprint(""), "cbf29ce484222325");
	assert.equal(fingerprint("a"), "af63dc4c8601ec8c");
	assert.equal(fingerprint("ab"), "89c4407b545986a");
	assert.notEqual(fingerprint("ä"), fingerprint("a"));
	assert.match(fingerprint("damaged"), /^[0-9a-f]+$/);
});
