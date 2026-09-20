// The mapping from a contributed command id to the function that serves it.
//
// This lived inside activate() until dinah-490, three ways at once: two array
// literals iterated by two loops, six standalone register calls whose handler
// was an inline arrow, and one array entry whose function was an inline
// wrapper. A unit test could reach none of them. test/unit/layers.test.ts
// forbids the unit layer from importing vscode, extension.ts is the one module
// in src/ that imports it as a value, and a unit file importing extension.ts
// would load vscode outside an extension host and fail at run time rather than
// at review.
//
// So the mapping is a value in a module importing no vscode symbol, and
// activate() iterates it. A test can then import the very table the editor
// registers from, look a command id up in it, and drive that entry's own
// invoke, rather than driving a function a test table claims corresponds to it
// (D-21). Extracting two handlers would have bought importability without
// buying the mapping, which is the distinction round 3 of that card missed.
//
// Every module named below already imports no vscode symbol, so the pure
// module this needs to be costs nothing.

import type { BulkReport } from "./bulk";
import type { CliOutcome, Spawner } from "./cli";
import type { CommandHost } from "./cardCommands";
import {
	invokeArchiveCard,
	invokeBlock,
	invokeClaim,
	invokeCopyCardRef,
	invokeDeleteAttachment,
	invokeMove,
	invokeOpenAttachment,
	invokeOpenCard,
	invokeOpenHistory,
	invokeOpenInstructions,
	invokeRelease,
	invokeUnblock,
} from "./cardCommands";
import type { ColumnCommandHost } from "./columnCommands";
import {
	invokeCommentOnColumn,
	invokeEditColumnInstructions,
} from "./columnCommands";
import { invokeOpenComment } from "./commentCommands";
import type { CommentBodyHost, OpenComments } from "./commentBody";
import { invokeAttachFile, invokeNewCard } from "./creationCommands";
import {
	invokeAddCriterion,
	invokeCommentOnItem,
	invokeFailItem,
	invokeOpenItem,
	invokeRaiseQuestion,
	invokeRecordDecision,
	invokeReopenItem,
	invokeResolveItem,
	invokeVerifyItem,
} from "./itemCommands";
import type { CatalogBuild } from "./verbCatalog";
import {
	COMMAND_ARCHIVE_CARD,
	COMMAND_ATTACH_FILE,
	COMMAND_BLOCK,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_CLAIM,
	COMMAND_COMMENT_ON_COLUMN,
	COMMAND_COMMENT_ON_ITEM,
	COMMAND_COPY_CARD_REF,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_DELETE_ATTACHMENT,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_FAIL_ITEM,
	COMMAND_ADD_CRITERION,
	COMMAND_RAISE_QUESTION,
	COMMAND_RECORD_DECISION,
	COMMAND_MOVE,
	COMMAND_NEW_CARD,
	COMMAND_OPEN_ATTACHMENT,
	COMMAND_OPEN_CARD,
	COMMAND_OPEN_COMMENT,
	COMMAND_OPEN_HISTORY,
	COMMAND_OPEN_INSTRUCTIONS,
	COMMAND_OPEN_ITEM,
	COMMAND_PULL,
	COMMAND_RELEASE,
	COMMAND_REOPEN_ITEM,
	COMMAND_RESOLVE_ITEM,
	COMMAND_UNBLOCK,
	COMMAND_VERIFY_ITEM,
} from "./identity";
import type { Localizer } from "./l10n";
import { invokePull } from "./pullCommands";
import type { TreeElement } from "./tree";
import type { WorkbenchCommandHost } from "./workbenchCommands";
import {
	invokeCheckWorkbench,
	invokeCopyWorkbenchPath,
	invokeEditWorkbenchDefinition,
} from "./workbenchCommands";

/**
 * What activate() holds and a row command may need some of.
 *
 * This is what lets the entries below be values in a pure module rather than
 * closures over activate(). Two commands needed a closure before: Check
 * Workbench reads a diagnostics object no context carries, and that arrives
 * here as applyCheckResult, declared structurally so this module names no type
 * from a module owning a DiagnosticCollection.
 */
