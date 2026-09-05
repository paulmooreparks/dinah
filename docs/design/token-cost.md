# What a real agentic work sequence costs

Dinah's content plane can be read two ways, and this document is the baseline
that says what each way costs. An agent working a card can ask the MCP head for
the card, for the instructions of the position it is standing in, and for an
attachment's bytes. It can equally read those three things off the filesystem
and keep the MCP head for the coordination acts, which are the pull, the
comment, and the move. The workstream that proposes the second shape rests on an
argument about cost, and nobody had measured it.

`scripts/measure_agentic_sequence.py` runs one fixed sequence both ways over
two cards standing in one column, counts the tokens each way, and attributes
the difference across named contributions. What follows is that harness's
output and what it means. This document carries no figure of its own: a figure
has one home and the tool is that home, so every number below lives inside the
fenced block, and a reader who wants one re-runs the harness rather than
trusting a sentence.

## The measurement of record

The measurement ran at commit `b54159a250805ff66ae2d0e1d8d750964a2e522c`,
against a binary built from that commit. This command reproduces it:

```
go build -o ./dinah ./cmd/dinah
python scripts/measure_agentic_sequence.py \
    --dinah ./dinah \
    --root <a scratch directory the harness may create and remove> \
    --counter api \
    --commit b54159a250805ff66ae2d0e1d8d750964a2e522c \
    --api-key-file <the file holding a console API key>
```

The credential is read from that file at the moment of use. Do not export it
into a shell-wide environment. A coding agent that finds `ANTHROPIC_API_KEY` in
its own environment may bill its work to that key by usage, which is the cost
this workstream exists to reduce.

## The output, verbatim

