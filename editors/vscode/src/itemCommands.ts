// The seven checklist-item commands the tree contributes: reading one item,
// arguing on it, settling it, reopening it, and filing a new one.
//
// Nothing here imports vscode, for the reason cardCommands.ts's header gives.
// Each handler is a function over an injected host, so the unit layer asserts
// on the argv a command composes and on the message a refusal produces without
// a VS Code window.
//
// One refusal is deliberately left to the ordinary refusal path and gets no
// message of its own here, and this is dinah-506/questions/2's answer. When a
// workbench declares an evidence block, closeItem refuses resolve, verify and
// fail on an acceptance criterion carrying no citation, under the name
// Uncited. Citing is an explicit cut on this card: the roster below carries no
// cite command, so a message explaining that wall would explain it without
// offering a way past it, and the reader would still have to leave the editor.
// The refusal reaches them named and detailed, exactly as every other refusal
// on this surface does. Nothing on the workbench this was written against
// misbehaves today, because it declares no evidence block; the card that adds
// a cite command is the one that closes the gap.

import type { BulkReport } from "./bulk";
import { runBulk } from "./bulk";
import type { CommandHost, PickItem } from "./cardCommands";
import {
	isRow,
	pinnedArgv,
	refusalMessage,
	rowOutcomeFor,
	rowRef,
	runVerb,
} from "./cardCommands";
import type { CliOutcome, Spawner } from "./cli";
import { runDinah } from "./cli";
import type { Wiring } from "./commandTable";
import { openCommentDraft } from "./commentDrafts";
import type { Localizer } from "./l10n";
import type { HoldDirection, TreeElement, WorkbenchData } from "./tree";
import {
	cardColumnRefOf,
	columnOrderOf,
	holdDirection,
	itemHoldDirection,
	itemKindWord,
} from "./tree";
import type { CatalogBuild, CatalogOk } from "./verbCatalog";
import type { ItemView, PathAnswer } from "./wire";

/** The tool the filing form reads its kind choices from. */
export const FILE_ITEM_TOOL = "file_item";

/** What an item command acts on: one item, and where its card stands. */
export interface ItemCommandContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	/** The workspace folder the item's row belongs to. */
	readonly folder: string;
	/** The workbench the item stands in, which the call is pinned to. */
	readonly root: string;
	/** The item's own reference, which every verb below takes. */
	readonly ref: string;
	/** The card the item hangs from, which the reading document composes from. */
	readonly card: string;
	/**
	 * The item's joined view, absent where the detail call did not answer.
	 *
	 * A row drawn without one carries no contextValue, so no menu clause
	 * matches it and none of the acting commands can be aimed at it. Open Item
	 * is the exception and it reads no view: it asks `path` about the item's
	 * own reference.
	 */
	readonly view?: ItemView;
	/** This workbench's own data, which the hold sentence is computed from. */
	readonly data?: WorkbenchData;
}

/**
 * The context for an item command, or undefined for a row that is not an item.
 *
 * The absent element is checked by isRow before any field is read off it,
 * which is the one place that check lives.
 */
export function contextForItem(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): ItemCommandContext | undefined {
	if (!isRow(element, "item")) {
		return undefined;
	}
	const ref = element.node.ref ?? element.view?.ref ?? "";
	if (ref === "") {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		folder: element.row.folder,
		root: element.root,
		ref,
		card: element.card,
		view: element.view,
		data: element.row.data,
	};
}

/**
 * Opens the anchor file one reference resolves to.
 *
 * `dinah path <ref>` takes the entity's own reference and resolves it whatever
 * holds it, so no holder is composed and nothing is cut off the reference. The
 * call is pinned and run from the root like every other call this extension
 * makes, and a refusal is shown and nothing is opened, which is what openCard
 * already does on both of those arms. No catalogue key is needed for the
 * failure, because refusalMessage composes one out of what the binary said.
 *
 * `path` rather than `show`, and that is forced rather than preferred:
 * `show <item> --fields path` is refused, because only a card takes a field
 * selector, and `show <comment>` answers the anchor's raw text rather than
 * JSON at all. openCard reads its path off `show` and is left alone, because
 * rewriting a working handler is not this card's work.
 *
 * It lives here rather than beside each of its two callers because there is
 * one call and one hand-off, and a second copy of them is a second thing to
 * get wrong. It reads no item and takes none.
 */
