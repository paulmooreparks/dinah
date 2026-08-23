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
fixes. `next_card` offers that same first card on its own, and it is the
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

## When to act and when to read

The loop is the whole method. Orient with `status` and `list_cards`, choose
with `next_card`, understand the card with `show`, then act with the verb
that names the work, and let the `affordances` of each answer tell you what
to do next. A response that names only the same reads is telling you there
is nothing else on that entity for you to do. A refusal is the surface
telling you the same thing with a reason attached. Neither is a dead end;
each names where to go next.