```
measure_agentic_sequence: one agentic work sequence, over the verbs and over the files
  commit                                                       b54159a250805ff66ae2d0e1d8d750964a2e522c
  counter                                                      api, [counter=api model=claude-opus-5]
  endpoint                                                     https://api.anthropic.com/v1/messages/count_tokens
  cards in the sequence                                                 2 cards [not a token count]

layers, composed from committed workbench text with git show <commit>:<path>
  global           CLAUDE.md                                   1182 bytes [bytes]
  standing         .dinah/149f228d48c3/workbench.md            8845 bytes [bytes]
  column:working   .dinah/149f228d48c3/columns/4fda9c9ca779/column.md 12981 bytes [bytes]
  column:next      .dinah/149f228d48c3/columns/4b38abe7ebd5/column.md 12668 bytes [bytes]
  card body        README.md                                   6334 bytes [bytes], digest 835855b78f32
  attachment       docs/design/surfaces.md                     23776 bytes [bytes], digest 67247634ecb8
  card comments    docs/design/renaming-a-word.md              3 paragraphs [not a token count], 1065 bytes [bytes], digest 0d7d79e36e50
  card links       each card links to the other                2 links [not a token count] of kinds relates, blocks

coordination acts, pinned digests, which prove the two runs are one sequence
  fx-1 pull                                                    verb 2a40755cfb09, file 2a40755cfb09, agree
  fx-1 comment                                                 verb 330dbc129d19, file 330dbc129d19, agree
  fx-1 move                                                    verb 5bb7039e7631, file 5bb7039e7631, agree
  fx-2 pull                                                    verb cf336e5a3e2c, file cf336e5a3e2c, agree
  fx-2 comment                                                 verb c818ea7a8776, file c818ea7a8776, agree
  fx-2 move                                                    verb bf81d7ee717a, file bf81d7ee717a, agree

headline totals, one line per run, with the caching assumption each rests on
  verb run, context footprint                                       88341 tokens [counter=api model=claude-opus-5] (the final transcript, tool definitions included; invariant to caching)
  verb run, cumulative billed input                                670167 tokens [counter=api model=claude-opus-5] (13 requests, computed under no caching; an upper bound on what a caching session pays, not a bill)
  file run, context footprint                                       83625 tokens [counter=api model=claude-opus-5] (the final transcript, tool definitions included; invariant to caching)
  file run, cumulative billed input                                890239 tokens [counter=api model=claude-opus-5] (19 requests, computed under no caching; an upper bound on what a caching session pays, not a bill)

  footprint, file run as a share of the verb run's footprint       94.662 %  [derived within one regime] (below 100 %, smaller by 5.338 %)
  cumulative, file run as a share of the verb run's cumulative    132.838 %  [derived within one regime] (above 100 %, larger by 32.838 %)
  footprint, file run less verb run                                 -4716 tokens [counter=api model=claude-opus-5]
  cumulative, file run less verb run                              +220072 tokens [counter=api model=claude-opus-5]

what the file run read, each path under the throwaway root
  read:card                                                    C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\cards\fa68cbea8361\card.md (6545 bytes [bytes])
  read:global layer                                            C:\dinah-probe\run\home\.dinah\instructions.md (1182 bytes [bytes])
  read:standing layer                                          C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\workbench.md (9007 bytes [bytes])
  read:column layer                                            C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\columns\fa68cbea8361\column.md (13029 bytes [bytes])
  read:attachment payload                                      C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\cards\fa68cbea8361\attachments\fa68cbea8361\payload\surfaces.md (23776 bytes [bytes])
  read:card                                                    C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\cards\fa68cbea8361\card.md (6545 bytes [bytes])
  read:global layer                                            C:\dinah-probe\run\home\.dinah\instructions.md (1182 bytes [bytes])
  read:standing layer                                          C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\workbench.md (9007 bytes [bytes])
  read:column layer                                            C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\columns\fa68cbea8361\column.md (13029 bytes [bytes])
  read:attachment payload                                      C:\dinah-probe\run\.dinah\01a07306b3047c8d9d07e9ad0f60f3e0\cards\fa68cbea8361\attachments\fa68cbea8361\payload\surfaces.md (23776 bytes [bytes])
  MCP tools the file run reached                               comment, move, pull

requested content and envelope, member by member, per act
  fx-1 pull                                                    content: card | envelope: outcome, verb, basis, legal_moves, affordances | chain: instructions
  fx-1 show-card                                               content: detail.card, detail.body | envelope: affordances, detail.links, detail.attachments, detail.comments, detail.path | chain: (none)
  fx-1 instructions                                            content: (none) | envelope: affordances, served.legal_moves, served.column | chain: served.instructions
  fx-1 show-attachment                                         content: text | envelope: affordances | chain: (none)
  fx-1 comment                                                 content: (none) | envelope: outcome, verb, detail, card, basis, affordances | chain: (none)
  fx-1 move                                                    content: card | envelope: outcome, verb, basis, legal_moves, affordances | chain: instructions
  fx-2 pull                                                    content: card | envelope: outcome, verb, basis, legal_moves, affordances | chain: instructions
  fx-2 show-card                                               content: detail.card, detail.body | envelope: affordances, detail.links, detail.attachments, detail.comments, detail.path | chain: (none)
  fx-2 instructions                                            content: (none) | envelope: affordances, served.legal_moves, served.column | chain: served.instructions
  fx-2 show-attachment                                         content: text | envelope: affordances | chain: (none)
  fx-2 comment                                                 content: (none) | envelope: outcome, verb, detail, card, basis, affordances | chain: (none)
  fx-2 move                                                    content: card | envelope: outcome, verb, basis, legal_moves, affordances | chain: instructions

served instruction chain, per layer, arrivals and repeats
  global layer, arrival serves (1)                                    408 tokens [counter=api model=claude-opus-5]
  global layer, repeats within one card's own acts (4)               1632 tokens [counter=api model=claude-opus-5]
  global layer, repeats across a card boundary (1)                    408 tokens [counter=api model=claude-opus-5]
  standing layer, arrival serves (1)                                 2791 tokens [counter=api model=claude-opus-5]
  standing layer, repeats within one card's own acts (4)            11164 tokens [counter=api model=claude-opus-5]
  standing layer, repeats across a card boundary (1)                 2791 tokens [counter=api model=claude-opus-5]
  column layer, arrival serves (2)                                   8080 tokens [counter=api model=claude-opus-5]
  column layer, repeats within one card's own acts (2)               8382 tokens [counter=api model=claude-opus-5]
  column layer, repeats across a card boundary (2)                   8080 tokens [counter=api model=claude-opus-5]
  chain, arrival serves, all layers                                 11279 tokens [counter=api model=claude-opus-5]
  chain, repeat serves, all layers                                  32457 tokens [counter=api model=claude-opus-5]

the attributed figures, as counts rather than as shares
  arrival serves of the instruction chain                           11279 tokens [counter=api model=claude-opus-5]
  repeat serves of the instruction chain                            32457 tokens [counter=api model=claude-opus-5]
  JSON re-encoding of the prose members                              5206 tokens [counter=api model=claude-opus-5]
  response envelope, measured directly                               4474 tokens [counter=api model=claude-opus-5]
  requested content                                                 21012 tokens [counter=api model=claude-opus-5]

the tool-definition block, and the round trips it is paid on
  tools the MCP head serves                                            36 tools [not a token count]
  tool-definition block, once                                       12837 tokens [counter=api model=claude-opus-5]
  verb run, tool-call rounds                                           12 rounds [not a token count]
  file run, tool-call rounds                                           18 rounds [not a token count]
  tool block over the verb run's rounds                            154044 tokens [counter=api model=claude-opus-5]
  tool block over the file run's rounds                            231066 tokens [counter=api model=claude-opus-5]
  round-trip component, file run less verb run                     +77022 tokens [counter=api model=claude-opus-5]

the reconciliation, against the verb run's context footprint
  sum of the attributed figures and the tool block once             87265 tokens [counter=api model=claude-opus-5]
  verb run, context footprint                                       88341 tokens [counter=api model=claude-opus-5]
  residual                                                          +1076 tokens [counter=api model=claude-opus-5]
  residual as a share of the footprint                             +1.218 %  [derived within one regime]

per-act check, the payload against the figures attributed to it
  fx-1 pull, payload less attributed                                   -3 tokens [counter=api model=claude-opus-5]
  fx-1 show-card, payload less attributed                              -1 tokens [counter=api model=claude-opus-5]
  fx-1 instructions, payload less attributed                           -3 tokens [counter=api model=claude-opus-5]
  fx-1 show-attachment, payload less attributed                        -1 tokens [counter=api model=claude-opus-5]
  fx-1 comment, payload less attributed                                +0 tokens [counter=api model=claude-opus-5]
  fx-1 move, payload less attributed                                   -3 tokens [counter=api model=claude-opus-5]
  fx-2 pull, payload less attributed                                   -3 tokens [counter=api model=claude-opus-5]
  fx-2 show-card, payload less attributed                              -1 tokens [counter=api model=claude-opus-5]
  fx-2 instructions, payload less attributed                           -3 tokens [counter=api model=claude-opus-5]
  fx-2 show-attachment, payload less attributed                        -1 tokens [counter=api model=claude-opus-5]
  fx-2 comment, payload less attributed                                +0 tokens [counter=api model=claude-opus-5]
  fx-2 move, payload less attributed                                   -3 tokens [counter=api model=claude-opus-5]

what this run does not measure
  listing acts                                                 none is in the sequence, so no figure here tracks how many cards a workbench holds
  caching                                                      the api counter applies no prompt-caching logic, so every figure above is an uncached request size
  the file run's own tools                                     Read and Bash belong to the agent harness rather than to this surface, so the tool block counted above is the MCP head's alone
```

