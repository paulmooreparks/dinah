---
title: An attachment can be added from the sidebar and never removed from it
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: workhorse
workstreams:
  - 58f3e3eb621a
---
The sidebar can attach a file to a card and open an attachment that is already there, and that is all it can do with one. An attachment row carries no context menu at all, so removing an attachment means leaving the editor for a terminal and typing `dinah delete <ref> --yes`. This is the shape dinah-375 and dinah-377 were both about: the tree shows you a thing and then gives you no way to act on it.

The tool's own side is already finished, so nothing new is needed below the editor. `delete` takes an attachment reference like any other entity, removes the attachment's directory and its bytes, and writes an attachment-removed line to the card's journal so the history still records that the file was there and went away. `archive` takes one too, and is the reversible half. Both are already served over MCP, so the extension needs no new tool and no new CLI work.

What the card is for: an attachment row in the sidebar gets a way to remove the attachment, running what the tool already does.

Questions the spec owes an answer to, none of which are the operator's:

Whether the row offers delete alone, or delete and archive as two actions. Delete is what a person means by "get this off the card", and it is irreversible; archive is recoverable and leaves the bytes in place. Offering both is two menu items and a second confirmation flow, and offering only delete gives a reader no undo at all from inside the editor. Rule it in the spec and say why.

What the confirmation looks like, given the CLI's own answer is a required `--yes` marker rather than a prompt. Whatever the extension shows has to name the file, because an attachment row's label is the filename and a reader who has several attached will otherwise be confirming a sentence that could mean any of them.

Where the action sits: a context-menu item on the attachment row, with the same `when: false` commandPalette exclusion every other row action carries, since a palette entry for a command that needs a selected row is a command that errors when it is run from the palette (dinah-342 is that defect, already fixed once).

What happens to the tree afterwards. The removal changes the card's attachment count as well as the group's contents, so the refresh has to reach both.

The work is small and the ground is well trodden: the argv-composing shape in cardCommands.ts, a manifest command plus a menu entry, and new strings across all eight locale catalogues. The existing guards apply unchanged, including the one that proves every declared command is actually registered and the localisation parity and fingerprint checks.

## Specification

An attachment row in the sidebar gains one context-menu item, Delete, which runs `dinah delete <ref> --yes` behind a modal confirmation that names the file. The work is confined to `editors/vscode/`, plus one Go regression test that pins the behaviour the tree's refresh depends on. No CLI verb, no MCP tool, and no wire field changes.

## What the reading established

Every claim below was read out of the tree at 178e264 and is cited so the next stage can check it rather than take it.

The attachment row carries no `contextValue` at all. `attachmentItem`'s `attachment` arm in `editors/vscode/src/tree.ts:803-833` composes a label, a description, a tooltip, an icon, and (when the payload reads) a command, and no `contextValue` key. VS Code matches a `view/item/context` entry against `viewItem`, so no menu can be attached to the row until the row carries a value. Minting that value is the first change.

`delete` accepts an attachment reference and requires `--yes`. `internal/verb/definition.go:342-345` declares `delete` as a `ref` slot plus a `yes` marker with `Required: true`, and `internal/verb/beyond.go:270-272` refuses with `contract.Unconfirmed` when `req.Confirm` is false. `Library.Delete` resolves any entity, so an attachment reference reaches it unchanged.

Deleting an attachment writes an `attachment_removed` line to the owning entity's journal. `removalRecord` in `internal/verb/beyond.go:722-729` switches the event from `contract.EventDeleted` to `contract.EventAttachmentRemoved` for `entity.Kind == "attachment"`, and carries the attachment identifier and the filename as of the event.

**Nothing undoes an archive, and the repository already says so in a maintained place.** `cmd/dinah/compat_test.go:128-130` declares `unwrittenEvents`, whose single entry exempts `contract.EventRestored` with the written reason `archive has no inverse verb in the command surface, so nothing restores an entity`. That table is what the coverage alarm at `compat_test.go:224-235` consults: an event `internal/contract` declares and the sample fixture carries no line of fails the build unless the table names it and gives the reason. The table is a standing, build-enforced statement about the command surface rather than a figure somebody typed, and it is the evidence D-1 rests on.

