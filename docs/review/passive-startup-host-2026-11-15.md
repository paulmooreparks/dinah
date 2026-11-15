# Passive startup host handover

The host now offers a passive all-workspace chat inventory and product-declared provider choices. Codex preparation inspects Codex alone. Devin initialization records its executable identity without fetching prices; the existing explicit prices request still refreshes asynchronously and reports the existing event.

The contract is [passive-startup.md](../design/passive-startup.md). The implementation is on `engine/startup-passive`, based on master `ff236bd1`. Front End accepted the two query shapes in messages 9138 and 9140 and confirmed open-chat prices requests in message 9146. Engine retains host corrections. Front End owns page wiring and the combined hidden run.

## Verification

The final selected desktop regression run passed in 33.495 seconds with race checking and all vet analyzers enabled. It used this FIFO command:

```powershell
pwsh -NoProfile -File scripts/with-test-lock.ps1 go test -tags desktop,production -race -vet=all ./cmd/dinah-desktop -run '^(TestSavedChats|TestDeclaredProviders|TestCodexPreparation|TestEveryPageMethodIsListed|TestOrgChart|TestRemovalIsReported|TestSessionOwnerFactoriesStayLazyAndStable|TestDevinPrices|TestProviderRegistry)' -count=1 -timeout 90s
```

The new host fixtures cover a switched chain, its separate fork, a chat in another workspace, an already-present runtime, a transcript-only chat, user hiding, a future removal phase, missing metadata and conflicting ownership. Repeated unchanged reads keep the generation token; a saved rename changes it. Provider-only registrations supply no local chat evidence. Counted preparation and transport observations show no provider starts, requests or session loads from `SavedChats` or `DeclaredProviders`.

The explicit native listing and availability scan provide positive controls for the same counted ports. Codex preparation inspects exactly one provider and retains its missing-executable refusal. The prices fixture starts with a different cached executable and proves that recording the initialized source fetches nothing; a later explicit request reads the current executable and emits prices normally. Existing org-chart and prices checks also passed in the selected run.

The complete desktop vet command passed with exit zero:

```powershell
pwsh -NoProfile -File scripts/with-test-lock.ps1 go vet -tags desktop,production -all ./cmd/dinah-desktop
```

The Windows linker retained its existing `.rsrc merge failure: multiple non-default manifests` warning. Earlier focused runs passed in 3.541, 11.629 and 11.719 seconds as the source grew. No assertion was weakened. No Engine runner remains active or queued.

## Receiving limits and next action

These are local host fixtures with invented homes, provider connections and price readers. They do not establish the seventeen-case combined page result, a complete desktop runtime regression or native-window startup behavior. No browser, visible desktop, real provider account or customer data was used. Nothing was pushed, installed or merged into trunk.

Native `App.startup` still schedules Claude adapter catch-up. That path can inspect Node/npm and install an adapter when an earlier release is present, even with no Claude chat. Lead accepted this limit in message 9145 and assigned its timing change as a separate bounded stage. Its one-week policy, release checks and notices must remain unchanged. This host handover therefore supports a zero-contact claim for page startup only.

Front End should stack the host commit with `frontend/startup-passive`, retain the baseline counts at `d76e4a54`, and run the seventeen zero-contact cases plus opened/restored-chat positive controls. Quality's independent acceptance follows that combined handover. The separate Claude maintenance change comes after this stage.
