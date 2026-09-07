// The served-text tab: its URI grammar, its Markdown rendering, and the timer
// that keeps an open tab from going quietly stale.
//
// Nothing here imports vscode, for the reason cardCommands.ts's header gives.
// extension.ts composes the real vscode.Uri out of the parts this module
// returns, registers the one content provider for the whole scheme, and binds
// the resolver table the provider dispatches on.
//
// The scheme is general on purpose. A URI's authority carries the kind of text
// being served, the provider looks that kind up in a table, and dinah-270 puts
// exactly one entry in that table. A second document type is a second entry
// rather than a change to the grammar, the provider or this module.

import type { Clock } from "./changes";
import type { Localizer } from "./l10n";
import type { InstructionChain, JournalEvent } from "./wire";

/** The first kind served here, which is a card's own instruction chain. */
export const KIND_INSTRUCTIONS = "instructions";

/**
 * The second kind, which is one topic of the guide dinah itself prints.
 *
 * A guide needs no workbench and no card, so its resolver ignores the root the
 * table's shape hands it. That is what makes this kind showable in the one
 * state where nothing else here can be fetched, which is a window that found
 * no workbench at all (dinah-423).
 */
export const KIND_GUIDE = "guide";

/**
 * The identity key a guide tab is opened under, where another kind carries a
 * workbench directory.
 *
 * `parseServedTextUri` refuses an empty root, and the refresh loop and the
 * open/close bookkeeping key on whatever is here, so a guide tab needs some
 * non-empty word. It is a fixed one because a guide is embedded in the binary
 * and belongs to no directory, and the guide resolver never reads it.
 */
export const GUIDE_ROOT = "embedded";

/**
 * The topic the first-session walkthrough opens.
 *
 * `internal/guide`'s own reading order puts this topic first among the
 * embedded guides, so a walkthrough opening one guide opens the one dinah
 * recommends starting with.
 */
export const GUIDE_TOPIC_FIRST_SESSION = "first-session";

/**
 * The three parts of a served-text URI.
 *
 * Identity and display are kept apart. The authority and the query say what to
 * fetch and are parsed back; the path is the tab's label and is never read.
 * That split is what lets the label be translated while a German reader's URI
 * still resolves the same way a Hindi reader's does.
 */
export interface ServedTextUriParts {
	/** The kind, which is what the provider dispatches on. */
	readonly authority: string;
	/** Display only, never parsed back. */
	readonly path: string;
	/** `root=<enc>&ref=<enc>`, which is the identity half. */
	readonly query: string;
}

/** Composes the parts of the URI that serves `ref` under `kind`. */
export function servedTextUriParts(
	kind: string,
	root: string,
	ref: string,
	title: string,
): ServedTextUriParts {
	return {
		authority: kind,
		path: `/${title}.md`,
		query: `root=${encodeURIComponent(root)}&ref=${encodeURIComponent(ref)}`,
	};
}

/** What a served-text URI names, once its identity half has been read back. */
export interface ParsedServedTextUri {
	readonly kind: string;
	readonly root: string;
	readonly ref: string;
}

/**
 * Reads a served-text URI's identity back, and answers undefined for one this
 * extension did not compose.
 *
 * A URI can reach the provider from the back stack, from a restored window, or
 * from a reader who typed one, so every part is checked rather than assumed.
 * An empty value is as unusable as an absent one and is refused the same way.
 */
export function parseServedTextUri(
	authority: string,
	query: string,
): ParsedServedTextUri | undefined {
	if (authority === "") {
		return undefined;
	}
	const params = new URLSearchParams(query);
	const root = params.get("root");
	const ref = params.get("ref");
	if (root === null || root === "" || ref === null || ref === "") {
		return undefined;
	}
	return { kind: authority, root, ref };
}

/** The heading each instruction layer is rendered under, already translated. */
export interface InstructionLabels {
	readonly global: string;
	readonly standing: string;
	readonly column: string;
}