The guard's reach stops short of catching a restore verb on the day it lands, and its own comment says so. `compat_test.go:265-272` records that an event nothing writes to a card journal "is caught in neither direction", and it names `EventRestored` as a live instance of exactly that. So I checked the present state directly as well. `grep -niI restore cmd/dinah/commands.go internal/mcp/tools.go` at 178e264 answers nothing on either file, and those two files are the sole declaration sites for the CLI's verbs and the MCP surface's tools, which `grep -rln 'name:.*command:' internal/mcp/` confirms for the tools by answering `tools.go` alone. `bench.OpRestore`, `bench.RestoreTarget`, and `contract.EventRestored` exist in the storage layer, and nothing above that layer reaches them. Archiving is reversible only by moving a directory by hand.

An attachment's published reference is positional and drifts. `attachmentRef` in `internal/verb/read.go:1024-1029` composes `<owner-ref>/attachments/<n>`, where `n` is `displayOrdinal`'s index into the collection sorted by `SortByOrdinal`. `memberPosition`'s own comment at `internal/verb/read.go:1005-1012` says the position and the stored ordinal "stop coinciding after one delete". A reference held by a drawn row therefore names a different attachment after somebody else deletes an earlier one and attaches another.

The same reference grammar accepts a stable identifier. `pick` in `internal/bench/resolve.go:368-376` tests `IsID(selector)` first and matches the member whose directory name equals it, before it tries the positional and named arms. `IsID` at `internal/bench/storage.go:111-124` accepts exactly twelve lowercase hex characters. `AttachmentView.ID` is declared `json:"id"` with no `omitempty` at `internal/verb/read.go:631-633`, so the identifier is present on every attachment a listing reports.

The listing carries a resolvable owner reference and the group does not. `Library.Attachments` at `internal/verb/read.go:937-948` substitutes the literal `workbench` for the workbench's own empty reference before composing anything, so `AttachmentListing.ref` is always a spelling a person could type. The tree's `attachmentsGroup` element carries `ref: ""` for the workbench (`editors/vscode/src/tree.ts:1799-1809`, and `tree.test.ts:1499` asserts it), which composes nothing.

The group's own root is the workbench's path and not the folder its row was drawn for. `rootChildren` fills the workbench's `attachmentsGroup` with `root: data.path` (`tree.ts:1799-1809`), while `RootRow.folder` (`tree.ts:183-190`) is the workspace folder that produced the row. The two coincide on a plain workbench row and differ on a forest member, where one folder holds several workbenches and each member's own `path` sits below it. AC-4 turns on that difference.

A checkpoint sees the deletion. `bench.WatchedEntities` at `internal/bench/changes.go:57-74` puts every live card's journal size into the live digest term, and `Library.Changes` at `internal/verb/changes.go:294-295` digests it. Appending `attachment_removed` to the card's journal moves that term, so `dinah changes` answers `changed: true` and `CheckpointLoop.check` (`editors/vscode/src/changes.ts:318-321`) runs `refresh(folder)` and fires the tree event. A card's `attachment_count` is `bench.CountAttachments(card.Dir)` at `internal/verb/library.go:467`, a live directory count, so the refreshed row reports the decremented number.

The attachments listing is never cached. The `attachmentsGroup` arm of `getChildren` (`editors/vscode/src/tree.ts:1719-1751`) calls `readAttachments` on every expansion, and the class comment at `tree.ts:1466-1469` says so.

## The names this card mints

This section spells every identifier, key, and literal the implementer needs, as it will appear.

### Identifiers in `editors/vscode/src/identity.ts`

```ts
/** The command an attachment row runs to remove the attachment (dinah-451). */
export const COMMAND_DELETE_ATTACHMENT = "dinah.tree.deleteAttachment";

/** The contextValue every attachment row carries, whatever its payload (dinah-451). */
export const CONTEXT_ATTACHMENT = "dinah.attachment";
```

