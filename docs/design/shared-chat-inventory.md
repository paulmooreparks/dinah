# Shared chat inventory

The Chats panel, org chart, active Clean Up Chats rows and their counts consume one captured inventory with identical identities, titles, states and workspace filtering for every program Dinah offers. This contract supersedes the local-only population in passive-startup.md. The immediate correction keeps today's Chats listing as the source while the passive replacement is prepared.

## Population and identity

The passive inventory combines local chat records with the native session listings the host has durably recorded. Devin and Claude rows that were visible only through native discovery remain legitimate inventory rows, including Unknown rows and their provider titles. Codex retains its separate explicit-import boundary. Observing external Codex history does not admit it, and that exception does not apply to any other program.

The provider registry declares this policy as NativeSessionsRequireImport, enabled only for Codex. Passive reference collection reads that fact instead of comparing provider names. An unrecognized program remains visible as a repair row with its safe title and cannot authorize opening or cleanup; an absent registry row does not grant admission through the fact's zero value.

Every admitted reference passes through logicalChatMembersFromSettings. Saved binding and transfer evidence resolves switched chains once; a native observation establishes no parent, successor or shared recipient relationship. Separate recipients remain separate. A conflict between the same native reference's provider or workspace and its existing ownership produces a repair row with no mutation or opening action. The query never guesses ownership from a title, process name or transcript ancestry.

SavedChats retains its all-workspace `{generation, chats}` response. Every row keeps its canonical chatId, current sessionId, provider, workspace, resolved title, state and resolution reason. Additive observation fields identify the source, observation time and freshness (`lastKnown` or `missing`); these do not assert that a closed program was contacted. A native listing that supplies no activity state yields Unknown. An existing live runtime can supply current state. Merely retaining a native row must never label it idle or open.

All consumers use that captured response. The chart adds team, lead and group relationships keyed by chatId; relationship-only identities remain visibly unresolved with a reason. Cleanup adds eligibility and refusal fields from host checks for each inventory row, with canonical member mappings. A page never manufactures a cleanup target. The Recycle Bin is a separate retained-record view. Workspace comparison uses the same existing path identity rules everywhere, and consumers reject stale asynchronous responses by request order.

## A chat's earlier stages

A chat that is switched to another program, compacted, or continued in another workspace goes on in a new session, and the saved conversation of that session records the one it continues. The earlier session is then a stage of the chat. It can no longer be worked in, and opening it opens the chat's newest stage. An earlier stage is therefore never a row in the Chats panel, the org chart or Clean Up Chats, and it is not counted (`continuedConversations` in `saved_chats.go`). The chat directory that agents address chats through has always left such stages out.

This rule was added on 16 November 2026. The inventory had grouped a chat's stages only through the bindings and transfers recorded in settings. Switches made before those records existed have none, so each of their stages was listed as a chat. On one person's state 60 of 103 rows were earlier stages, and one chat that had been switched fourteen times was shown as twelve chats. The parent recorded in the saved conversation is used for this one purpose, which is deciding that a session is no longer a chat of its own. It still establishes no ownership, recipient or chain membership, and the resolver described above is unchanged.

Two cases keep an earlier stage out of the lists or in them on purpose. When the newest stage has been removed or deleted, the earlier stages stay out, because the person deleted that chat. When the only newer session is the target of a switch that did not finish, the earlier stage stays in, and the chat goes on where it was.

## One chat, one newest stage

A chat is all of its stages, and one of them is its newest. The lists, opening a chat, its pins and its removal all use the same answer to which one that is. They did not on 16 November 2026: a chat was listed as its middle stage, opened as a newer stage that an older build had left on the hidden list, refused deletion from that window because a hidden session was in no inventory, and was pinned by its first stage, which the page could not match to the chat.

The newest stage is found by following the committed bindings and, where there is none, the one saved conversation that continues a stage (`sessionhost.NewestStage`, and `Continuations.Resolve` for opening). The target of a switch that did not finish is passed over (`sessionhost.UnfinishedTargets`). A switch hides its new session until it commits, and a failure after the handoff was written leaves that session hidden with the handoff never delivered, so a session that is hidden, was never handed the conversation and is continued by nothing is such a target. Opening used to follow the link into it.

The hidden list no longer keeps a session out of the lists when the session continues a listed stage and was handed the conversation (`hiddenChats` in `saved_chats.go`). Builds before the switch records existed left that mark on the newest stage of some chats, which were then worked in for weeks. A hidden session that continues nothing stays out, and so does one whose older stages are all hidden, removed or gone.

A pin is on the chat's newest stage. The settings write that commits a switch moves the pins of the stage it continues (`repinChat`), and each start moves the pins that older switches left behind (`pinsFollowChats`). A pinned chat can be deleted: its pins come off with it and come back on restore (chat-cleanup.md).

Removing a chat also takes what has no chat of its own to be removed with: the targets that unfinished switches left on its stages, and the conversations an older build's deletion left marked deleted on them.