/**
 * Composes the three layers into one Markdown document.
 *
 * Each layer's own text is passed through byte for byte. The extension reads
 * the machine surface and never re-renders what the binary already rendered,
 * so the only thing composed here is the heading above each layer and the
 * blank line between two of them. A layer that is absent or empty contributes
 * no heading, because a heading over nothing tells a reader the layer is empty
 * when what is true is that the layer was never served.
 */
export function renderInstructionsMarkdown(
	chain: InstructionChain,
	labels: InstructionLabels,
): string {
	const sections: string[] = [];
	if (chain.global !== undefined && chain.global !== "") {
		sections.push(`## ${labels.global}\n\n${chain.global}`);
	}
	if (chain.standing !== undefined && chain.standing !== "") {
		sections.push(`## ${labels.standing}\n\n${chain.standing}`);
	}
	if (chain.column !== undefined && chain.column !== "") {
		sections.push(`## ${labels.column}\n\n${chain.column}`);
	}
	return sections.join("\n\n");
}

/**
 * The third kind, which is a card's own journal read back as prose.
 *
 * dinah-422 D-1. The header above says a second document type is a second
 * entry in the provider's table rather than a change to the grammar, the
 * provider or this module, and this kind is the second card-scoped one to
 * take that route.
 */
export const KIND_HISTORY = "history";

/** What one journal event contributes to its own sentence, already resolved. */
type HistoryParams = Record<string, string>;

/** Reads an optional wire field as text, so an absent one fills as nothing. */
function field(value: string | undefined): string {
	return value ?? "";
}

/**
 * One row builder per event name internal/contract's Events slice declares.
 *
 * The keys are the event names rather than catalogue keys, and the catalogue
 * key is composed from the name at render time, so a row can never be filed
 * under a key naming a different event than the row it renders. A name outside
 * this table falls through to `history.event.unknown`, which is what keeps a
 * newer binary's event from blanking a line this catalogue cannot name.
 *
 * The values a row reads are the raw wire fields. Three of them carry a bare
 * identifier with no display name captured at write time (`archived`'s note,
 * the workstream membership events' workstream, and `tier_override_dropped`'s
 * column), and those print as the identifier and are never resolved against
 * the bench, because resolving them would need a lookup this surface does not
 * otherwise make and would let a line's rendering depend on whether the entity
 * it names still exists (dinah-422 D-4).
 *
 * Two events carry fields no row surfaces. `moved` carries override, reject and
 * reshape, which v1 leaves out because saying which two columns the move was
 * between is what a reader scanning a card's history is after (D-5).
 * `tier_overridden` carries `against` and `reason`, and this card drops both by
 * a decision of its own rather than by oversight: against is the column's own
 * tier default at the moment a relative expression was resolved, which is
 * provenance for the number rather than the number, and reason is populated
 * only by a raise, so an ordinary per-column write would render a template
 * variant with an empty clause in it. Each would want its own variant to read
 * as a sentence, and neither changes what the row already reports happened.
 */
export const HISTORY_ROWS: Readonly<
	Record<string, (event: JournalEvent, t: Localizer) => HistoryParams>