`COMMAND_DELETE_ATTACHMENT` is appended to `TREE_COMMANDS` immediately after `COMMAND_OPEN_ATTACHMENT`, and to `ROW_COMMANDS` immediately after `COMMAND_OPEN_ATTACHMENT` as well. The two arrays hold different indices and nothing pins `ROW_COMMANDS`'s order, so each placement is described by the neighbour it follows rather than by a position. The command is not added to `GLOBAL_COMMANDS`. `package.json` declares it immediately after `dinah.tree.openAttachment`, because `manifest.test.ts:933` compares the manifest's order against `TREE_COMMANDS`.

### The wire-grammar segment, in `editors/vscode/src/wire.ts`

```ts
/**
 * The collection segment an attachment reference carries.
 *
 * This is the value of `bench.AttachmentsDir` as the reference grammar spells
 * it, written here because the extension composes a reference of its own
 * rather than only reading ones the tool composed.
 */
export const ATTACHMENTS_SEGMENT = "attachments";
```

### The tree element, in `editors/vscode/src/tree.ts`

The `attachment` arm of `TreeElement` (currently `tree.ts:279-283`) gains two members:

```ts
| {
        readonly kind: "attachment";
        readonly row: RootRow;
        /**
         * The workbench root the listing that produced this row was pinned to,
         * taken from the group element's own `root` rather than from
         * `row.folder`. A forest row's folder holds several workbenches, so
         * the folder is not a workbench root at all there.
         */
        readonly root: string;
        /**
         * The reference of the entity this attachment hangs from, taken from
         * the listing's own `ref` rather than from the group's.
         *
         * The listing resolves the workbench's own reference to the literal
         * `workbench`, while the group carries the empty string the binary is
         * asked with, so only the listing's answer composes a reference a
         * later call can resolve.
         */
        readonly owner: string;
        readonly view: AttachmentView;
  };
```

The `attachmentsGroup` arm of `getChildren` fills both from the read that produced the row:

```ts
return listing.attachments.map((view) => ({
        kind: "attachment" as const,
        row: element.row,
        root: element.root,
        owner: listing.ref,
        view,
}));
```

`attachmentItem`'s `attachment` arm gains `contextValue: CONTEXT_ATTACHMENT`, set unconditionally. An attachment whose payload will not read is the one a reader most wants gone, so the value does not depend on `openable`. The `command` key stays conditional exactly as it is.

### The host method, in `editors/vscode/src/cardCommands.ts`

`CommandHost` gains one member:

```ts
/**
 * Asks the reader to confirm an act that cannot be undone, and answers
 * whether they did.
 *
 * Two arguments rather than one because a modal dialog supplies its own
 * Cancel and takes the affirmative button's label from the caller, and both
 * strings are prose a reader meets, so both go through the localizer.
 */
readonly confirmDestructive: (
        message: string,
        confirmLabel: string,
) => Promise<boolean>;
```

Bound in `editors/vscode/src/extension.ts`'s `commandHost`:

```ts
confirmDestructive: async (message, confirmLabel) => {
        const picked = await vscode.window.showWarningMessage(
                message,
                { modal: true },
                confirmLabel,
        );
        return picked === confirmLabel;
},
```

That is the documented three-argument overload `showWarningMessage<T extends string>(message: string, options: MessageOptions, ...items: T[])`, with the documented `MessageOptions.modal`. The extension declares `engines.vscode` and `@types/vscode` at `^1.90.0`, and both members predate that by many releases. This spec could not read the local `.d.ts`, because `node_modules` is not installed in the worktree the spec was written from; the claim rests on the published API reference, and the build type-checks it.

### The two commands, in `editors/vscode/src/cardCommands.ts`

