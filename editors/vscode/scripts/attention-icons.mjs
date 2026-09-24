// Composes the operator-attention icons: the tree's ordinary glyphs, each
// with a small tinted dot drawn in the icon's upper-right corner, the way VS
// Code's own `bell-dot` composes a badge onto `bell`.
//
// The source drawings are the SVGs the `@vscode/codicons` package ships
// under `src/icons/`, licensed under Creative Commons Attribution 4.0 rather
// than MIT, so a build that composes and ships modified copies of them owes
// the attribution `generateAttentionIcons` writes as `NOTICE.md` beside the
// output (dinah-599 section 6.6, on the operator's ruling to ship them).
//
// Nothing here is committed. `generate()` in esbuild.mjs calls
// `generateAttentionIcons` on every compile, test and package run, the same
// way it already writes the generated pairing module, and the output
// directory is removed and rebuilt each time so a glyph id dropped from
// `attention-glyphs.json` leaves no stale file behind.

import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

/** The dot's own geometry: the same circle drawn last on every composed icon. */
export const DOT = Object.freeze({ cx: 12.5, cy: 3.5, r: 2.75 });

/**
 * The mask's cutout radius around the dot, larger than the dot itself so a
 * ring of the glyph is cut away and the dot reads as separate from it at
 * sixteen pixels, the way VS Code's own `bell-dot` does.
 */
const CUT_RADIUS = 4;

/** The dot's own colour, the same purple for every id, light and dark. */
const DOT_COLOUR = Object.freeze({ light: "#652D90", dark: "#B180D7" });

/** The glyph colour every id but the two below takes. */
const DEFAULT_GLYPH_COLOUR = Object.freeze({ light: "#424242", dark: "#C5C5C5" });

/**
 * The two ids whose glyph keeps its own meaning-bearing colour instead of the
 * default: an active card's blue and a blocked card's red, so a card carrying
 * the dot is still told apart from another by the same colour it draws today.
 */
const GLYPH_COLOUR_OVERRIDES = Object.freeze({
	"record-small": Object.freeze({ light: "#1A85FF", dark: "#3794FF" }),
	"circle-slash": Object.freeze({ light: "#E51400", dark: "#F14C4C" }),
});

/** The glyph colour one id draws in one theme variant. */
export function glyphColour(id, variant) {
	const table = GLYPH_COLOUR_OVERRIDES[id] ?? DEFAULT_GLYPH_COLOUR;
	return table[variant];
}

/** The dot's own colour in one theme variant. */
export function dotColour(variant) {
	return DOT_COLOUR[variant];
}

/**
 * Resolves an icon id to the one codicons source file that draws it.
 *
 * A font name is not always a file name: `circle-outline`, for one, has no
 * `src/icons/circle-outline.svg`, because its drawing is `src/icons/circle.svg`
 * under another of the names sharing its code point. `mapping.json` maps each
 * code point to every name sharing that drawing, so this finds the group
 * containing the id and takes the one member a source file exists for. An id
 * that resolves to no file, or to more than one, is a build error naming the
 * id, because a silent fallback here would draw the wrong glyph, or none,
 * with nothing in a build log saying so.
 */
export function resolveIconFile(id, mapping, iconExists) {
	for (const names of Object.values(mapping)) {
		if (!names.includes(id)) {
			continue;
		}
		const existing = names.filter((name) => iconExists(name));
		if (existing.length !== 1) {
			throw new Error(
				`attention glyph "${id}" resolves to ${String(existing.length)} source file(s) in @vscode/codicons (${existing.join(", ") || "none"})`,
			);
		}
		return existing[0];
	}
	throw new Error(`attention glyph "${id}" is not a codicon id at all`);
}

/** The inner markup of an SVG's root element: everything between the tags. */
export function innerMarkup(svgSource) {
	const match = /<svg[^>]*>([\s\S]*)<\/svg>/.exec(svgSource);
	if (match === null) {
		throw new Error("not a recognisable single-root SVG");
	}
	return match[1];
}

