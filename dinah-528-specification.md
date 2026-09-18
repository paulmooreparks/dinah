---
card: dinah-528
station: Spec
---

# A collection listing serves an index for comments and checklist items, and the tree walk serves everything else

## 1. What this contract covers

`dinah list <card>/comments` and `dinah list <card>/checklist` currently route through the containment tree walk, which reads every member's anchor file and produces a `Tree` whose nodes carry the first line of each member's text as a title. On dinah-514, the checklist listing costs 44,028 bytes in JSON and 39,618 bytes on the terminal, because a checklist item's first line can be hundreds of words and every one of them travels in full. A station asking "which items are still pending?" pays for all of that on every call, and the only way to answer the question is to read every item's own reference in a second round.

This contract gives those two collections the same treatment dinah-527 gave the `comments` and `checklist` members of `show <card>`: an index carrying what a reader needs to choose, with the member's own reference serving the rest. Two filters travel with it, `--since` and `--unresolved`, answering the same questions the `show` filters answer but on the `list` verb.

Every other collection reference keeps the tree walk. `dinah list <card>/attachments` and `dinah list <workbench>/columns` and every other walk continue to produce the `ShapeContents` tree they produce today.

## 2. What the surface does today, and what it costs

Every line number here was read on `origin/main` at `d6d4ab34`, which is the trunk as of this writing.

`ListRef` at `internal/verb/list.go:349` routes a collection reference to the containment walk. The branch at line 385 checks `collection != nil`, refuses `--depth` and `--archived` for collections that are not attachments (line 386), and falls through to `listContents` for everything else (line 392). `collectionContents` at `internal/verb/tree.go:980` builds a tree whose root carries `Kind: "collection"` and whose children each have `Kind`, `ID`, `Ref`, `Title`, and `Count`.

`containedNode` at `tree.go:1317` fills `Title` by calling `anchorTitle` at line 1333. `anchorTitle` at line 1407 reads the full anchor file through `bench.ReadText`, parses the frontmatter, and returns the first value it finds among `title`, `text`, `description`, and `filename`, or the first non-empty line of the body if none of those carry a value. For a comment, the frontmatter has `author` and `ts` but no `title`, so `anchorTitle` returns the first line of the body. For a checklist item, the frontmatter has `kind`, `state`, and `column` but no `title`, so `anchorTitle` returns the first line of the text field.

The cost is the first line carried in full. On dinah-514, which holds 30 comments and 65 checklist items:

```
dinah list dinah-514/comments --json                    8,198 bytes
dinah list dinah-514/checklist --json                   44,028 bytes
dinah list dinah-514/comments (terminal)                 5,951 bytes
dinah list dinah-514/checklist (terminal)               39,618 bytes

the show index for comparison:
dinah show dinah-514 --fields comments                   5,198 bytes
dinah show dinah-514 --fields checklist                 28,849 bytes
```

The checklist listing is more expensive than the show index because the tree node carries the whole first line as `title` where the show index caps it at 120 runes, and because the tree structure itself adds JSON nesting and the `count`, `id`, and `kind` fields per node.

The `collectionListing` function at `read.go:1605` is dead code. It was the `show` verb's collection reader before dinah-523 retired it, and it reads each member's full anchor text into `Text`. Its doc comment at line 1595 says the same thing the card's framing says: "a collection listing is the anchor text rather than a promise the two reads agree." No caller reaches it, and this contract removes it.

## 3. The listing surface after this card

### 3.1 Two paths, not one

A collection reference whose kind is `comment` or `item` routes to a dedicated listing function that builds an index. Every other collection reference, including `attachment` and `column`, routes to the containment tree walk exactly as it does today. The decision is made in `ListRef` after the resolver returns a `CollectionRef`, which is where the attachment shortcut at line 389 already branches.

The dedicated listing replaces the tree walk for comments and checklist items. The comment listing accepts `--since` and the item listing accepts `--unresolved`; both also accept `--depth` and `--archived`, which route to the existing containment tree walk. The tree walk never receives either filter, because a containment tree is not a collection of items with an ordinal or a state.