## The sequence

Both runs perform the same six acts on each of two cards, in one order. The
agent pulls the card into the working column, reads the card, reads the
instructions of the position it is now standing in, reads the card's first
attachment, leaves a handoff comment, and moves the card on. Nothing in those
acts was chosen to make either run look better, because every one of them is a
read or a write an agent on this board performs on an ordinary card.

The sequence carries two cards rather than one, and the reason is the column
instruction layer. The global layer and the workbench's standing layer are
served again on every serve whatever route the cards take, so a single card
already repeats most of the chain. The column layer repeats inside one card as
well, because the third act asks for the position the pull has already served.
What only a second card produces is a column layer repeating across a card
boundary, which is a second card's pull serving the column a first card was
worked at. That is what a session on this board actually generates, and it is
the figure a serve-once change is judged against, so the harness reports
within-card repeats and across-card repeats as two figures rather than one.

The fixture's instruction layers are this repository's own committed workbench
text, read with `git show` at the commit named above, and so are the card body,
the attachment payload, and the comments the cards arrive carrying. Invented
prose would have decided the answer in advance, because the served chain is the
largest single payload in the sequence and its size is the whole question. No
`dinah` command is ever run against this repository's own workbench, and no run
reads the operator's home, because the harness points `DINAH_HOME` at a
directory under a throwaway root it creates and removes.

