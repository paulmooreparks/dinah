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

/**
 * The mime type the tree publishes its own dragged rows under.
 *
 * VS Code recommends `application/vnd.code.tree.<treeidlowercase>` for a
 * tree's own drags, so the value is composed from VIEW_ID rather than typed a
 * second time. A unit test recomputes it the same way, so the two cannot drift
 * if the view is ever renamed.
 */
export const DRAG_MIME_TYPE = `application/vnd.code.tree.${VIEW_ID.toLowerCase()}`;

/**
 * The URI scheme served text opens under.
 *
 * One content provider is registered for the whole scheme, and the URI's
 * authority carries the kind of text being served. dinah-270 populates the
 * provider's resolver table with `instructions`, and a later kind is an entry
 * in that table rather than a change to the provider or to this scheme.
 */
export const SERVED_TEXT_SCHEME = "dinah-served";

/**
 * The id the manifest and the registration call must both spell.
 *
 * The API's own doc comment requires an extension to declare a
 * `contributes.mcpServerDefinitionProviders` entry with the id it later
 * registers under, so the manifest and `extension.ts` spell one value from
 * here rather than two literals a diff can part.
 */
export const MCP_PROVIDER_ID = "dinah.workbenches";

/** The settings key holding an explicit path to the binary. */
export const SETTING_PATH = "dinah.path";

/** The settings key pinning which workbench a folder uses. */
export const SETTING_WORKBENCH = "dinah.workbench";

/** The settings key holding how often the tree checks for changes. */
export const SETTING_POLL_INTERVAL = "dinah.pollIntervalSeconds";

/** The settings key turning the filesystem watcher off. */
export const SETTING_WATCH_FILES = "dinah.watchFiles";

/** The settings key a reader turns MCP registration off with. */
export const SETTING_REGISTER_MCP = "dinah.registerMcpServer";

/** The settings key a reader turns the language server off with. */
export const SETTING_LSP_ENABLED = "dinah.lsp.enabled";

/**
 * The settings key that draws the inline annotation in prose as well as in
 * front matter. The server reads it, because the server decides what an
 * annotation says.
 */
export const SETTING_LSP_ANNOTATE_PROSE = "dinah.lsp.annotateProse";

/**
 * The settings key holding how often the language server rereads the
 * workbench. The server reads this one too, for the same reason.
 */
export const SETTING_LSP_POLL_INTERVAL = "dinah.lsp.pollIntervalSeconds";

/** The settings key that traces the language server's own wire. */
export const SETTING_LSP_TRACE = "dinah.lsp.trace.server";

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

/** The command an attachment row runs to remove the attachment (dinah-451). */
export const COMMAND_DELETE_ATTACHMENT = "dinah.tree.deleteAttachment";

/** The command a column row runs to file a new card into it. */
export const COMMAND_NEW_CARD = "dinah.tree.newCard";

/** The command a workbench, column or card row runs to attach a file. */
export const COMMAND_ATTACH_FILE = "dinah.tree.attachFile";

/** The command a queue column row runs to pull its head-of-ready card onward. */
export const COMMAND_PULL = "dinah.tree.pull";

/** The card row's act that opens its served instruction chain as a tab (dinah-270). */
export const COMMAND_OPEN_INSTRUCTIONS = "dinah.tree.openInstructions";

/** The command palette's own two commands, which need no row (dinah-420). */
export const COMMAND_RUN_VERB = "dinah.runVerb";
export const COMMAND_REFRESH_VERB_CATALOG = "dinah.refreshVerbCatalog";

/**
 * The card row's act that opens its own journal as a tab (dinah-422).
 *
 * It sits beside openInstructions in every list a command belongs to,
 * because the two are the same shape of act: each is invoked on a card row,
 * each opens a read-only served-text tab, and neither changes anything.
 */
export const COMMAND_OPEN_HISTORY = "dinah.tree.openHistory";

/**
 * The card row's act that archives the card off the board (dinah-490).
 *
 * The name carries `Card` so that offering the same act on a column row later
 * is a second command rather than a rename of this one. A column refuses under
 * `dinah.occupied` and `dinah.last-column`, and explaining either to a reader
 * is work that card will own.
 */
