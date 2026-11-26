# Direct-message hop warning

A direct message beyond its workspace pair's automatic-handoff limit could receive a plain send receipt even though it would not wake its recipient. The receipt now names the pending recipient and effective limit, explains the wait, and suggests either a shared group chat or asking the person to configure a trusted link in Group Chats.

The bus observation uses the existing `hopLimitSQL` expression. It ignores delivered or cancelled deliveries, removed recipients, recycled recipients, unlimited links and group posts. It does not update delivery state or the loop guard. Recipient names use the existing display-title logic.

The group-chat design note documents the default three-hop guard, self-trust links, the offered limit of ten, and the current development workspace's unlimited self-link. Group turn budgets remain separate from direct-message hop limits.

The focused receipt test passed for the default boundary, a past-limit message, custom limits on either side of the boundary, an unlimited link, a self-link and a group post. It checks the effective number and the next-action wording, and verifies that receipt generation does not change the bus counter. The locked, uncached Go gate passed `internal/bus`, `internal/mcpserver` and `internal/profile` with exit zero.

No browser suite was run because no page or screen changed. Integration, the full Go gate and installed-program verification remain the integrator's next steps. No merge, push or install was performed on this branch.
