# Chat skill discovery

The chat picker had switched to the human skill catalogue, which omitted personal Devin packages and never queried Codex. The Skills button and slash completion therefore showed no skills even when the agent had installed skills.

ChatSkillCatalog now filters shared skills to the chat workspace and provider, restores personal agent packages, and queries Codex through its documented skills/list method. Codex owns discovery of its installed skills and enabled state. Discovery does not create a thread or run a model turn. The protocol reference is https://learn.chatgpt.com/docs/app-server#skills.

Provider packages retain their own files. Dinah reads them through OpenCatalogFile and presents personal packages as read-only in the Skills browser. A selected package is rediscovered before sending, checked against its revision, and supplied with its instruction path so relative resources have a base. The scoped skills applet remains limited to workspace-owned packages; personal discovery does not widen its authority.

Both chat pickers use the same result. Keyboard selection uses stable package identities, including when names collide. Discovery errors remain visible alongside successful results, and Refresh Skills retries discovery. Provider or workspace changes discard stale asynchronous responses.

The live discovery-only check found 29 usable Codex skills and no discovery problems. The focused browser checks cover both pickers, selecting a duplicate name, preparation before sending, visible failure and retry availability. Go checks cover provider isolation, disabled packages, personal packages, stale revisions and partial discovery failure.

Paul's README logo addition is carried into the fix/desktop-skills-and-branding branch with its original relative image path, which resolves to the Clear Chime SVG in this worktree. His original studio edit is preserved.
