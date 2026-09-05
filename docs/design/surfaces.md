# The Dinah surfaces

A note for readers arriving fresh: "the hosted product" cited throughout is
Dinah.Team, which is multi-seat and runs on Dinah's own library rather than on
a second codebase written to the same contract. Andoneer, cited later in this
document, is the separately-coded interface Dinah was spun out of.

This document captures the architecture of Dinah's implementation and the
surfaces exposed over it, as agreed in discussion. It is the companion to
`format.md`, which owns the on-disk format; the boundary between the two is
that format.md says what is true on disk and this document says who may ask
and how. The CLI verb set itself is still under design and is recorded here
as it settles.

## Library-first, one binary, many heads

The real implementation is a library: the workbench store, the verbs, check,
the token registry, the locale catalogs. Every user-facing surface is a thin
head over that library, and all the heads live in one binary as subcommands:
the CLI verbs, an MCP server (`dinah mcp`, stdio), an HTTP server (`dinah
serve`), and an LSP (`dinah lsp`). There is exactly one implementation of
every verb no matter which protocol asked.

The cautionary tale is git, which was never embeddable, so the ecosystem
reimplemented it (libgit2 for embedders, JGit for the JVM), each lagging
canonical git for years, while GUIs shelled out and scraped output. Dinah
wants independent implementations of the contract; it does not want its own
CLI, MCP, HTTP, and LSP surfaces to be four of them.

The language is Go: static single binary, trivial cross-compilation, stdlib
HTTP, cheap concurrency for the servers, and language coherence with the
hosted product (one brain maintains both; the conformance suite pins Dinah's
own behaviour against what this document and the profile say the contract
requires, and keeps that contract specific enough for an independent
implementer to build against if one is ever wanted).

## The machine contract is the verb plus JSON

Git's scriptable surface was never "the CLI" generically but plumbing
commands with frozen output contracts. Dinah has an equivalent. Every verb
has a canonical JSON form, canonical tokens only, carrying the three-outcome
vocabulary from format.md (refused, stale, unreachable, kept distinct in
exit codes and payloads). That JSON contract is the frozen surface. The MCP
tools and HTTP routes are thin mappings of it, and the human-facing CLI
output is a rendering of it through the locale catalogs.

A verb with nothing to report on success, because it hands control to an
interactive subprocess (`edit`) or answers with documentation text about the
tool itself rather than data about a workbench (`guide`, `help`), still
answers a refusal in the shared machine form and carries no JSON payload on
success because there is nothing to carry. `export`'s payload is already the
interchange document, itself the frozen JSON form, so wrapping it in a second
envelope would add nothing. `config get <key>` and `workbench get <field>`
still write a bare scalar line under `--json`, unlike every other read verb,
because each has a sibling verb with no subcommand, `config` and `workbench`,
whose own JSON form already carries that same field among every other field a
caller might ask for. `config set <key> <value>` answers `--json` with no
payload at all on success; that gap is recorded here rather than closed,
because no caller today depends on it and there is no sibling shape to give
it one.

One verb definition generates four projections: CLI flags and help, MCP tool
schema, HTTP route, and the human rendering. Surfaces cannot drift from one
another because none of them is hand-maintained against the others. Nobody
ever parses human output, so localized rendering breaks no tooling.

## The C ABI is designed for and not shipped

C ABI embedders are a real market and the FFI base for every language, so
the library-first core is kept C-exportable by construction: no exported
surface depends on Go-only semantics that could not cross `extern "C"`. Go
can produce the artifact (`buildmode=c-shared`), with known caveats (the
runtime rides along; callback and thread-pinning friction), so `libdinah`
is an additive later surface, not a rearchitecture. If C-embeddability ever
becomes a primary requirement rather than an additive one, that is the one
argument that would reopen the language decision, and it is named here so
the fork is taken consciously or not at all. The default embedding story is
the process boundary: talk MCP or HTTP to the local binary.

## The GUI boundary

A GUI wrapper is wanted sooner rather than later, and the architecture makes
it cheap: it is one more head over the same verbs, with no new core surface.
The shape that fits the operator's standing principles is a local,
server-rendered web UI (`dinah ui`: the binary serves localhost and opens
a browser), URL-is-king, no SPA framework, embedded static assets, which is
the same stack discipline as the hosted product.

The boundary that keeps it from competing with the hosted product is the
gitk precedent. Git ships gitk, a workmanlike local viewer of one
repository, and GitHub lost nothing to it, because the products answer
different questions. The Dinah GUI is a single-workbench, single-seat view:
walk the board, read cards, perform the verbs. The portfolio view, the
multi-user board, live coordination between seats, analytics, and operator
surfaces across many workbenches are the hosted product, and the GUI does not
grow toward them. Where the GUI's ceiling is reached, the answer is the
upgrade path, not a bigger GUI.

