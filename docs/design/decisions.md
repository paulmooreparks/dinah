# Questions and decisions

The Decisions window reads durable bus requests. Engine owns the bus store, agent tool and window methods on engine/decisions. Front End owns the window on frontend/decisions. These branches start at 29a02bc1. The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.

## Agent request

The bus tool `ask_person` accepts `{operationId, group?, questions:[{number,text,recommendation,choices?:string[]}]}`. The operation identity is a caller-generated nonempty string of at most 128 characters, scoped to the asking session. Retrying identical input returns the original request. Reusing an identity with different input fails. There are 1..20 questions with distinct positive integer numbers, text and recommendation of 1..8000 characters, and at most 20 nonempty choices of at most 2000 characters. Choices are suggestions; free text remains valid.

The optional group is a group address such as `#Dinah Desktop Team`. The asker must belong to it. The request is posted there with the person as action recipient; otherwise it goes directly to the person. The request and root bus message commit together. Reading the inbox does not close it. This operation asks the person and returns a receipt; it does not wait for an answer.

## Window API

All names below are App methods. Times are UTC strings. Arrays are always arrays, including when empty. Requests use string identities, so the page must not parse them.

`Decisions(before:int64)` returns `{items:Decision[],nextBefore:int64}` in descending root-message order, up to 100 per page. Zero starts the list; the next cursor is exclusive. Refresh replaces the first page without changing selection or drafts. `bus:changed` invalidates the list.

`Decision` is `{id:string,revision:int,messageId:int64,kind:"request"|"mention",createdAt:string,askerSession:string,asker:string,workspace:string,group:string,groupTitle:string,destination:string,text:string,questions:Question[]}`. A Question is `{number:int,text:string,recommendation:string,choices:string[],answered:bool,answer:string}`. Revision begins at one and advances once per partial-answer operation. The list contains only open rows. A request remains open while any question is unanswered. A mention has no questions and revision one. Its source is a group message addressed to the person that is neither a structured request nor a system result. A direct person reply to that root, or explicit dismissal, closes a mention. Other agents replying do not resolve the person's attention. Earlier mentions are included. Reading does not resolve either kind.

`AnswerDecision(input)` accepts `{operationId:string,id:string,revision:int,answers:[{number:int,text:string,asRecommended:bool}]}` and returns a DecisionReceipt. Each supplied question must still be unanswered. `asRecommended:true` requires empty text and uses the stored recommendation. Otherwise text must contain 1..8000 characters. Omitted question numbers remain open. At least one answer is required. For a mention, supply exactly one answer with number zero and free text. A request's answers commit with one person-authored reply to its root. A group reply wakes the original asker; a direct reply targets that exact session even if its window is closed. The reply records the numbers, questions and answers. A stale revision fails without sending.

`DismissDecision({operationId,id,revision})` closes only a plain mention and returns a DecisionReceipt. Structured requests must be answered. Dismissal sends nothing.

A DecisionReceipt is `{operationId:string,state:"committed",id:string,revision:int,closed:bool,messageIds:int64[]}`. Operation identities are globally scoped for window mutations, with the same 128-character limit. Identical retries return their original receipt, even after the row closed; changed payloads fail. `DecisionOperation(operationId)` returns `{operationId,state:"unknown"|"committed",id,revision,closed,messageIds}`. Unknown has empty/zero fields and an empty array. After a lost response, retain the draft and operation identity, query this method, and retry the identical call if unknown. No client timeout implies cancellation. The transaction includes reply insertion and receipt storage, so reconciliation cannot duplicate delivery.

## Broadcast

`PreviewDecisionBroadcast(scope,text)` accepts scope `active` or `all`, plus 1..16000 characters of text. It returns `{token:string,scope:string,text:string,createdAt:string,expiresAt:string,count:int,targets:[{session:string,title:string,workspace:string}]}` without sending anything. Active means working at preview time, as reported by the window runtime or an active bus hook turn. All includes registered live sessions and known closed window chats, excluding retired team members. External sessions are listed honestly; the bus cannot open an external terminal. Targets are unique and sorted by session identity. The preview freezes the text and target identities for five minutes, including their displayed titles. Later arrivals are excluded; later state changes do not retarget it. The page shows destination count and titles before asking for confirmation.

`ConfirmDecisionBroadcast({operationId,token})` returns a DecisionReceipt with empty id, revision zero and closed true. It requires an unexpired, unconsumed token, inserts one person message per frozen session and stores the receipt atomically. An identical retry reconciles after expiry as well. A token cannot send twice under a different operation identity. The page must request a fresh preview after expiration. Delivery uses the existing bus watcher and bounded background scheduler, so All queues closed chats instead of opening them immediately. No runtime restart, direct prompt injection or desktop interaction belongs to this API.

## Verification boundary

Focused Go checks cover persistence, atomic partial replies, stale revisions, operation conflicts and lost receipts, group authorization/routing, mention resolution, preview expiration/reuse and background-safe delivery. Window bridge methods are classified as queries or exit-accounted work. No live bus requests, visible windows, desktop automation, installation or full browser suite are part of this task.
