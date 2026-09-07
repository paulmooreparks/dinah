// The status bar item's text and tooltip.
//
// Both are composed by a pure function so that an integration test can assert
// on them. A status bar item cannot be read back through the VS Code API at
// all, so the composed strings ride on the object activate() returns and this
// module is what produces them.

import type { BinaryState, WorkbenchResolution } from "./api";
import {
	AMBIGUOUS_WORKBENCH,
	NO_CONFIGURED_WORKBENCH,
	NO_WORKBENCH_FOUND,
} from "./workbench";
import { describeVersion } from "./version";
import { ENGLISH } from "./l10n";
import type { Localizer } from "./l10n";
import type { CardView } from "./wire";

/** What the status bar shows, or that it shows nothing. */
export interface StatusView {
	readonly hidden: boolean;
	readonly text: string;
	readonly tooltip: string;
}

/**
 * Every value the `dinah.binary` context key can hold. It reads "missing" when
 * no usable binary was found and "ok" otherwise.
 *
 * The welcome blocks in package.json partition the product of this set and the
 * one below, and the manifest test enumerates that product from these two
 * arrays rather than from a list somebody typed out again. A value added here
 * with no welcome block to receive it turns that test red, because the state it
 * names would match no block at all.
 */
export const BINARY_KEY_VALUES = ["missing", "ok"] as const;

/** Every value the `dinah.workbench` context key can hold. */
export const WORKBENCH_KEY_VALUES = ["none", "ambiguous", "ok", "unknown"] as const;

export type BinaryKey = (typeof BINARY_KEY_VALUES)[number];
export type WorkbenchKey = (typeof WORKBENCH_KEY_VALUES)[number];

/** The context key values the welcome view's `when` clauses read. */
export interface ContextKeys {
	readonly binary: BinaryKey;
	readonly workbench: WorkbenchKey;
}

const HIDDEN: StatusView = { hidden: true, text: "", tooltip: "" };

/**
 * The lines describing which binary this window is driving.
 *
 * describeVersion's own line is left alone. It names the tool's release, its
 * conformance profile and its storage format, which are machine vocabulary the
 * CLI spells the same way under every language setting, so translating this
 * extension's copy of it would show a reader words the CLI never says.
 */
function binaryLines(
	binary: BinaryState,
	pairedRelease: string,
	t: Localizer,
): string[] {
	const lines: string[] = [];
	if (binary.state === "ok") {
		lines.push(describeVersion(binary.version));
		lines.push(
			t("status.binary.withSource", {
				path: binary.path,
				source: binary.source,
			}),
		);
	} else if (binary.state !== "no-binary") {
		lines.push(binary.detail);
		if (binary.path) {
			lines.push(t("status.binary.plain", { path: binary.path }));
		}
	}
	lines.push(t("status.pairedWith", { release: pairedRelease }));
	return lines;
}

/** One card the reader is holding, as the status bar needs it. */
export interface HeldCardView {
	/** The card's human reference. */
	readonly ref: string;
	/** The title of the workbench the card is held in. */
	readonly workbenchTitle: string;
	/**
	 * CardView.expires verbatim, empty when the claim carries no lease at
	 * all, which is what a claim asking for none leaves behind.
	 */
	readonly expiresAt: string;
}

/**
 * What this window knows about the cards the reader is holding.
 *
 * `uncertain` is a second question rather than a rephrasing of an empty
 * `cards`. A reader who holds nothing and a reader whose last read failed are
 * two different people, and telling the second one they hold nothing is the
 * failure this card exists to prevent.
 */
export interface HoldingSummary {
	/** Every held card, across every workbench whose data is fresh. */
	readonly cards: readonly HeldCardView[];
	/**
	 * True when no fresh workbench is left to speak for the reader, so
	 * nothing here can be asserted either way. It stays false whenever
	 * `cards` can be trusted, including when it is legitimately empty.
	 */
	readonly uncertain: boolean;
}

/** A confirmed empty hand, which is what a caller naming no holding means. */
export const NOTHING_HELD: HoldingSummary = { cards: [], uncertain: false };

/** How long the countdown's own tick waits between redraws, in milliseconds. */
export const COUNTDOWN_INTERVAL_MS = 30_000;

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;

/**
 * How long a workbench's last answer is trusted for, given the poll interval.
 *
 * Three intervals rather than a second hardcoded number, and the same floor
 * CheckpointLoop.startTimer applies to its own timer, so lengthening the poll
 * lengthens the patience that goes with it instead of leaving the two to
 * drift apart.
 */
export function staleAfterMs(pollIntervalSeconds: number): number {
	return 3 * Math.max(2, pollIntervalSeconds) * 1000;
}

/**
 * One workbench's last answer about what the reader holds there.
 *
 * This is the shape DinahTreeProvider.holdingSnapshot() returns, spelled
 * structurally here so that this module goes on importing nothing from the
 * tree. The provider's own entry carries more than these three members and
 * satisfies this by having them.
 */