## Onboarding: guidance is served, templates are instantiated

An agent that has the binary and the MCP wiring must be able to build a
rich, fully-featured workbench from the word go, and that implies a lot of
instruction text. The rule that governs all of it is inherited from the
hosted product's hardest-won lesson: guidance is served live from the
binary, so an upgrade updates every agent at once, and is never seeded
into storage; templates are the opposite, deliberately copied at creation
and owned by the workbench thereafter, because a starting point that keeps
moving under its owner is not a starting point. Nothing uses the
write-once-then-orphaned middle shape.

Guidance reaches an agent through four channels. The MCP initialize
response's instructions field carries the working agreement and
orientation, delivered before an agent's first tool call. Verb descriptions
and response payloads carry per-verb method and next-action hints, and that
is where most agent behavior is shaped. Embedded guides (workbench authoring,
instruction-writing conventions, operator-station discipline) are served on
demand by a guide verb and as MCP resources. Shipped templates are complete
workbench definitions embedded in the binary, instantiated by `dinah init`;
every shipped template must pass check and the conformance suite in CI, so
the scaffold is provably legal, and the experiment workbenches are the first
drafts. The machine-surface rule holds throughout: served guidance is
content an agent reads, but the tokens inside examples stay canonical.

### Translation

Localization has three layers with different contributors and different
mechanics.

Token display names live in one plain YAML file per BCP 47 tag under
locales/, with flat keys derived from the token registry, no toolchain, and
editing in a browser. A generator emits the skeleton (every key, the
English value, and a context comment explaining the token), a coverage
command reports what is missing, and CI validates keys against the
registry, so the only human review a catalog needs is a native reader
judging wording.

Message templates carry named placeholders only, never positional, so a
translator may reorder a sentence around its parameters; plurals follow
CLDR categories with per-category keys, which Czech makes non-optional
from the first release; messages are designed to avoid count-dependence
where possible so the category cost is paid only where counts appear.

Long prose (guides, shipped template instructions) is translated as whole
documents under a per-tag directory, falling back per-document to
English, because document-level translation produces readable prose and a
stitched sentence-catalog essay does not. Regional overlays and per-token
fallback apply to the first two layers as already specified in the format
document.

Dinah's own name sits outside this. The product name stays in Latin
script in every language and every layer above, and a translator never
transliterates it into a target script. A catalog entry that names the
product renders every surrounding word and leaves the name as it is. The
name is said DYE-nuh, and a document introducing the product to a new
reader may gloss the pronunciation beside the Latin spelling; that gloss
belongs to documentation and does not license a respelled name inside a
catalog string.

### Template libraries by URL

A template is just a workbench directory, so the on-disk format is already the
template format and a library is a repository of workbench directories:
`dinah init --from <git-url>[//subdir][@ref]` clones, validates, and
instantiates, and a direct archive URL works the same way. Shortnames
resolve against embedded templates first, then against `template_sources:`
repos configured in the user config (how an organization points everyone
at an internal library), then explicit URLs; nothing is ever auto-fetched.

Trust is the design work, because a template's column instructions are
prompts that agents will obey, making a malicious template an injection
vector with a delivery mechanism. Remote templates are pinned by ref or
sha and shown to the operator (column list and instruction summaries)
before instantiation. Instantiate-and-own caps the blast radius: nothing
upstream can mutate a workbench already owned. Provenance (source and sha) is
recorded in the new workbench's workbench.md as display-tier fact. A template
repository runs check and the conformance suite in CI, so contributed
templates are machine-checked before a human reads them. After locale
catalogs, they are the second well-shaped community contribution.

Extraction closes the authoring loop: a command copies the definition out
of a live workbench (workbench.md and columns/, keeping their identifiers) and
leaves the work (cards, workstreams, archive, journals), so nobody authors
a template from scratch; a workbench that already works gets promoted. Kept
identifiers mean intra-definition references survive untouched and workbenches
born of one template stay structurally comparable. The round trip,
extract(instantiate(T)) equals T, is a conformance property once key order
is canonical. Extraction is mechanical: instance details inside
instruction prose travel with the template, and scrubbing them is the
author's editorial pass, with a lint that flags suspicious fragments as a
candidate, never a gate.

## The VS Code extension

The workbench is made of Markdown files, so VS Code is already half a client,
and the extension's ladder builds on that rather than on a webview. The
first rung is the LSP plus verbs: registry-driven validation and
completion in anchors, and CodeLens actions on a card file (claim, move,
release) that shell to the CLI and refresh from the receipt. The second
rung is the sidebar tree the operator asked for by name: workbenches at the
top (from the same discovery walk and user base the CLI uses), columns
beneath in definition order, cards beneath those grouped by state,
every node opening its anchor on click because every node is a file.
State rides the tree's badge-and-color decorations, a WIP-limited
column shows its count against the limit, and a node's context menu is the
affordances block rendered as menu items, so illegal actions are absent
from the representation here exactly as they are everywhere else. The
in-binary TUI runs in the integrated terminal at every rung for free, and
the webview rung comes last, embedding the server-rendered ui pages once
they exist. The extension speaks to the binary over the machine surfaces
and never parses human output.