export async function openAnchorFile(
	spawner: Spawner,
	exe: string,
	host: CommandHost,
	root: string,
	ref: string,
): Promise<void> {
	const outcome = await runDinah(spawner, exe, pinnedArgv(root, ["path", ref]), {
		cwd: root,
	});
	if (outcome.kind !== "ok") {
		host.showError(refusalMessage(outcome));
		return;
	}
	const path = (outcome.json as PathAnswer).path;
	if (path === undefined || path === "") {
		host.log(`dinah path ${ref} answered with no path`);
		return;
	}
	await host.openDocument(path);
}

/**
 * Opens the item's own anchor file.
 *
 * No document is composed over it. Clicking a card already opens `card.md`
 * with its front matter and lets the reader edit it, and an item behaves the
 * same way from here: the operator ruled on 2026-09-15 that he wants to deal
 * with what is in the workbench rather than with a page this extension
 * assembles over it.
 *
 * Editing that file bypasses the verb, so no journal entry is made. That is
 * already true of `card.md` and of `workbench.md`, whose own format
 * documentation calls a hand edit legal and deliberately unjournaled.
 */
export async function openItem(context: ItemCommandContext): Promise<void> {
	await openAnchorFile(
		context.spawner,
		context.exe,
		context.host,
		context.root,
		context.ref,
	);
}

/**
 * Asks for the one-line note the three terminal verbs take.
 *
 * The prompt is one line because vscode.window.showInputBox is a single-line
 * control and VS Code contributes no multi-line one. That is a fact about the
 * editor and it is the whole reason: the store would accept a two-line note
 * from `dinah resolve`, which escapes the newline and reads it back as two
 * lines, and nothing here relies on its refusing one. The refusal on a
 * newline belongs to `dinah set <item> note` alone.
 *
 * These four prompts are deliberately not routed through the draft buffer, and
 * that is a separate call from the ceiling. A resolution note records what
 * settled an item in a sentence somebody scanning the checklist can read, and
 * an argument long enough to need an editor belongs in the item's comment
 * thread, which is what the Comment command opens.
 *
 * One note applied to five different questions is a false record and the tool
 * would accept it in silence, so a selection of more than one row is refused
 * here, inside the one question the command asks, rather than narrowed.
 */
export function askItemNote(
	prompt: (ref: string, t: Localizer) => string,
): (
	resolved: readonly ItemCommandContext[],
	host: CommandHost,
) => Promise<string | undefined> {
	return async (resolved, host) => {
		if (resolved.length > 1) {
			host.showError(host.t("dialog.bulk.oneRowOnly"));
			return undefined;
		}
		const first = resolved[0];
		if (first === undefined) {
			return undefined;
		}
		const note = await host.input(prompt(first.ref, host.t));
		if (note === undefined || note.trim() === "") {
			return undefined;
		}
		return note.trim();
	};
}

/**
 * Asks once for the reason Reopen requires, over the whole selection.
 *
 * One reason for returning several items to pending is a true record rather
 * than a flattened one, which is why this command asks once and acts on every
 * row where the three terminal verbs refuse a selection outright.
 */
export async function askReopenReason(
	resolved: readonly ItemCommandContext[],
	host: CommandHost,
): Promise<string | undefined> {
	const reason = await host.input(
		host.t("prompt.item.reopen", { count: String(resolved.length) }),
	);
	if (reason === undefined || reason.trim() === "") {
		return undefined;
	}
	return reason.trim();
}

/** Runs one closing verb on one item with the note the reader gave. */
export async function closeItem(
	context: ItemCommandContext,
	verb: string,
	note: string,
): Promise<CliOutcome> {
	return runVerb(context, [verb, context.ref, note]);
}

/** Returns one closed item to pending with the reason the reader gave. */
export async function reopenItem(
	context: ItemCommandContext,
	reason: string,
): Promise<CliOutcome> {
	return runVerb(context, ["reopen", context.ref, reason]);
}

// ---------------------------------------------------------------------------
// The filing form
// ---------------------------------------------------------------------------

/** What the filing form acts on: one card, and the flow it stands in. */
export interface FileItemContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: CommandHost;
	readonly folder: string;
	readonly root: string;
	/** The card the item is filed against. */
	readonly ref: string;
	/** This workbench's own data, which the column step reads the flow from. */
	readonly data?: WorkbenchData;
}