export interface WorkbenchHoldingReport {
	readonly title: string;
	readonly holding: readonly CardView[];
	/** Milliseconds, as of the last status call that answered ok. */
	readonly fetchedAt?: number;
}

/**
 * Turns what every open workbench last said into one summary for the bar.
 *
 * A workbench is fresh when its last ok answer is inside `staleAfter`, and a
 * workbench nobody has heard an ok answer from at all is stale rather than
 * fresh. Only fresh workbenches contribute cards, so a claim that lapsed
 * while a read was failing is never asserted on the strength of an old
 * answer.
 *
 * A stale workbench beside a fresh one that reports a held card does not make
 * the window uncertain. Something confirmed is already on screen, and warning
 * beside it would undermine a card the window does know about.
 */
export function summarizeHolding(
	snapshot: readonly WorkbenchHoldingReport[],
	now: number,
	staleAfter: number,
): HoldingSummary {
	const cards: HeldCardView[] = [];
	let stale = false;
	for (const entry of snapshot) {
		if (entry.fetchedAt === undefined || now - entry.fetchedAt > staleAfter) {
			stale = true;
			continue;
		}
		for (const card of entry.holding) {
			cards.push({
				ref: card.ref ?? card.id,
				workbenchTitle: entry.title,
				expiresAt: card.expires ?? "",
			});
		}
	}
	return { cards, uncertain: stale && cards.length === 0 };
}

/**
 * When a lease lapses, in milliseconds, for sorting.
 *
 * A claim carrying no lease never lapses, so it sorts behind every claim that
 * does. A stamp that will not parse is treated the same way, because the
 * honest answer for it is that we cannot say when it lapses, and sorting it
 * first would let an unreadable stamp displace the real deadline a reader is
 * counting on.
 */
function expiryOf(card: HeldCardView): number {
	if (card.expiresAt === "") {
		return Number.POSITIVE_INFINITY;
	}
	const at = Date.parse(card.expiresAt);
	return Number.isNaN(at) ? Number.POSITIVE_INFINITY : at;
}

/**
 * The clause saying how much of a lease is left.
 *
 * The five renderings are mutually exclusive and cover every case: no lease
 * at all, a lease already lapsed, and the three bands a positive remainder
 * falls into. None of them can produce a negative number, because the lapsed
 * case is answered before any remainder reaches the arithmetic below it.
 */
export function remainingClause(
	card: HeldCardView,
	now: number,
	t: Localizer = ENGLISH,
): string {
	const at = expiryOf(card);
	if (at === Number.POSITIVE_INFINITY) {
		return t("status.holding.remaining.noLease");
	}
	const left = at - now;
	if (left <= 0) {
		return t("status.holding.remaining.expired");
	}
	if (left >= HOUR_MS) {
		return t("status.holding.remaining.hours", {
			hours: Math.floor(left / HOUR_MS),
			minutes: Math.floor((left % HOUR_MS) / MINUTE_MS),
		});
	}
	if (left >= MINUTE_MS) {
		return t("status.holding.remaining.minutes", {
			minutes: Math.floor(left / MINUTE_MS),
		});
	}
	return t("status.holding.remaining.soon");
}

/** The held cards, soonest lapse first, with the leaseless ones behind them. */
function bySoonest(cards: readonly HeldCardView[]): HeldCardView[] {
	return [...cards].sort((left, right) => expiryOf(left) - expiryOf(right));
}

/**
 * What the held cards add to the bar's own text.
 *
 * One card names itself and the time it has left. Several name the one whose
 * lease lapses first and count the rest, because the bar has room for one
 * card and the card a reader most needs is the one whose deadline arrives
 * next. The "+N" is a bare count with a plus sign, which diff tools and git
 * status lines already spell that way in every language, so it is a symbol
 * and is not routed through the catalogue.
 */
function heldSuffix(
	holding: HoldingSummary,
	now: number,
	t: Localizer,
): string {
	const sorted = bySoonest(holding.cards);
	const first = sorted[0];
	if (first === undefined) {
		return "";
	}
	const clause = remainingClause(first, now, t);
	const rest = sorted.length - 1;
	if (rest === 0) {
		return ` ${first.ref} (${clause})`;
	}
	return ` ${first.ref} (${clause}) +${String(rest)}`;
}

/** The lines the held cards put at the head of the hover text. */
function heldLines(
	holding: HoldingSummary,
	now: number,
	t: Localizer,
): string[] {
	const sorted = bySoonest(holding.cards);
	const first = sorted[0];
	if (first === undefined) {
		// Nothing confirmed. A reader whose data went stale is told that it
		// could not be confirmed rather than told they hold nothing, which is
		// a claim this window is in no position to make.
		return holding.uncertain ? [t("status.holding.uncertainLead")] : [];
	}
	if (sorted.length === 1) {
		return [
			t("status.holding.tooltipLead", {
				ref: first.ref,
				workbench: first.workbenchTitle,
				clause: remainingClause(first, now, t),
			}),
		];
	}
	return [
		t("status.holding.tooltipLeadMany", {
			count: sorted.length,
			ref: first.ref,
			workbench: first.workbenchTitle,
			clause: remainingClause(first, now, t),
		}),
		...sorted.slice(1).map((card) =>
			t("status.holding.tooltipRow", {
				ref: card.ref,
				workbench: card.workbenchTitle,
				clause: remainingClause(card, now, t),
			}),
		),
	];
}

