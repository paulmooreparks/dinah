// The client half of `dinah lsp`: what the extension asks the server for, and
// what it draws with the answer.
//
// Nothing here imports vscode. The middleware, the chip's text and the colour
// a chip is drawn in are pure functions over the structured annotation the
// server publishes, so the unit layer reaches all three without a VS Code
// host, and extension.ts does the wiring.
//
// The division of labour is the card's own ruling. The server says what an
// annotation means, in the canonical tokens the machine surface already uses;
// the client decides what that looks like. So nothing below parses a
// sentence: a chip is coloured from `fields`, and `label` is for a person and
// is translated.

/// The settings section the server reads its own two settings out of.
export const LSP_SECTION = "dinah.lsp";

/// The namespaced request a client makes to read one document's structured
/// annotations, which no standard member of an inlay hint can carry.
export const ANNOTATIONS_REQUEST = "dinah/annotations";

/// The namespaced notification the server sends when an open document's model
/// was recomputed into something different from what it replaced.
export const ANNOTATIONS_CHANGED = "dinah/annotationsChanged";

/// A zero-based position in a document, as the protocol spells one.
export interface LspPosition {
	line: number;
	character: number;
}

/// A span of one document.
export interface LspRange {
	start: LspPosition;
	end: LspPosition;
}

/// What an annotation's reference resolved to, in canonical tokens. It is
/// null when the reference resolved to nothing.
export interface AnnotationTarget {
	kind: string;
	id: string;
	ref: string;
	card: string | null;
}

/// One annotation the server published. `label` and `tooltip` are for a
/// person and are translated; `target` and `fields` are for the machine and
/// never are.
export interface DinahAnnotation {
	range: LspRange;
	label: string;
	tooltip: string;
	target: AnnotationTarget | null;
	fields: Record<string, string>;
	link?: string;
}

/// The answer to one dinah/annotations request.
export interface AnnotationsAnswer {
	annotations: DinahAnnotation[];
}

/// The parameters of one dinah/annotationsChanged notification.
export interface AnnotationsChanged {
	uris: string[];
}

/// The provider the middleware wraps, written structurally so this module
/// needs no vscode types.
///
/// The parameters are `never` on purpose. A middleware member is assigned to
/// the client's own Middleware type, and a function parameter is checked
/// contravariantly, so a `next` whose parameters were `unknown` would refuse
/// the concrete signature the client hands it.
export type HintProvider<Hint> = (
	document: never,
	viewPort: never,
	token: never,
) => Hint[] | null | undefined | PromiseLike<Hint[] | null | undefined>;

/// suppressInlayHints is the middleware the card's ruling turns on.
///
/// VS Code draws the annotation as a decoration built from the structured
/// answer rather than as a hint, so a chip and a hint must never draw
/// together. The middleware asks the server anyway and then delivers none of
/// what it answered, which costs one round trip against a model the server
/// has already computed.
///
/// Calling `next` rather than short-circuiting is deliberate. A middleware
/// that never asks would make the suppression indistinguishable from a server
/// that answers nothing, and the test that pins this asserts both halves: that
/// the server answered a nonzero number of hints, and that none of them
/// reached the editor.
export function suppressInlayHints<Hint>(observe?: (delivered: number) => void): {
	provideInlayHints: (
		document: unknown,
		viewPort: unknown,
		token: unknown,
		next: HintProvider<Hint>,
	) => Promise<Hint[]>;
} {
	return {
		provideInlayHints: async (
			document: unknown,
			viewPort: unknown,
			token: unknown,
			next: HintProvider<Hint>,
		): Promise<Hint[]> => {
			// The one cast in this module. next's parameters are declared
			// never so the member is assignable to the client's Middleware
			// type, which leaves no way to call it but to widen it back.
			const forward = next as unknown as (
				document: unknown,
				viewPort: unknown,
				token: unknown,
			) => Hint[] | null | undefined | PromiseLike<Hint[] | null | undefined>;
			const answered = await forward(document, viewPort, token);
			observe?.(answered?.length ?? 0);
			return [];
		},
	};
}

/// The canonical tokens a chip is coloured on. They are the values the
/// server's own `fields` member carries, never words read out of a label.
export const STATE_BLOCKED = "blocked";
export const STATE_ACTIVE = "active";
export const HOLD_ON = "on";
export const HOLD_OUT = "out";
export const HOLD_BOTH = "both";

/// The classes a chip is drawn in. They are this client's own vocabulary
/// rather than the server's, which is the point of the division: a later
/// client colours differently without the server changing.
export type ChipTone = "blocked" | "active" | "holding" | "unresolved" | "plain";

/// toneOf decides how one chip is drawn, from the canonical tokens alone.
export function toneOf(annotation: DinahAnnotation): ChipTone {
	if (annotation.target === null) {
		return "unresolved";
	}
	const fields = annotation.fields ?? {};
	if (fields.state === STATE_BLOCKED) {
		return "blocked";
	}
	if (fields.state === STATE_ACTIVE) {
		return "active";
	}
	if (fields.hold === HOLD_ON || fields.hold === HOLD_OUT || fields.hold === HOLD_BOTH) {
		return "holding";
	}
	return "plain";
}

/// The brackets a chip is drawn inside. They are the client's own rendering
/// of an annotation, so the server's label stays a sentence rather than
/// becoming a format.
const CHIP_OPEN = "⟨";
const CHIP_CLOSE = "⟩";

/// chipTextOf is what a chip reads, which is the server's label between the
/// client's own brackets.
export function chipTextOf(annotation: DinahAnnotation): string {
	return `${CHIP_OPEN}${annotation.label}${CHIP_CLOSE}`;
}

/// One chip to draw: where it goes, what it reads, how it is toned, and what
/// its tooltip says.
export interface Chip {
	range: LspRange;
	text: string;
	tone: ChipTone;
	tooltip: string;
}

/// chipsFrom turns one document's annotations into the chips a client draws.
export function chipsFrom(answer: AnnotationsAnswer | null | undefined): Chip[] {
	return (answer?.annotations ?? []).map((annotation) => ({
		range: annotation.range,
		text: chipTextOf(annotation),
		tone: toneOf(annotation),
		tooltip: annotation.tooltip,
	}));
}
