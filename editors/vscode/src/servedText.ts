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
import type { InstructionChain } from "./wire";

/** The one kind dinah-270 serves: a card's own instruction chain. */
export const KIND_INSTRUCTIONS = "instructions";

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