/**
 * Rewrites `fill="currentColor"` inside a glyph's own inner markup to the
 * glyph colour, so a child that names its own fill still takes it. None of
 * the twelve source drawings this build composes does, since each is a
 * single path inheriting `fill="currentColor"` from its own root, but a
 * source that does is not guessed at: it is rewritten.
 */
export function recolourGlyph(inner, colour) {
	return inner.replaceAll('fill="currentColor"', `fill="${colour}"`);
}

/**
 * One id's composed SVG for one theme variant, in the shape dinah-599 section
 * 6.4 fixes: a mask cutting the glyph away in a ring around the dot, the
 * glyph drawn through that mask in its own colour, then the dot drawn last so
 * it sits above the cut.
 */
export function composeIconSvg(sourceSvg, id, variant) {
	const glyph = recolourGlyph(innerMarkup(sourceSvg), glyphColour(id, variant));
	return [
		'<svg width="16" height="16" viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">',
		"  <defs>",
		'    <mask id="dinah-attention-cut" maskUnits="userSpaceOnUse" x="0" y="0" width="16" height="16">',
		'      <rect width="16" height="16" fill="white"/>',
		`      <circle cx="${String(DOT.cx)}" cy="${String(DOT.cy)}" r="${String(CUT_RADIUS)}" fill="black"/>`,
		"    </mask>",
		"  </defs>",
		`  <g fill="${glyphColour(id, variant)}" mask="url(#dinah-attention-cut)">${glyph}</g>`,
		`  <circle cx="${String(DOT.cx)}" cy="${String(DOT.cy)}" r="${String(DOT.r)}" fill="${dotColour(variant)}"/>`,
		"</svg>",
		"",
	].join("\n");
}

/**
 * The attribution notice `generateAttentionIcons` writes beside the icons,
 * naming the work, linking its licence, stating the package version the
 * drawings came from, and saying what this build did to them (dinah-599
 * section 6.6).
 */
export function noticeText(packageVersion) {
	return [
		"# Attention icon drawings",
		"",
		"The composed icons in this directory are modified copies of drawings from",
		'the Visual Studio Code product icon library ("codicons"), copyright',
		"Microsoft Corporation.",
		"",
		"Licence: Creative Commons Attribution 4.0 International",
		"https://creativecommons.org/licenses/by/4.0/",
		"",
		`Source package: @vscode/codicons ${packageVersion}`,
		"",
		"Each icon was modified by composing a notification dot onto the glyph and",
		"recolouring it.",
		"",
	].join("\n");
}

/**
 * Writes every id's light and dark composed icon, and the notice beside them,
 * into `outDir`, removing whatever `outDir` held first so a glyph id dropped
 * from the list leaves no stale file behind.
 */
export function generateAttentionIcons({ glyphIds, codiconsRoot, outDir }) {
	const mapping = JSON.parse(
		readFileSync(join(codiconsRoot, "src", "template", "mapping.json"), "utf8"),
	);
	const iconsDir = join(codiconsRoot, "src", "icons");
	const iconExists = (name) => existsSync(join(iconsDir, `${name}.svg`));
	const packageVersion = JSON.parse(
		readFileSync(join(codiconsRoot, "package.json"), "utf8"),
	).version;

	if (existsSync(outDir)) {
		rmSync(outDir, { recursive: true, force: true });
	}
	mkdirSync(outDir, { recursive: true });

	for (const id of glyphIds) {
		const file = resolveIconFile(id, mapping, iconExists);
		const source = readFileSync(join(iconsDir, `${file}.svg`), "utf8");
		for (const variant of ["light", "dark"]) {
			writeFileSync(
				join(outDir, `${id}-${variant}.svg`),
				composeIconSvg(source, id, variant),
				"utf8",
			);
		}
	}
	writeFileSync(join(outDir, "NOTICE.md"), noticeText(packageVersion), "utf8");
}
