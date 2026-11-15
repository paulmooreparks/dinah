# Task questions in Decisions and Notifications

This frontend review responds to Engine's message 9074 and Lead's requirement in 9075. It is based on master ff236bd1 and the task-host implementation progress note. Engine confirmed the representation in message 9078 and froze its host API at 43c1e14f. Engine retains host implementation ownership. Lead's sequencing instruction in 9090 limits the current frontend implementation to Notifications. Decisions will gain this variant with its approved redesign.

A task owns its question. Decisions and Notifications show the same pending question, identified by the host-generated key. They display the service name, task title and workspace name in the places currently occupied by the asking chat. The source link opens the task's own addressable page. Neither surface offers Open Chat, creates a chat-cache entry, writes a chat transcript answer row or changes a chat's approval state.

## Requested representation

The task:permission event and PendingTaskPermissions() query return the same representation. Engine confirmed the following field names.

| Field | Meaning |
| --- | --- |
| key | The existing host-generated question key accepted by Answer. |
| service | The registered service's opaque identity. |
| serviceName | The service's display name from its registration. |
| workspace | The saved workspace's opaque identity. |
| workspaceName | The workspace's display name. |
| task | The task's opaque identity. |
| generation | The task generation that owns the question. |
| taskTitle | The task's current display title from its registered consumer. |
| question | The question text, suitable for display as plain text. |
| options | The existing optionId, name and kind values. |
| askedAt | The host's question creation time, stable across reconnects. |
| target | Exactly one of {href:string} and {reason:string}, with a host-validated task-page destination. |

There is no sessionId, execution handle, review token or raw operation input in this representation. A task ID is never used as a title or as a URL recipe. The registered consumer supplies the title and page destination through a bounded host port. A missing destination leaves the question answerable and shows why its task page is unavailable.

The task:answered event identifies the same key and task scope. Answer, cancellation and revocation all end the pending question. The frontend removes both copies by key without interpreting the event as proof that the person chose an answer. An answer uses only the existing person Answer(key, optionId) path; the host validates that the question and choice are still current. Displaying a generation grants no authority.

## Reconciliation and presentation

Pending task questions are read on initial load and resync, independently of PendingPermissions. A chat-only pending snapshot must never remove a task question, and a task-only snapshot must never remove a chat question. A terminal event received during a pending query wins over that query's older result. Ended keys remain suppressed during the page lifetime, as current permission keys are, and keys are never reused.

Decisions lists the task question alongside other requests. Selecting it displays the question, source identity and offered answer controls. Its page address uses the opaque question key under the Decisions page's own encoding. An answered, cancelled or revoked question leaves the pending list. No transcript record is invented for it. Notifications uses the same question state and answer controls, so answering on either surface disables both copies while the call is pending. An established refusal remains visible with its reason; an uncertain response triggers authoritative reconciliation before another submission is offered.

The current frontend calls the chat-cache constructor in both permissionAsked and answerPermission. Its success handler also writes a transcript answer row. All three must branch on explicit ownership before task questions can be shown. Task questions must also bypass chat-specific refreshApprovals, session disposal and chat-target notification persistence. The ordinary chat permission behavior remains covered separately.

## Receiving checks

- Both surfaces show one task-owned question with its service, task title and workspace, and its link opens only the task page.
- Rendering and answering create no chat-cache entry, chat window, transcript row or provider start.
- An answer on either surface disables both copies and sends the same question key and option ID once.
- Answer, cancellation and revocation remove both copies; late snapshots and duplicated events do not restore them.
- Reconnect recovers pending task questions without reopening tasks or chats, and independent chat/task snapshots cannot erase each other's questions.
- Invalid choices, stale generations and revoked questions are refused by the host before task execution continues.
- The question and result remain readable with visible focus in light and dark at wide and narrow widths; frame checks cover answer and terminal transitions.

Design Review owns the new Decisions layout. Front End's current Notifications implementation keeps question state and rendering in task-questions.js for the redesigned Decisions page to reuse. The current Decisions source remains unchanged. No execution consumer should be exposed before the combined host and rendered checks pass.

## Decisions integration on 16 November 2026

The approved Decisions layout now consumes the same task question state and answer renderer as Notifications. Legacy questions retain the existing Answer call. Typed text and multiple-selection questions use AnswerTaskQuestion with their declared JSON shape. Both views share durable unsent values and disable together while an answer is pending. A failed or lost answer response is reconciled through the pending-question read before another submission is offered.

The current host does not record an answer-versus-cancellation disposition in QuestionEnd. Both views remove the ended question without inventing that disposition. Engine is adding an authoritative event and passive record read with request settlement. The current UI therefore says only that the question is no longer waiting. Dismiss is visible disabled in Decisions pending that host.