### 3.2 The comment index

A `dinah list <card>/comments` that routes to the dedicated listing produces a `CommentListing` carrying one `CommentIndexEntry` per comment, in ordinal order:

```go
type CommentIndexEntry struct {
    ID      string `json:"id"`
    Ordinal int    `json:"ordinal"`
    Ref     string `json:"ref"`
    TS      string `json:"ts"`
    Author  string `json:"author"`
    Subject string `json:"subject"`
    Size    int    `json:"size"`
    Body    string `json:"body"`
}
```

Every field is the same as the identically named field on `CommentView` in `show`, and is filled from the same source. `Subject` is `subjectOf` applied to the comment's body, and `Size` is `len(comment.Body)`, both computed the same way `commentViews` at `read.go:1380` computes them. `Body` is the empty string on every entry the listing serves as an index, and is filled from `comment.Body` on entries after the `--since` ordinal, exactly the way `detailOf` fills it on the show side. A comment's attachments do not travel in the listing, because a comment's attachments cost what the one card costs and are reachable through the comment's own reference.

`CommentListing` carries the collection's reference and kind:

```go
type CommentListing struct {
    Ref      string              `json:"ref"`
    Kind     string              `json:"kind"`
    Members  []CommentIndexEntry `json:"members"`
    Archived bool                `json:"archived,omitempty"`
}
```

The shape mirrors `CollectionListing` for the wrapper and `CommentView` for each entry, with the body empty on index entries and attachments removed. A machine reader sees:

```json
{
  "ref": "dinah-514/comments",
  "kind": "comment",
  "members": [
    {
      "id": "2f2c486e28db",
      "ordinal": 1,
      "ref": "dinah-514/comments/1",
      "ts": "2026-09-15T08:21:44Z",
      "author": "devin",
      "subject": "Independently verified from dinah-305's Agent Design Review, and the anchor measurements reproduce exactly.",
      "size": 2841
    }
  ]
}
```

The terminal renderer draws a table with one row per entry:

```
dinah-514/comments contains 30 comments.
  Reference              When                  Who     Subject                                  Size
  ---------------------  --------------------  ------  ---------------------------------------  -----
  dinah-514/comments/1   2026-09-15T08:21:44Z  devin   Independently verified from ...          2841
```

The columns are the same six that the comment index in `show <card>` draws: Reference, When, Who, Subject, Size, in the same order. The header sentence names the collection and the count, on the pattern the tree header already uses: `<ref> contains <count> comments.`

### 3.3 The checklist index

A `dinah list <card>/checklist` that routes to the dedicated listing produces an `ItemListing` carrying one `ItemIndexEntry` per checklist item, in creation order:

```go
type ItemIndexEntry struct {
    ID           string `json:"id"`
    Ordinal      int    `json:"ordinal"`
    Ref          string `json:"ref"`
    Kind         string `json:"kind"`
    State        string `json:"state"`
    Column       string `json:"column,omitempty"`
    ColumnTitle  string `json:"column_title,omitempty"`
    Owner        string `json:"owner,omitempty"`
    Text         string `json:"text"`
    CommentCount int    `json:"comment_count,omitempty"`
}
```

Every field is the same as the identically named field on `ItemView` in `show`. `Text` carries the item's first line capped at 120 runes by `capRunes(firstLine(item.Text), subjectCap)`, which is the same cap the show index applies. The listing has no `Note` field: it is an index, and an item's own reference recovers its resolution note.

`ItemListing` carries the same wrapper as the comment listing:

```go
type ItemListing struct {
    Ref      string             `json:"ref"`
    Kind     string             `json:"kind"`
    Members  []ItemIndexEntry   `json:"members"`
    Archived bool               `json:"archived,omitempty"`
}
```

The terminal renderer draws a table whose columns are the same as the checklist table in `show <card>`, plus a `Size` column showing the byte count of the item's text for the same reason the comment listing shows a comment's size:

