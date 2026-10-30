# Sequenced Events with Resync

This is phase 3 step 1 of the [cleanup plan](cleanup.md). Dinah Architecture proposed it on 29 October 2026, and Paul approved it the same day (message 502). The answers to the questions at the end are recorded there.

## What Happens Today

The engine sends every event to the page through `Bridge.Emit` in `internal/bridge`. Each page holds one `EventSource` on `/bridge/events`, a server-sent event stream, and the generated bridge script dispatches each message to the listeners that `dinahHost.on` registered. A subscriber's queue holds 256 events. When it is full, `Emit` closes that subscriber's stream and drops it, so the page loses the event that did not fit and everything sent until it reconnects. The browser reconnects by itself, but nothing tells the page that anything was lost, and nothing sends it again.

That loss is silent today. A long agent turn that streams faster than a busy page reads it leaves a chat's transcript with a hole, a permission question can fail to appear, and a session's state can stay stale until something else changes it. The daemon makes it worse: when the engine runs in its own process, the page's connection to it can drop for reasons that have nothing to do with load, and a second window or a remote client connects late and needs to catch up.

The engine sends about forty kinds of event. They fall into four groups, and the group decides what recovery means.

| Group | Examples | What a lost event costs | Recovery |
| --- | --- | --- | --- |
| State | `agent:session`, `agent:state`, `agent:options`, `settings:changed`, `workspace:changed`, `packages:changed`, `bus:changed`, `documents:recent` | A view shows old state until the next change | Read the current state again |
| Stream | `agent:update`, `agent:updates`, `agent:subagent-update`, `terminal:output` | A hole in a transcript or a terminal | Replay the stream from a known point |
| Request | `agent:permission`, `page:ask`, `dinah:exit-confirmation` | A question nobody sees, so a turn or the exit waits forever | Read the pending requests again |
| Notice | `agent:file`, `agent:sent`, `applet:error`, `extension:review` | A toast or a mark that never shows | None needed, though the loss is reported |

The work in step 1 is the first two parts of the design below, which make every loss either repaired or announced. Part three repairs each group.

## The Design

### 1. Every event has a sequence number and an epoch

The bridge numbers every event it sends, starting from 1 when the process starts, and gives the process a random epoch at the same time. Each message on the stream carries the server-sent events `id` field, set to `<epoch>.<sequence>`.

The WHATWG HTML standard defines this mechanism, which is why I chose it over a protocol of our own. A browser's `EventSource` remembers the last `id` it received and, when it reconnects, sends it in the `Last-Event-ID` request header. The page does not have to track anything, and the bridge script does not change for the browser. The standard also defines the `retry` field, which sets how long the browser waits before reconnecting, so the bridge sends `retry` once when a stream opens rather than leaving the delay to the browser's default. A client that is not a browser, such as a remote client or the daemon's own tests, passes the same value as an `after` query parameter. The header wins when both are present.

The epoch makes a restarted engine visible. Sequence numbers start again at 1 in a new process, so a cursor from the old process means nothing in the new one. A cursor with a different epoch is treated as having no cursor.

### 2. The bridge keeps recent events and replays them

The bridge keeps the most recent events in a ring, bounded by both count and size: 8,192 events or 16 MiB, whichever is reached first. These are constants in `internal/bridge` with a test for each bound. They are not settings.

When a stream opens, the bridge reads the cursor and does one of three things.

1. With no cursor, it sends live events only. This is a page's first connection, and the page reads its starting state through its usual calls.
2. With a cursor from this epoch whose next event is still in the ring, it replays every event after the cursor and then continues live, with no gap and no repeat.
3. With any other cursor (another epoch, or a gap older than the ring), it sends one `dinah:resync` event, which has its own sequence number, and then continues live.

Taking the subscription and copying the replay must happen under the bridge's one lock, the same lock `Emit` takes to number an event. That is the whole correctness argument for "no gap and no repeat": an event is numbered, stored and queued to every subscriber in one step, and a new subscriber is registered and handed its replay in one step, so every event is either in the replay or in the live queue and never in both.