export const COMMAND_ARCHIVE_CARD = "dinah.tree.archiveCard";

/**
 * The walkthrough a reader with no workbench is offered, and its one step.
 *
 * VS Code addresses a walkthrough as `<publisher>.<name>#<walkthroughId>`, so
 * the welcome view's link is that composition rather than a second spelling of
 * the identifier, and a unit test composes it the same way to hold the
 * manifest's own link to it. Both ids are dot-namespaced because VS Code's
 * manifest schema requires that shape of a contribution id (dinah-423 D-3).
 */
export const WALKTHROUGH_FIRST_SESSION = "dinah.firstSession";
export const WALKTHROUGH_STEP_READ_GUIDE = "dinah.firstSession.readGuide";

/** The command that step's button runs, which opens the guide as a tab. */
export const COMMAND_OPEN_FIRST_SESSION_GUIDE = "dinah.walkthrough.openFirstSessionGuide";

/**
 * The checklist-item commands dinah-506 contributes.
 *
 * The first six are invoked on an item row. The three filing commands below
 * them are invoked where an item does not exist yet, which is a card row or
 * one of the judgement branches hanging from it.
 */
export const COMMAND_OPEN_ITEM = "dinah.tree.openItem";
export const COMMAND_COMMENT_ON_ITEM = "dinah.tree.commentOnItem";
export const COMMAND_RESOLVE_ITEM = "dinah.tree.resolveItem";
export const COMMAND_VERIFY_ITEM = "dinah.tree.verifyItem";
export const COMMAND_FAIL_ITEM = "dinah.tree.failItem";
export const COMMAND_REOPEN_ITEM = "dinah.tree.reopenItem";

/**
 * The three filing commands dinah-517 contributes, one per item kind.
 *
 * They replace the single File Checklist Item, which named the storage
 * category rather than the act and then asked which of three kinds the reader
 * meant. Each command here carries its own kind, so the form asks the three
 * questions that carry a trap and no more, and a reader who already knows
 * what they are raising never answers a question they have answered by
 * choosing the command.
 */
export const COMMAND_RAISE_QUESTION = "dinah.tree.raiseQuestion";
export const COMMAND_RECORD_DECISION = "dinah.tree.recordDecision";
export const COMMAND_ADD_CRITERION = "dinah.tree.addCriterion";

/**
 * The comment command a column row offers, contributed by dinah-518.
 *
 * It sits beside the item command above rather than with it, because the
 * column is a second holder of comments rather than a second kind of item.
 * Like the item command, it mints an empty comment on the row it is invoked
 * on and opens that comment's file, and only the row and the reference that
 * row resolves to differ.
 */
export const COMMAND_COMMENT_ON_COLUMN = "dinah.tree.commentOnColumn";

/**
 * The comment command a card row offers, contributed by dinah-549.
 *
 * The card is the third and last holder of comments, after the item and the
 * column. The command mints an empty comment on the card and opens that
 * comment's file, as the other two do on their own rows.
 */
export const COMMAND_COMMENT_ON_CARD = "dinah.tree.commentOnCard";

/**
 * The one command a comment row offers, which opens the comment's own anchor
 * file (dinah-519).
 *
 * Nothing else is offered. `dinah comment` records a comment on a card, on a
 * column, or on one of a card's items, and refuses a comment's own reference,
 * so a Reply entry here would offer a refusal, and Dinah has no verb that
 * edits or deletes a comment.
 */
export const COMMAND_OPEN_COMMENT = "dinah.tree.openComment";