## Extension processing

The format side of extensions is declaration (dot-named kinds in the workbench
definition); the behavior side is a ladder, and each rung is machinery the
design already owns. Rung zero is free: the uniform entity shape lets the
CLI create, list, show, archive, journal, and structurally check any
declared kind with no kind-specific code. Rung one is instructions: what
an extension entity means, and when agents act on it, is method text in
the workbench, with no CLI involvement. Rung two is the deferred hook design
generalized to entity lifecycle: when determinism is wanted, an on-event
hook runs a command with the entity's JSON on stdin. Rung three is git's
external-subcommand convention: `dinah acme-report` dispatches to a
`dinah-acme-report` binary found on PATH, a companion process speaking the
frozen canonical-JSON verb contract like any other client, which makes
third-party tooling first-class without touching the core.

The refusal above the ladder is permanent: no in-process plugins, no
dynamic loading, no embedded scripting runtime. The single binary is a
security boundary (a plugin runtime is a supply-chain door), a stability
boundary (in-process extension couples to internals), and the process
boundary plus the JSON contract already serves extenders better.

## External-system wrappers: the Jira shape

A workbench can wrap an external system of record without synchronizing with
it. The workbench models the honest method; the external system holds the
corporate ceremony; and they meet at two narrow edges with asymmetric
authority. The workbench is truth for coordination (claims, method position,
WIP); the external system is truth for the corporate record. Workbench
events project outward (transitions performed, comments posted, fields
filled, each at the workbench moment a mapping declares, outbound writes
happening under the workbench's operator-owned approval gates). External
changes surface inward as flags and journaled events, never as automatic
card moves; the operator decides what the method does about them. Two-way
auto-sync is two arbiters and is refused.

The process arcana are encoded once, in the mapping, and compliance becomes
a side effect of working the method. The wrapper acts as an ordinary API
user, so it needs no administrator and no workflow changes on the wrapped
system. The core never learns the wrapped system's vocabulary; everything
lives in instructions and domain fields, so the same workbench minus the
mapping runs the identical method where the wrapped system does not exist.

The first cut needs no new mechanism: the agent working the card performs
the projections per column instructions, through whatever tool surface it
already has for the external system. Deterministic per-column hooks
(on-enter and on-exit commands fed the card's JSON) are the later candidate
for mechanical projections, held out of the first cut so experience decides
which projections deserve determinism.

## The verb surfaces: first pass

The library owns behavior once. A workbench handle is opened by discovery, and
on it live the reads (resolve, list, serve-instructions, pull-next under
the deterministic ordering) and the mutations (create, claim, release,
move, block, comment, attach, archive), each taking a required actor and
an optional basis, each executing as one transaction (card lock,
write-temp-rename, journal append), and each returning a receipt: what
happened, the resulting position, and the served instructions with the
legal next moves, because the hosted product proved the claim response is
where agent behavior is shaped. Errors are the trichotomy as types
(refused with a coded reason; stale carrying the current revision), never
strings the heads would have to parse. Check and template init/extract
round out the surface.

Two of those mutations sit outside the core profile deliberately. The profile
rules free prose attached to a card, and anything attached to a card as a
file, out of the core, so `comment` and `attach` are this tool's own work
rather than contract obligations. Under the profile they are layer material,
and a second implementation conforms without either of them. The surface is
wider than the contract on purpose: the contract is the part another tool has
to match, and the rest is what makes this one worth using.

`query` is the general read beside the positional ones. It takes one string of
qualifier terms, `column:doing holder:alka at>=2026-08-01` and the like, over a
closed vocabulary of ten field names, and it returns the matching live cards in
the same arrival order every other listing uses. The language is Dinah's own
tool surface rather than contract material, so a second implementation
conforms without it. The CLI takes the string as its one free-text argument and
the MCP tool takes it as a string member. The HTTP head does not exist yet, and
when it is built the recommendation is a GET on the cards collection with the
query riding a parameter named `q`, whose value is character for character the
string the other two heads take; that head's own card rules on it, together
with the status code a refusal maps to.

The CLI is subcommand porcelain over the library: `--json` emits the
frozen canonical form, default output renders through the locale
catalogs, and exit codes carry the trichotomy distinctly, because a
driver loop's control flow hangs on never confusing no, not-yet, and
could-not-ask.

REST follows Fielding with no invented verbs, and the format design
turns out to have converged on REST's own mechanics already: the basis
guard is ETag and If-Match, stale is 412, refused is 403 or 409, and
entity revisions serve as ETags throughout. The claim is a resource, not
an action: POST to a card's claim creates it (409 when held), DELETE
releases it, a lease renewal is a PUT on it. A move is a change to the
card's column and rides PATCH, with a Dinah-defined media type
(application/dinah.move+json) rather than generic merge-patch: the media
type's definition is where the board semantics live in the contract (WIP
refusal on entry, operator-owned stations offering agents no such
request, mandatory If-Match), per Fielding's own instruction that a REST
API spends its descriptive effort on media types and hypermedia. The
pattern generalizes: any mutation whose refusal rules are the interesting
part gets its own small media type; creations and deletions that are
already unambiguous do not. Journals are GET-only, since events are the
server's witness. Every representation lists only the legal next
transitions and names the media type each accepts, so illegal actions are
absent from the representation and legal ones arrive with their paperwork
attached; the contract version rides in the media type.

Two commands read the workbench as a tree rather than as a listing, and
both build the same node so that one renderer draws them and one shape
carries them to every head. `dinah tree` nests the cards along an ordered
chain of axes, taking the query language as its filter, and its
no-argument form is the status tree of columns over states over cards.
`dinah contents` walks the containment grammar down from any entity the
reference resolver reaches, reading a table of what contains what rather
than kind-specific code, so a declared extension kind appears in it with
no line written for it. Every node truncated by depth or thinned by the
filter reports what it is holding back, so no view of the workbench
silently hides work. The sidebar this document describes is that first
tree with its default chain, so the extension renders nodes rather than
reimplementing the grouping.

MCP mirrors the verbs one to one as tools, canonical JSON both ways, the
working agreement in the initialize instructions, guides as resources,
and the same affordances block in every tool response, so an agent on
either machine surface sees the identical statement of what it may do
next.

## Findings from the first hand-executed run

A complete card (the check mini-concept) was run through a five-column
workbench with every verb performed by hand-editing files, three
fresh-context review cycles, and a real operator gate. The run validated the
format's redundancy twice for real (a fabricated citation caught by review;
a frontmatter-versus-journal divergence introduced by hand error and caught
by the C31 rule) and produced the verb-design agenda below. Each item needs
a ruling before or during CLI design.