A slow subscriber is still closed when its queue fills, as today, because holding up the engine for a page that is not reading is worse than reconnecting it. The difference is that the page now reconnects with its cursor and either gets a replay or is told to resync.

### 3. The page recovers by group

`dinahHost` gains `onResync(handler)`. The bridge script dispatches `dinah:resync` like any other event, and `dinahHost` runs every resync handler. Each module that owns a view registers a handler that does what its row in the table above says.

- **State.** The handler reads the state again through the call it already uses at startup. State events carry whole values rather than changes (`agent:session` sends the session's full `Info`), so applying an event that arrives during the reread is safe whichever comes first.
- **Requests.** The handler reads the pending requests again and shows the ones it is not already showing, keyed by the request's own ID. Where no call returns pending requests today, the engine adds one. A request answered in the meantime is withdrawn by the answered event the page already handles.
- **Notices.** Nothing is repaired. The page shows one notice saying that some activity may not have been shown, because hiding a loss is the problem this step exists to fix.
- **Streams.** This is the hard group, and it is the only one where applying an event twice, or missing one between a reread and a live event, corrupts what the person sees. An open chat recovers by reloading its transcript through the path it already uses when a chat is reopened. That reload must say which event it is consistent with, and the page drops buffered stream events for that session at or before that point and applies the ones after it. The engine can only report that point if the transcript is appended and the `agent:update` for it is numbered under the same lock, so that a reading of the transcript and the current sequence number agree. Whether that holds today has to be established from the code before the stream commit is written. A terminal does the same with its scrollback if the engine keeps one; if it does not, the terminal shows a mark where output was lost.

Replay inside the ring needs none of this. The page sees every event once and in order, which covers almost every reconnect, so part three only runs after a restart or a long stall.

## What Stays the Same

The event names, their payloads and the page's listeners do not change, except that a module registers a resync handler beside its listeners. `EventsEmit`, which the page uses to send an event to itself, stays local and is not numbered. The browser suites' shared stub in `e2e/lib/page.js` gains a way to send `dinah:resync` and to drop events, so that each module's recovery can be tested without a real stream.

## The Commits

1. **The bridge (Engine).** Numbers, the epoch, the ring, replay, `dinah:resync`, the `retry` field and the `after` parameter. Tests cover replay with no gap and no repeat while events are being emitted concurrently, a cursor older than the ring, a cursor from another epoch, both ring bounds, a slow subscriber being closed and then replayed, and the header winning over the parameter. Nothing in the page changes yet, and a resync is still unhandled, which is no worse than today.
2. **State, requests and notices (Front End, with an Engine addition if a pending-requests call is missing).** `dinahHost.onResync`, a handler for every state and request event in the table, and the lost-activity notice. Before writing it, Front End lists every event the page listens to with its group and the call its handler uses, and that list goes in this document.
3. **Streams (Engine, then Front End).** Engine first reports whether transcript appends and their events are ordered under one lock, and proposes how the transcript reload reports its sequence point. That proposal comes back for review before any code, because it is the one place in this step where a plausible mistake still passes the tests.

Each commit passes the full Go suite, both vet runs, the production build and the full browser gate, and comes back for review.

## Questions for Paul

1. Is the lost-activity notice wanted? It is the honest choice for notices that cannot be repaired, but it adds something the person sees, and it will appear after every engine restart while a window is open.
2. The ring bounds, 8,192 events or 16 MiB, are my estimate of a few minutes of a busy agent turn. Is memory of that order acceptable for the window, and later for the daemon?

Paul's answers, 29 October 2026:

1. The lost-activity notice is wanted, including after an engine restart.
2. The bounds are acceptable. Paul asked whether they should be configurable, and Architecture recommended against a setting. A bound that is too small costs a resync, never lost data, so nobody needs to tune it to stay correct, and a setting would add a control and a support question for a value nobody can sensibly choose. The bounds are fields of the bridge's options with the constants as defaults, so the daemon or a test can choose others in code without a person-facing setting.