/**
 * The two commands a comment draft's own editor tab offers.
 *
 * They carry the `dinah.comment.` prefix rather than `dinah.tree.` because
 * neither reads a tree row. TREE_COMMANDS is documented as every command this
 * extension contributes rather than as a list of tree commands, and it
 * already carries the walkthrough's own command, so naming these two tree
 * commands would be the only false thing in the roster.
 */

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
	COMMAND_DELETE_ATTACHMENT,
	COMMAND_NEW_CARD,
	COMMAND_ATTACH_FILE,
	COMMAND_PULL,
	COMMAND_OPEN_INSTRUCTIONS,
	COMMAND_OPEN_HISTORY,
	COMMAND_ARCHIVE_CARD,
	COMMAND_OPEN_FIRST_SESSION_GUIDE,
	COMMAND_RUN_VERB,
	COMMAND_REFRESH_VERB_CATALOG,
	COMMAND_OPEN_ITEM,
	COMMAND_COMMENT_ON_ITEM,
	COMMAND_COMMENT_ON_COLUMN,
	COMMAND_COMMENT_ON_CARD,
	COMMAND_RESOLVE_ITEM,
	COMMAND_VERIFY_ITEM,
	COMMAND_FAIL_ITEM,
	COMMAND_REOPEN_ITEM,
	COMMAND_RAISE_QUESTION,
	COMMAND_RECORD_DECISION,
	COMMAND_ADD_CRITERION,
	COMMAND_OPEN_COMMENT,
];

/**
 * Commands that read their element argument and can do nothing without one.
 *
 * Each one is hidden from the Command Palette by a `when: "false"` entry in
 * package.json's `commandPalette` menu, because a palette invocation hands a
 * command no argument at all and a command that cannot act on a row is an
 * illegal action there. dinah-330 and dinah-335 extend this array with the
 * row-scoped commands they add, and they do not repeat the reasoning.
 * dinah-331's two creation commands join them for the same reason: each one
 * aims at the row it was invoked on, and a palette invocation names no row.
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
	COMMAND_DELETE_ATTACHMENT,
	COMMAND_NEW_CARD,
	COMMAND_ATTACH_FILE,
	COMMAND_PULL,
	COMMAND_OPEN_INSTRUCTIONS,
	COMMAND_OPEN_HISTORY,
	COMMAND_ARCHIVE_CARD,
	COMMAND_OPEN_ITEM,
	COMMAND_COMMENT_ON_ITEM,
	COMMAND_COMMENT_ON_COLUMN,
	COMMAND_COMMENT_ON_CARD,
	COMMAND_RESOLVE_ITEM,
	COMMAND_VERIFY_ITEM,
	COMMAND_FAIL_ITEM,
	COMMAND_REOPEN_ITEM,
	COMMAND_RAISE_QUESTION,
	COMMAND_RECORD_DECISION,
	COMMAND_ADD_CRITERION,
	COMMAND_OPEN_COMMENT,
];

/**
 * Commands that need no row and stay visible in the Command Palette.
 *
 * Every entry of TREE_COMMANDS belongs to exactly one of this array,
 * ROW_COMMANDS and EDITOR_COMMANDS. A manifest test holds the three to being
 * a complete and non-overlapping partition of TREE_COMMANDS, so a command
 * added there without a classification fails a test rather than shipping
 * unclassified.
 */
export const GLOBAL_COMMANDS: readonly string[] = [
	COMMAND_REFRESH,
	COMMAND_OPEN_FIRST_SESSION_GUIDE,
	COMMAND_RUN_VERB,
	COMMAND_REFRESH_VERB_CATALOG,
];

/**
 * Commands that read no tree row and are reachable in the Command Palette
 * only under a clause of their own.
 *
 * A row command is hidden from the palette outright because a palette
 * invocation names no row, and a global command is left undeclared there
 * because VS Code's own default makes it visible everywhere. An editor
 * command is neither: it acts on the active editor, so a palette invocation
 * reaches it correctly while that editor is open and nowhere else, and the
 * clause in the manifest is what says where. dinah-506's two draft commands
 * are the first of these.
 */
export const EDITOR_COMMANDS: readonly string[] = [
	// Empty since dinah-525, which deleted dinah-506's two draft commands
	// along with the apparatus behind them. The bucket is kept rather than
	// retired, because the partition it belongs to is the thing that stops a
	// command shipping unclassified, and a command acting on the active editor
	// is a shape this extension may want again.
];

