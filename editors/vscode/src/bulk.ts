// One run over several rows, and the one message it produces.
//
// The loop lives here rather than in each command, so that a report's counts
// cannot disagree with the list they describe: `selected` is the length of the
// row list this module was handed, one entry is appended per row before the
// loop advances, and no command accumulates a count of its own.
//
// One entry point owns a run from the prompt to the message. runBulk resolves
// the rows, asks the command's one question, chooses the host, runs the loop,
// performs a oneCall command's single act, checkpoints, writes the channel
// lines and shows the one message. A command that wants to say anything to the
// reader about a run either hands its rows to runBulk or refuses before the
// run starts. The perimeter is a property of this module rather than a rule
// each caller has to remember, which is what four rounds of review found no
// other shape of it could be: patching the routes somebody had noticed left
// the next one to be found.
//
// Nothing here imports vscode.
//
// The report mark and summaryFor's reconciliation arm both have one route
// past them, and it is recorded here rather than closed. A caller holding any
// real report can read the mark off it with Object.getOwnPropertySymbols and
// mint a fresh object carrying that key, with counts and entries that add up
// to whatever it likes. The mark arm accepts that object because it carries
// the mark, and the reconciliation arm accepts it because it reconciles. The
// route stays open because what this module defends against is a future
// author taking a shortcut rather than an adversary, and closing it would
// cost a private field and a class and buy nothing the shortcut case already
// lacks.

import type { Localizer } from "./l10n";
import type { ReporterHost } from "./reporter";

/**
 * The key every report this module builds carries, and which nothing outside
 * it can obtain.
 *
 * Deliberately not exported. A hand-built object literal cannot name it, so
 * summaryFor's first arm refuses a report the bulk layer did not build, and
 * the reconciliation arm below stops being the only thing between a future
 * caller and a fabricated summary.
 */
const reportMark: unique symbol = Symbol("dinah.bulk.report");

/** What one row's act came to. */
export type RowOutcome =
	| { readonly kind: "done" }
	| { readonly kind: "failed"; readonly failure: string }
	| { readonly kind: "skipped"; readonly why: string };

/** One row of the run, and what became of it. */
export interface BulkEntry {
	readonly ref: string;
	readonly outcome: RowOutcome;
}

/** What a run in which nothing was acted on says to the reader. */
export type EmptyRunVoice = "speak" | "silent";

/** A collected message, and the level it would have been shown at. */
export interface CollectedNote {
	readonly level: "info" | "warning";
	readonly text: string;
}

/** What one run did, whole. Only this module can mint one. */
export interface BulkReport {
	readonly [reportMark]: true;
	/** The rows targetsFor handed the command; equals entries.length by construction. */
	readonly selected: number;
	readonly entries: readonly BulkEntry[];
	/** True when no act ran, because a prompt was declined or the command refused. */
	readonly cancelled: boolean;
	readonly emptyRun: EmptyRunVoice;
	/** What the collecting host captured, empty on a single-row run. */
	readonly notes: readonly CollectedNote[];
	/** A oneCall command's own success sentence, composed by runBulk. */
	readonly doneMessage?: string;
}

/** Mints one report, which is the only way a report comes into existence. */
function mintReport(
	selected: number,
	entries: readonly BulkEntry[],
	cancelled: boolean,
	emptyRun: EmptyRunVoice,
	notes: readonly CollectedNote[],
	doneMessage: string | undefined,
): BulkReport {
	return {
		[reportMark]: true,
		selected,
		entries,
		cancelled,
		emptyRun,
		notes,
		doneMessage,
	};
}

/**
 * Runs one act over every row and records exactly one entry per row.
 *
 * The report is built here rather than by the caller so that its counts cannot
 * disagree with the list they describe: `selected` is `rows.length`, one entry
 * is appended per row before the loop advances, and a row whose act throws is
 * recorded as failed rather than ending the run.
 */
