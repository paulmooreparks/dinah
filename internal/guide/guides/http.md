# Working over HTTP

`dinah serve` lets a browser, or any other HTTP client on this machine, work
with one workbench. Dinah opens the workbench afresh for every request, so you
get the answer a terminal command would give you at that moment, even when
another process added a card or a column a second earlier. Every body Dinah
sends you is the JSON that `dinah mcp` sends for the same call, byte for byte.

When you run `dinah serve`, Dinah prints the address it is listening on and
keeps running until you interrupt it. It listens on `127.0.0.1:7340` unless
you name another address with `--listen`. If you write `--listen 127.0.0.1:0`,
the system chooses a free port and the address Dinah prints carries it.

## Who can reach the server, and who a request acts as

Dinah listens on the loopback interface and nowhere else. If you give
`--listen` any other host, Dinah refuses with `dinah.not-loopback` before it
binds anything. Loopback keeps other machines out, but it does not keep out
other accounts on this machine or the processes they run. Dinah authenticates
nobody here, so any process on this machine that can reach the address can
act on your workbench. Keeping those accounts out, and serving other
machines, both need authentication, and `dinah serve` does not have it yet.

A request names the actor it acts as in the `Dinah-Actor` header. If a request
names nobody, Dinah acts as whoever started `dinah serve`, which is normally
you, so an agent that leaves the header out acts with your name and your
authority. You declare what is performing an act in the headers
`Dinah-Harness`, `Dinah-Provider`, `Dinah-Model`, and `Dinah-Server`. For any
of the four you leave out or send empty, Dinah uses what the server's own
environment declared.

## Reading

You read with GET, and Dinah answers HEAD wherever it answers GET. The query
string carries the command's own parameters, each spelled as the parameter's
name, so `/cards?query=state:ready` runs `dinah query state:ready`. You turn
on a flag that takes no value by naming it with no value or with `true`, and
turn it off with `false`. If you send a parameter the route does not take, or
send one twice, Dinah answers 400.

| Path | What Dinah answers |
|---|---|
| `/` | `dinah status` |
| `/workbench` | `dinah show workbench` |
| `/cards` | `dinah list cards`, or `dinah query` when you name the `query` parameter |
| `/cards/<card>` | `dinah show <card>` |
| `/cards/<card>/<rest>` | `dinah list` when the reference names a collection, such as `comments` or `journal`, and `dinah show` otherwise |
| `/cards/<card>/instructions` | `dinah instructions <card>` |
| `/cards/<card>/claim` | a 303 redirect to the card, because the card carries its claim |
| `/cards/<card>/window` | the card's floating window, as HTML alone |
| `/columns` and `/columns/<column>` | `dinah list columns` and `dinah show <column>` |
| `/columns/<column>/<rest>` and `/columns/<column>/instructions` | the same as for a card |
| `/workstreams` and `/workstreams/<slug>` | `dinah list workstreams` and `dinah show workstream/<slug>` |
| `/routes` and `/attachments` | `dinah list routes` and `dinah list attachments` |
| `/next` | `dinah next` |
| `/changes` | `dinah changes`; you poll it with the cursor it gives you, because `--wait` is not served |
| `/tree` and `/search` | `dinah tree` and `dinah search` |
| `/views` and `/views/<view>` | `dinah view` |
| `/whoami` and `/version` | `dinah whoami` and `dinah version` |
| `/affordances` | the table that tells you how to send each act, described at the end of this guide |

A reference under `/cards/` has to name a card, and one under `/columns/` has
to name a column, so a card and a column that share a spelling never answer
each other's URL. If no route matches your path, Dinah answers 404 with
`dinah.unknown-resource`.

## Acting

| Act | Request | Body type |
|---|---|---|
| add | POST `/cards` | `application/json` |
| move | PATCH `/cards/<card>` | `application/vnd.dinah.move+json` |
| block | PATCH `/cards/<card>` | `application/vnd.dinah.block+json` |
| unblock | PATCH `/cards/<card>` | `application/vnd.dinah.unblock+json` |
| claim | POST `/cards/<card>/claim` | `application/json`, or no body |
| release | DELETE `/cards/<card>/claim` | no body |
| pull | POST `/claims` | `application/vnd.dinah.pull+json` |
| comment | POST `<path>/comments` | `application/json`, or no body |

You send one JSON object whose members are the command's parameters, a string
for a parameter that takes a value and a boolean for a flag that does not. To
move a card to the doing column you send `{"column": "doing"}`. If you send a
member the act does not take, Dinah answers 400, and it does the same for a
member naming what the path already says, such as `card` in a move. Dinah
chooses among move, block and unblock by the body type alone, so a move you
send as `application/json` gets a 415 whose `Accept-Patch` header lists the
three types.

When you create something, Dinah answers 201 with a `Location` header naming
the new card, the claim, the pulled card's claim, or the new comment. A pull
that names `no-claim` answers 200, as does every other act that succeeds.
Dinah does not serve the workbench's other acts over HTTP yet, such as `file`,
`link`, and `archive`.

## Revisions

Dinah sends a card's revision as its entity tag. You get
`ETag: "<revision>"` from `GET /cards/<card>` and from every act that answers
with a card. Send it back as `If-Match` on a move, a block, an unblock, a
claim, a release, or a pull, and Dinah refuses the act as stale if somebody
changed the card since you read it. That answer is a 412 carrying the card as
it now stands and its current `ETag`, so you can try again from what you were
just given.

