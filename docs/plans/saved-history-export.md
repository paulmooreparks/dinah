# Saved chat history snapshot

ShowSavedChatHistory(sessionID) creates a separate Markdown snapshot from
SavedConversation and reveals that file in the system file manager. The control
is labelled Show Saved Chat History. The file identifies its capture time and
states that it does not update with the running conversation.

The export includes saved parent entries followed by current entries in their
stored order. Each entry keeps its role, provider, session origin, boundary text
and other saved metadata. Incomplete-history warnings appear before the entries.
The existing applet-context redaction policy applies to the export. The action
does not load a provider, summarize, rotate or truncate records. It leaves the
original transcript and operational logs unchanged.

Each invocation creates a unique file in the state directory's history-exports
folder. It reveals the file through the existing system file-manager operation;
Dinah's bounded transcript renderer and operational log writer are not involved.
External viewers may have their own limits, so verification must include reading
the exported bytes and checking oldest, middle and newest entries. An unavailable
saved conversation returns an error instead of exporting an empty success.

Engine validation passed the focused export and page-method registration tests
and live-tag desktop vet. A separate preserved-snapshot check exported all
120,271 entries from the 35,500,482-byte captured file, counted every entry header,
verified oldest/middle/newest text after redaction and verified the source bytes
were unchanged. The generated file and check source are retained under Engine's
.diagnostics/recurring-chat-20261031 directory. This checks exported bytes;
external viewer behavior and the UI control still require UI verification.
