// The guard over what a translation does with the placeholders its English
// carries.
//
// A message names its values in braces and the renderer fills them. A
// translator who drops one ships a sentence with a hole in it: `Copied {ref}`
// translated as ` kopiert` tells a reader something was copied and never tells
// them what. The parity, honesty, glossary and staleness guards all stay green
// through that edit, because none of them reads a message's placeholders.
//
// The reverse is the same defect wearing the other hat. A translation naming a
// value nobody passes renders the literal characters of the name at a reader,
// because fill leaves a name it was given no value for alone. That is a defect
// that looks like a translation, and internal/msg's own placeholder guard is
// one-directional by construction too, so neither half of this project caught
// it before this file.
//
// Both directions are reported, per language, and comparison is by set rather
// than by count. Word order and repetition are the translator's business:
// English naming {ref} twice where German names it once is correct German and
// must not fire, in a language nobody here reads. A name present on one side
// and absent on the other is the defect.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { BASE_TAG, SUPPORTED_TAGS } from "../../src/l10n";
import type { LocaleCatalog, LocaleEntry, SupportedTag } from "../../src/l10n";

const extensionRoot = join(__dirname, "..", "..", "..");
const localesDir = join(extensionRoot, "src", "locales");

/**
 * The placeholder names a message carries, as a set.
 *
 * The pattern is character for character the one `fill` in src/l10n.ts uses.
 * Nothing else may be written here: a guard whose idea of a placeholder
 * differs from the renderer's checks a different thing from the one that
 * ships, and the divergence shows up as a name the guard thinks is safe and
 * the renderer leaves unfilled.
 */
function placeholdersIn(text: string): Set<string> {
	const found = new Set<string>();
	for (const match of text.matchAll(/\{([A-Za-z0-9_]+)\}/g)) {
		found.add(match[1]);
	}
	return found;
}

/** What one language's walk found, and how much of it there was to find. */
interface PlaceholderVerdict {
	/** Key pairs compared. */
	readonly pairs: number;
	/** English placeholder names examined, counted per pair. */
	readonly placeholders: number;
	/** One entry per name the English carries and the translation does not. */
	readonly dropped: string[];
	/** One entry per name the translation carries and the English does not. */
	readonly invented: string[];
}

/**
 * Compares one catalogue's placeholders against the base catalogue's.
 *
 * The two catalogues are arguments rather than reads off disk, for
 * localizerOver's reason: a fixture can then drive this over a catalogue that
 * really does drop a placeholder, and no shipped catalogue has to be allowed
 * to carry a defect in order for the check to have an armed path.
 */
function placeholderVerdict(
	base: Readonly<Record<string, LocaleEntry>>,
	other: Readonly<Record<string, LocaleEntry>>,
	tag: string,
): PlaceholderVerdict {
	const dropped: string[] = [];
	const invented: string[] = [];
	let pairs = 0;
	let placeholders = 0;

	for (const [key, english] of Object.entries(base)) {
		const entry = other[key];
		if (entry === undefined) {
			continue;
		}
		pairs += 1;
		const wanted = placeholdersIn(english.text);
		const carried = placeholdersIn(entry.text);
		placeholders += wanted.size;
		for (const name of wanted) {
			if (!carried.has(name)) {
				dropped.push(`${tag}/${key}: {${name}}`);
			}
		}
		for (const name of carried) {
			if (!wanted.has(name)) {
				invented.push(`${tag}/${key}: {${name}}`);
			}
		}
	}

	return { pairs, placeholders, dropped, invented };
}

function readCatalog(tag: string): LocaleCatalog {
	return JSON.parse(readFileSync(join(localesDir, `${tag}.json`), "utf8")) as LocaleCatalog;
}

/** The seven catalogues a translation could differ from English in. */
const OTHER_TAGS: readonly SupportedTag[] = SUPPORTED_TAGS.filter((tag) => tag !== BASE_TAG);

// One test per tag rather than one over all seven, matching
// l10n-staleness.test.ts's shape so a failure names the language. The floors
// are per tag for the same reason: the seven split into two populations that
// fail differently, 236 real pairs of German and Hindi against 590 trivially
// true ones from the five skeletons, and a single counter would hide either
// behind the other.
//
// The five skeletons are compared rather than skipped. A skeleton is
// byte-identical English, so the comparison is trivially true today and the
// honesty guard in l10n.test.ts is what keeps it so. It stays in because the
// day somebody translates one of the five is the day this check starts doing
// work there, with nothing to edit for that to happen.
for (const tag of OTHER_TAGS) {
	test(`${tag} keeps the placeholder names its English carries, and invents none`, () => {
		const base = readCatalog(BASE_TAG).entries;
		const verdict = placeholderVerdict(base, readCatalog(tag).entries, tag);

		assert.ok(
			verdict.pairs > 0,
			`${tag} shares no key with the base catalogue, so this guard compared nothing`,
		);
		assert.ok(
			verdict.placeholders > 0,
			`${tag} read no English placeholder at all, so this guard is asserting nothing`,
		);

		assert.deepEqual(
			verdict.dropped,
			[],
			"each line above is a value the English names and the translation does not, so the reader is told something happened and never told what",
		);
		assert.deepEqual(
			verdict.invented,
			[],
			"each line above is a value the translation names and nobody passes, so fill leaves the braces alone and the reader sees the name of the value",
		);
	});
}

test("the verdict reports a dropped name, an invented one, and nothing for a reordering", () => {
	// The clean case pinned beside the two refusing ones. Without the third
	// key, a verdict function that reported every pair would satisfy both
	// per-tag assertions above when they were armed.
	const base: Record<string, LocaleEntry> = {
		"fixture.dropped": { text: "Copied {ref}" },
		"fixture.invented": { text: "Moved {card}" },
		"fixture.reordered": { text: "{from} became {to}, and {from} is gone" },
	};
	const other: Record<string, LocaleEntry> = {
		"fixture.dropped": { text: " kopiert" },
		"fixture.invented": { text: "{card} verschoben nach {ziel}" },
		"fixture.reordered": { text: "{to} kommt von {from}" },
	};

	const verdict = placeholderVerdict(base, other, "xx");

	assert.equal(verdict.pairs, 3);
	assert.equal(verdict.placeholders, 4);
	assert.deepEqual(verdict.dropped, ["xx/fixture.dropped: {ref}"]);
	assert.deepEqual(verdict.invented, ["xx/fixture.invented: {ziel}"]);
});

test("a manifest value carries no placeholder, because VS Code fills none", () => {
	// The manifest namespace gets this rather than a parity check. VS Code
	// substitutes nothing into a package.nls value, so a placeholder written
	// into one renders as its own characters, and a parity check over the
	// namespace would compare every pair against zero placeholders and assert
	// nothing at all.
	const findings: string[] = [];
	let values = 0;
	for (const tag of SUPPORTED_TAGS) {
		const file = tag === BASE_TAG ? "package.nls.json" : `package.nls.${tag}.json`;
		const catalog = JSON.parse(readFileSync(join(extensionRoot, file), "utf8")) as Record<
			string,
			string
		>;
		for (const [key, value] of Object.entries(catalog)) {
			values += 1;
			if (value.includes("{") || value.includes("}")) {
				findings.push(`${tag}/${key}: ${value}`);
			}
		}
	}

	assert.ok(values > 0, "no manifest value was read at all, so this guard is asserting nothing");
	assert.deepEqual(
		findings,
		[],
		"each line above carries braces in a manifest value, which the editor fills nothing into, so a reader meets the characters themselves",
	);
});