/**
 * Composes the status bar item for this window.
 *
 * `resolution` is the workspace folder's resolution the item speaks for.
 * Multi-root windows resolve every folder and this renders the first; showing
 * more than one entry is the tree card's job. The held cards are the one
 * thing this function aggregates over every open workbench, because a claim
 * belongs to the reader rather than to whichever folder happens to be first.
 *
 * `now` is passed in rather than read here, so that the countdown's own tick
 * and every test drive the same arithmetic against a clock they name.
 */
export function composeStatus(
	binary: BinaryState,
	resolution: WorkbenchResolution | undefined,
	pairedRelease: string,
	holding: HoldingSummary = NOTHING_HELD,
	now: number = Date.now(),
	t: Localizer = ENGLISH,
): StatusView {
	const trailer = binaryLines(binary, pairedRelease, t);

	if (binary.state === "no-binary") {
		return {
			hidden: false,
			text: "$(checklist) Dinah $(error)",
			tooltip: [
				t("status.noBinary.notFound"),
				t("status.noBinary.install"),
				...trailer,
			].join("\n"),
		};
	}
	if (binary.state !== "ok") {
		return {
			hidden: false,
			text: "$(checklist) Dinah $(error)",
			tooltip: trailer.join("\n"),
		};
	}

	if (!resolution) {
		return HIDDEN;
	}

	if (resolution.state === "refused") {
		if (
			resolution.refusal === NO_WORKBENCH_FOUND ||
			resolution.refusal === NO_CONFIGURED_WORKBENCH
		) {
			return HIDDEN;
		}
		if (resolution.refusal === AMBIGUOUS_WORKBENCH) {
			const candidates = (resolution.candidates ?? []).map(
				(candidate) => `  ${candidate.path}`,
			);
			return {
				hidden: false,
				text: "$(checklist) Dinah $(warning)",
				tooltip: [
					t("status.ambiguous"),
					...candidates,
					...trailer,
				].join("\n"),
			};
		}
		return {
			hidden: false,
			text: "$(checklist) Dinah $(warning)",
			tooltip: [
				t("status.refused", { refusal: resolution.refusal }),
				...(resolution.detail ? [resolution.detail] : []),
				...trailer,
			].join("\n"),
		};
	}

	const title = resolution.title === "" ? "Dinah" : resolution.title;
	const common = [
		t("status.resolvedBy", { source: resolution.source }),
		...trailer,
	];

	// Held-card content is appended in these two branches alone, because they
	// are the only states where this window is confidently talking to a
	// workbench it resolved.
	const lines = heldLines(holding, now, t);
	// Nothing fresh answered and nothing trustworthy is on screen, so the bar
	// warns rather than showing the idle rendering, which a reader would take
	// to mean they hold nothing. The glyph is appended the way the ambiguous
	// branch above appends it, and the outside-workspace branch below already
	// carries one, so it lands once rather than twice.
	//
	// The flag is read on its own. HoldingSummary defines `uncertain` as
	// false whenever `cards` can be trusted, so re-testing the cards here
	// would restate that definition in a second place and no fixture could
	// ever drive the difference.
	const glyph =
		resolution.insideWorkspace && !holding.uncertain ? "" : " $(warning)";
	const text = `$(checklist) ${title}${heldSuffix(holding, now, t)}${glyph}`;

	if (resolution.insideWorkspace) {
		return {
			hidden: false,
			text,
			tooltip: [...lines, resolution.root, ...common].join("\n"),
		};
	}
	// The dinah-241 visibility rule. The walk climbed past this folder, so the
	// absolute path leads, before the title a reader would otherwise trust.
	return {
		hidden: false,
		text,
		tooltip: [
			...lines,
			t("status.outsideWorkspace", { root: resolution.root }),
			...common,
		].join("\n"),
	};
}

/** The two context keys the welcome view's `when` clauses are driven by. */
export function composeContextKeys(
	binary: BinaryState,
	resolution: WorkbenchResolution | undefined,
): ContextKeys {
	const binaryKey = binary.state === "ok" ? "ok" : "missing";
	if (!resolution) {
		return { binary: binaryKey, workbench: "unknown" };
	}
	if (resolution.state === "ok") {
		return { binary: binaryKey, workbench: "ok" };
	}
	if (resolution.refusal === AMBIGUOUS_WORKBENCH) {
		return { binary: binaryKey, workbench: "ambiguous" };
	}
	if (
		resolution.refusal === NO_WORKBENCH_FOUND ||
		resolution.refusal === NO_CONFIGURED_WORKBENCH
	) {
		return { binary: binaryKey, workbench: "none" };
	}
	return { binary: binaryKey, workbench: "unknown" };
}
