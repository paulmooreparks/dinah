// The guard for the drift no other check in this suite can see.
//
// A translation that matched its English when somebody wrote it goes on
// reading as German or as Hindi after the English moves, so the parity and
// honesty guards in l10n.test.ts pass while the translation says the older
// thing. Each translated entry records a fingerprint of the English it was
// made from, and this file recomputes that fingerprint from the English of the
// day and reports the entry when the two disagree.
//
// The two sides of the comparison come from different places on purpose. The
// stored side is read off the catalogue file as it was committed, and the
// computed side is taken from the base catalogue's current text. A guard that
// derived both from one read of one file at one moment would agree with itself
// whatever either side said.
//
// This is internal/msg's TestATranslationTracksItsEnglishSource ported to the
// extension's two catalogues. Same idea, same two failure messages, separate
// implementation over separate files; nothing here compares a fingerprint
// against one the Go side computed, and no such comparison is claimed
// anywhere.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { fingerprint } from "../../src/fingerprint";
import { BASE_TAG, SUPPORTED_TAGS } from "../../src/l10n";
import type { LocaleCatalog, SupportedTag } from "../../src/l10n";

const extensionRoot = join(__dirname, "..", "..", "..");
const localesDir = join(extensionRoot, "src", "locales");

/** The two languages that ship translated, and so the two that can go stale. */
const TRANSLATED_TAGS: readonly SupportedTag[] = ["de", "hi"];

function readJson<T>(...parts: string[]): T {
	return JSON.parse(readFileSync(join(...parts), "utf8")) as T;
}

interface Flags {
	readonly skeleton: Record<string, string[]>;
	readonly source: Record<string, Record<string, string>>;
}

const flags = readJson<Flags>(localesDir, "flags.json");

/** What one namespace's walk found, so an empty walk can be reported as one. */
interface Verdict {
	readonly checked: number;
	readonly missing: string[];
	readonly stale: string[];
}

/**
 * Walks the runtime namespace for one tag.
 *
 * A skeleton entry is skipped, because it carries the English text rather than
 * a translation and so has no source it could have fallen behind.
 */
function runtimeVerdict(tag: SupportedTag): Verdict {
	const base = readJson<LocaleCatalog>(localesDir, `${BASE_TAG}.json`).entries;
	const entries = readJson<LocaleCatalog>(localesDir, `${tag}.json`).entries;
	const missing: string[] = [];
	const stale: string[] = [];
	let checked = 0;
	for (const [key, english] of Object.entries(base)) {
		const entry = entries[key];
		if (entry === undefined || entry.skeleton === true) {
			continue;
		}
		checked += 1;
		const want = fingerprint(english.text);
		if (entry.source === undefined || entry.source === "") {
			missing.push(key);
		} else if (entry.source !== want) {
			stale.push(key);
		}
	}
	return { checked, missing, stale };
}

/**
 * Walks the manifest namespace for one tag.
 *
 * VS Code fixes a manifest catalogue's value to a bare string, so the
 * fingerprint cannot live on the entry and lives in flags.json's source object
 * instead. That is the same constraint that keeps the two schemas apart.
 */
function manifestVerdict(tag: SupportedTag): Verdict {
	const base = readJson<Record<string, string>>(
		extensionRoot,
		"package.nls.json",
	);
	const skeleton = new Set(flags.skeleton[tag] ?? []);
	const recorded = flags.source[tag] ?? {};
	const missing: string[] = [];
	const stale: string[] = [];
	let checked = 0;
	for (const [key, english] of Object.entries(base)) {
		if (skeleton.has(key)) {
			continue;
		}
		checked += 1;
		const want = fingerprint(english);
		const stored = recorded[key];
		if (stored === undefined || stored === "") {
			missing.push(key);
		} else if (stored !== want) {
			stale.push(key);
		}
	}
	return { checked, missing, stale };
}

function report(tag: string, namespace: string, verdict: Verdict): void {
	// The two failures read differently on purpose, exactly as internal/msg's
	// own guard reports them. An entry with no source was never stamped; an
	// entry whose source disagrees was stamped against English that has since
	// moved, and the reader needs to know which English to go and read.
	for (const key of verdict.missing) {
		assert.fail(
			`${tag}/${namespace}/${key}: carries no recorded source, so write one with fingerprint of the English text before this entry ships`,
		);
	}
	for (const key of verdict.stale) {
		assert.fail(
			`${tag}/${namespace}/${key}: the source is stale, so the English changed after this entry was translated; git log -p ${
				namespace === "runtime"
					? "editors/vscode/src/locales/en.json"
					: "editors/vscode/package.nls.json"
			} shows what changed`,
		);
	}
}

test("the staleness sweep reads every language that ships translated", () => {
	// dinah-424 AC-8's third floor. The per-tag tests below are generated from
	// TRANSLATED_TAGS, so a tag dropped from that roster generates no test at
	// all and every remaining check stays green: the `checked > 0` guards
	// inside each test speak for the language they run on and say nothing
	// about a language that stopped running. The floor is derived from
	// SUPPORTED_TAGS and the shipped flags rather than from the roster it
	// counts, because a floor built out of its own population cannot notice
	// that population shrinking.
	const shipped = SUPPORTED_TAGS.filter(
		(tag) => tag !== BASE_TAG && (flags.skeleton[tag] ?? []).length === 0,
	);
	const absent = shipped.filter((tag) => !TRANSLATED_TAGS.includes(tag)).sort();
	assert.equal(
		TRANSLATED_TAGS.length,
		shipped.length,
		`the staleness sweep covers ${String(TRANSLATED_TAGS.length)} of the ${String(shipped.length)} languages that ship translated, so it never reads ${absent.join(", ")}`,
	);
	assert.deepEqual(
		[...TRANSLATED_TAGS].sort(),
		[...shipped].sort(),
		`the staleness sweep covers the right number of languages and not the right ones: it never reads ${absent.join(", ")}`,
	);
});

for (const tag of TRANSLATED_TAGS) {
	test(`${tag}'s runtime entries still track the English they were translated from`, () => {
		const verdict = runtimeVerdict(tag);
		// A namespace with nothing to check is a failure rather than a pass.
		// Without this, a change that emptied German down to skeletons would
		// leave the guard green while asserting nothing at all, which is the
		// shape internal/msg's own checked == 0 fatal exists to catch.
		assert.ok(
			verdict.checked > 0,
			`${tag} carries no translated runtime entry, so this guard is asserting nothing`,
		);
		report(tag, "runtime", verdict);
	});

	test(`${tag}'s manifest entries still track the English they were translated from`, () => {
		const verdict = manifestVerdict(tag);
		assert.ok(
			verdict.checked > 0,
			`${tag} carries no translated manifest entry, so this guard is asserting nothing`,
		);
		report(tag, "manifest", verdict);
	});
}