Verb semantics.

- Whether a claim survives a move is unstated; the run carried it through,
  Andoneer-style, and a second implementer could legally invert it.
- Claim/move/release ordering across a column boundary is unstated; the two
  reviewers did it opposite ways, and the released event can name a column
  its actor never worked. A related hazard was named: with no rule, the
  first fixture's arbitrary choice becomes the de facto contract.
- A re-review has no verb and no relation between findings comments; cycles
  are reconstructed from timestamps and prose.
- Operator-by-proxy needs an attribution rule: a scribe recording the
  operator's stated ruling writes actor operator, with the scribe named in
  the note, or the operator-owned-column invariant reads as violated.

Data the format has no slot for.

- The lease TTL a claim is supposed to carry has nowhere to live, in
  frontmatter or the claimed event.
- The basis (expected revision) is request metadata with no storage slot;
  the format should say so explicitly so nobody adds one.
- Structured exit data: the Review exit rule wanted finding counts carried,
  and a prose move note is all there is; anything downstream parses
  English. The riding-findings gap is the same hole seen from the gate: an
  operator inherits open findings carried only by prose, with nothing
  marking one open, accepted, or waived. Andoneer's structured checklist
  items are the missing organ, felt by their absence; their entry into the
  format is a boundary-table row.
- A card citing external documents has no way to witness that a source
  changed mid-flow; a replayed fixture cannot tell a drafting error from a
  source that moved.
- Canonical frontmatter key order (or an explicit order-is-insignificant
  rule) is needed before fixtures can be compared byte-wise.
- The journal calls the person actor while a comment says author, two
  passages of the format disagreeing; the registry resolves it as a
  correction, not a first decision.
- Backwards timestamps across journal lines are legal (file order rules)
  but nothing flags them; a lint candidate.

Locking: the run's finding (lockfile filename and encoding undescribed, so
hand editors could not participate) is settled; format.md now specifies the
`lock` file's name, location, exclusive-create acquisition, single-JSON-line
content, and delete-on-release, along with why a lock never lives in
frontmatter.

## Open questions

- The exact CLI grammar and flag conventions, now that the verb set and
  exit-code contract are settled in the first pass above.
- The MCP tool naming and how closely it tracks Andoneer's tool surface,
  including which Andoneer MCP improvements (the expected-revision guard
  among them) are adopted here first.
- The full media-type roster for REST (which mutations earn a dedicated
  type beyond move) and whether the HTTP surface also serves the
  mirror/interchange representations.
- LSP scope: registry-driven diagnostics and completion first; what else.
- GUI timing: which milestone it enters after the CLI and MCP heads exist.