export async function runOverRows<T>(
	rows: readonly T[],
	refOf: (row: T) => string,
	act: (row: T) => Promise<RowOutcome>,
	emptyRun?: EmptyRunVoice,
): Promise<BulkReport> {
	const entries: BulkEntry[] = [];
	for (const row of rows) {
		const ref = refOf(row);
		// A rejecting act is caught, recorded as failed, and the loop advances.
		// This is what puts an unanticipated failure into the failed count
		// instead of outside all three, and the paths that need it are live:
		// openDocument, copyToClipboard, openFile and openServedText all reach
		// the editor and all can reject, so a fan-out over Open Card or Copy
		// Reference would otherwise lose a whole run at row three.
		try {
			entries.push({ ref, outcome: await act(row) });
		} catch (err) {
			entries.push({
				ref,
				// The spelling extension.ts and servedText.ts already use, so a
				// rejection carrying a string or undefined records something a
				// reader can read rather than the word undefined.
				outcome: {
					kind: "failed",
					failure: err instanceof Error ? err.message : String(err),
				},
			});
		}
	}
	return mintReport(rows.length, entries, false, emptyRun ?? "speak", [], undefined);
}

/**
 * A host that captures what a per-row call would have shown, so a run of
 * several rows produces one message rather than one message per row.
 *
 * Generic over the host, because three host interfaces extend ReporterHost and
 * a run collects whichever one its command was given. Every member named in
 * REPORT_CHANNELS is intercepted, `checkpoint` is intercepted where the host
 * carries one, and every other member, the PROMPT_CHANNELS among them, is the
 * real host's: the prompts are the reader's own question and must arrive,
 * and a collected confirmDestructive would accept a destructive act nobody
 * confirmed.
 *
 * The interception is written member by member rather than looked up by
 * string at run time. REPORT_CHANNELS is what the perimeter guards hold this
 * hand-written set against.
 *
 * The intercepted showWarning answers undefined, which is what the editor
 * answers when the reader dismisses a toast without choosing an action, so a
 * caller that reveals the channel on assent reveals nothing and a run of five
 * rows cannot reveal the output channel five times.
 */
export function collectingHost<H extends ReporterHost>(
	host: H,
): {
	readonly host: H;
	readonly drain: () => {
		readonly errors: readonly string[];
		readonly notes: readonly CollectedNote[];
		readonly folders: readonly string[];
	};
} {
	const errors: string[] = [];
	const notes: CollectedNote[] = [];
	const folders: string[] = [];
	const wrapped: H = {
		...host,
		showError: (message: string) => {
			errors.push(message);
		},
		showInfo: (message: string) => {
			notes.push({ level: "info", text: message });
		},
		showWarning: async (message: string) => {
			notes.push({ level: "warning", text: message });
			return undefined;
		},
	};
	// CommandHost carries a checkpoint and the other two do not, so the member
	// is read off the value rather than declared on ReporterHost: a host that
	// carries none needs no interception, and one that does needs its folders
	// recorded so runBulk can checkpoint once per distinct folder.
	const checkpointing = host as H & {
		readonly checkpoint?: (folder: string) => Promise<void>;
	};
	if (typeof checkpointing.checkpoint === "function") {
		Object.assign(wrapped, {
			checkpoint: async (folder: string): Promise<void> => {
				folders.push(folder);
			},
		});
	}
	return {
		host: wrapped,
		drain: () => ({ errors, notes, folders }),
	};
}

