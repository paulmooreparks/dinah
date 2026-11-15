# Recycle Bin refresh correction

An external restore left its row in Clean Up Chats' Recycle Bin with enabled Restore and Delete Now actions. Authority refresh appended removed records without reconciling records absent from the new list. The installed executable reproduces this fault while all active-inventory comparisons pass.

The page now removes absent actionable bin rows and disables bin actions while reading authority. A lifecycle event received during an older read schedules another read. Completed local operation receipts remain disabled. The host's eligibility checks and deletion operations are unchanged.

The standalone branch starts at master a99581fd. Production source is 7bb00219. The regression pauses an older listing, restores externally, then releases the old response and requires the stale bin row to disappear. The fixture now maintains its authoritative lists after successful mutations. Existing refusal, confirmation, draft-preservation and frame assertions remain intact.

The cleanup, inventory, chart and chart-stability page suites pass together in 10 seconds. The hidden production executable passes all fifteen lifecycle phases, covering every offered program, workspace/search filtering, rename, removal, restore, native-row final deletion and retained provider copies. Each phase checks bin consistency against the host alongside identity, title and state agreement across all three surfaces. Documents remain unchanged, with zero remote writes and browser errors.

The executable SHA-256 is cec5153ba968546ccfab83cac295d4fe04ffcb67cb9f1fba6c0a5d067603a7a0. Machine-specific logs and pictures remain outside the public repository. Runs used isolated state and invented providers, with headless browsers and hidden desktop copies. No installation, push or trunk merge occurred. The standalone fix can be received independently of passive startup; its startup branch also retains the correction.
