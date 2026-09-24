// Declares the shapes verify-package.mjs's own tests import, so a unit test
// can drive the real packaging assertion under tsc's strict checking. This
// script is deliberately plain JavaScript, on the terms every other file
// under scripts/ is; this declaration file exists for the one caller outside
// its own CLI run that needs its exports typed.

export const ATTENTION_GLYPH_IDS: readonly string[];
export const ATTENTION_NOTICE_ENTRY: string;
export const attentionIconEntries: readonly string[];
export function attentionProblems(
	label: string,
	entries: readonly string[],
	glyphEntries?: readonly string[],
): string[];
