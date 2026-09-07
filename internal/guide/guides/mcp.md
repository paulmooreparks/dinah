# Working over MCP

Dinah serves its workbenches over MCP, on stdio, and a fresh
agent on that surface is a caller like any other: it does not see the
terminal commands a person types, it sees a fixed set of tools and the JSON
they answer with. This guide is that caller's read-then-act loop. You are
reading it through `resources/read`, which is itself the first thing to
learn: the guides are resources, not tools. Enumerate them with
`resources/list`, read one with `resources/read`, and list the whole tool
surface with `tools/list` before you call anything.

`initialize` carries the working agreement, the four rules that bind an
owner rather than a tool. Read it before the first tool call. The claim
rule comes first and matters most: claim a card before producing work on it,
and release a claim on a card you have stopped working.

## The shape of a call and its answer

A tool call is one JSON-RPC request. The answer you get is an envelope, and
the useful part is the `text` member, which carries the canonical JSON of
the response indented for reading.

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"status","arguments":{}}}
```

```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\n  \"affordances\": [\"status\", \"columns\", \"list_cards\", \"next_card\"],\n  \"status\": { \"workbench\": \"Your workbench\", \"profile\": \"dinah-core/0.7\", ... }\n}"}]}}
```

Every payload carries two things at the top: the answer the tool was asked
for, and an `affordances` member naming what you may do next. Treat that
member as the loop. When the answer is a read, the affordances are the other
reads of the same workbench, `status`, `columns`, `list_cards`, `next_card`. When
the answer is a card, they name the acts that card will accept, and a refused
call carries its own affordances telling you where to go instead. The machine
answer and the affordances together are the whole surface; if you follow the
affordances you cannot dead-end.

## Orient before you act

Three reads give you the workbench you have been handed.

`status` reports the workbench itself: who its operator is, whether you are
that person, and the columns of the flow with each one's occupancy. The
`operator_owned` flag on a column is the answer to whether that column is yours
to move cards out of, and it repeats the rule in `initialize`. Read the
`awaiting_outside` flag beside it, which answers a different question: a column
carrying it waits on somebody outside the workbench, so no claim of yours will
be taken there and no pull will take a card out of it or land one in it. What
you can still do at such a column is move a card on from it once the answer
comes, which needs no claim.

Read `takes_work_up` beside both of them. A column answering false is one where
nobody works a card: the card waits there until a pull carries it into the
column beyond, so `pull` rather than `claim` is the act to reach for, and a
claim there is refused with `dinah.takes-no-work`. An intake column, a done
column and a column a workbench uses to buffer for the station after it all
answer false.

`list_cards` returns the cards of a column, in the order the workbench
fixes. `next_card` offers the first ready card in that order, and it is the
cheapest way to learn what is pullable without enumerating everything.
`columns` lists the columns of the workbench alone.

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_cards","arguments":{}}}
```

```json
{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\n  \"affordances\": [\"status\", \"columns\", \"list_cards\", \"next_card\"],\n  \"listing\": { \"cards\": [] }\n}"}]}}
```

An empty `cards` array is the honest answer for a column nobody has filed
into. It is not an error.

## Take a card and carry it