```
dinah-514/checklist contains 65 items.
  Reference               Kind                 State       Column  Owner   Text                                #
  -----------------------  -------------------  ----------  ------  ------  ---------------------------------  ---
  dinah-514/criteria/1    acceptance_criterion  resolved    Merge           A body whose lines end CRLF, ...  0
```

The `#` column heading and value are the comment count, which is what the show table already draws for the checklist. `Size` is not a column in the terminal table for items, because a reader scanning the table is choosing by kind, state, and text rather than by item size. The JSON carries `Text` at the cap rather than a separate `Size`, because a machine reader pricing a round trip needs to know how much it would recover, and the cap already bounds the text field.

### 3.4 `--since <ordinal>` on comments

`--since` on `dinah list <card>/comments` serves the same question dinah-527's `--since` serves on `show <card>`: what happened after the last ordinal I saw? It carries the same semantics, is refused under `dinah.usage` beside a reference that is not a comments collection, and is refused beside `comments.full` for the same reason section 3.7 of dinah-527's contract gives.

The flag is available on `dinah list` and on `dinah show`, and the two are not combined: `dinah show <card> --since 4` and `dinah list <card>/comments --since 4` answer the same question about the same collection through different verbs.

On the `list` side, the refusal is raised in `ListRef` before any resolution, the same way the `show` side raises its refusal in `Show` before resolving anything. The flag reads `--since` on both verbs, the value name is `ordinal` on both, and the parameter is `SinceComment` in `Request` on both, because the `list` verb already declares its own `--since` binding that value to the same field.

### 3.5 `--unresolved` on checklist items

`--unresolved` on `dinah list <card>/checklist` serves the same question dinah-527's `--unresolved` serves on `show <card>`: what still holds this card? It carries the same semantics, filtering by `bench.ItemLiftsColumnHold`, and is refused under `dinah.usage` beside a reference that is not a checklist collection.

The flag is available on `dinah list` and on `dinah show`. On the `list` side, `--unresolved` beside `--depth` is refused, because a containment walk and an item filter do not compose. `--unresolved` beside `--archived` is refused, because an item in the archive half is not holding any card. `--unresolved` beside any other collection reference is refused, because only checklist items have state.

### 3.6 `comments.full` and `checklist.full` on the listing

The listing does not carry a modifier. A reader who wants the full body of a comment asks for that comment by its reference (`dinah show <card>/comments/<n>`), and a reader who wants the full text of an item asks for the item by its reference (`dinah show <item>`). The `show` verb already serves both, and the listing is the index that tells the reader which reference to ask for.

A `.full` name on `dinah list` would therefore answer the same question the reader has already answered by reaching for the collection's members in full, and the index is what saves the reader from doing that. The two modifiers stay on `show`, where they replace the index with the full body on the card's own read.

### 3.7 The `collectionListing` function is removed

`collectionListing` at `read.go:1605` and `CollectionMember` at `read.go:1064` have no caller. They served the `show` verb's collection read before dinah-523 retired it into `list`, and `list` never reached them. This contract removes both and the `CollectionListing` type at `read.go:1045`.

## 4. What changes, file by file

### 4.1 `internal/verb/list.go`

`ListRef` gains a branch after the collection check at line 385. Where the collection's mount kind is `comment` or `item`, the function delegates to one of two new library methods rather than falling through to `listContents`. The two flags `--since` and `--unresolved` are accepted on the comment and item collection branches and refused on every other shape, in the same place the existing flag checks already run.

The branch for comments accepts `--since`, `--depth`, and `--archived`, and refuses `--ready` and `--unresolved`. The branch for items accepts `--unresolved`, `--depth`, and `--archived`, and refuses `--ready` and `--since`. A depth or archive request routes either collection to the existing containment tree walk; either filter beside one is refused because the two shapes do not compose. A collection reference whose mount kind is neither accepts `--depth` and `--archived` and refuses the two filters.

Two new `ListShape` constants are declared: `ShapeComments` and `ShapeItems`. Two new members are added to `ListResult`: `Comments *CommentListing` and `Items *ItemListing`. `Answer` and `MarshalJSON` each gain an arm for the two new shapes.

