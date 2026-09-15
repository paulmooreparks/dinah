// What every contributed command does with a selection of more than one row,
// and how the rows a command was aimed at are resolved.
//
// The operator ruled on 2026-09-13 that multi-select applies to the other
// commands too, acting on all selected rows where that makes sense, so every
// command owes a decision rather than only the ones this card was filed for.
// SELECTION_POLICIES is that decision, one entry per contributed command, held
// against TREE_COMMANDS in both directions by test/unit/selection.test.ts.
//
// Nothing here imports vscode.

import {
	COMMAND_ARCHIVE_CARD,
	COMMAND_ATTACH_FILE,
	COMMAND_BLOCK,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_CLAIM,
	COMMAND_COMMENT_ON_ITEM,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_DELETE_ATTACHMENT,
	COMMAND_DISCARD_DRAFT,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_FAIL_ITEM,
	COMMAND_FILE_ITEM,
	COMMAND_MOVE,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_CARD,
	COMMAND_OPEN_COMMENT,
	COMMAND_OPEN_FIRST_SESSION_GUIDE,
	COMMAND_OPEN_HISTORY,
	COMMAND_OPEN_INSTRUCTIONS,
	COMMAND_OPEN_ITEM,
	COMMAND_POST_COMMENT,
	COMMAND_PULL,
	COMMAND_REFRESH,
	COMMAND_REFRESH_VERB_CATALOG,
	COMMAND_RELEASE,
	COMMAND_REOPEN_ITEM,
	COMMAND_RESOLVE_ITEM,
	COMMAND_RUN_VERB,
	COMMAND_UNBLOCK,
	COMMAND_VERIFY_ITEM,
} from "./identity";

/** How one command treats a selection of more than one row. */
export type SelectionPolicy = "fanOut" | "oneInput" | "rowOnly" | "noRow";

/** What a fanOut command's effect looks like over several rows. */
export type FanOutEffect = "perRow" | "oneCall";

/** One command's declared treatment of a selection. */
export interface SelectionEntry {
	readonly policy: SelectionPolicy;
	/** Declared by every fanOut entry and by no other. */
	readonly effect?: FanOutEffect;
}

/**
 * What every contributed command declares, keyed by command id.
 *
 * The four meanings:
 *
 * - fanOut: the command acts on every selected row of the kind it handles.
 * - oneInput: the command asks the reader once, before it spawns anything, and
 *   applies that one answer to every selected row of the kind it handles.
 * - rowOnly: the command acts on one row, and a selection of more than one
 *   actionable row is refused with a message rather than narrowed in silence.
 * - noRow: the command reads no row, so a selection cannot reach it.
 *
 * The effect field is declared here rather than in a test's own table, because
 * two of the fanOut commands produce one call rather than one call per row by
 * design and the expectation has to live in the same record the policy does.
 * A second list would be a list for the first to drift from.
 *
 * What this table is and is not. It is a wiring contract: a command declaring
 * fanOut is a command the registration loop hands every targeted row, and
 * test/unit/selection.test.ts drives every fanOut entry and asserts the effect
 * its own entry names. It is not a behavioural contract for the particulars of
 * each command, and four commands carry criteria of their own for those:
 * Move, Block, New Card and Archive.
 */