/**
 * The nine contextValues a checklist item row carries, composed as
 * `dinah.item.<kind>.<state>` with an optional `.locked` suffix.
 *
 * The state axis collapses resolved, verified and failed to `closed`, because
 * all three offer exactly Reopen and nothing else, and a state outside the
 * four the format declares reads as closed too, so a damaged anchor offers
 * Reopen rather than a terminal verb.
 *
 * The suffix is what withholds the terminal verbs from a window that is not
 * the operator on an item the operator owns. Only the pending values take it,
 * because closeItem is where the owner is checked and Reopen deliberately
 * does not land there: reopening can only re-impose a hold and never lift
 * one. The manifest's clauses anchor on `pending$`, so a suffixed value
 * matches none of them.
 */
export const CONTEXT_ITEM_PREFIX = "dinah.item";
export const CONTEXT_ITEM_LOCKED_SUFFIX = "locked";

/** The state word a contextValue carries for an item that is still open. */
export const CONTEXT_ITEM_PENDING = "pending";

/** The state word every closed item carries, whichever way it closed. */
export const CONTEXT_ITEM_CLOSED = "closed";

/**
 * The head of the contextValue every collection row carries, completed with
 * the entity kind the collection holds: `dinah.collection.item` for a card's
 * checklist, `dinah.collection.attachment` for any attachments row, and
 * `dinah.collection.comment` for a thread.
 *
 * The kind is singular, as `contents` spells it, rather than the plural
 * directory name, so the attachments row under a card and the one under the
 * workbench root carry one value between them (dinah-519).
 *
 * A collection that narrows by an item kind carries a third segment naming
 * that kind, so a Questions branch carries `dinah.collection.item.question`
 * (dinah-517). The segment is the short kind token the item rows already use,
 * which is `question`, `decision` or `criterion`, rather than the stored
 * spelling the payload's `narrow` member carries. Two spellings are live in
 * this codebase, and the menus keep the one a reader already meets on
 * `dinah.item.question`. A collection that narrows by nothing carries the two
 * segments it carries today and gains no third.
 */
export const CONTEXT_COLLECTION_PREFIX = "dinah.collection";

/**
 * The contextValue every comment row carries.
 *
 * One value and no axes, on the terms CONTEXT_ATTACHMENT carries one. A
 * comment has no state, no owner and no kind, so there is nothing for an axis
 * to say.
 */
export const CONTEXT_COMMENT = "dinah.comment";

/**
 * The four answers actionsFor composes for a card row, and the row kinds
 * above them.
 *
 * A ready card's differentiator is its own column's published TakesWorkUp,
 * and there is deliberately no pull spelling here: dinah's pull verb takes a
 * destination rather than a card, so a card-scoped Pull could never be aimed
 * at the row that was clicked (dinah-265 D-2). dinah-375 gave the act to the
 * column row instead, which is the level the verb actually operates at, and
 * its two contextValues are the suffixed pair below.
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
/**
 * The contextValue every attachment row carries, whatever its payload (dinah-451).
 *
 * One value and no state axis, so the manifest clause matching it is an
 * equality rather than a regex. An attachment Dinah cannot open is exactly the
 * one a reader most wants off the card, so the value does not depend on
 * whether the payload reads.
 */
export const CONTEXT_ATTACHMENT = "dinah.attachment";

export const CONTEXT_COLUMN = "dinah.column";
export const CONTEXT_STATE_GROUP = "dinah.stateGroup";

/**
 * The two answers columnActionsFor composes for a column whose ColumnView the
 * status/tree join found, by whether that column will take another card.
 *
 * CONTEXT_COLUMN itself stays the value a column row carries when the join
 * missed and no ColumnView reached the row. Neither suffixed value matches it,
 * so such a row offers neither creation until the next checkpoint resolves it.
 */
export const CONTEXT_COLUMN_OPEN = "dinah.column.open";
export const CONTEXT_COLUMN_FULL = "dinah.column.full";

/**
 * The same two answers on a queue column a pull can be run from (dinah-375).
 *
 * Capacity and pullability are independent axes, so the suffix is appended to
 * whichever of the two above capacity decided rather than replacing it. A pull
 * takes a card out of the queue rather than putting one in, so a full queue is
 * as pullable as an open one, and both spellings carry the act.
 */
export const CONTEXT_COLUMN_OPEN_PULL = `${CONTEXT_COLUMN_OPEN}.pull`;
export const CONTEXT_COLUMN_FULL_PULL = `${CONTEXT_COLUMN_FULL}.pull`;