```ts
/**
 * The context an attachment row's own verbs run in.
 *
 * The reference is composed from the attachment's identifier rather than
 * from `view.ref`. `view.ref` ends in the attachment's position within its
 * collection, and a position shifts when an earlier attachment is deleted,
 * so a row drawn before somebody else's delete-then-attach would address a
 * different file with nothing refused. The identifier the listing reported
 * is what the row was drawn from and it names one attachment for as long as
 * that attachment exists.
 *
 * The absent element is checked by isRow before any field is read, which is
 * the ordering dinah-342 found wrong under six commands at once.
 */
export function contextForAttachment(
        element: TreeElement | undefined,
        exe: string,
        host: CommandHost,
): CommandContext | undefined {
        if (!isRow(element, "attachment")) {
                return undefined;
        }
        if (element.owner === "" || element.root === "" || element.view.id === "") {
                return undefined;
        }
        return {
                spawner: nodeSpawner,
                exe,
                host,
                folder: element.row.folder,
                root: element.root,
                ref: `${element.owner}/${ATTACHMENTS_SEGMENT}/${element.view.id}`,
        };
}

/**
 * Deletes an attachment, after asking the reader to confirm it.
 *
 * The confirmation is the extension's own, because the tool's answer to the
 * same question is a required `--yes` marker rather than a prompt, and a
 * marker composed in code asks nobody anything. The sentence names the file
 * and the address the row was drawn from, since an attachment row is
 * labelled by filename alone and one entity may carry several.
 *
 * A declined confirmation returns undefined having spawned nothing, so the
 * board is not re-read for an act that did not happen. Everything after the
 * confirmation is runVerb's, which reports a refusal and checkpoints either
 * way.
 */
export async function deleteAttachment(
        element: TreeElement | undefined,
        exe: string,
        host: CommandHost,
        log: (line: string) => void,
): Promise<CliOutcome | undefined> {
        if (!isRow(element, "attachment")) {
                log(`${COMMAND_DELETE_ATTACHMENT} was invoked on a row that names no attachment`);
                return undefined;
        }
        const context = contextForAttachment(element, exe, host);
        if (context === undefined) {
                log(`${COMMAND_DELETE_ATTACHMENT} was invoked on an attachment row that composes no reference`);
                return undefined;
        }
        const confirmed = await host.confirmDestructive(
                host.t("dialog.attachment.delete.confirm", {
                        filename: element.view.filename,
                        ref: element.view.ref,
                }),
                host.t("dialog.attachment.delete.action"),
        );
        if (!confirmed) {
                return undefined;
        }
        return runVerb(context, ["delete", context.ref, "--yes"]);
}
```

The `isRow` call at the top of `deleteAttachment` is what narrows `element`, so `element.view` is read with no type assertion. `isRow` is a type predicate (`cardCommands.ts:139-144`), and an `Extract<...>` cast in its place would keep compiling if `contextForAttachment`'s own guard were ever loosened, which is the failure the narrowing exists to prevent.

`view.ref` is what the confirmation shows and `context.ref` is what the argv carries.

The resulting argv, after `pinnedArgv` and `composeArgv`, is exactly:

```
["--json", "--workbench", <root>, "delete", "<owner>/attachments/<12-hex id>", "--yes"]
```

### The registration, in `editors/vscode/src/extension.ts`

Beside the existing `COMMAND_OPEN_ATTACHMENT` registration at `extension.ts:1044-1047`, and for the same reason its comment gives:

```ts
register(COMMAND_DELETE_ATTACHMENT, async (element: TreeElement | undefined) => {
        await deleteAttachment(
                element,
                binary.state === "ok" ? binary.path : "",
                host,
                (line) => channel.appendLine(line),
        );
});
```

### The manifest, in `editors/vscode/package.json`

One command, declared after `dinah.tree.openAttachment`:

```json
{
  "command": "dinah.tree.deleteAttachment",
  "title": "%manifest.command.dinah.tree.deleteAttachment.title%"
}
```

One `commandPalette` entry, after the `openAttachment` one:

```json
{
  "command": "dinah.tree.deleteAttachment",
  "when": "false"
}
```

One `view/item/context` entry, appended to that array:

