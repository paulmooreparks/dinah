# Workspace choice lost during an earlier switch

A person choosing B and then A while B is still opening ends up in B. The second choice is silently ignored because chooseWorkspace returns when A matches the page's old workspace, even though the pending operation is switching to B. The selector later changes to B.

A deterministic headless browser check delayed SetWorkspace(B), selected A, and completed B. The recorded final workspace and selector were B. The only host call was SetWorkspace(B).

The required correction is to retain the last requested workspace and select it after the current switch completes. The selector must retain the person's last choice. This defect has its own follow-up branch after the inventory fix ships, as requested in message 9264. No workspace-switch product code changed in the inventory patch.