## The two runs

The verb run performs all six acts over the MCP head, speaking JSON-RPC on
stdio to `dinah mcp`.

The file run performs the pull, the comment, and the move over the MCP head,
because those are the coordination plane and the arbiter is what makes them
mean anything. It performs the three reads off the filesystem instead: the card
anchor, the three instruction layers in their own files, and the attachment's
payload. It reaches the card's directory with one `dinah path` call at the
shell, which is a command the MCP head deliberately does not serve, and the
harness counts that call's one-line output like any other. The harness refuses
any MCP read verb during the file run, so a file run that quietly asked for a
card would stop rather than report a saving it had not made.

The file run still pays for the served instruction chain on its pull and on its
move, because those verbs serve it whatever the caller does afterwards. The
difference between the two runs bounds what a filesystem-first shape can save, and the chain that survives
in both bounds what stopping the repeated serve can save on top of it.

The two runs are made identical by construction rather than by assertion. Each
run builds its own workbench from one committed definition, the sequence is held
as one declarative list that both runs execute, and the harness prints a pinned
digest of each coordination act's response and exits non-zero when a pair
disagrees.

## What the run does not measure

No listing act appears in the sequence. A listing's payload size tracks how many
cards a workbench holds rather than how the content plane is read, so including
one would move both totals by an amount belonging to a different card's work. A
reader should not take this baseline for a whole-session figure.

The counting endpoint applies no prompt-caching logic, so every figure above is
a request's uncached size rather than an effective billed cost. Uncached size is
the number this workstream wants, because the re-served instruction chain is
exactly the repeated prefix a cache would otherwise mask, and the proposal is to
stop sending that text rather than to rely on its being cached. The consequence
is that the cumulative figure is an upper bound on what a caching session pays,
and it is labelled as an upper bound rather than as a bill.

The file run's own Read and Bash tools belong to the agent harness rather than
to this surface, so the tool-definition block counted above is the MCP head's
alone.

## The counting regime

The operator ruled for the deterministic counter, so every figure above came
from an Anthropic `/v1/messages/count_tokens` response for a named model, and
the harness records the endpoint and the model beside the figures. That
endpoint's own documentation calls the result an estimate that a real message's
input-token count may differ from by a small amount, so nothing here claims to
equal billed usage. Reproducibility at a fixed commit is the check that stands
in for accuracy, and two consecutive invocations of the harness print identical
figures and identical digests.

Every figure the harness prints names the regime that produced it, and the
harness refuses outright to print any ratio, difference, or sum whose operands
came from two regimes. That refusal is the whole of what keeps a local encoder's
proxy figure from being read as a measurement.