export interface Wiring {
	readonly exe: string;
	/** How the resolved binary described itself, which a workbench message names. */
	readonly binaryLabel: string;
	readonly spawner: Spawner;
	readonly t: Localizer;
	readonly cardHost: CommandHost;
	readonly workbenchHost: WorkbenchCommandHost;
	readonly columnHost: ColumnCommandHost;
	readonly applyCheckResult: (
		path: string,
		label: string,
		outcome: CliOutcome,
	) => Promise<void>;
	/**
	 * What the Comment command writes a draft through, bound to the editor's
	 * global storage and to vscode.workspace.fs.
	 */
	readonly commentHost: CommentBodyHost;
	/**
	 * The comment files this window has opened, so a save of one reaches the
	 * verb. It is mutable state rather than a value, because the set changes
	 * as tabs open and close and every command shares the one window's answer.
	 */
	readonly openComments: OpenComments;
	/**
	 * The catalogue the filing form reads its kind choices from.
	 *
	 * A function rather than the built catalogue, because the build is one
	 * round trip against a process that has to start and the form is the only
	 * caller here that needs it. It is the same VerbCatalog the command
	 * palette holds, so opening the form after the palette costs no spawn.
	 */
	readonly verbCatalog: () => Promise<CatalogBuild>;
}

/** One contributed command that reads rows. */
export interface RowCommand {
	readonly id: string;
	/** Runs the command over every targeted row and answers the one report. */
	readonly invoke: (
		elements: readonly TreeElement[],
		wiring: Wiring,
	) => Promise<BulkReport>;
}

/**
 * Every contributed command whose declared policy is not `noRow`.
 *
 * The ids here and the keys of SELECTION_POLICIES are held to each other in
 * both directions by test/unit/selection.test.ts, so a command given a table
 * entry but no policy fails as unexpected and one given a policy but no entry
 * fails as missing. The order is TREE_COMMANDS' own, which is the manifest's.
 */
export const ROW_COMMAND_TABLE: readonly RowCommand[] = [
	{ id: COMMAND_OPEN_CARD, invoke: invokeOpenCard },
	{ id: COMMAND_CLAIM, invoke: invokeClaim },
	{ id: COMMAND_MOVE, invoke: invokeMove },
	{ id: COMMAND_RELEASE, invoke: invokeRelease },
	{ id: COMMAND_BLOCK, invoke: invokeBlock },
	{ id: COMMAND_UNBLOCK, invoke: invokeUnblock },
	{ id: COMMAND_COPY_CARD_REF, invoke: invokeCopyCardRef },
	{ id: COMMAND_CHECK_WORKBENCH, invoke: invokeCheckWorkbench },
	{ id: COMMAND_COPY_WORKBENCH_PATH, invoke: invokeCopyWorkbenchPath },
	{ id: COMMAND_EDIT_WORKBENCH_DEFINITION, invoke: invokeEditWorkbenchDefinition },
	{ id: COMMAND_EDIT_COLUMN_INSTRUCTIONS, invoke: invokeEditColumnInstructions },
	{ id: COMMAND_OPEN_ATTACHMENT, invoke: invokeOpenAttachment },
	{ id: COMMAND_DELETE_ATTACHMENT, invoke: invokeDeleteAttachment },
	{ id: COMMAND_NEW_CARD, invoke: invokeNewCard },
	{ id: COMMAND_ATTACH_FILE, invoke: invokeAttachFile },
	{ id: COMMAND_PULL, invoke: invokePull },
	{ id: COMMAND_OPEN_INSTRUCTIONS, invoke: invokeOpenInstructions },
	{ id: COMMAND_OPEN_HISTORY, invoke: invokeOpenHistory },
	{ id: COMMAND_ARCHIVE_CARD, invoke: invokeArchiveCard },
	{ id: COMMAND_OPEN_ITEM, invoke: invokeOpenItem },
	{ id: COMMAND_COMMENT_ON_ITEM, invoke: invokeCommentOnItem },
	{ id: COMMAND_COMMENT_ON_COLUMN, invoke: invokeCommentOnColumn },
	{ id: COMMAND_RESOLVE_ITEM, invoke: invokeResolveItem },
	{ id: COMMAND_VERIFY_ITEM, invoke: invokeVerifyItem },
	{ id: COMMAND_FAIL_ITEM, invoke: invokeFailItem },
	{ id: COMMAND_REOPEN_ITEM, invoke: invokeReopenItem },
	{ id: COMMAND_RAISE_QUESTION, invoke: invokeRaiseQuestion },
	{ id: COMMAND_RECORD_DECISION, invoke: invokeRecordDecision },
	{ id: COMMAND_ADD_CRITERION, invoke: invokeAddCriterion },
	{ id: COMMAND_OPEN_COMMENT, invoke: invokeOpenComment },
];
