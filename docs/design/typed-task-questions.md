# Typed task questions

Team Lead approved this generic extension in messages 9665 and 9745. Engine owns the host branch from a3826b6b. Existing single-option questions and their trusted Answer(key, optionID) path retain their representations and results. This contract adds text and multiple-selection questions for registered task consumers, including Flow. The Architectural Principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.

## Question and answer contract

AskPerson keeps its existing title and choices input. A typed request instead supplies title and answer, where answer is a QuestionSpec. It cannot supply both choices and answer. The existing task Review, Begin, permit fingerprint, operation identity and receipt machinery remains in the execution path. Constructing or reading questions starts no provider or external work.

QuestionSpec has type, options, minLength, maxLength, minSelections and maxSelections. Type is text or multiple. Text requires an explicit maxLength from 1 through 16384 Unicode scalar values, with minLength between zero and that limit. Text has no options or selection bounds. The host accepts valid UTF-8, counts Unicode scalar values, and preserves the submitted text without trimming or normalization. The UI must use the same count rather than HTML's UTF-16 maxlength semantics. An empty answer is permitted only when minLength is zero.

Multiple selection requires 1 through 16 offered options with distinct nonempty optionId and name fields. Each option follows the existing option ID and label bounds. Selection bounds satisfy zero <= minSelections <= maxSelections <= option count, with maxSelections at least one. Multiple selection has no text length bounds. Duplicate or unoffered IDs refuse; the host returns selected IDs in offered-option order. An empty selection is permitted only when minSelections is zero.

Pending typed questions use the existing Question owner, key, text, options and askedAt fields, plus answerType and the applicable explicit bounds. Legacy questions omit all new fields. They continue to return one string option ID. The question key identifies one pending asking, including its task generation; a new asking always gets another key. The consumer maps that opaque key to its own asking sequence and never infers identity from a title.

The typed answer value is {type:"text",text:"..."} or {type:"multiple",optionIds:["..."]}. The two shapes are exclusive. Text must be supplied, including an explicit empty string when allowed. Multiple selection permits an empty list when declared. Unexpected fields, wrong JSON types, mismatched answer types, invalid Unicode, duplicate IDs and out-of-range answers refuse without consuming the pending question. The window JSON entry point rejects bodies larger than 131072 bytes, unknown fields and trailing JSON values.

AnswerTaskQuestion(key, answerJSON) is a trusted window entry point called from the person's action. It never accepts task identity, generation, owner or a person flag from the caller. The host looks up the opaque key, rechecks current authority and generation, validates its declared schema and settles it once. The legacy Answer entry point refuses typed questions, and AnswerTaskQuestion refuses legacy questions. Services receive no answering port and cannot synthesize a person answer through CallService. Flow removes its proposed flows.Answer method.

Typed AskPerson results are {answer:{type,text or optionIds}} under the ordinary durable operation receipt. The legacy result stays {answer:"option ID"}. All views observe the same pending question and its single settlement. A copied Pending result cannot mutate the host's offered schema. Cancellation, generation change, removal and shutdown withdraw the pending question through the existing task:answered event. A stale or duplicate answer cannot revive it or make another provider/action request. An unknown durable operation is never replayed to reconstruct a lost question.

## Delivery and verification

Pending snapshots recheck the consumer's current local task generation. If that record changed or disappeared, the host cancels and withdraws its pending question before returning the refreshed view. This also applies to existing single-option questions; their input, output and trusted submission representations remain unchanged. The recheck starts no provider or action work.

This branch changes the generic host only. Notifications, Decisions and Flow need the same typed rendering and trusted submission adapter. Dinah Front End owns the question UI after its small interactive mockup update is approved. Host support alone does not claim rendered or installed acceptance.

Quality should freeze independent checks for the declared type bounds, Unicode counting, strict input shapes, offered-option validation, copied snapshots, single settlement across views, cancellation, stale generations, trusted submission and absence of service answering authority. Engine will bind those checks to the real App and reviewed AskPerson operation path. Existing task-question tests and frozen host assertions remain unchanged. Tests use invented tasks and isolated stores and prove zero provider/action/remote writes with counted boundaries and positive controls where needed.