```json
{
  "command": "dinah.tree.deleteAttachment",
  "when": "view == dinah.workbenchView && viewItem == dinah.attachment",
  "group": "1_attachment@1"
}
```

The group name `1_attachment` is new. Reading the whole array rather than the entries I had open, `python -c "import json;print(sorted({e['group'] for e in json.load(open('editors/vscode/package.json',encoding='utf-8'))['contributes']['menus']['view/item/context']}))"` answers `1_column@1..4`, `1_flow@1..3`, `1_workbench@1..4`, `2_attachment@1`, `2_view@1..2`, and `3_copy@1`, so nothing there begins `1_attachment`. `2_attachment@1` is Attach File on a card row, and the two never appear on one row, but a distinct namespace keeps the menu readable and it makes `1_attachment@1` a selector naming exactly one entry, which AC-2 relies on. The clause is an equality rather than a regex, because the attachment row carries one value and has no state axis to widen over.

### The test helpers this card mints

Two criteria need helpers the repository does not have, so they are spelled here rather than left to the implementer.

In `editors/vscode/test/unit/manifest.test.ts`:

```ts
/**
 * The `when` clause of the one view/item/context item in a menu group,
 * whatever command it names.
 *
 * attachClauseFor (manifest.test.ts:1603-1613) answers the same question for
 * Attach File alone, because that command holds three entries in three
 * groups. A group holding exactly one item needs no command filter, and
 * filtering on the command under test would ask the manifest to confirm what
 * the test already assumed.
 */
function soleClauseFor(group: string): string {
        const menus = contributes.menus as Record<
                string,
                { command: string; when: string; group: string }[]
        >;
        const matched = menus["view/item/context"].filter((entry) => entry.group === group);
        assert.equal(matched.length, 1, `${group} holds ${matched.length} items`);
        return matched[0].when;
}
```

In `editors/vscode/test/unit/tree.test.ts`:

```ts
/** A workbench-scoped attachments listing, for the group whose own ref is "". */
const WORKBENCH_ATTACHMENTS: AttachmentListing = {
        kind: "workbench",
        ref: "workbench",
        attachments: [
                {
                        id: "aa11bb22cc33",
                        ordinal: 1,
                        ref: "workbench/attachments/1",
                        filename: "policy.md",
                        provenance: "copy",
                        path: "C:\\customers\\carter\\board\\attachments\\policy.md",
                },
        ],
};

/**
 * A forest holding one member that reports attachments.
 *
 * The forest shape is what makes this fixture distinguishing. The row is
 * drawn for the folder C:\customers while the member's own workbench root is
 * C:\customers\carter\board, so an element taking its root from row.folder
 * and one taking it from the group disagree. attachingBench() cannot tell
 * the two apart, because every path in it is C:\work\bench.
 */
async function forestAttachingBench(): Promise<DinahTreeProvider> {
        const spawner: Spawner = async (_exe, argv) => {
                if (argv.includes("attachments")) {
                        return ok(WORKBENCH_ATTACHMENTS);
                }
                const member = {
                        title: "Carter LLP",
                        slug: "carter",
                        path: "C:\\customers\\carter\\board",
                        ...(argv.includes("tree")
                                ? { tree: THREE_COLUMNS }
                                : argv.includes("status")
                                        ? { status: ATTACHING_STATUS }
                                        : { listing: THREE_LISTING }),
                };
                return ok({ root: "C:\\customers", workbenches: [member] });
        };
        const view = provider(spawner);
        await view.load([folder({ folder: "C:\\customers", resolution: NOTHING })]);
        return view;
}
```

The member spelling is the one `tree.test.ts:1567-1587` already uses for a forest member driven with `ATTACHING_STATUS`, so this is a variation on a shape the file exercises rather than a new one. The `attachments` arm is tested before the three root-scoped verbs because `forestSpawner` (`tree.test.ts:838-857`) answers every argv with a forest envelope, which `readAttachments` cannot parse.

### The strings

