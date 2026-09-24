// Declares the shapes attention-icons.mjs's own tests import, so a unit test
// can drive the real composition logic under tsc's strict checking. This
// script is deliberately plain JavaScript, on the terms every other file
// under scripts/ is; this declaration file exists for the one caller outside
// esbuild.mjs that needs its exports typed.

export const DOT: { readonly cx: number; readonly cy: number; readonly r: number };
export function glyphColour(id: string, variant: "light" | "dark"): string;
export function dotColour(variant: "light" | "dark"): string;
export function resolveIconFile(
	id: string,
	mapping: Record<string, readonly string[]>,
	iconExists: (name: string) => boolean,
): string;
export function innerMarkup(svgSource: string): string;
export function recolourGlyph(inner: string, colour: string): string;
export function composeIconSvg(sourceSvg: string, id: string, variant: "light" | "dark"): string;
export function noticeText(packageVersion: string): string;
export function generateAttentionIcons(options: {
	readonly glyphIds: readonly string[];
	readonly codiconsRoot: string;
	readonly outDir: string;
}): void;