The corroborating live run produced nothing, and the reason is worth recording.
The live counter drives `claude -p --output-format json` and reads real token
counts off the result object's usage block. It can corroborate this measurement
only if the sequence the agent actually performed can be compared against the
scripted one, and the result object carries a usage block with no record of
which tools the run called. The counter asks that question of a trivial prompt
before it runs anything, so the selection is refused at probe time rather than
after a full sequence has been paid for. Corroboration therefore waits on a live
driver that can report its own tool-call sequence.

## What the attribution means

Each contribution is computed as its own figure rather than as a share of a
total, because a share cannot be compared against a saving. For each act of the
verb run the harness holds the payload as a structure, produces variants of it,
and counts each variant.

Requested content is the member or members carrying the thing the act names.
Every other member is envelope. The served instruction member is on neither
side, because the chain figure counts it and counting it twice is what the
reconciliation's residual exists to catch. The harness prints, for every act,
which members it put on which side, so the split is inspectable rather than
buried in the script, and a member falling through both sets stops the run.

The envelope figure is measured directly, as the payload marshalled with the
requested-content members and the instruction member emptied. It is not derived
by subtraction. A subtractive envelope would make the reconciliation's residual
identically zero, so the check would pass whatever any member cost and a
miscounted member would go through unnoticed.

The reconciliation is taken against the verb run's context footprint rather than
against its cumulative billed input, because each attributed figure is counted
once and the cumulative total counts one payload once per request. The sum it
reconciles is the attributed figures together with the tool-definition block
counted once, so it stands above the attributed figures on their own. A byte-pair
tokenizer is not additive across concatenation boundaries, so the figures are
not expected to sum exactly to the footprint. The harness prints the residual on
every run and exits non-zero when its magnitude leaves the declared bound.

## What each figure bounds

A different figure bounds each of the three cards waiting on this baseline.

- The filesystem-first guidance, dinah-380, is bounded by the difference between
  the two runs, on both the context footprint and the cumulative billed input.
  Both differences are in the block above, each carrying its sign.
- Stopping the repeated serve, dinah-382, is bounded by the repeat-serve figure
  of the instruction chain. The block breaks that figure out per layer and,
  within each layer, into repeats inside one card's own acts and repeats across
  a card boundary. Both kinds are text a serve-once change removes, so both
  count toward the bound.
- Shaping the read verbs, dinah-383, is bounded by the response envelope figure,
  which is what a payload costs once the thing the act asked for is taken out of
  it. The card view's three listings and the anchor path sit on the envelope
  side precisely so that this card can be bounded at all.

## What the measurement says, including where it argues against the workstream

Two of the findings below run against the argument the workstream was built on.

The file run's context footprint came out below the verb run's, so reading the
content plane off the filesystem does save context. The margin is far smaller
than the workstream's argument assumes, because the file run still pays for the
served instruction chain on its own pull and its own move, and because the card
anchor it reads carries most of what the card view serves: the card's
frontmatter, its seeded links, and its body. The anchor does not carry the
card's comments, which sit in a sibling directory the file run never opens, so
the file run is not charged for the comments the verb run's card view serves.
That gap runs the same way as everything else here, so the true
filesystem-first saving is smaller still.

The file run's cumulative billed input came out well above the verb run's, so on
that total the filesystem-first shape costs more rather than less. The reason is
round trips rather than payload. The file run performs more tool-call rounds
than the verb run, because a file read is a tool call and a shell
call is a tool call, and every round pays for the whole conversation so far
together with the tool-definition block. The harness computes the signed
difference from the rounds each run actually performed rather than asserting a
direction, and the sign came out against the file run.

The largest single attributed figure is the repeat serves of the instruction
chain. It is larger than the arrival serves of that same chain, and it dwarfs
the response envelope. That reverses the workstream's stated ordering. Stopping
the repeated serve is the biggest saving available, the filesystem-first shape
is second and is smaller than expected, and shaping the read verbs is the
smallest of the three.

The response envelope is the smallest of the attributed figures, so a card that
shapes the read verbs should be scoped against that figure rather than against
either total.