Manifest catalogue key `manifest.command.dinah.tree.deleteAttachment.title`.

| Catalogue | Value |
|---|---|
| `package.nls.json` | `Delete...` |
| `package.nls.de.json` | `Löschen...` |
| `package.nls.hi.json` | `हटाएँ...` |
| `package.nls.af.json`, `.cs.json`, `.es.json`, `.fil.json`, `.id.json` | `Delete...` |

The title is bare rather than `Dinah: ` prefixed, because a row command the palette never shows would be saying the product's name to nobody, and it ends in `...` because it asks before it acts. That is the convention `Move...`, `Block...`, `New Card...`, and `Attach File...` already carry, held by `manifest.test.ts:1626`.

Runtime catalogue keys, added to `editors/vscode/src/locales/en.json` and its seven siblings.

`dialog.attachment.delete.confirm`

- text: `Delete {filename} ({ref})? Dinah destroys the file and cannot bring it back.`
- context: `The modal shown before an attachment is deleted from the sidebar. {filename} is the attachment's filename, which is also the row's label; {ref} is the address a person would type to reach it.`
- de: `{filename} ({ref}) löschen? Dinah zerstört die Datei und kann sie nicht wiederherstellen.`
- hi: `{filename} ({ref}) हटाएँ? Dinah फ़ाइल को नष्ट कर देता है और उसे वापस नहीं ला सकता।`

`dialog.attachment.delete.action`

- text: `Delete`
- context: `The affirmative button of that modal. The editor supplies Cancel itself.`
- de: `Löschen`
- hi: `हटाएँ`

The five skeleton languages carry the English text byte for byte with `"skeleton": true`, and carry no `source`. German and Hindi carry no `skeleton` and carry `source` equal to `fingerprint(<the English text above>)`, computed with `editors/vscode/src/fingerprint.ts`. `src/locales/flags.json` gains the manifest key in each of the five `skeleton` arrays and a `source` entry under both `de` and `hi`; it gains nothing for the two runtime keys, because a runtime entry carries its own flags.

The name `Dinah` stays in Latin script in both translations, per the workbench's prose standard. Neither string contains any term the glossary in `flags.json` declares.

Both keys take the `dialog.<subject>.<act>.<part>` shape that the catalogue's other four-segment `dialog.` keys already use. That is a claim about precedent rather than about universality, because the catalogue is not uniformly four-segment: `status.ambiguous` and its two-segment siblings are the counterexample, and anyone wanting the distribution runs `python -c "import json,collections;print(collections.Counter(k.count('.')+1 for k in json.load(open('editors/vscode/src/locales/en.json',encoding='utf-8'))['entries']))"` rather than reading a figure from this paragraph. The workbench's DIN-HIN dash convention governs Go package names, CLI verb names, and the contract's own tokens; no key in either extension catalogue follows it, and inventing the first one here would put one schema in two shapes.

## The refresh path, stated exactly

`runVerb` calls `context.host.checkpoint(context.folder)` after the spawn, whatever the outcome. `checkpoint` is bound in `extension.ts:928` to `checkpointing.checkNow(folder)`, which runs `CheckpointLoop.check` ignoring the debounce and the throttle. That call asks `dinah changes --since <cursor>`; because the deletion appended `attachment_removed` to the owning entity's journal, the live digest term has moved and the answer carries `changed: true`; `check` then awaits `refresh(folder)` and calls `fire()`.

The row ships on three kinds of owner, and one mechanism carries all three. `attachmentItem` sets `contextValue` unconditionally, and the tree draws attachment groups for the workbench and for columns as well as for cards, so a Delete run from a workbench group has to move the digest too. It does. `journalFor` (`internal/verb/beyond.go:635-643`) sends an event about an entity with no enclosing card to the bench's own journal, and `WatchedEntities` (`internal/bench/changes.go:58`) puts the bench journal into the live digest term ahead of the workstreams and the cards. AC-10 pins the card case, which is the one whose scope a wrong `journalFor` would silently widen, and the other two ride the same digest term by construction.