Dinah requires a revision on every PATCH, and answers 428 with
`dinah.basis-required` when you send none. If you mean to act without one,
send `If-Match: *`. Dinah answers 400 to a list of tags, to a weak tag, and to
`If-Match` on an add or a comment, which compare no revision.

## Statuses

When Dinah refuses an act or a read, the status follows the refusal's name.
You get 400 for a malformed request, 403 when the refusal is about who is
asking, 404 when a reference names nothing, and 409 for every other refusal,
because most refusals say the workbench stands in a state that forbids the
act. A stale answer is 412, and Dinah answers 503 when it could not reach the
workbench at all. The body is the same refusal every other way of asking
would give you, with its name and its detail.

## Representations

Dinah sends every body as `application/vnd.dinah+json`, with a `profile`
parameter naming the contract version, unless your `Accept` header asks for
`application/json` and not the vendor type, in which case it sends the same
bytes as `application/json`. A request with no `Accept` header, or with
`*/*`, gets the vendor type. When your `Accept` header ranks `text/html`
above both, as a browser's does, Dinah answers with a page, which the last
section of this guide describes. If your `Accept` header admits none of the
three, Dinah answers 406. Every answer carries `Cache-Control: no-store`,
because every answer is the workbench as it stands.

## Acting from an HTML form

An HTML form can send only GET and POST, cannot set a header, and cannot
choose its body type. Every act therefore also takes a POST of
`application/x-www-form-urlencoded` to the act's own path, and you use four
form members for what the form cannot say.

| Member | What it stands for |
|---|---|
| `_method` | the method, `PATCH` or `DELETE` |
| `_type` | the PATCH body type, which chooses the act; you need it with `_method=PATCH` |
| `_basis` | `If-Match`, written bare without quotes, or `*` |
| `_actor` | `Dinah-Actor` |

A checkbox with no `value` attribute sends `on`, and Dinah reads that as true.
A browser sends every field of a form, the empty ones included, so Dinah reads
an empty member as absent unless the act cannot run without it. If you name a
member twice, Dinah answers 400. It also answers 400 when
`_basis` and `If-Match` disagree, or when `_actor` and `Dinah-Actor` disagree,
because the request has not said which one it means.

A web page on any other site can make your browser post a form to this
address. Dinah therefore takes a form only when it can tell the form came from
its own pages: the request carries an `Origin` header naming this server, or,
with no `Origin`, the header `Sec-Fetch-Site: same-origin`. A form carrying
neither gets a 403 with `dinah.origin-required`, and a form from another page
gets a 403 with `dinah.foreign-origin`. The JSON body types need no such
proof, because a page cannot send them to another site without first asking
that site's permission, and Dinah never gives it. If you write a client that
is not a browser, send the JSON types and leave `Origin` out.

## Finding out how to send each act

Every answer names what you may do next in its `affordances` member, spelled
the way `dinah mcp` spells it, and `GET /affordances` tells you how to send
each of those names:

```json
{"affordance": "move", "method": "PATCH", "href": "/cards/{card}", "accepts": ["application/vnd.dinah.move+json"], "form": {"method": "POST", "href": "/cards/{card}", "members": {"_method": "PATCH", "_type": "application/vnd.dinah.move+json"}}}
```

In an `href`, `{card}` stands for the card's reference and `{path}` for the
path of whatever the answer was about. `accepts` lists the body types the
method takes. `form` tells you how an HTML form sends the same act: a POST to
that URL, carrying the listed members as hidden fields beside the act's own
members and any `_basis` and `_actor` you add. A read's `form` is null.

## Pages

When a browser asks for any of these paths, Dinah answers with a page, drawn
on the server from the same answer a JSON client gets. `dinah ui` starts this
server and opens a browser on it, and a browser you point at `dinah serve`
gets the same pages. The board is at `/`, a column's page at
`/columns/<column>`, a card's page at `/cards/<card>`, the card list at
`/cards`, and the tree, the search, the views and the command log at `/tree`,
`/search`, `/views` and `/commands`. For any other path, Dinah shows the JSON
answer on a page. A card also opens as a floating window over any page, and the URL
names the open windows in its `open`, `top`, `min` and `p.<card>` parameters,
which a page takes and a JSON client's request is refused for.

Every act on a page is a form posting to the act's own route, and the page
works with script turned off. After the act Dinah sends you back to the page
you posted from, whatever the outcome, and the outcome appears in the command
log along the bottom of every page. The log shows each act as the command line
that would have done it at a terminal, newest first. It holds the last 500
entries, typed lines included, until the server stops. Run again performs
an entry's command a second time. It is guarded by the card's revision only
on an entry made from a card's form, so it answers stale once the card has
changed, and on any other entry it acts on the card as it stands now.

You can also type a command line into the log. Dinah splits it into words at
spaces, keeps words inside double quotes together, reads a backslash before a
double quote as making the quote part of the word, and expands nothing. It
then parses the words with the terminal's own parser and performs the command
as a click would. The typed line runs the commands this
guide's route table names, and refuses any other command by its name with
`dinah.not-served`.

Dinah does not let a page on another site show these pages inside a frame.
Every page carries `Content-Security-Policy` with `frame-ancestors 'self'`
and `X-Frame-Options: SAMEORIGIN`. Without them, another site could frame a
page and trick you into clicking one of its forms, and Dinah could not tell
that click from one you meant.