/** The single message a run produces, or none. */
export function summaryFor(
	report: BulkReport,
	t: Localizer,
): { readonly level: "info" | "warning" | "none"; readonly message: string } {
	// Case 1. A report the bulk layer did not build must not be able to
	// produce a message, whatever it says about itself.
	if (report[reportMark] !== true) {
		throw new Error(
			"summaryFor was handed a report that was not built by the bulk layer",
		);
	}
	let done = 0;
	let failed = 0;
	let skipped = 0;
	for (const entry of report.entries) {
		if (entry.outcome.kind === "done") {
			done += 1;
		} else if (entry.outcome.kind === "failed") {
			failed += 1;
		} else {
			skipped += 1;
		}
	}
	// Case 2. A report that cannot add up must not be able to produce a
	// cheerful one. This is the arm that makes "the report cannot lie" a
	// checkable claim rather than an asserted one.
	if (done + failed + skipped !== report.selected) {
		throw new Error(
			`summaryFor was handed a report selecting ${String(report.selected)} rows and holding ${String(report.entries.length)} entries`,
		);
	}
	// Case 3. Nothing ran, and the command's own ask has already said whatever
	// the reader needed to hear.
	if (report.cancelled) {
		return { level: "none", message: "" };
	}
	// Case 4. Only the drop path declares silence: a drag that missed has to
	// go on looking like a drag that missed.
	if (report.emptyRun === "silent" && done === 0 && failed === 0) {
		return { level: "none", message: "" };
	}
	// Case 5. A oneCall command's own sentence is the whole of what a
	// successful copy shows at any row count, including one, which is why this
	// case stands ahead of the single-row one.
	if (failed === 0 && skipped === 0 && report.doneMessage !== undefined) {
		return { level: "info", message: report.doneMessage };
	}
	// Case 6. The single-row path, which keeps today's behaviour exactly: the
	// per-row call is the whole of what a refusal shows, because a single-row
	// run is given the real host rather than the collecting one. A single
	// targeted row that yielded no context is this case too, so it writes its
	// channel line and shows nothing, which is what shipped before this card.
	if (report.selected <= 1) {
		return { level: "none", message: "" };
	}
	// Case 7. Filled from the number of rows that actually finished rather
	// than from selected, which is the defect this case exists to close.
	if (failed === 0 && skipped === 0 && report.notes.length === 0) {
		return {
			level: "info",
			message: t("dialog.bulk.allSucceeded", { count: String(done) }),
		};
	}
	// Case 8. The notes are in the channel, and the level follows what they
	// were: three workbench checks that found defects escalate exactly as one
	// check does today, and four empty pulls do not.
	if (failed === 0 && skipped === 0) {
		return {
			level: report.notes.some((note) => note.level === "warning")
				? "warning"
				: "info",
			message: t("dialog.bulk.allSucceededNotes", {
				count: String(done),
				notes: String(report.notes.length),
			}),
		};
	}
	// Case 9. A partial run needs no notes clause, because its own sentence
	// already points at the channel the notes were written to. A menu
	// invocation that attempted nothing reaches here rather than staying
	// silent, which is the "reports success because it never looked" failure
	// this case exists to make impossible.
	return {
		level: "warning",
		message: t("dialog.bulk.partial", {
			succeeded: String(done),
			selected: String(report.selected),
			refused: String(failed),
			skipped: String(skipped),
		}),
	};
}

/** What one command declares about its own run. */
export interface BulkDeps<R, H extends ReporterHost> {
	readonly host: H;
	readonly t: Localizer;
	/** The channel line for a row the command cannot act on. */
	readonly skipReason: string;
	/** Archive alone sets this; every row it attempted reaches the channel. */
	readonly lineEveryRow?: boolean;
	/**
	 * What a finished row's channel line says when lineEveryRow is set.
	 *
	 * Archive alone sets the pair. The word belongs to the command rather than
	 * to this module, and the spec's own declaration of BulkDeps carried only
	 * the boolean while section 4 fixed the line as `<ref>: archived`, so this
	 * field is what keeps one command's vocabulary out of the shared loop.
	 */
	readonly doneLine?: string;
	/** The drop path alone sets "silent". */
	readonly emptyRun?: EmptyRunVoice;
	/**
	 * The one act a oneCall command performs after the loop, over the rows
	 * that finished. It shows nothing.
	 */
	readonly finish?: (finished: readonly R[], host: H) => Promise<void>;
	/** The sentence a wholly successful run shows instead of the bulk one. */
	readonly successMessage?: (finished: readonly R[], t: Localizer) => string;
}

/**
 * Runs one command over every row the reader aimed at, and shows one message.
 *
 * `rows` is what targetsFor answered, unfiltered. `resolve` maps a row to the
 * context the command acts on, or to undefined for a row of the wrong kind,
 * and this function records that row as skipped. Filtering a targeted row out
 * of a run happens in no command and in no loop: resolve here is the one place
 * the extension reads whether a row yielded a context, and it records the row
 * either way. So the number of rows a run reports is the number of rows the
 * reader aimed at, and the host switch below reads that same number.
 *
 * `ask` runs once, before anything spawns, on the real host, which is what
 * lets a command state a refusal the reader actually sees: handing ask the
 * collecting host makes a refused run say nothing at all.
 */