/** The context for File Item, or undefined for a row that is not a card. */
export function contextForFileItem(
	element: TreeElement | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): FileItemContext | undefined {
	if (!isRow(element, "card")) {
		return undefined;
	}
	const ref = element.view?.ref ?? element.node.ref;
	const root = element.row.data?.path;
	if (ref === undefined || ref === "" || root === undefined) {
		return undefined;
	}
	return {
		spawner,
		exe,
		host,
		folder: element.row.folder,
		root,
		ref,
		data: element.row.data,
	};
}

/** The five answers the form collects before anything spawns. */
export interface FileItemAnswer {
	readonly kind: string;
	readonly text: string;
	readonly column: string;
	readonly owner: string;
}

/**
 * The kind choices, read out of the tool's own schema.
 *
 * They are not written into this extension, which is dinah-420's operator
 * ruling applied unchanged: a fourth kind added to the tool reaches this form
 * without anybody editing it. The value sent is the token verbatim, and the
 * label is the kind word the tree row already uses, so a reader who meets one
 * name in the form meets the same name on the row.
 *
 * A token this extension has no word for renders as the token. That is what
 * keeps the ruling true rather than nominally true, and the fallback is the
 * token rather than a skip, because a kind the form silently refuses to offer
 * is a kind nobody can file.
 */
export function kindPickItems(build: CatalogOk, t: Localizer): PickItem[] {
	const verb = build.verbs.find((entry) => entry.name === FILE_ITEM_TOOL);
	const argument = verb?.args.find((entry) => entry.name === "kind");
	const values =
		argument?.prompt.kind === "choice" ? argument.prompt.values : [];
	return values.map((value) => ({
		label: labelForKind(value, t),
		value,
	}));
}

/** The kind word for a token, falling back to the token itself. */
function labelForKind(token: string, t: Localizer): string {
	const known = ["acceptance_criterion", "open_question", "decision"];
	return known.includes(token) ? itemKindWord(token, t) : token;
}

/**
 * The column choices, each stating what the chosen column would do.
 *
 * The pick has no "none" entry and cannot be skipped. An open question or a
 * decision carrying an empty column refuses every claim on its card and
 * nothing on the card says why, which is the worst of the four outcomes the
 * tool allows, and the form removes it by construction.
 *
 * The detail is composed from the same twelve-cell holdDirection the item row
 * reads, so the two surfaces cannot disagree about what a column is doing. The
 * sentences say "this column" and interpolate nothing, because the pick's own
 * label already carries the title.
 *
 * The three sentences saying the column will hold nothing are the ones that
 * matter most. They are the silent failure this form exists to surface, and it
 * says so before the item exists rather than leaving somebody to notice months
 * later that a stop they meant to create was never there.
 */
export function columnPickItems(
	data: WorkbenchData | undefined,
	cardRef: string,
	t: Localizer,
): PickItem[] {
	const order = columnOrderOf(data);
	const cardIndex = order.indexOf(cardColumnRefOf(data, cardRef) ?? "");
	return order.flatMap((ref, at) => {
		const view = data?.columns.get(ref);
		if (view === undefined) {
			return [];
		}
		// A card whose own column the flow does not carry leaves cardIndex at
		// -1, which would put every candidate ahead of it. The direction is
		// then unknowable rather than wrong, so it is reported as no hold,
		// which is the answer that claims least.
		const direction: HoldDirection =
			cardIndex < 0 ? "nothing" : holdDirection(view.hold, at, cardIndex);
		return [
			{
				label: view.title,
				detail: t(`form.file.column.detail.${direction}`),
				value: columnValueOf(view),
			},
		];
	});
}

/**
 * What `--column` is sent, which is the column's slug where it has one.
 *
 * Library.File resolves either spelling to an identifier before it writes, so
 * a stored value can never be a spelling a gate would fail to match.
 */
function columnValueOf(view: {
	readonly slug?: string;
	readonly id: string;
}): string {
	return view.slug !== undefined && view.slug !== "" ? view.slug : view.id;
}

/** The one value the tool enforces, whose meaning its own schema publishes. */
export const OWNER_OPERATOR = "operator";

/** This workbench's own word for whoever is holding the card. */
export const OWNER_HOLDER = "holder";