> = {
	created: (event) => ({
		actor: event.actor,
		title: field(event.title),
		toTitle: field(event.to_title),
	}),
	claimed: (event) => ({ actor: event.actor }),
	moved: (event) => ({
		actor: event.actor,
		fromTitle: field(event.from_title),
		toTitle: field(event.to_title),
	}),
	released: (event) => ({ actor: event.actor }),
	blocked: (event) => ({ actor: event.actor, reason: field(event.reason) }),
	unblocked: (event) => ({ actor: event.actor }),
	expired: (event) => ({ actor: event.actor }),
	commented: (event) => ({ actor: event.actor }),
	attached: (event) => ({ actor: event.actor, filename: field(event.filename) }),
	attachment_replaced: (event) => ({
		actor: event.actor,
		filename: field(event.filename),
	}),
	attachment_removed: (event) => ({
		actor: event.actor,
		filename: field(event.filename),
	}),
	attachment_renamed: (event) => ({
		actor: event.actor,
		from: field(event.from),
		filename: field(event.filename),
	}),
	archived: (event) => ({ actor: event.actor }),
	restored: (event) => ({ actor: event.actor }),
	deleted: (event) => ({ actor: event.actor }),
	manual_correction: (event) => ({
		actor: event.actor,
		fromTitle: field(event.from_title),
		toTitle: field(event.to_title),
	}),
	workstream_joined: (event) => ({
		actor: event.actor,
		workstream: field(event.workstream),
	}),
	workstream_left: (event) => ({
		actor: event.actor,
		workstream: field(event.workstream),
	}),
	// An absent from or to is a first write to a field that carried nothing or
	// a write that cleared one, and both are things that happened rather than
	// gaps in the record, so each fills with the catalogue's own word for it
	// instead of leaving an empty span or an unfilled token on screen.
	card_updated: (event, t) => ({
		actor: event.actor,
		field: field(event.field),
		from: event.from ?? t("history.value.none"),
		to: event.to ?? t("history.value.none"),
	}),
	// column_title is captured at write time by a raise alone. The ordinary
	// per-column tier write carries none, and journal.go's own comment on that
	// field says a renderer falls back to Column for every line carrying one
	// without the other.
	tier_overridden: (event) => ({
		actor: event.actor,
		columnTitle: event.column_title ?? field(event.column),
		to: field(event.to),
		expr: field(event.expr),
	}),
	tier_override_dropped: (event) => ({
		actor: event.actor,
		column: field(event.column),
		from: field(event.from),
	}),
};

/**
 * Renders a card's journal as one Markdown line per event.
 *
 * The sentence is composed here rather than passed through, and that is not the
 * departure from the raw-text rule it looks like. `dinah --json log` answers
 * bare struct fields with no English anywhere on the wire; cmd/dinah renders
 * those same events for its own terminal output, and that rendering is never
 * what the machine surface returns. So there is no CLI-composed prose here to
 * relay, and filling a catalogue template from wire fields is what
 * pullCommands.ts's emptyPullMessage and tree.ts's columnDescription already do
 * (dinah-422 D-3).
 *
 * Events render in the order they arrive, which is the order the journal was
 * appended in, and nothing here re-sorts them.
 *
 * Two things this surface cannot say, which a reader should know before drawing
 * a conclusion from a rendered history:
 *
 * A torn trailing journal line is not detectable through this surface today.
 * bench.ReadJournal returns a second value reporting that a crash truncated the
 * final line and that the line was dropped, and internal/bench/check.go:483 is
 * the only caller in the repository that keeps it. internal/verb/read.go's
 * History function, which backs the `log` machine surface this kind's resolver
 * calls, discards it with `_`, so a torn journal reaches this function looking
 * exactly like a complete one. Closing the gap means carrying that flag through
 * log and History the way check already carries it, which is a change to the Go
 * binary and outside an extension-only card's reach. This renderer therefore
 * says nothing about a tail it cannot see rather than inventing a client-side
 * signal for one (dinah-422 D-6, AC-13).
 *
 * `history.empty` fires for a genuinely zero-event read and never means
 * "nothing has happened." A read that failed is refused upstream and renders
 * through servedText.refused instead, and an ordinary card is never empty here,
 * because the one write of a `created` event reaching a card's own journal
 * happens at creation. A card that reads as empty is an anomaly worth looking
 * at rather than a card nothing has been done to.
 */
export function renderHistoryMarkdown(
	events: readonly JournalEvent[],
	t: Localizer,
): string {
	if (events.length === 0) {
		return t("history.empty");
	}
	return events
		.map((event) => {
			const row = HISTORY_ROWS[event.event];
			if (row === undefined) {
				return t("history.event.unknown", {
					actor: event.actor,
					event: event.event,
				});
			}
			return t(`history.event.${event.event}`, row(event, t));
		})
		.join("\n");
}