export async function runBulk<T, R, A, H extends ReporterHost>(
	rows: readonly T[],
	refOf: (row: T) => string,
	resolve: (row: T) => R | undefined,
	deps: BulkDeps<R, H>,
	ask: (resolved: readonly R[], host: H) => Promise<A | undefined>,
	act: (resolved: R, answer: A, host: H) => Promise<RowOutcome>,
): Promise<BulkReport> {
	const pairs = rows.map((row) => ({ row, context: resolve(row) }));
	const resolved: R[] = [];
	for (const pair of pairs) {
		if (pair.context !== undefined) {
			resolved.push(pair.context);
		}
	}
	const answer = await ask(resolved, deps.host);
	if (answer === undefined) {
		// The cancelled report is built here rather than by the caller, so it
		// reconciles like any other and summaryFor never meets a report whose
		// selected count exceeds its entry count.
		const cancelled = mintReport(
			rows.length,
			pairs.map((pair) => ({
				ref: refOf(pair.row),
				outcome: { kind: "skipped", why: deps.skipReason } as const,
			})),
			true,
			deps.emptyRun ?? "speak",
			[],
			undefined,
		);
		await showSummary(cancelled, deps);
		return cancelled;
	}
	// The host switch lives here and nowhere else, and it reads the targeted
	// count, which is the same number summaryFor's single-row case reads. The
	// per-row toast and the summary are therefore mutually exclusive by
	// arithmetic rather than by two call sites agreeing.
	const collecting = rows.length > 1 ? collectingHost(deps.host) : undefined;
	const runningHost = collecting === undefined ? deps.host : collecting.host;
	const finished: R[] = [];
	const ran = await runOverRows(
		pairs,
		(pair) => refOf(pair.row),
		async (pair): Promise<RowOutcome> => {
			if (pair.context === undefined) {
				return { kind: "skipped", why: deps.skipReason };
			}
			const outcome = await act(pair.context, answer, runningHost);
			if (outcome.kind === "done") {
				finished.push(pair.context);
			}
			return outcome;
		},
		deps.emptyRun,
	);
	if (deps.finish !== undefined) {
		await deps.finish(finished, deps.host);
	}
	const drained = collecting?.drain() ?? { errors: [], notes: [], folders: [] };
	// One checkpoint per distinct folder, whatever the outcome. A run of a
	// dozen cards in one folder checkpoints once rather than a dozen times. A
	// single-row run took the real host, so its checkpoint already happened as
	// it does today.
	const checkpointing = deps.host as H & {
		readonly checkpoint?: (folder: string) => Promise<void>;
	};
	if (typeof checkpointing.checkpoint === "function") {
		for (const folder of new Set(drained.folders)) {
			await checkpointing.checkpoint(folder);
		}
	}
	const report = mintReport(
		ran.selected,
		ran.entries,
		false,
		deps.emptyRun ?? "speak",
		drained.notes,
		deps.successMessage?.(finished, deps.t),
	);
	// Every row that did not finish reaches the channel, because the skipped
	// rows are what a reader needs when a gesture did less than they expected.
	// A collected showError is discarded: the row that produced it also
	// recorded the same text as its failure, which reaches the channel as that
	// row's own line.
	const lines: string[] = [];
	for (const entry of report.entries) {
		if (entry.outcome.kind === "failed") {
			lines.push(`${entry.ref}: ${entry.outcome.failure}`);
		} else if (entry.outcome.kind === "skipped") {
			lines.push(`${entry.ref}: ${entry.outcome.why}`);
		} else if (deps.lineEveryRow === true && deps.doneLine !== undefined) {
			lines.push(`${entry.ref}: ${deps.doneLine}`);
		}
	}
	for (const note of report.notes) {
		lines.push(note.text);
	}
	// A gesture that missed goes on looking like a gesture that missed, in the
	// channel as well as on screen. The drop path is the one caller declaring
	// silence, and a drag that acted on nothing writes nothing at all rather
	// than leaving a reader lines about a drag they did not mean to make. Any
	// run that acted on something lines its rows as every other run does.
	const missed =
		deps.emptyRun === "silent" &&
		report.entries.every((entry) => entry.outcome.kind === "skipped");
	if (lines.length > 0 && !missed) {
		deps.host.appendLines(lines);
	}
	await showSummary(report, deps);
	return report;
}

/** Shows the one message a run produces, through the real host. */
async function showSummary<R, H extends ReporterHost>(
	report: BulkReport,
	deps: BulkDeps<R, H>,
): Promise<void> {
	const summary = summaryFor(report, deps.t);
	if (summary.level === "info") {
		deps.host.showInfo(summary.message);
		return;
	}
	if (summary.level === "warning") {
		const picked = await deps.host.showWarning(summary.message, [
			deps.t("dialog.openOutput.label"),
		]);
		if (picked !== undefined) {
			deps.host.revealOutput();
		}
	}
}