Pulling is `claim`. It takes the card into your name, moves it from ready to
active, and answers with the instructions of the position the card now sits
at, minus whatever this connection has already sent you. Read that answer
before you work, because the position tells you what the workbench expects
there.

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"claim","arguments":{"card":"proj-3"}}}
```

The work that follows the claim is the point of the claim, and each act is
its own tool call: `comment` to leave a note, `move` to carry the card along
the flow, `attach` to bind a file, `release` to hand a card back to the
queue unfinished. A successful `claim` or `move` carries the instructions of
the new position, minus the layers you already hold, and the moves the flow
allows, so follow the one you need rather than guessing the next column's
name. The moves are never withheld. `show` returns one card in full, its
body, its links, and its comments, and that is the call to make before you
act on a card you have not already met.

## What a withheld layer means, and how you get it back

The chain has three layers: the user-global text, the workbench's standing
text, and the column's own. Serving all three on every act would send you the
same prose a dozen times in a session, so the head remembers what it has
already sent you on this connection and withholds a layer whose current text
you have already been given.

A response that withholds says so:

```json
"instructions": {"withheld": ["global", "standing", "column"], "reread": "working"}
```

`withheld` names the layers, most general first. It is a statement rather than
a silence: each name says that layer's current text is byte-identical to text
this connection already sent you in this session. A layer that is neither
carried nor named is empty, and you may assume nothing about it.

Ask yourself one question per name: can you still see that text? Where the
answer is yes for every name, carry on. Where it is no for any of them, call
`instructions` with the value of `reread` as the `card` argument:

```json
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"instructions","arguments":{"card":"working"}}}
```

That value names a column rather than a card. A column-shaped request never
withholds, so the answer carries all three layers in full whatever you already
hold, and one call is the whole of the recovery. There is no window to be
inside and no digest to compare. Calling it when you did not need to costs one
arrival and nothing else.

The answer to a column-shaped request is thinner than a claim's in one way you
should expect rather than read as a fault: it carries no `legal_moves` and no
`loop`, because a column named on its own carries no card to compute either
for. You still hold both from the response that withheld the chain.

Nothing is lost if you never ask. A withheld layer is served again unasked
after fifteen minutes, or after twenty further tool calls on this connection,
whichever comes first, so an agent that neither notices the marker nor asks is
sent the whole chain again inside a bounded window.

One thing this head does not do is see an edit you make while it is running.
It loads the workbench's standing text and each column's text once, at startup,
so a change to either reaches you when the process restarts and not before.
The user-global layer is read from disk on every serve, so an edit to that one
does reach a running session, and it arrives as a `global` layer served in full
where you expected it withheld.

## Ask show for the members you want

`show` takes an optional `fields` argument, and it selects which members of a
card the answer carries. The six names are `card`, `body`, `links`,
`attachments`, `comments`, and `path`. Write them comma-separated, and leave
the argument out to be served all six, which is what `show` answers when
nobody asks:

```json
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"show","arguments":{"card":"wb-1","fields":"card,body"}}}
```

Naming what you want on the first call costs nothing. Coming back for a member
you did not ask for costs a whole round trip, which is the most expensive thing
on this surface, because the conversation so far is sent again with it. Ask
narrowly where you know what you want, and leave the argument out where you do
not.

A shaped answer says what it held back:

```json
"withheld": ["links", "comments"], "reread": "wb-1"
```

`withheld` is a statement rather than a silence, on the same terms the
instruction chain's marker is. Each name says the card holds that member and
this answer did not carry it, so a member that is neither carried nor named is
empty on the card and no second call is needed to learn that. `reread` is the
card's own reference, and you pass it back to `show` with the fields you now
want.

A name outside the six is refused with `dinah.unknown-field`. The refusal names
every unrecognised name you gave, sorted, together with the set you may choose
from, and nothing is read before it is raised.

## Reading the bodies of many cards at once

A survey act reads the bodies of several cards in one stretch, and you perform
one before you can choose between cards, compare them, or report across a
column. Over the verbs that act is one `show` per card, and each of those calls
sends the whole conversation so far again along with the tool definitions. One
shell command over the anchors you name sends all of that once.

Where you hold a shell on the machine the workbench's files live on, name the
references you want and read their anchors in a single command:

```
for ref in <ref> <ref> <ref>; do printf '=== %s\n' "$ref"; cat "$(dinah path "$ref")"; done
```

`dinah path` resolves one reference to that card's anchor file, and it runs
inside the command substitution rather than as a call back to the head, so the
whole read is one round however many references you name. The command reads the
cards you named and no others.

A reference the workbench does not know does not stop the read. `dinah path`
writes its refusal to standard error and leaves standard output empty, `cat`
then fails on an empty name, and the loop carries straight on to the next
reference, so that card contributes no body while its `=== ` line still stands
where the card would have been. What you get back is a partial read rather than
a wrong one, because standard output never carries a card you did not name.

The exit status will not tell a complete read from a partial one. A `for` loop
exits with the status of its last iteration alone, so a mistyped reference
anywhere before the last one leaves the status reporting success, and a run
that got every card but the middle one looks from the status exactly like a run
that got them all. Read standard error instead. Each failed reference leaves a
refusal there naming itself, followed by `cat`'s own complaint about the empty
name, so an empty standard error is a complete read and anything on it names
the cards you asked for and did not get.

`docs/design/token-cost.md` records the measurement in its dated section for
this command, and the figures here are transcribed from that section's fenced
block. The command came out cheaper than one `show` per card from the first
card at both measured card sizes, so the crossover is 1 card, and the saving
grows steeply with the number of cards read. At 12 cards of the large measured
size the calls cost 794859 tokens of cumulative billed input against 110627 for
the one command, and at the small size they cost 207597 against 27935. The
crossover did not move between a card body of 2330 bytes and one of 23776
bytes, so what card size changes is the size of the saving rather than the
point at which the saving begins.

Three preconditions bound the route. The first two announce themselves the
first time you run the command, and the third can go wrong without saying
anything at all.

- You need a local shell on the machine the workbench's files live on. Over a
  remote head there is no filesystem to read, and the verbs are the whole
  surface.
- You need the `dinah` binary on that shell's `PATH`, because the command calls
  it once per reference to resolve an anchor.
- You need to run the command somewhere Dinah's discovery resolves to the
  workbench you mean, because the command names no workbench of its own and
  discovery climbs from the directory you are standing in. A directory that
  reaches some other workbench answers out of that one, and where the two
  workbenches share a slug your references resolve there without a refusal and
  without anything on standard error. Confirm the workbench rather than waiting
  to be told about it: `dinah status` prints the workbench discovery resolves
  to, and its path, on its first line.

For a single card, call `show` and name the fields you want, as the section
above describes. The crossover means the command is never the dearer route
inside the measured range, so the reason to keep `show` for one card is what
its answer carries beyond the body. It states what it withheld and it names
what you may do next, and an anchor file carries neither of those.

## When a call is refused

A refusal is an answer, not an error. The answer carries `outcome` of
`refused`, a `refusal` naming the rule that stopped it, and a `detail`
naming the subject the rule was about. It is the same contract a refusal
follows everywhere on this surface.

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"claim","arguments":{"card":"0"}}}
```