The Chats list is read again, up to three times, when a chat changed while it was being read (`savedChatsReadAgain`). Each read is still refused whole when it meets a change. The window reads the list while its chats are opening after a start, and the single refusal was shown to the person as an error although the next read succeeded.

Removing a chat by its identity still accepts an earlier stage (`savedChatsWithStages`). A stage from an older build can have its own queue on the bus, and when its messages cannot be routed to the newer stage's queue, the person removes that stage alone from the notice about it.

Removing a chat takes its earlier stages with it (`chatWithEarlierStages` in `chat_earlier_stages.go`). The members of a removal are the chat's own members followed upward through the parents recorded in the saved conversations, oldest first, and every one of them gets the removal record, goes into the same Recycle Bin entry, comes back on restore, and is erased at final deletion. Without this an earlier stage was listed as a chat again as soon as the saved conversation that continued it had moved to the Recycle Bin, because nothing in place continued it any longer. The row in Clean Up Chats covers the same members, so its message count, its program copies and its refusals (a pin on an earlier stage, a team that an earlier stage's recipient leads) are those of the whole chat.

Three kinds of earlier stage are not taken. A stage that another chat still goes on from stays, and the last of those chats to be removed takes it. A stage with a removal in progress or in the Recycle Bin is governed by its own record. A stage that an older build removed for good is passed over, and the stages older than it are still taken. The identity and the head of the chat do not change, and the lists and the ownership of a chat still follow the bindings alone.

A saved conversation that is marked deleted is also never a row. Only a deletion by the person writes that mark, and older builds left the bus recipient or the provider's listing of such a chat in the settings, where the inventory found it and showed it with the state Unknown.

## Addresses of chats that are gone

Builds before a switch moved a chat's bus recipient to its new session, and before a removal took every stage of a chat, left recipients behind: each earlier stage of a switched chat kept one of its own, and so did a chat that an older build deleted. They shared their titles with live chats, so a chat could not be addressed by its title, and a message sent to one waited for a session that would never read it.

Each start retires them (`retireStaleAddresses` in `stale_addresses.go`). A recipient is stale only when every session that owns it is an earlier stage of a listed chat whose newest stage has a recipient of its own, a session whose saved conversation is marked deleted or whose removal is finished for good, or a stage of a chat whose newest stage is one of those. A recipient that a listed chat's newest stage owns, that an open chat is joined to, or that a hidden chat with a saved conversation owns is never stale, and one that no chat in the settings owns is left alone, because it was registered from outside the window. Retiring withdraws the address the way a chat's removal does (`bus.BinRecipient`): it takes no new message and is in no list, its queue is kept, and `bus.RestoreRecipient` brings it back. Each retirement is logged as a warning, so it is on record at the default log level.

A window of the saved layout can be of a chat that has since gone to the Recycle Bin, when it was saved under another spelling of the chat's folder and so was not closed with the chat. Opening it failed at every start with "this chat was removed". The page now asks the host whether the chat is gone (`ChatRemoved`) and closes such a window without reporting anything.

One thing is known to be unfinished. A hidden chat that continues no listed chat still has its saved conversation and its recipient, and can still be woken by a message. Older builds hid such chats where the person had asked for them to be deleted.

## Durable provider observations

The host records native identity, provider, workspace scope, safe provider title, supplied state and update time, and host observation time separately from SessionTitles. SessionTitles continues to hold the person's override. The common title resolver chooses that override first, then applies the existing judged-title rule to the retained safe provider title. Existing bus metadata supplies a title only when neither is available; no prompt preview is promoted into provider metadata. Empty or unsafe provider titles cannot erase a safe retained title. The existing private-context title filter applies before persistence and display.

The quick native-cleanup amendment introduces Settings.ListedChats for durable per-identity provider metadata. The passive stage extends that record with listing-scope and freshness metadata rather than introducing a competing cache. Observations are local evidence for admission only under the provider rules above. They do not create native recipients, bus registrations, transcript files or execution grants.

Provider-open, explicit refresh and the Agents settings inventory path record every native listing they fetch before exposing it to consumers. Capturing a listing does not open or load any returned chat. Startup and passive reads perform no Live call, native history query, credential lookup or availability probe. An opened chat retains its existing preparation and loading behavior.

Each refresh names its provider, exact workspace scope and a host-issued sequence before dispatch. Pages are collected under that refresh. Successfully observed rows may be retained after a partial listing, but absence means nothing until the final cursor is reached successfully. An error, repeated cursor, page bound, cancellation or unsupported listing leaves unobserved rows unchanged. Only a complete scoped result marks a previously recorded row missing within that scope. Missing rows keep their identity and last safe title and remain visible with their missing status. An older refresh cannot overwrite a newer accepted observation or its completeness decision. Storage failures are reported and do not produce a successful inventory update.

## Reservations and conversion

