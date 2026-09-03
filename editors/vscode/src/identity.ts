// The extension's own identity, in one place.
//
// The marketplace identifier is `<publisher>.<name>` and it is permanent from
// the first publish, so it is spelled here and nowhere else. The integration
// tests reach the extension through `vscode.extensions.getExtension(EXTENSION_ID)`,
// the packaging script names the vsix after it, and `package.json` declares the
// two halves separately because a manifest cannot import a module. A unit test
// asserts the manifest and this file agree, which is what keeps the duplication
// from becoming a divergence.

/** The marketplace publisher this extension ships under. */
export const PUBLISHER = "paulmooreparks";

/** The extension's own name, the second half of the marketplace identifier. */
export const EXTENSION_NAME = "dinah";

/** The marketplace identifier, `<publisher>.<name>`. */
export const EXTENSION_ID = `${PUBLISHER}.${EXTENSION_NAME}`;

/** The id of the view container this extension contributes to the activity bar. */
export const VIEW_CONTAINER_ID = "dinah";

/** The id of the single view inside that container. */
export const VIEW_ID = "dinah.workbenchView";

/** The settings key holding an explicit path to the binary. */
export const SETTING_PATH = "dinah.path";

/** The settings key pinning which workbench a folder uses. */
export const SETTING_WORKBENCH = "dinah.workbench";

/** The settings key holding how often the tree checks for changes. */
export const SETTING_POLL_INTERVAL = "dinah.pollIntervalSeconds";

/** The settings key turning the filesystem watcher off. */
export const SETTING_WATCH_FILES = "dinah.watchFiles";

/** The view title bar's refresh command. */
export const COMMAND_REFRESH = "dinah.tree.refresh";

/** The command every card row runs on a plain click. */
export const COMMAND_OPEN_CARD = "dinah.tree.openCard";

/** The five flow verbs the card context menu offers. */
export const COMMAND_CLAIM = "dinah.tree.claim";
export const COMMAND_MOVE = "dinah.tree.move";
export const COMMAND_RELEASE = "dinah.tree.release";
export const COMMAND_BLOCK = "dinah.tree.block";
export const COMMAND_UNBLOCK = "dinah.tree.unblock";

/** Puts a card's own reference on the clipboard, touching no board state. */
export const COMMAND_COPY_CARD_REF = "dinah.tree.copyCardRef";

/** The two acts a workbench row offers, neither of which touches the board. */
export const COMMAND_CHECK_WORKBENCH = "dinah.tree.checkWorkbench";
export const COMMAND_COPY_WORKBENCH_PATH = "dinah.tree.copyWorkbenchPath";

/** The workbench row's third act: opens workbench.md for editing (dinah-332). */
export const COMMAND_EDIT_WORKBENCH_DEFINITION = "dinah.tree.editWorkbenchDefinition";

/** The column row's first act: opens that column's own column.md for editing (dinah-332). */
export const COMMAND_EDIT_COLUMN_INSTRUCTIONS = "dinah.tree.editColumnInstructions";

/** The command an attachment row runs on a plain click, when its file can open. */
export const COMMAND_OPEN_ATTACHMENT = "dinah.tree.openAttachment";

/**
 * Every command this extension contributes, in the order package.json
 * declares them. A manifest test reads this array back, which is what keeps
 * a command registered in code but undeclared (or the reverse) from shipping.
 */
export const TREE_COMMANDS: readonly string[] = [
	COMMAND_REFRESH,
	COMMAND_OPEN_CARD,
	COMMAND_CLAIM,
	COMMAND_MOVE,
	COMMAND_RELEASE,
	COMMAND_BLOCK,
	COMMAND_UNBLOCK,
	COMMAND_COPY_CARD_REF,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_OPEN_ATTACHMENT,
];

/**
 * Commands that read their element argument and can do nothing without one.
 *
 * Each one is hidden from the Command Palette by a `when: "false"` entry in
 * package.json's `commandPalette` menu, because a palette invocation hands a
 * command no argument at all and a command that cannot act on a row is an
 * illegal action there. dinah-330 and dinah-335 extend this array with the
 * row-scoped commands they add, and they do not repeat the reasoning.
 */
export const ROW_COMMANDS: readonly string[] = [
	COMMAND_OPEN_CARD,
	COMMAND_CLAIM,
	COMMAND_MOVE,
	COMMAND_RELEASE,
	COMMAND_BLOCK,
	COMMAND_UNBLOCK,
	COMMAND_COPY_CARD_REF,
	COMMAND_CHECK_WORKBENCH,
	COMMAND_COPY_WORKBENCH_PATH,
	COMMAND_EDIT_WORKBENCH_DEFINITION,
	COMMAND_EDIT_COLUMN_INSTRUCTIONS,
	COMMAND_OPEN_ATTACHMENT,
];

/**
 * Commands that need no row and stay visible in the Command Palette.
 *
 * Every entry of TREE_COMMANDS belongs to exactly one of this array and
 * ROW_COMMANDS. A manifest test holds the two to being a complete and
 * non-overlapping partition of TREE_COMMANDS, so a command added there
 * without a classification fails a test rather than shipping unclassified.
 */
export const GLOBAL_COMMANDS: readonly string[] = [COMMAND_REFRESH];

/**
 * The four answers actionsFor composes for a card row, and the row kinds
 * above them.
 *
 * A ready card's differentiator is its own column's published TakesWorkUp,
 * and there is deliberately no pull spelling here: dinah's pull verb takes a
 * destination rather than a card, so no read-only call publishes the fact a
 * card-scoped Pull item would need (dinah-265 D-2, dinah-280).
 */
export const CONTEXT_CARD_READY_CLAIM = "dinah.card.ready.claim";
export const CONTEXT_CARD_READY_NONE = "dinah.card.ready.none";
export const CONTEXT_CARD_ACTIVE = "dinah.card.active";
export const CONTEXT_CARD_BLOCKED = "dinah.card.blocked";

/** The contextValue a root row carries, by how the folder resolved. */
export const CONTEXT_WORKBENCH_ROOT = "dinah.workbenchRoot";
export const CONTEXT_WORKBENCH_CANDIDATE = "dinah.workbenchCandidate";
export const CONTEXT_WORKBENCH_FOREST = "dinah.workbenchForest";

/** The contextValue a column row and a state group row carry. */
export const CONTEXT_COLUMN = "dinah.column";
export const CONTEXT_STATE_GROUP = "dinah.stateGroup";