That one call reaches both things the removal changed.

- The card's own row: `refresh` re-reads the workbench, so the card's `attachment_count` comes back from a fresh `bench.CountAttachments` over the directory the attachment no longer sits in. The row's expand arrow is driven by that count (`tree.ts:783-789`), so a card whose last attachment was deleted reports no count at all, since the field is `omitempty`, and collapses to `collapsibleState: "none"`.
- The group's contents: `fire()` invalidates the tree, and the `attachmentsGroup` arm of `getChildren` calls `readAttachments` on every expansion with no cache between, so the redrawn group is a fresh listing.

Nothing else is needed. In particular the command does not read the delete's receipt back into the tree, which is the rule `cardCommands.ts`'s header states for every mutating command.

## Out of scope

**Archive is not offered.** The row gets Delete alone. Archiving an attachment works, and `cards/<id>/archive/attachments/<id>/` is a frozen compatibility path in `cmd/dinah/compat_test.go:62`, but no verb undoes it: `unwrittenEvents` at `cmd/dinah/compat_test.go:128-130` carries `contract.EventRestored` with the written reason that "archive has no inverse verb in the command surface, so nothing restores an entity", and the case-insensitive sweep of the two files that declare the surfaces confirms that is still true at 178e264. A menu offering Archive would therefore offer a reader a second irreversible act while implying it is the recoverable one, and recovery would mean moving a directory by hand in a layout the reader has never seen. If a restore verb is ever added, adding Archive here is a small card against a real undo.

**No mockup.** The card contributes one context-menu entry to an existing row and one modal whose chrome, button placement, and dismissal the editor owns entirely. A hand-drawn approximation would show what VS Code draws rather than what this card decides.

**No integration test.** `editors/vscode/test/integration/` starts a real editor host, and the workbench forbids running that suite against the operator's editor. Whether VS Code shows the menu item and whether the modal renders are the editor's promises; `src/registrationGuard.ts`'s own header already says no test in this repository settles them.

**No change to any CLI verb, MCP tool, wire field, or Go product code.** The one Go file this card touches is a new test.

## The files that change

| File | Change |
|---|---|
| `editors/vscode/src/identity.ts` | `COMMAND_DELETE_ATTACHMENT`, `CONTEXT_ATTACHMENT`, and both roster arrays |
| `editors/vscode/src/wire.ts` | `ATTACHMENTS_SEGMENT` |
| `editors/vscode/src/tree.ts` | `root` and `owner` on the attachment element, filled in `getChildren`; `contextValue` on the row |
| `editors/vscode/src/cardCommands.ts` | `confirmDestructive` on `CommandHost`; `contextForAttachment`; `deleteAttachment` |
| `editors/vscode/src/extension.ts` | the `confirmDestructive` binding and the command registration |
| `editors/vscode/package.json` | one command, one `commandPalette` entry, one `view/item/context` entry |
| `editors/vscode/package.nls.json` and seven siblings | one manifest key |
| `editors/vscode/src/locales/en.json` and seven siblings | two runtime keys |
| `editors/vscode/src/locales/flags.json` | five skeleton entries and two source entries for the manifest key |
| `editors/vscode/test/unit/l10n-coverage.test.ts` | a `confirmDestructive` branch in the audit, and its own fixture-driven test |
| `editors/vscode/test/fixtures/confirmDestructive-call-site.ts.txt` | new; the audit's own arming fixture |
| `editors/vscode/test/unit/manifest.test.ts` | `soleClauseFor` and the two menu criteria |
| `editors/vscode/test/unit/tree.test.ts` | `WORKBENCH_ATTACHMENTS`, `forestAttachingBench`, and the two row criteria |
| `editors/vscode/test/unit/cardCommands.test.ts` | the four command criteria |
| `internal/verb/changes_test.go` | one regression test; `grep -c ttachment` over that file answers 0 today |

## Branch

dinah-451-an-attachment-can-be-added-from-the-sidebar-and-never-removed-from-it
