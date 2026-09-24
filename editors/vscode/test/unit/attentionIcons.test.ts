// scripts/attention-icons.mjs, dinah-599 criteria/11: the generator composes
// each attention glyph's light and dark file from the same codicons source
// esbuild.mjs's generate() step reads, with the dot always drawn at the same
// place in the same colour.
//
// The script under test is a plain ECMAScript module and this suite compiles
// to CommonJS, so it is reached through a dynamic import in a `before` hook
// rather than a static import, which tsc would refuse to turn into a
// `require` call.

import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import { pathToFileURL } from "node:url";

const extensionRoot = join(__dirname, "..", "..", "..");
const codiconsRoot = join(extensionRoot, "node_modules", "@vscode", "codicons");
const glyphIds: readonly string[] = JSON.parse(
	readFileSync(join(extensionRoot, "scripts", "attention-glyphs.json"), "utf8"),
);

// The type is resolved against this source file's own location, which is
// where scripts/attention-icons.d.mts sits beside its sibling .mjs, two
// directories up. The module itself is loaded at run time from an absolute
// path instead (below), because this suite compiles to out/test/unit, one
// directory deeper than the source tree, and a literal relative specifier
// cannot resolve correctly for both trees at once.
type AttentionIcons = typeof import(
	"../../scripts/attention-icons.mjs",
	{ with: { "resolution-mode": "import" } }
);
let attentionIcons: AttentionIcons;

before(async () => {
	const modulePath = join(extensionRoot, "scripts", "attention-icons.mjs");
	attentionIcons = (await import(pathToFileURL(modulePath).href)) as AttentionIcons;
});

test("circle-outline resolves to the package's circle.svg, its one existing source", () => {
	const mapping = JSON.parse(
		readFileSync(join(codiconsRoot, "src", "template", "mapping.json"), "utf8"),
	) as Record<string, string[]>;
	const iconExists = (name: string): boolean =>
		existsSync(join(codiconsRoot, "src", "icons", `${name}.svg`));
	assert.equal(attentionIcons.resolveIconFile("circle-outline", mapping, iconExists), "circle");
});

test("an id resolving to no source file throws, naming the id", () => {
	const mapping = { "1": ["not-a-real-glyph"] };
	assert.throws(
		() => attentionIcons.resolveIconFile("not-a-real-glyph", mapping, () => false),
		/not-a-real-glyph/,
	);
});

test("an id resolving to more than one source file throws, naming the id", () => {
	const mapping = { "1": ["a", "b"] };
	assert.throws(() => attentionIcons.resolveIconFile("a", mapping, () => true), /"a"/);
});

test("circle-outline-light.svg carries the path data of the package's circle.svg, and the dot is drawn at the tabled place and colour", () => {
	const source = readFileSync(join(codiconsRoot, "src", "icons", "circle.svg"), "utf8");
	const pathData = /d="([^"]+)"/.exec(source)?.[1];
	assert.ok(
		pathData !== undefined && pathData.length > 0,
		"the source itself carries no path data to compare against",
	);

	const { DOT, composeIconSvg, dotColour, glyphColour } = attentionIcons;
	const composed = composeIconSvg(source, "circle-outline", "light");
	assert.ok(composed.includes(pathData as string), "the composed svg does not carry the source's own path data");
	assert.ok(
		composed.includes(
			`<circle cx="${String(DOT.cx)}" cy="${String(DOT.cy)}" r="${String(DOT.r)}" fill="${dotColour("light")}"/>`,
		),
		"the composed svg does not draw the dot at the tabled place and colour",
	);
	assert.ok(
		composed.includes(`fill="${glyphColour("circle-outline", "light")}"`),
		"the composed svg does not draw the glyph in the default glyph colour",
	);
});

test("record-small and circle-slash keep their own glyph colour rather than the default", () => {
	const { glyphColour } = attentionIcons;
	assert.equal(glyphColour("record-small", "light"), "#1A85FF");
	assert.equal(glyphColour("record-small", "dark"), "#3794FF");
	assert.equal(glyphColour("circle-slash", "light"), "#E51400");
	assert.equal(glyphColour("circle-slash", "dark"), "#F14C4C");
	assert.equal(glyphColour("tools", "light"), "#424242");
	assert.equal(glyphColour("tools", "dark"), "#C5C5C5");
});

test("the dot is purple in both themes, for every id", () => {
	const { dotColour } = attentionIcons;
	assert.equal(dotColour("light"), "#652D90");
	assert.equal(dotColour("dark"), "#B180D7");
});

test("innerMarkup strips the root svg tags and keeps what is between them", () => {
	assert.equal(
		attentionIcons.innerMarkup('<svg width="16"><path d="M0 0"/></svg>'),
		'<path d="M0 0"/>',
	);
});

test("a child naming its own currentColor fill still takes the glyph colour", () => {
	const { composeIconSvg, glyphColour } = attentionIcons;
	const composed = composeIconSvg(
		'<svg width="16" height="16"><path fill="currentColor" d="M0 0"/></svg>',
		"tools",
		"dark",
	);
	assert.ok(!composed.includes('fill="currentColor"'), "currentColor survived into the composed output");
	assert.ok(composed.includes(`fill="${glyphColour("tools", "dark")}" d="M0 0"`));
});

// Arming: reading back a variant this run did not write, or an id this run
// did not compose, fails by name rather than passing vacuously, because the
// directory generateAttentionIcons writes into is empty before this call.
test("generateAttentionIcons writes a light and a dark file for every listed id, and the NOTICE", () => {
	const { DOT, generateAttentionIcons } = attentionIcons;
	const outDir = mkdtempSync(join(tmpdir(), "dinah-attention-"));
	try {
		generateAttentionIcons({ glyphIds, codiconsRoot, outDir });
		const written = new Set(readdirSync(outDir));
		assert.ok(written.has("NOTICE.md"), "no NOTICE.md was written");
		const notice = readFileSync(join(outDir, "NOTICE.md"), "utf8");
		assert.match(notice, /Creative Commons Attribution 4\.0/);
		assert.match(notice, /@vscode\/codicons/);
		for (const id of glyphIds) {
			for (const variant of ["light", "dark"]) {
				const name = `${id}-${variant}.svg`;
				assert.ok(written.has(name), `${name} was not written`);
				const svg = readFileSync(join(outDir, name), "utf8");
				assert.ok(svg.includes("<mask"), `${name} carries no mask`);
				assert.ok(
					svg.includes(`r="${String(DOT.r)}"`),
					`${name} does not draw the dot at the tabled radius`,
				);
			}
		}
		// Every file this run wrote is accounted for above: two per id, plus
		// the notice, and nothing else, which is what "removed first" buys.
		assert.equal(written.size, glyphIds.length * 2 + 1);
	} finally {
		rmSync(outDir, { recursive: true, force: true });
	}
});

test("a second run removes a file left by a dropped id", () => {
	const { generateAttentionIcons } = attentionIcons;
	const outDir = mkdtempSync(join(tmpdir(), "dinah-attention-"));
	try {
		generateAttentionIcons({ glyphIds: [...glyphIds, "gear"], codiconsRoot, outDir });
		assert.ok(existsSync(join(outDir, "gear-light.svg")));
		generateAttentionIcons({ glyphIds, codiconsRoot, outDir });
		assert.ok(
			!existsSync(join(outDir, "gear-light.svg")),
			"the id dropped from the list left a stale file behind",
		);
	} finally {
		rmSync(outDir, { recursive: true, force: true });
	}
});
