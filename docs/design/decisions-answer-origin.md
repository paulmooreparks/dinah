# Decisions answer origins

An answer sent from Decisions remains an ordinary text reply for agents. The reply additionally carries optional typed `answerOrigin` metadata so the window can render an answer card without interpreting prose. Historical replies are unchanged; their recommendation choice cannot be recovered reliably from text.

## Message shape

The optional field has the same lower-case JSON shape on `BusMessage`, `sessionhost.DeliveredMessage` and saved transcript entries' `delivered` object. The internal bus message carries the corresponding typed pointer. Version 1 has this shape:

```json
{
  "answerOrigin": {
    "version": 1,
    "requestId": "request:123",
    "messageId": 123,
    "kind": "request",
    "questions": [
      {"number": 2, "text": "Which build should we use?", "answer": "Use the tested build.", "asRecommended": true}
    ]
  }
}
```

The Go types are `bus.AnswerOrigin` and `bus.AnsweredQuestion`. Their exported fields are `Version int`, `RequestID string`, `MessageID int64`, `Kind string`, `Questions []AnsweredQuestion`, and `Number int`, `Text string`, `Answer string`, `AsRecommended bool`, respectively. Both window message DTOs use `AnswerOrigin *bus.AnswerOrigin` with `json:"answerOrigin,omitempty"`.

`requestId` is the existing stable Decisions identity. `messageId` names the original question message, rather than this reply. `kind` is `request` for ask_person or `mention` for a plain mention. Questions contain only answers committed in this reply, in the original request's question order. Their text and resolved answer are immutable snapshots. An As Recommended answer stores the recommendation's words in `answer` and preserves `asRecommended:true`. A typed answer that happens to equal the recommendation still has `asRecommended:false`.

A Decisions answer to a plain mention uses `requestId:"mention:123"`, `kind:"mention"`, and one question with `number:0`, the original mention's full text as `text`, the person's answer as `answer`, and `asRecommended:false`. The existing plain-mention input contract remains unchanged.

Only the successful AnswerDecision transaction constructs this metadata, from the stored question and accepted answers after the existing redaction. It stores the origin with the reply and operation receipt atomically. Repeating an operation creates no second origin or reply. Ordinary send/reply callers cannot supply an origin. Dismissal and broadcast create none. Version 1 is optional; renderers retain ordinary message presentation when it is absent or its version is unsupported.

The bus persists metadata additively beside each originating reply. Every message read/delivery path retains it, including group reads, transcript capture, saved transcripts, provider-prompt replay reconciliation and reopen. The agent still receives the existing complete prose. An old build ignores the additive metadata and can still open the bus and transcript files. No schema replacement or history rewrite is permitted.

The `announceArriving` entries add optional `decisionAnswer:true` when the message has an answer origin. The preview keeps its existing text bound and carries no question snapshots. Front End uses the marker to reserve the answer card's fixed height while delivery is pending. Ordinary arrivals omit it. Engine owns this marker and its conversion tests.

## Read-only lookup

The window query is `DecisionByID(id string) (bus.DecisionDetail, error)`, with a query row in pageMethods. The bus method is `DecisionByID(ctx context.Context, id string) (DecisionDetail, error)`. It returns:

```json
{"decision": {"id":"request:123", "kind":"request", "messageId":123, "questions":[]}, "closed":true, "readOnly":true}
```

`decision` is the existing complete `bus.Decision` DTO, including its latest question answers, revision, source, text and destination. The abbreviated example above omits those existing fields for space. `DecisionDetail` has `Decision Decision`, `Closed bool` and `ReadOnly bool`, tagged `decision`, `closed` and `readOnly`. ReadOnly is always true, including for a still-open request, and the linked view offers no answer or dismissal action. Open-list behavior and mutations keep their existing contracts.

The lookup accepts the existing `request:<messageId>` and `mention:<messageId>` identities and verifies the kind. Structured requests remain readable after answer or dismissal. Plain mentions remain readable after a reply, group conversation, dismissal or first-use boundary exclusion, provided the original agent text in its group had a human delivery. A missing message, wrong kind or identity that never named such a request returns an error. The page displays that the request is unavailable. The query marks nothing read, changes no attention or Decisions state, and sends nothing.

## Ownership and checks

Engine owns bus persistence, message conversion, transcript and replay preservation, lookup and the pageMethods query row. Front End owns the chat card, navigation and read-only view against these fields. Tests must cover partial answers, explicit versus recommended answers, plain mentions, retry identity, delivery and transcript round trips, replay, closed lookup, kind refusal and unchanged legacy messages. The contract is frozen before implementation so Front End can use fixtures while Engine finishes the stand-alone Markdown assignment.