```json
{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\n  \"outcome\": \"refused\",\n  \"verb\": \"claim\",\n  \"refusal\": \"unknown-card\",\n  \"detail\": \"0\",\n  \"affordances\": [\"status\", \"columns\", \"list_cards\", \"next_card\"]\n}"}]}}
```

`unknown-card` names what the call did wrong, and the affordances name where
to go to recover. A card you name must exist, a card another owner holds is
not yours to claim, and a card in an operator-owned column is not yours to
move. The refusal tells you which rule stopped you and on what.

A transport error is a different thing. An unknown `method` or a malformed
request answers with an `error` block and a JSON-RPC code. A refusal lives
in `result`, because a refused act is a legitimate answer the contract
defines.

## The three arguments beyond the verb's own

Three arguments come from no verb's parameter list, and each is worth knowing
before you need it. `actor` reaches every tool, and `workbench` reaches every
tool but the one named below. `basis` reaches only the tools that read it. The
schema tells you which tools those are, because a tool publishes one of these
arguments when it consumes it and refuses the name when it does not.

`actor` is the name you act as. It overrides whatever owner the process
defaults to, and it is how a caller makes a call in a name that is not the
server's default. Every tool takes it.

`basis` is the revision the call is to be evaluated against. When a response
carries a basis and you want to follow it with a write, pass that basis back
in the next call. If the card changed between your read and your write, the
write is refused as stale rather than silently clobbering the newer state.
That is the optimistic check, and it is why you pass basis forward.