The vocabulary table at `definition.go` gains `--since` and `--unresolved` as parameters on the `list` entry, where `--since` names `SinceComment` and `--unresolved` is a marker. The two parameters already exist on `show` and bind to the same fields in `Request`, so no new `Request` field is needed.

### 4.2 `internal/verb/read.go`

- `collectionListing` at line 1605, `CollectionListing` at line 1045, and `CollectionMember` at line 1064 are removed.
- Two new listing methods are added: `commentListing` builds a `CommentListing` from a `CollectionRef`, and `itemListing` builds an `ItemListing` from a `CollectionRef`. Both reuse the same `commentViews` and item-construction logic that `detailOf` uses, with the body and attachments cleared, the same way the show index clears them.
- The `subjectOf` function and the `capRunes` function are already in this file from dinah-527 and are shared by both paths.

### 4.3 `internal/verb/tree.go`

No changes. The tree walk keeps its current shape and continues to serve every other collection. Comments and items that appear as children in a tree walk still draw their `Title` from `anchorTitle`, because a tree walk is a containment listing, not a collection listing, and a comment buried in a walk rooted at a card or a column is not the same read as a comment collection listed on its own.

### 4.4 `cmd/dinah/render.go`

Two new renderers are added: `renderCommentListing` draws the six-column table described in section 3.2, and `renderItemListing` draws the table described in section 3.3. The `renderTree` function is unchanged.

### 4.5 `cmd/dinah/commands.go`

The `list` command's result switch gains two arms, one for `ShapeComments` calling `renderCommentListing` and one for `ShapeItems` calling `renderItemListing`.

### 4.6 `internal/msg/locales/`

Four new keys land in all eight catalogues:

```
column.listing-comments.who
column.listing-comments.size
column.listing-items.kind
column.listing-items.comment-count
```

The comment listing reuses the `when`, `author`, `subject`, and `size` column keys the show table already declares, and adds a `who` key because the table header says "Who" while the show table says "Author". The item listing reuses `state`, `owner`, and `text` from the show table and adds `kind` and `comment-count` because those are new columns in the listing.

### 4.7 `internal/guide/guides/mcp.md`

The section on `list` gains a paragraph describing the two dedicated listings and their flags, parallel to what dinah-527's contract added for `show`.

### 4.8 `docs/design/token-cost.md`

A new dated section records this card's paired runs, per the workstream's standing bar: the run must show `list` as the only row that moved under `--per-tool`, and the measured block delta multiplied by the session's rounds must stay below the cumulative saving the same run reports.

## 5. What this card does not do

- It does not change `dinah show <card>` or any of the show-side shapes dinah-527 introduced. Those are this card's prior art, not its surface.
- It does not change what `dinah show <card>/comments/<n>` or `dinah show <item>` serves. Those are the recovery paths.
- It does not change the containment tree walk. Every collection reference that is not `comments` or `checklist` routes to the tree walk and produces the same `ShapeContents` it produces today.
- It does not add a `.full` modifier to `dinah list`. The listing is the index; the recovery path is the member's own reference on `show`.
- It does not touch the journal or the archive half.
- It does not convert a card or rewrite a comment. Nothing on disk changes.

## 6. dinah-523, the merge, and what the implementer reads

dinah-523 is ahead of this card on the trunk. Every line number in sections 2 and 4 is to be read again on the trunk before implementation starts, because dinah-523 and dinah-527 have both landed since the card was filed and line numbers shift. The implementer reads the trunk before starting and resolves any merge conflict at Implement time.

## 7. The measurement this card owes

Every surface change in this workstream carries its own measurement. The paired run records the `--per-tool` block delta for `list` before and after the change, and the savings are the difference between the current `dinah list dinah-514/checklist --json` (44,028 bytes) and the new listing's output. The bar in `docs/design/token-cost.md` applies: the run must show `list` as the only row that moved under `--per-tool`, and the measured block delta multiplied by the session's rounds must stay below the cumulative saving the same run reports.

The run also records the new listing's byte count for both `comments` and `checklist` on the same card, so the derived figures in this contract can be checked against the built thing.
