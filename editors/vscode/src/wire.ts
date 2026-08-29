// The Go JSON shapes this extension reads, mirrored in TypeScript.
//
// Every interface here is a mirror of a struct in internal/verb, named after
// it and carrying the field names that struct's own json tags publish. They
// are gathered in one module rather than declared beside their readers so
// that a rename on the Go side is answered in one place: dinah-287 renamed a
// column's own vocabulary across the whole surface, and a mirror scattered
// through four modules would have been four separate repairs.
//
// Optional fields are optional here exactly where the Go tag says omitempty,
// because that is the difference between a key holding an empty string and a
// key that is not there at all, and a reader that conflates the two renders
// "undefined" into somebody's sidebar.

/** verb.ColumnView, one column of a Status. */
export interface ColumnView {
	readonly id: string;
	readonly slug?: string;
	readonly title: string;
	readonly kind: string;
	readonly operator_owned: boolean;
	readonly awaiting_outside: boolean;
	readonly takes_work_up: boolean;
	readonly capacity?: number;
	readonly reject_to?: string;
	readonly count: number;
	readonly attachment_count?: number;
}

/** verb.CardView, one card as a read reports it. */
export interface CardView {
	readonly id: string;
	readonly ref?: string;
	readonly title?: string;
	readonly column?: string;
	readonly column_title?: string;
	readonly state?: string;
	readonly severity?: string;
	readonly priority?: string;
	readonly holder?: string;
	readonly block_reason?: string;
	readonly block_kind?: string;
	readonly workstreams?: readonly string[];
	readonly revision?: string;
	readonly attachment_count?: number;
}

/** verb.Status, what `dinah --json status` emits. */
export interface StatusAnswer {
	readonly workbench?: string;
	readonly root?: string;
	readonly profile?: string;
	readonly columns?: readonly ColumnView[];
	readonly attachment_count?: number;
}

/** verb.Listing, what `dinah --json ls` emits. */
export interface ListingAnswer {
	readonly cards?: readonly CardView[];
}

/** verb.Hidden, a node's account of what it is not showing. */
export interface HiddenAccount {
	readonly reason?: readonly string[];
	readonly children?: number;
	readonly subjects?: number;
	readonly filtered?: number;
}

/** verb.TreeNode, one node of a projected tree. */
export interface TreeNode {
	readonly kind: string;
	readonly id?: string;
	readonly ref?: string;
	readonly title?: string;
	readonly axis?: string;
	readonly value?: string;
	readonly count: number;
	readonly hidden?: HiddenAccount;
	readonly children?: readonly TreeNode[];
}

/** verb.Tree, what `dinah --json tree` emits. */
export interface TreeAnswer {
	readonly producer: string;
	readonly subject: string;
	readonly group_by?: readonly string[];
	readonly depth: string;
	readonly root: TreeNode;
}

/** verb.Detail, what `dinah --json show <ref>` emits. */
export interface DetailAnswer {
	readonly card?: CardView;
	readonly body?: string;
	readonly path?: string;
}

/**
 * verb.AttachmentView, one attachment as a read reports it.
 *
 * `path` is what lets this extension open the file without a second call, and
 * the Go side omits it when the payload will not read, so a reader shows such
 * an attachment as present and unopenable rather than dropping the row
 * (dinah-334's own ruling on the field).
 */
export interface AttachmentView {
	readonly id: string;
	readonly ordinal: number;
	readonly ref: string;
	readonly filename: string;
	readonly description?: string;
	readonly provenance: string;
	readonly path?: string;
}

/**
 * verb.AttachmentListing, what `dinah --json attachments [ref]` emits.
 *
 * The list is the entity's own attachments alone, never those of anything it
 * contains, and an entity carrying none reports an empty list rather than
 * nothing at all, which is why `attachments` is required here.
 */
export interface AttachmentListing {
	readonly kind: string;
	readonly ref: string;
	readonly attachments: readonly AttachmentView[];
}

/** verb.LegalMove, one departure the workbench allows right now. */
export interface LegalMove {
	readonly column: string;
	readonly ref: string;
	readonly title: string;
	readonly direction: string;
	readonly reject?: boolean;
}

/** verb.Served, what `dinah --json instructions <ref>` emits. */
export interface ServedAnswer {
	readonly legal_moves?: readonly LegalMove[];
}

/** The two directions verb.LegalMove.Direction carries. */
export const FORWARD = "forward";
export const BACKWARD = "backward";

/**
 * bench.Candidate, the identity a walk read off a workbench's anchor.
 *
 * `refused` says the workbench itself would not read. It never carries a
 * refusal raised by a question asked of a workbench that opened; that fact
 * rides `unanswered` on the member shapes below, and the two are separate
 * fields precisely so a client draws them as two different rows.
 */
export interface Candidate {
	readonly title: string;
	readonly slug?: string;
	readonly path: string;
	readonly refused?: string;
}

/** One workbench's row of a verb.Forest. */
export interface WorkbenchTree extends Candidate {
	readonly unanswered?: string;
	readonly tree?: TreeAnswer;
}

/** verb.Forest, what `dinah --json tree --root <dir>` emits. */
export interface ForestAnswer {
	readonly root: string;
	readonly workbenches?: readonly WorkbenchTree[];
}

/** One workbench's row of a verb.RootStatus. */
export interface WorkbenchStatus extends Candidate {
	readonly unanswered?: string;
	readonly status?: StatusAnswer;
}

/** verb.RootStatus, what `dinah --json status --root <dir>` emits. */
export interface RootStatusAnswer {
	readonly root: string;
	readonly workbenches?: readonly WorkbenchStatus[];
}

/** One workbench's row of a verb.RootListing. */
export interface WorkbenchListing extends Candidate {
	readonly unanswered?: string;
	readonly listing?: ListingAnswer;
}

/** verb.RootListing, what `dinah --json ls --root <dir>` emits. */
export interface RootListingAnswer {
	readonly root: string;
	readonly workbenches?: readonly WorkbenchListing[];
}

/** verb.ChangeSet, what `dinah --json changes` emits for one workbench. */
export interface ChangeSetAnswer {
	readonly cursor: string;
	readonly changed: boolean;
}

/** One workbench's row of a verb.RootChangeSet. */
export interface WorkbenchChangeSet extends Candidate {
	readonly new?: boolean;
	readonly unanswered?: string;
	readonly changes?: ChangeSetAnswer;
}

/**
 * verb.RootChangeSet, what `dinah --json changes --root <dir>` emits.
 *
 * `cursor` is one opaque merged token covering every workbench beneath the
 * root. It is stored and replayed verbatim: nothing here decodes it or
 * derives a per-workbench token from it, which is the contract rootCursor's
 * own doc comment states from the other side.
 */
export interface RootChangeSetAnswer {
	readonly root: string;
	readonly cursor: string;
	readonly changed: boolean;
	readonly workbenches?: readonly WorkbenchChangeSet[];
}

/** The three states a card can stand in, as CardView.State spells them. */
export const STATE_READY = "ready";
export const STATE_ACTIVE = "active";
export const STATE_BLOCKED = "blocked";

/** The node kinds the grouped producer emits, as TreeNode.Kind spells them. */
export const NODE_GROUP = "group";
export const NODE_CARD = "card";

/** The axis a column-axis group node groups on, as TreeNode.Axis spells it. */
export const AXIS_COLUMN = "column";