Eight tools carry `basis` in their schema and read it: `claim`, `move`,
`release`, `block`, `unblock`, `join_workstream`, `leave_workstream` and
`pull`. Every other tool refuses the name, and the refusal names what it does
accept in its place. That includes writes such as `comment`, `add_card`,
`attach`, `archive`, `rename`, and `delete`, which change a workbench and do
not consult a basis, so there is no optimistic check to express on them. An
argument a tool would accept and then drop tells you a check ran when none
did, so the surface turns the call away instead.

`workbench` names the workbench a call targets when a process serves more
than one. The value is a path to a workbench directory, the directory that
holds its `workbench.md`. Write that path in absolute form. The
`workbenches` tool hands you absolute paths, and a client registration you
store should keep them that way, since an absolute path means the same
directory no matter who is reading it.

Dinah does accept a relative path, and resolves it against the directory
the server process was started in. Whoever launched the server chose that
directory, and you may not be the one who launched it, so reach for a
relative path only when you know where the process started.

If you leave `workbench` out, Dinah resolves the call against the server's
default workbench, when it has one; a call that names none and finds no
default refuses `no-workbench-found`. A named value that resolves and opens
is refused `outside-root` only when the server was given a root and the
value lies outside it; a path holding no `workbench.md` is refused
`no-workbench` either way. The refusal names the path it tried, so it
points you at what is reachable.

The one tool that takes no `workbench` is `workbenches`. It searches the
directory the server was given to serve, and it reports a workbench held by
that directory itself or by any directory beneath it, at any depth. The
descent skips two kinds of directory along with everything under them: a
directory whose name begins with a dot, and a symbolic link.

A server started with no `--root` and no `DINAH_MCP_ROOT` carries no
boundary at all: any workbench you can name by its absolute path is
reachable, whether or not the server discovered one to serve as its default
at startup. The one exception is `workbenches` itself, called with no path:
it needs a root to search, so it cannot run that search here. When the
server also carries no default, it refuses `no-workbench-found`. When it
does carry a default, it answers that one workbench alone, with `unbounded`
set on the response so you can tell this is not the outcome of a search:
naming a second, unrelated workbench directly may still reach it, whether or
not this listing named it. Name the workbench you want directly rather than
relying on `workbenches` to find every one for you.

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"status","arguments":{"workbench":"/srv/dinah/incident"}}}
```

## Checkpoint before you act on what you remember

The board moves while you are working, and nothing on this surface can tell
you so on its own initiative: every message you get is an answer to a call you
made. `changes` is how you ask. Call it with no cursor at the start of a
session and keep the token it answers with. That first call reports nothing on
purpose, because you are asking what happens from now rather than what ever
happened, and `log` is the call for the second question.

```json
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"changes","arguments":{"since":"the token the previous call answered with"}}}
```

Call it again with that token whenever you are about to act on something you
believe about the board rather than something you just read. The answer
carries `changed`, which is a fact about the whole workbench, the lines that
landed since your token, the cards that moved with their new column, and a
fresh token to carry forward. A `changed` of true is a reason to re-read
before you act, even where the arrays are empty, which is what a narrowed call
answers when the board moved somewhere you did not ask about.

Read `kind` on a `gone` entry before you conclude anything from it. An entry
whose kind is `card` is a card that was archived, and the workbench can prove
it. An entry whose kind is empty is an identifier that was destroyed, of a
kind the history does not record, so match it against the identifiers you are
holding and ignore it where you do not know it. It does not mean a card you
have not met.

The `card` argument keeps working after the card it names has left, which is
the moment you most want it: an archived card is still found by the reference
you were using, and an identifier that resolves nowhere is accepted and matched
against the departures. A reference that names nothing and is not an identifier
is still refused, so a mistyped one comes back as `unknown-card` rather than as
silence.

## When to act and when to read

The loop is the whole method. Orient with `status` and `list_cards`, choose
with `next_card`, understand the card with `show`, then act with the verb
that names the work, and let the `affordances` of each answer tell you what
to do next. A response that names only the same reads is telling you there
is nothing else on that entity for you to do. A refusal is the surface
telling you the same thing with a reason attached. Neither is a dead end;
each names where to go next.
