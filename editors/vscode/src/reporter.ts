// What every host carries, so that one wrapper can collect any host's
// reporting.
//
// Three host interfaces stand between a command and the reader: CommandHost in
// cardCommands.ts, WorkbenchCommandHost in workbenchCommands.ts and
// ColumnCommandHost in columnCommands.ts. Before dinah-490 each declared its
// own subset of the reporting members, so a wrapper built around one of them
// left the other two outside the perimeter, and a run over several workbench
// rows produced one toast per row while a run over several cards produced one
// message. Declaring the members once, in an interface every host extends,
// makes a command that can speak a command whose messages a run can collect,
// and a fourth host would have to extend this to be usable at all.
//
// Nothing here imports vscode. The two channel sets are read by the guards in
// test/unit/perimeter.test.ts at test time rather than copied into them, so
// the sets and the code that honours them cannot drift.

import type { Localizer } from "./l10n";

/**
 * Every member through which a command puts words in front of the reader.
 *
 * A multi-row run intercepts all three and answers with one message instead,
 * so a member added here is a member bulk.ts's collectingHost has to
 * intercept and a member the perimeter guards expect to find bound in
 * extension.ts.
 */
export const REPORT_CHANNELS = ["showError", "showInfo", "showWarning"] as const;

/**
 * Every member through which a command asks the reader a question.
 *
 * These reach the real host under a multi-row run, because the prompts are the
 * reader's own question and must arrive. A collected confirmDestructive would
 * silently accept a destructive act nobody confirmed.
 */
export const PROMPT_CHANNELS = ["pick", "input", "confirmDestructive"] as const;

/** What every host carries, so that one wrapper can collect any host's reporting. */
export interface ReporterHost {
	/**
	 * Renders one message in the language the editor is displaying.
	 *
	 * Injected alongside the window calls rather than imported, for the reason
	 * l10n.ts's own header gives: no module here imports a vscode symbol, so
	 * none can reach vscode.l10n, and extension.ts is the one place that reads
	 * the editor's display language and binds a Localizer to it.
	 */
	readonly t: Localizer;
	readonly showError: (message: string) => void;
	/** Reports an act that succeeded and shows nothing else, such as a copy. */
	readonly showInfo: (message: string) => void;
	readonly showWarning: (
		message: string,
		actions: readonly string[],
	) => Promise<string | undefined>;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly revealOutput: () => void;
}