/** What the refresh loop needs in order to run. */
export interface ServedTextRefreshDeps {
	/** The same Clock the checkpoint loop takes, injected for the same reason. */
	readonly clock: Clock;
	/** How often an open tab is re-fetched, in seconds. */
	readonly pollIntervalSeconds: number;
	/** Fetches the text one open tab shows, by the kind it was opened under. */
	readonly resolve: (kind: string, root: string, ref: string) => Promise<string>;
	/** Announces that a tab's text has moved, so the provider can be re-asked. */
	readonly onChanged: (uriKey: string, text: string) => void;
	readonly log: (line: string) => void;
}

/**
 * The timer that keeps an open served-text tab current.
 *
 * A virtual document is a snapshot, and the two signals the tree already runs
 * both miss the cases that matter here. `dinah changes` answers on the
 * workbench anchor, the workstream journals and the card journals, and never on
 * a column's own instructions file, so a column-only edit never reports as a
 * change at all. The workspace file watcher would see that file but cannot see
 * the user-global layer, which lives outside the workspace by construction. So
 * this loop asks the question itself: it re-fetches on its own timer and
 * compares the text it got against the text the tab is showing.
 *
 * It runs only while a tab is open. An empty map means no reason to spawn
 * anything, so the timer is started by the first open and stopped by the last
 * close.
 */
export class ServedTextRefreshLoop {
	/** Open tabs, keyed by `uri.toString()`. */
	private readonly open = new Map<
		string,
		{ kind: string; root: string; ref: string; lastText: string }
	>();
	private timer?: unknown;

	constructor(private readonly deps: ServedTextRefreshDeps) {}

	/** Records a tab that has just opened, and starts the timer if it was idle. */
	noteOpened(
		uriKey: string,
		kind: string,
		root: string,
		ref: string,
		initialText: string,
	): void {
		this.open.set(uriKey, { kind, root, ref, lastText: initialText });
		this.startTimer();
	}

	/** Forgets a tab that has closed, and stops the timer when it was the last. */
	noteClosed(uriKey: string): void {
		this.open.delete(uriKey);
		if (this.open.size === 0) {
			this.stopTimer();
		}
	}

	/**
	 * Records text the content provider fetched itself.
	 *
	 * Without this the loop's first tick would compare against whatever the
	 * document held when it opened and announce a change nobody made.
	 */
	recordFetched(uriKey: string, text: string): void {
		const entry = this.open.get(uriKey);
		if (entry !== undefined) {
			entry.lastText = text;
		}
	}

	/** Stops the timer and forgets every tab, which deactivation calls. */
	stop(): void {
		this.stopTimer();
		this.open.clear();
	}

	/** How many tabs the loop is watching, which is what the unit layer reads. */
	get openCount(): number {
		return this.open.size;
	}

	private startTimer(): void {
		if (this.timer !== undefined) {
			return;
		}
		const ms = Math.max(2, this.deps.pollIntervalSeconds) * 1000;
		this.timer = this.deps.clock.setInterval(() => {
			void this.tick();
		}, ms);
	}

	private stopTimer(): void {
		if (this.timer !== undefined) {
			this.deps.clock.clearInterval(this.timer);
			this.timer = undefined;
		}
	}

	/**
	 * One pass over every open tab.
	 *
	 * A fetch that fails is logged and the tab goes on showing its last good
	 * text. Blanking a tab that was showing something a moment ago would turn a
	 * transient refusal into a loss of what the reader was reading, and the
	 * cached text is left alone so the next successful tick still compares
	 * against what is on screen.
	 */
	private async tick(): Promise<void> {
		for (const [uriKey, entry] of this.open) {
			try {
				const text = await this.deps.resolve(entry.kind, entry.root, entry.ref);
				if (text !== entry.lastText) {
					entry.lastText = text;
					this.deps.onChanged(uriKey, text);
				}
			} catch (err) {
				this.deps.log(
					`servedText refresh of ${uriKey}: ${err instanceof Error ? err.message : String(err)}`,
				);
			}
		}
	}
}