/**
 * The owner choices, with no default and no way to omit the answer.
 *
 * An unstamped item reads downstream as the operator's, which is how a large
 * share of a workbench's pending items become his, and a form whose default
 * silently enlarges his queue is the failure this command exists to undo.
 *
 * `operator` is the one value the tool enforces. `holder` is a workbench's own
 * convention rather than a token the tool knows, which is why the third entry
 * exists: a workbench using another word types it and is not blocked by a
 * two-entry pick.
 */
export function ownerPickItems(t: Localizer): PickItem[] {
	return [
		{
			label: t("form.file.owner.operator"),
			detail: t("form.file.owner.operatorDetail"),
			value: OWNER_OPERATOR,
		},
		{ label: t("form.file.owner.holder"), value: OWNER_HOLDER },
		{ label: t("form.file.owner.other"), value: "" },
	];
}

/**
 * Walks the five steps, and answers undefined where any of them was declined.
 *
 * Nothing spawns until all five have answers, and each step is cancellable.
 * There is no confirmation step: the quick picks are the confirmation, and a
 * modal on top of four deliberate choices would be a turnstile.
 *
 * A selection of more than one card is refused here, inside the one question
 * this command asks first, because filing one item across several cards would
 * multiply the column mistake the form exists to prevent.
 */
export async function askFileItem(
	resolved: readonly FileItemContext[],
	host: CommandHost,
	catalog: CatalogBuild,
): Promise<FileItemAnswer | undefined> {
	if (resolved.length > 1) {
		host.showError(host.t("dialog.bulk.oneRowOnly"));
		return undefined;
	}
	const first = resolved[0];
	if (first === undefined) {
		return undefined;
	}
	// Where the catalogue did not build, the form refuses with the
	// catalogue's own reason rather than falling back to a typed list. A
	// typed list is exactly what the ruling above forbids, and a form that
	// quietly stops reflecting the tool is worse than one that says it cannot
	// run.
	if (catalog.kind !== "ok") {
		// The command palette's own two strings for this condition, because
		// the condition is the same one and a second pair would say the same
		// thing in two voices.
		host.showError(host.t("dialog.runVerb.enumerationFailed.toast"));
		host.appendLines([
			host.t("dialog.runVerb.enumerationFailed.channel", {
				detail: catalog.detail,
			}),
		]);
		return undefined;
	}
	const kinds = kindPickItems(catalog, host.t);
	if (kinds.length === 0) {
		host.showError(host.t("dialog.runVerb.enumerationFailed.toast"));
		host.appendLines([
			host.t("dialog.runVerb.enumerationFailed.channel", {
				detail: `${FILE_ITEM_TOOL} publishes no kind choices`,
			}),
		]);
		return undefined;
	}
	const kind = await host.pick(kinds, host.t("form.file.kind.placeholder"));
	if (kind === undefined) {
		return undefined;
	}
	const text = await host.input(host.t("form.file.text.prompt"));
	if (text === undefined || text.trim() === "") {
		return undefined;
	}
	// An empty pick cannot arise on a workbench that opened, because a
	// workbench declaring no column does not open at all, and host.pick over
	// an empty array answers undefined, which is read as a cancellation. So
	// the case needs no sentence of its own.
	const columns = columnPickItems(first.data, first.ref, host.t);
	const column = await host.pick(
		columns,
		host.t("form.file.column.placeholder"),
	);
	if (column === undefined) {
		return undefined;
	}
	const owner = await host.pick(
		ownerPickItems(host.t),
		host.t("form.file.owner.placeholder"),
	);
	if (owner === undefined) {
		return undefined;
	}
	let ownerValue = owner.value;
	if (ownerValue === "") {
		const typed = await host.input(host.t("form.file.owner.otherPrompt"));
		if (typed === undefined || typed.trim() === "") {
			return undefined;
		}
		ownerValue = typed.trim();
	}
	return {
		kind: kind.value,
		text: text.trim(),
		column: column.value,
		owner: ownerValue,
	};
}

/** Files the item the form composed, in one call. */
export async function fileItem(
	context: FileItemContext,
	answer: FileItemAnswer,
): Promise<CliOutcome> {
	return runVerb(context, [
		"file",
		context.ref,
		answer.kind,
		answer.text,
		"--column",
		answer.column,
		"--owner",
		answer.owner,
	]);
}