export const SELECTION_POLICIES: Readonly<Record<string, SelectionEntry>> = {
	[COMMAND_REFRESH]: { policy: "noRow" },
	[COMMAND_OPEN_CARD]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_CLAIM]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_MOVE]: { policy: "oneInput" },
	[COMMAND_RELEASE]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_BLOCK]: { policy: "oneInput" },
	[COMMAND_UNBLOCK]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_COPY_CARD_REF]: { policy: "fanOut", effect: "oneCall" },
	[COMMAND_CHECK_WORKBENCH]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_COPY_WORKBENCH_PATH]: { policy: "fanOut", effect: "oneCall" },
	[COMMAND_EDIT_WORKBENCH_DEFINITION]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_EDIT_COLUMN_INSTRUCTIONS]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_OPEN_ATTACHMENT]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_DELETE_ATTACHMENT]: { policy: "oneInput" },
	[COMMAND_NEW_CARD]: { policy: "rowOnly" },
	[COMMAND_ATTACH_FILE]: { policy: "oneInput" },
	[COMMAND_PULL]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_OPEN_INSTRUCTIONS]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_OPEN_HISTORY]: { policy: "fanOut", effect: "perRow" },
	[COMMAND_ARCHIVE_CARD]: { policy: "oneInput" },
	[COMMAND_OPEN_FIRST_SESSION_GUIDE]: { policy: "noRow" },
	[COMMAND_RUN_VERB]: { policy: "noRow" },
	[COMMAND_REFRESH_VERB_CATALOG]: { policy: "noRow" },
	// Open is fanOut for the reason Open Attachment is: three selected items
	// open three tabs and no answer is shared between them.
	[COMMAND_OPEN_ITEM]: { policy: "fanOut", effect: "perRow" },
	// Open Comment is the same shape as Open Item: three selected comments
	// open three files and no answer is shared between them.
	[COMMAND_OPEN_COMMENT]: { policy: "fanOut", effect: "perRow" },
	// Comment asks nothing and spawns nothing. It writes a draft and opens
	// it, and the post happens later from an editor with the tree selection
	// long gone, so oneInput's definition does not describe it. One draft
	// naming three items would also put a partial failure inside the one act
	// the draft design exists to make safe: a post that succeeded on the
	// first and was refused on the second cannot be retried without
	// commenting twice on the first.
	[COMMAND_COMMENT_ON_ITEM]: { policy: "rowOnly" },
	// The three terminal verbs are rowOnly because one note applied to five
	// different questions is a false record, and the tool would accept it
	// without complaint.
	[COMMAND_RESOLVE_ITEM]: { policy: "rowOnly" },
	[COMMAND_VERIFY_ITEM]: { policy: "rowOnly" },
	[COMMAND_FAIL_ITEM]: { policy: "rowOnly" },
	// Reopen asks once for a reason and applies it to every selected row,
	// which is what oneInput means: one reason for returning several items to
	// pending is a true record rather than a flattened one.
	[COMMAND_REOPEN_ITEM]: { policy: "oneInput" },
	// Filing one item across several cards would multiply the column mistake
	// the form exists to prevent.
	[COMMAND_FILE_ITEM]: { policy: "rowOnly" },
	// The two draft commands read the active editor rather than any row.
	[COMMAND_POST_COMMENT]: { policy: "noRow" },
	[COMMAND_DISCARD_DRAFT]: { policy: "noRow" },
};

/**
 * The rows a command was aimed at, given what the editor handed the handler.
 *
 * TreeViewOptions.canSelectMany documents that "the first argument to the
 * command is the tree item that the command was executed on and the second
 * argument is an array containing all selected tree items". It does not
 * promise that the executed-on item appears in that array, so the third case
 * below includes it rather than trusting that it is already there. Dropping
 * the row the reader aimed at is the shape the operator ruled against, in its
 * worst form: a reader right-clicks one card and Dinah acts on five others.
 *
 * Deduplication is by a key the caller supplies rather than by object
 * identity, because nothing documents that the editor hands back the same
 * object in both arguments. The row type stays generic so that the drop path
 * and a later caller can supply their own key.
 */
export function targetsFor<T>(
	element: T | undefined,
	selection: readonly T[] | undefined,
	keyOf: (item: T) => string,
): readonly T[] {
	if (selection === undefined || selection.length === 0) {
		return element === undefined ? [] : [element];
	}
	const deduplicated: T[] = [];
	const seen = new Set<string>();
	for (const item of selection) {
		const key = keyOf(item);
		if (seen.has(key)) {
			continue;
		}
		seen.add(key);
		deduplicated.push(item);
	}
	if (element === undefined || seen.has(keyOf(element))) {
		return deduplicated;
	}
	return [element, ...deduplicated];
}
