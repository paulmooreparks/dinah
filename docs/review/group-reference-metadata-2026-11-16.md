# Group reference metadata handover

The host adds optional shownText and ordered references to BusMessage. Agent-facing text remains unchanged. Each reference retains kind, name and token, with the chat workspace label where applicable. An empty original body is represented explicitly; messages predating this change have neither field. Existing attachment input accepts token without changing the send method signature.

The message presentation row commits in the message transaction. An injected SQLite write refusal proves that a failed metadata write rolls back the message. Reload and a reopened database retain the presentation. Recipient selection still follows the explicit mentions list, independently of references.

Three race/all-vet repetitions pass across the focused bus and desktop selections. Bus completed in 8.329 seconds and desktop in 14.460 seconds, covering metadata persistence, rollback, member delivery text, the actual App send/reload path, empty-body Add references, existing group context and file sending. The existing linker manifest warning remains. The first new routing fixture compared volatile reach versions between separate sends; its correction compares the actual recipients and everyone flag. The first App fixture also lacked a required chat title; it now names that invented chat before group creation. Original failed command receipts remain private. No existing assertion changed.

The branch starts at eaf3c402 and contains no page changes, fork files, installations or rendered receiving claim. Design Review should receive this host branch into its approved group-mentions page and run its independent rendered and routing gates. The urgent bin consistency investigation continues separately.