Every removal reservation, including unknown future phases and permanent tombstones, overrides native observations. Refresh cannot release a hold, recreate a binned chat or widen its archived member set. Typed quarantine and internal task/judgement purpose remain authoritative. Restore is an explicit host operation using the retained snapshot. Final deletion keeps the existing documented provider-delete path and honest unsupported or failed outcomes.

Existing installs have not recorded every historical native listing. The migration must preserve all native metadata already available in saved records and current runtime observations and persist the current listing before changing a live page to the passive source. A new empty cache cannot be described as a complete previous listing. The receiving gate compares the old visible set against the replacement before the consumer cutover; any unmatched identity blocks shipping. We cannot recover a native-only title that an older build never persisted without an authorized provider fetch. That limitation must remain visible rather than being disguised as an empty successful inventory.

Release's eventual logical model conversion includes admitted provider observations in its baseline and preserves their metadata and identity across projection reconciliation and older-build writes. External Codex cache rows remain external. Corrupt ownership or cache records block the affected operation visibly; they cannot silently disappear or release lifecycle holds. Engine retains the sole cleanup recovery loop, archive member order and existing phase semantics.

## Receiving checks and ownership

Engine owns observation persistence, SavedChats, the common resolver and authoritative cleanup rows. Release reviews the cache-to-model conversion and admission boundary. Front End moves the consumers together after that review. The current org chart correction may ship first using the existing panel cache and the standalone native-cleanup amendment; it does not claim passive startup acceptance.

The combined gate compares canonical identity, current reference, provider, workspace, title and state across the panel, chart, cleanup and counts for every offered program. It covers Unknown native-only rows, closed chats, title overrides, provider-only titles, switched chains and separate forks through rename, bin, restore and restart. Further fixtures cover failed and partial pagination, repeated cursors, missing rows, concurrent refresh ordering, corrupt scope, quarantine, future removal phases and Codex external versus imported identities. Native-only rows must complete bin, restore and supported, unsupported and failing final deletion through the real host entry points with invented providers.

The seventeen passive-startup cases and their explicit-use positive controls remain required. These gates use hidden desktops, isolated homes and headless browsers. Release accepted this contract boundary at 1a0af2df984d3b44dc5fd351233ccb492ad18e7c. That review claims no code conversion, combined acceptance or installed-product result.

## Interim implementation details

Settings.ListedChats retains native titles, freshness, scope sequence and observation time. A separate LocalTitle records already-admitted runtime metadata, which may contain an effective title; it cannot replace the provider's title. LocalObservation records that the host already held the conversation. This preserves an already-admitted runtime across restart, including a locally opened Codex chat, without admitting an external Codex listing. Native listing capture cannot set that flag.

The independent chat-inventory.json projection contains these records and ListingScopes. It uses the existing interrupted-write recovery mechanism. Settings loading reconciles it before dependent queries, preserving the fields when an older settings writer omits them. A first read of an existing ListedChats cache creates that projection before another build can discard the fields. Unreadable data blocks loading visibly. A legible identity whose projection conflicts with its settings scope remains a repair row and cannot authorize opening or deletion. Refresh cannot clear its repair condition.

SavedChats captures already-present runtime metadata locally before resolving the snapshot. This may persist a newly observed local title, but starts no provider or native session. It does not overwrite title provenance. Membership mutations capture that metadata before selecting their generation, so the first cleanup read cannot invalidate its own removal request.

OrgChart and ListChatCleanup project the captured SavedChats rows, then add relationships and cleanup eligibility. Both recheck the inventory generation before returning. OrgChart state is nullable for a provider-only row whose activity is Unknown; an observed local closed chat retains its closed state. Retired status is captured from local team records and participates in generation validation. Repair rows have no chart opening target.

RefreshProviderSessions(workspace, provider) is the explicit page trigger for one offered program. It records the native listing and returns the all-workspace SavedChats response. The page coalesces successful open/restore-open triggers by workspace and provider. Its explicit all-program refresh invokes that entry for declared programs, then rereads SavedChats once; intermediate per-program responses cannot replace the final common view. Ordinary startup and all-surface refresh call SavedChats only. Sessions and SessionsAt retain the historical discovery path with the same recording callbacks.

## Contradictory observations and legacy metadata

A native refresh that contradicts recorded program or workspace ownership persists an affected repair hold before returning its error. The original record retains its identity, scope and safe title. Its Conflicts field keeps the contradictory provider, workspace, safe title, observation time and revision as evidence without granting authority. A contradictory refresh cannot establish completeness or replace unrelated observations. Later successful listings cannot clear the hold. Removal reservations still take precedence.

A settings row written without a sequence cannot be ordered against a sequenced projection. When its title or observation metadata differs, reconciliation keeps the settings observation, retains the competing projection observation in Conflicts, and marks the row for repair. The comparison does not treat timestamps as an ordering contract. The hold and both safe observations survive subsequent writes and restart. A repair row retains its known title even when no person override exists, and it offers no execution or deletion authority.
