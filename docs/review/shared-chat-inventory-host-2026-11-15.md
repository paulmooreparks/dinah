# Shared inventory host handover

Engine's standalone branch engine/shared-chat-inventory is based on current master c399ac9d663995c77141c4825389673c25de5a61. It includes the earlier passive startup queries and judgement correction, together with the accepted recorded-inventory contract. Front End must receive this complete host stage rather than substitute the old local-only SavedChats implementation.

## Result and bindings

SavedChats now includes admitted recorded native identities and titles with observation provenance and last-known or missing freshness. Provider-only rows retain Unknown activity. Local closed chats and team retirement retain their existing meanings. Native titles remain separate from person overrides and effective runtime titles. Cached external Codex history does not establish admission; the host's already-present runtime observation does preserve an existing admitted chat. Membership remains behind the accepted logical-member seam, with no journal or unfinished logical-model dependency.

RefreshProviderSessions(workspace, provider) refreshes only the selected offered program and returns SavedChats. Front End agreed to coalesce successful open/restore-open triggers by workspace/provider, preserve explicit Refresh Chats as all-program discovery, and reread SavedChats after that explicit batch. Startup and ordinary refresh read SavedChats alone. The existing Sessions/SessionsAt discovery path remains available and records observations too.

Native refreshes carry host-issued per-program/workspace sequences. Only completed pagination marks unseen rows missing. Failed, repeated-cursor and page-bound outcomes retain observations without inferring absence. Older accepted responses cannot overwrite a newer result. Identity-scope conflicts refuse; removal reservations and repair conditions remain authoritative. Missing identities remain visible with their safe metadata.

OrgChart adds relationships to SavedChats rows and returns null state for Unknown activity. ListChatCleanup adds authoritative eligibility to those same rows, preserving refusals and the retained Recycle Bin view. Both validate the generation before return. Cleanup captures runtime metadata before its membership selection, preserving its first-call behavior and its generation recheck.

The independent chat-inventory.json projection preserves recorded identities and title provenance when an older settings writer drops cache fields. First-read migration is covered. Corrupt projection data blocks loading; a legible scope conflict becomes an affected repair row. This does not recover native-only titles that an older build never saved. The old-visible-set comparison remains required before consumer cutover, and unmatched identities or titles block shipping.

## Verification and limitations

The focused inventory and pagination tests passed before the broader gate. They cover native Devin/Claude/Ollama population with Codex external exclusion, provider-title provenance, overrides, common chart/cleanup projection, partial and complete scopes, out-of-order responses, failed pagination, repeated cursors, page bounds, missing rows, future reservation phases, first-read migration, older-build cache-field loss, corruption and scope repair. The runtime reconstruction test verifies preservation of an already-admitted Codex chat without converting its effective title into provider metadata.

Earlier gate runs caught two integration errors. Runtime event counters advance on a read, so generation validation now compares content without those counters. Persisting previously uncaptured runtime metadata during listing also invalidated cleanup's earlier membership selection; capture now precedes that selection. An existing chart assertion caught a repair row receiving an opening head, which is now empty. None of these assertions was weakened.

The final selected gate passed with race detection and all vet analyzers: desktop 42.399 seconds and sessionhost 1.138 seconds, exit zero. The existing linker manifest warning remains. The FIFO command was:

```text
go test -tags desktop -race -vet=all ./cmd/dinah-desktop ./internal/sessionhost -run Test(SharedInventory|ListingObservation|SavedChats|NativeList|Cleanup|SessionLifecycle|OrgChart|EnabledJudgementState|VisibleJudgementInventory|DeclaredProviders|CodexPreparation|DevinPrices|EveryPageMethod) -count=1 -timeout=90s
```

The output is retained in shared-chat-inventory-host-2026-11-15/focused-race.txt. The selected gate includes the unchanged cleanup, native-row lifecycle, SavedChats, session-lifecycle and org-chart tests alongside the new inventory checks. It also selects passive judgement, declared-provider, Codex preparation, Devin prices and page-method checks. The existing Windows manual-calibration runtime crash remains an open separate defect in the earlier calibration report; this selection does not claim that test passed.

The combined headless page run and independent Quality acceptance remain pending. Front End must move all three consumers together, compare the previous visible set, run the seventeen startup cases and explicit-use controls, and prove agreement through rename, bin, restore and restart with every offered program. Native Wails startup's Claude adapter maintenance remains outside the page-startup claim and retains its separately assigned timing change. No executable installation, push, real provider contact or visible window occurred in this host stage.

## Corrections after Release review

Release blocked ba43df29 with three reproduced cases. A fresh ownership conflict returned an error without retaining a hold, an unsequenced legacy write could lose its changed title to the projection, and repair presentation could erase a safe title when no person override existed.

The correction persists contradictory observations and a repair hold before returning the refresh error. Such a refresh cannot establish completeness. Legacy metadata that cannot be ordered retains both observations and requires repair. Repair rows preserve safe titles. A later refresh cannot release the hold. The contract now describes these persistence rules and the evidence fields.

Release's three fixtures from d1b8bac4f242daa6700c95576e633b66c18c775a are included unchanged in release_inventory_review_test.go. Additional checks cover a same-program workspace conflict, restart, later-refresh hold retention, deletion refusal and preservation of both legacy observations through rewrite. The existing whole-record migration equality assertion now uses reflect.DeepEqual because the evidence collection makes the record noncomparable; it still compares the complete record.

The corrected selected gate passed with race detection and all vet analyzers. Desktop passed in 74.506 seconds and sessionhost passed in 1.145 seconds, exit zero. The command is the earlier selected gate with ReleaseInventoryReview added to its selection. Raw output is shared-chat-inventory-host-2026-11-15/correction-race.txt. An initial compile check caught the old struct comparison after the evidence collection was added; that comparison was corrected before the successful gate. The existing linker warning remains.

Release must review this correction and rerun its frozen fixtures on the receiving candidate. Front End must receive the correction before the combined acceptance is claimed. The broader page, independent Quality, both-product and installed-product gates retain their earlier scope and remain separate. No installation or real provider contact occurred.