// ---------------------------------------------------------------------------
// What the registration loop calls
// ---------------------------------------------------------------------------

// The two channel lines a row of the wrong kind gets. They go through the
// localizer where the older skip reasons in this extension are plain English
// constants, because these two are the first the specification named as
// catalogue entries and a reader meets them in the output channel.

/** Opens the document of every selected item, one tab each. */
export async function invokeOpenItem(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForItem(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notAnItemRow"),
		},
		async () => true as const,
		async (context, _answer, host) => {
			await openItem({ ...context, host });
			return { kind: "done" } as const;
		},
	);
}

/**
 * Writes a draft for the one selected item and opens it, and spawns nothing.
 *
 * The draft's own commands post it and throw it away, from its editor tab, so
 * this command asks nothing and reports nothing but where the draft is.
 */
export async function invokeCommentOnItem(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForItem(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notAnItemRow"),
		},
		async (resolved, host) => {
			if (resolved.length > 1) {
				host.showError(host.t("dialog.bulk.oneRowOnly"));
				return undefined;
			}
			return resolved.length === 1 ? (true as const) : undefined;
		},
		async (context) => {
			await openCommentDraft(wiring.draftHost, {
				root: context.root,
				folder: context.folder,
				ref: context.ref,
			});
			return { kind: "done" } as const;
		},
	);
}

/** Builds the invoke for one of the three terminal verbs. */
function invokeCloseItem(
	verb: string,
	prompt: (ref: string, t: Localizer) => string,
): (elements: readonly TreeElement[], wiring: Wiring) => Promise<BulkReport> {
	return async (elements, wiring) =>
		runBulk(
			elements,
			(element) => rowRef(element, wiring.t),
			(element) =>
				contextForItem(element, wiring.exe, wiring.cardHost, wiring.spawner),
			{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notAnItemRow"),
		},
			askItemNote(prompt),
			async (context, note, host) =>
				rowOutcomeFor(await closeItem({ ...context, host }, verb, note)),
		);
}

/** Resolves the one selected question or decision. */
export const invokeResolveItem = invokeCloseItem("resolve", (ref, t) =>
	t("prompt.item.resolve", { ref }),
);

/** Verifies the one selected acceptance criterion. */
export const invokeVerifyItem = invokeCloseItem("verify", (ref, t) =>
	t("prompt.item.verify", { ref }),
);

/** Fails the one selected acceptance criterion. */
export const invokeFailItem = invokeCloseItem("fail", (ref, t) =>
	t("prompt.item.fail", { ref }),
);

/** Returns every selected closed item to pending, on one reason. */
export async function invokeReopenItem(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForItem(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notAnItemRow"),
		},
		askReopenReason,
		async (context, reason, host) =>
			rowOutcomeFor(await reopenItem({ ...context, host }, reason)),
	);
}

/** Files one new checklist item against the one card the reader aimed at. */
export async function invokeFileItem(
	elements: readonly TreeElement[],
	wiring: Wiring,
): Promise<BulkReport> {
	return runBulk(
		elements,
		(element) => rowRef(element, wiring.t),
		(element) =>
			contextForFileItem(element, wiring.exe, wiring.cardHost, wiring.spawner),
		{
			host: wiring.cardHost,
			t: wiring.t,
			skipReason: wiring.t("skip.notACardRow"),
		},
		async (resolved, host) =>
			askFileItem(resolved, host, await wiring.verbCatalog()),
		async (context, answer, host) =>
			rowOutcomeFor(await fileItem({ ...context, host }, answer)),
	);
}

/**
 * The hold sentence one item row shows.
 *
 * It is a thin name over tree.ts's own itemHoldDirection rather than a second
 * computation, because a second copy of a twelve-cell branch is what would
 * drift.
 *
 * A row whose detail call did not answer carries no ItemView, and there is
 * nothing for a hold sentence to be composed out of, so it reads as nothing
 * held rather than as a direction guessed at.
 */
export function directionForItem(
	context: ItemCommandContext,
): HoldDirection {
	return context.view === undefined
		? "nothing"
		: itemHoldDirection(context.data, context.card, context.view);
}

/** Re-exported so a caller composing a refusal message names one function. */
export { refusalMessage };
