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
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\n  \"affordances\": [\"status\", \"states\", \"list_cards\", \"next_card\"],\n  \"status\": { \"workbench\": \"Your workbench\", \"profile\": \"dinah-core/1.0\", ... }\n}"}]}}
```

Every payload carries two things at the top: the answer the tool was asked
for, and an `affordances` member naming what you may do next. Treat that
member as the loop. When the answer is a read, the affordances are the other
reads of the same workbench, `status`, `states`, `list_cards`, `next_card`. When
the answer is a card, they name the acts that card will accept, and a refused
call carries its own affordances telling you where to go instead. The machine
answer and the affordances together are the whole surface; if you follow the
affordances you cannot dead-end.

## Orient before you act

Three reads give you the workbench you have been handed.

`status` reports the workbench itself: who its operator is, whether you are
that person, and the states of the flow with each one's occupancy. The
`operator_owned` flag on a state is the answer to whether that state is yours
to move cards out of, and it repeats the rule in `initialize`.

`list_cards` returns the cards of a state, in the order the workbench
fixes. `next_card` offers the first ready card in that order, and it is the
cheapest way to learn what is pullable without enumerating everything.
`states` lists the states of the workbench alone.

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_cards","arguments":{}}}
```

```json
{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\n  \"affordances\": [\"status\", \"states\", \"list_cards\", \"next_card\"],\n  \"listing\": { \"cards\": [] }\n}"}]}}
```

An empty `cards` array is the honest answer for a state nobody has filed
into. It is not an error.

## Take a card and carry it

Pulling is `claim`. It takes the card into your name, moves it from ready to
active, and answers with the instructions of the position the card now sits
at. Read that answer before you work, because the position tells you what
the workbench expects there.

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"claim","arguments":{"card":"proj-3"}}}
```

The work that follows the claim is the point of the claim, and each act is
its own tool call: `comment` to leave a note, `move` to carry the card along
the flow, `attach` to bind a file, `release` to hand a card back to the
queue unfinished. A successful `claim` or `move` carries the instructions of
the new position and the moves the flow allows, so follow the one you need
rather than guessing the next state's name. `show` returns one card in full,
its body, its links, and its comments, and that is the call to make before
you act on a card you have not already met.

## When a call is refused

A refusal is an answer, not an error. The answer carries `outcome` of
`refused`, a `refusal` naming the rule that stopped it, and a `detail`
naming the subject the rule was about. It is the same contract a refusal
follows everywhere on this surface.

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"claim","arguments":{"card":"0"}}}
```

```json
{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\n  \"outcome\": \"refused\",\n  \"verb\": \"claim\",\n  \"refusal\": \"unknown-card\",\n  \"detail\": \"0\",\n  \"affordances\": [\"status\", \"states\", \"list_cards\", \"next_card\"]\n}"}]}}
```

`unknown-card` names what the call did wrong, and the affordances name where
to go to recover. A card you name must exist, a card another owner holds is
not yours to claim, and a card in an operator-owned state is not yours to
move. The refusal tells you which rule stopped you and on what.

A transport error is a different thing. An unknown `method` or a malformed
request answers with an `error` block and a JSON-RPC code. A refusal lives
in `result`, because a refused act is a legitimate answer the contract
defines.

## Three arguments every tool accepts

Every tool's schema carries three arguments beyond the ones the verb itself
takes, and each is worth knowing before you need it.

`actor` is the name you act as. It overrides whatever owner the process
defaults to, and it is how a caller makes a call in a name that is not the
server's default.

`basis` is the revision the call is to be evaluated against. When a response
carries a basis and you want to follow it with a write, pass that basis back
in the next call. If the card changed between your read and your write, the
write is refused as stale rather than silently clobbering the newer state.
That is the optimistic check, and it is why you pass basis forward.

`workbench` names the workbench a call targets when a process serves more
than one. The value is a path to a workbench directory, the directory that
holds its `workbench.md`. Omit it and the call resolves to the server's
default workbench. A value outside the served directory is refused with
`outside-root`, and a path holding no `workbench.md` is refused with
`no-workbench`; the refusal names the path it tried, so it points you at
what is reachable. The one tool that takes no `workbench` is `workbenches`,
and its output, the enumerated workbench paths, is exactly the set of valid
values to pass here.

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
landed since your token, the cards that moved with their new state, and a
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
