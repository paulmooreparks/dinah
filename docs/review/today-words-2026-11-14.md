# Review of today's visible wording

The source commit is afbac6d7 on frontend/today-words.

This branch reviews the text introduced since yesterday's last source, 4fe5a52c, against master ae66547a. It changes wording in Decisions, Broadcast, notification actions, Updates, Set-aside Programs, Working with Chats, the What's New card and stand-alone Docs help. It also changes the Go errors and refusal reasons that those pages can show. Pause and restart notes and menu labels were also read. The restart refusals now use complete sentences.

The interface uses request for an item in Decisions, chat for a conversation, and message for what a chat sends. Send results explain whether sending was confirmed and whether the asking chat may have read the answer. Set-aside Programs explains the protected copy as the copy kept for going back. Labels may be noun phrases; explanatory prose uses complete sentences.

No command, permission, error code, saved identity, retry rule or layout was changed. A token comparison checks that the production JavaScript and Go edits affect only string literals. The HTML comparison checks that the markup structure stays the same. Tests change only where an assertion expects the previous words. Saved messages and notification records from earlier runs are not rewritten.

## Before and after

The tables list each distinct changed string in its source file. Text assembled from several parts is shown one changed part at a time. Repeated occurrences in the same file share a row.

### cmd/dinah-desktop/background_limit.go

| Before | After |
| --- | --- |
| choose a background chat limit from 1 to %d | Choose a maximum number of background chats from 1 to %d. |

### cmd/dinah-desktop/frontend/decision-answer-card.js

| Before | After |
| --- | --- |
| As recommended. | You chose As Recommended. |

### cmd/dinah-desktop/frontend/decision-drafts.js

| Before | After |
| --- | --- |
| The saved decision draft is invalid. | Your saved draft could not be read. |
| The saved delivery identity is invalid. | Your saved send request could not be read. |
| The saved broadcast identity is invalid. | Your saved broadcast could not be read. |
| The saved request identity is invalid. | Your saved request could not be read. |
| The saved delivery receipt is invalid. | Your saved send result could not be read. |

### cmd/dinah-desktop/frontend/decision-record.js

| Before | After |
| --- | --- |
| Reading the request. | Loading the request. |
| Reading the original request without changing it. | Loading the original request without changing it. |
| This question has no answer. | This question has not been answered. |
| This request is unavailable. Its answer remains in the transcript. | This request is unavailable. You can still read its answer in the chat. |
| Retry Request Lookup | Try Again |

### cmd/dinah-desktop/frontend/decisions-applet.js

| Before | After |
| --- | --- |
| Reading requests. | Loading requests. |
| Draft could not be saved.  | Your draft could not be saved.  |
| This request is not in the loaded open list. Its draft is kept. Refresh or load earlier requests. | This request is not in the list of open requests. Your draft is saved. Refresh or load earlier requests. |
| Saving draft. | Your draft is being saved. |
| Draft saved. | Your draft is saved. |
| Delivery is unconfirmed. Check its status before sending again. | Sending has not been confirmed. Check the result before sending again. |
| Broadcast queued on the bus for the previewed chats. | Your message is waiting for the chats in the preview. |
| Delivered to the bus. The asking chat may not have read it yet. | Your reply was sent. The asking chat may not have read it yet. |
| Check Delivery | Check Send Result |
| The selected request is not in the loaded page. Load earlier requests or choose another. Saved drafts are retained. | The selected request is not in this list. Load earlier requests or choose another. Your draft is saved. |
| Group Mention | Group Message |
| This closes the plain request without sending a reply to  | This closes the request without sending a reply to  |
| The delivery response could not be confirmed. | The send result could not be confirmed. |
| Delivery is unconfirmed.  | Sending has not been confirmed.  |
| The reply committed, but its local state could not be saved.  | The result was confirmed, but could not be saved on this computer.  |
| Delivery is still unconfirmed.  | Sending has still not been confirmed.  |
| The preview lists chats working when the engine reads them. | The preview lists chats that are working when you request it. |
| The preview lists every eligible chat on the bus. Closed chats wait for a background place. External terminals cannot be opened by Dinah. | The preview lists all available chats. Closed chats wait until there is room to open them in the background. Chats in separate terminal windows cannot be opened here. |
|  open requests loaded. |  awaiting an answer |
| No open requests. | There are no open requests. |

### cmd/dinah-desktop/frontend/decisions-context.js

| Before | After |
| --- | --- |
| Reading the group conversation. | Loading the group conversation. |
| Conversation context is unavailable in this build. | Earlier group messages are unavailable in this build. |
| The conversation response is invalid. | The earlier group messages could not be loaded. |
| Retry Conversation | Try Again |

### cmd/dinah-desktop/frontend/document.html

| Before | After |
| --- | --- |
| A saved file keeps its unsaved draft for recovery when you reopen it. | Your unsaved changes to an existing file are kept so that you can recover them when you reopen it. |

### cmd/dinah-desktop/frontend/index.html

| Before | After |
| --- | --- |
| All addresses chats on the bus, including closed chats. | All includes every available chat, including closed chats. |
| Active addresses chats that are working now, | Active includes chats that are working now, |
| Closed chats wait for a background place rather than all opening at once. | Closed chats wait until there is room to open them in the background. |
| A card names the chat or group, | Each card shows which chat or group asked, |
| Open Decision takes you to its row in Decisions, | Open Request in Decisions takes you to its row, |
| A changed limit applies at the next wake check | A changed limit applies the next time messages are checked |
| Other working chats are waited for. | The program waits for other working chats to finish. |
| the recorded chats | the chats from before the restart |
| Running copies and the newest fallback stay protected. | Running copies and the newest copy kept for going back cannot be removed. |
| Process inspection warnings will appear here. | Any problems checking running programs will appear here. |
| Selected total: 0 bytes. | The selected files total 0 bytes. |
| Sizes are logical file lengths. Hard links and compression can change the physical disk space freed. Select at most 100 copies per removal. | The sizes shown are file sizes. Shared file storage and compression can change the disk space freed. Select at most 100 copies at a time. |
| Release has not been checked. | An update has not been checked for. |
| No download in progress. | No download is in progress. |
| Repair details from the service | Repair details |
| How many closed chats messages can open in the background, from 1 to 64. The default is 8. Changes apply on the next message check without restarting. Lowering the limit leaves working chats open; new chats wait for a place. Chats with visible windows and startup-warmed chats are outside this limit. | Messages can open up to this many closed chats in the background, from 1 to 64. The default is 8. Changes apply the next time messages are checked, without restarting. Lowering the limit leaves working chats open. Other chats wait until there is room to open them. Chats with visible windows and chats prepared at startup do not count towards this limit. |

### cmd/dinah-desktop/frontend/notifications-applet.js

| Before | After |
| --- | --- |
| Retry Waiting Delivery | Retry Waiting Message |
| Open Decision | Open Request in Decisions |

### cmd/dinah-desktop/frontend/settings-set-aside.js

| Before | After |
| --- | --- |
|  selected; total logical size:  |  selected. Combined file size:  |
| <br>Running process IDs:  | <br>Running program IDs:  |
| This copy is no longer listed. Its row is kept in place. | This copy is no longer listed. |
| No process inspection warnings were reported. | No problems were reported while checking running programs. |
|  copies listed; total logical size:  |  copies are listed. Combined file size:  |
| Reading set-aside programs. | Loading set-aside programs. |
| The program inventory is invalid. | The list of program copies could not be read. |
|  The previous rows are retained; removal is disabled. |  The previous list is still shown. You cannot remove copies until it has been refreshed. |
|  copies with a total logical size of  |  copies with a combined file size of  |
| <br>The service checks protection again before removing each copy. | <br>Each copy is checked again before removal to protect running programs and the copy kept for going back. |
| Reported removed size:  | Combined size of removed files:  |
| Removal completion is unconfirmed.  | Removal has not been confirmed.  |
|  The list will be read again. No removal is retried automatically; select and confirm again if needed. |  The list will be refreshed. Select the copies and confirm again if you still want to remove them. |

### cmd/dinah-desktop/frontend/settings-updates.js

| Before | After |
| --- | --- |
| This build has no configured release or agent-settings service. | Updates and agent settings checks are unavailable in this build. |
| No newer release was found. The service reports  | No newer release was found. The current version is  |
|  and latest release  |  and the latest release is  |
| No download in progress. | No download is in progress. |
| The program inventory was not a supported status result. | The installed and running program details could not be read. |
| The release reported more than three components. | The list contains more programs than this page can show. |
| <br>No component was reported. | <br>No program was reported. |
| older copy; restart its owner when ready | older copy; restart the program that started it when ready |
| owner not reported | not reported |
| <br>No longer reported as running. Its row is retained in place. | <br>This copy is no longer reported as running. |
| Inventory read. One or more installed components are missing or unidentifiable. | Some installed programs are missing or their versions could not be identified. |
| Installed versions read. Running versions and source identifiers are reported separately. | The installed versions are shown. Running versions are listed separately. |
|  older running copies reported. An external client needs its own restart; this page stops none. |  older copies are still running. Restart the programs that started them when ready. |
|  The rows retain the last successful reading. |  The previous details are still shown. |
| Restart to Update is available in Notifications.  | You can restart to use the update from Notifications.  |
| No desktop restart is currently offered.  | No update is waiting for a restart.  |
|  The last successful release reading remains visible. |  The previous release details are still shown. |
| No newer compatible version found. | No newer compatible version was found. |
| Not checked yet. | This package has not been checked yet. |
| Shows the installer preview before applying this package. | Preview the changes before updating this package. |
| This package is no longer reported as installed. Its row is retained in place. | This package is no longer reported as installed. |
| Package check finished; no newer compatible installed-package versions were found. | The check finished. No newer compatible versions were found for your installed packages. |
| Installed packages read. Check to look for newer versions. | Your installed packages are shown. Check for newer versions. |
| No skill package is installed in this scope. | No skill packages are installed for you or this workspace. |
|  Rows retain the last successful reading. |  The previous details are still shown. |
| The repair service did not return supported findings. | The agent settings check could not be read. |
| The repair preview lacks its token or proposed destination. Nothing can be repaired from this preview. | The repair preview is incomplete. Check agent settings again before repairing them. |
| Repaired | This entry was repaired. |
| Would repair this owned entry | This entry would be repaired. |
| Current; no change | This entry is up to date. No change is needed. |
| No entry; none will be added | There is no entry. None will be added. |
| Custom entry kept; ownership or arguments differ | This entry has custom settings and will be kept. |
| Settings could not be read | The settings could not be read. |
| user scope | your settings |
| This entry was not reported by the latest check. Its row is retained in place. | This entry was not found in the latest check. |
| Some settings could not be repaired. Read each outcome; no automatic retry occurred. | Some settings could not be repaired. Read the result for each entry before trying again. |
| Repair finished. Existing agent processes are unchanged. | The repair finished. Running agents have not been restarted. |
| Check finished. Review the proposed destinations before pressing Repair Agent Settings. | The check finished. Review the proposed changes before pressing Repair Agent Settings. |
| Check finished. No owned stale entry needs repair. | The check finished. No entries need repair. |
| Repair did not confirm completion:  | The repair could not be confirmed:  |
| <br>Waiting for the version inventory. | <br>Waiting for installed and running versions. |

### cmd/dinah-desktop/restart_left.go

| Before | After |
| --- | --- |
| These chats could not reopen. Their carry-on messages are waiting. Open each chat to review its recovery options. | These chats could not reopen. Messages asking them to continue are waiting. Open each chat to review its recovery options. |

### cmd/dinah-desktop/update.go

| Before | After |
| --- | --- |
|  is waiting to restart, so nothing is saved or published until it has restarted. The document keeps its changes; save it after the restart, or cancel the restart |  is waiting to restart. You cannot save or publish until it has restarted. Your changes are kept. Save them after the restart, or cancel the restart. |
|  restarts when the running turns end. Your message is still in the box; send it after the restart |  will restart when the working chats finish. Your message is still in the box. Send it after the restart. |
| a restart is already waiting | A restart is already waiting. |
| finding the running program: %w | The running program could not be found: %w |

### cmd/dinah-desktop/whats_new.go

| Before | After |
| --- | --- |
| Decisions keeps questions and recommendations in one place. Notifications keeps requests you missed, and Working with Chats explains Broadcast and Pause Chats and Restart. Settings lets you choose how many chats can open in the background and remove older program copies while protecting running copies and the newest fallback. Products with Updates also show installed and running versions there. | Open Decisions to read questions and recommendations and send your answers. Open Notifications to find requests you missed, and read Working with Chats for help with Broadcast and Pause Chats and Restart. In Settings, you can choose how many chats open in the background and remove older program copies. Running copies and the newest copy kept for going back cannot be removed. If your product has Updates, you can compare installed and running versions there. |

### internal/bus/answer_origin.go

| Before | After |
| --- | --- |
| this decision is no longer available | This request is no longer available. |
| decision identity has the wrong kind | This request could not be identified. Refresh the list. |

### internal/bus/decision_broadcast.go

| Before | After |
| --- | --- |
| choose active or all and give broadcast text | Choose Broadcast to Active or Broadcast to All and write a message. |
| give an operation identity and preview token | Preview the recipients again before sending. |
| This broadcast preview has already been sent. Preview again to send a new message. | This message has already been sent to the chats in the preview. Preview again to send a new message. |

### internal/bus/decisions.go

| Before | After |
| --- | --- |
| give an operation identity and 1..20 numbered questions | Ask between 1 and 20 numbered questions. |
| each question needs a unique positive number, text and recommendation | Give each question a different positive number, its text and a recommendation. |
| invalid question choice | One of the suggested answers could not be read. |
| operation identity was used with different questions | The questions changed after this request was saved. Submit a new request. |
| invalid decision identity | This request could not be identified. Refresh the list. |
| The asking chat or its group destination is no longer available. | The asking chat or group is no longer available. |
| invalid decisions cursor | Earlier requests could not be identified. Refresh the list. |
| operation identity was used with different input | This request changed after it was saved. Submit a new request. |
| invalid operation identity | This action could not be identified. Refresh and try again. |
| give an operation identity and answers | Provide an answer before sending. |
| question answered twice | The same question was answered twice. |
| invalid answer question number | The answer does not refer to a valid question number. |
| give answer text or as recommended with empty text | Write an answer or choose As Recommended. |
| This decision is no longer available. Refresh the list. | This request is no longer available. Refresh the list. |
| decision identity has the wrong kind | This request could not be identified. Refresh the list. |
| question is unknown | This question could not be found. Refresh the request. |
| This decision has already closed. Refresh the list. | This request is already closed. Refresh the list. |
| This decision changed. Refresh it before sending your answers. | This request changed. Refresh it before sending your answers. |
| The asking chat or its group destination is no longer available. No answer was sent. | The asking chat or group is no longer available. No answer was sent. |
| One of these questions has already been answered. Refresh the decision. | One of these questions has already been answered. Refresh the request. |
| This decision changed. Refresh it before dismissing. | This request changed. Refresh it before dismissing it. |

### internal/bus/group_context.go

| Before | After |
| --- | --- |
| the target message is not in this group | This message is not in this group. |

### internal/programs/set_aside.go

| Before | After |
| --- | --- |
| A copy has an unrecognized date; the fallback cannot be established. | The date of a copy could not be read. No copies can be removed until the newest copy kept for going back can be identified. |
| A running process uses this executable file. | This copy is in use by a running program. |
| choose between 1 and 100 set-aside copies | Choose between 1 and 100 set-aside copies. |
| selection IDs must be unique 64-character hexadecimal values | The selected copies could not be identified. Refresh the list and select them again. |
| selection IDs must be hexadecimal | The selected copies could not be identified. Refresh the list and select them again. |
| the copy changed or is now protected; refresh the list | This copy changed or is now protected. Refresh the list. |

### internal/programs/set_aside_other.go

| Before | After |
| --- | --- |
| removing set-aside programs is supported only on Windows | Removing set-aside programs is supported only on Windows. |

### internal/programs/set_aside_windows.go

| Before | After |
| --- | --- |
| the path is not an ordinary executable file | This copy is not a regular program file. |
| the process list exceeded the inspection limit | There are too many running programs to check safely. |
| %d processes could not be inspected or exited during inspection. Windows checks executable use again when removing a copy. | %d running programs could not be checked or closed during the check. Windows checks whether a copy is in use again before removing it. |
| Windows refused exclusive deletion access; the copy may be in use or access is denied | Windows would not allow removal. This copy may be in use, or you may not have permission to remove it. |
| the selected executable changed; refresh the list | The selected program copy changed. Refresh the list. |
| Windows refused deletion; the executable may be running or access is denied | Windows would not allow removal. This copy may be running, or you may not have permission to remove it. |

## Verification

The three complete touched Go packages passed under the shared test queue. The bus package took 169.831 seconds, programs took 8.361 seconds, and the desktop package took 340.923 seconds. After the last restart-refusal edits, focused restart, pause and update tests passed in 28.022 seconds. All commands exited zero. The final background-limit and frontend guard checks passed in 1.331 seconds.

The first browser run checked 24 affected suites in 103 seconds. Twenty passed. Four still asserted fragments of the old wording, including background place, last successful reading and lacks its token. Those text assertions were updated, preserving their other conditions. The four corrected suites passed together in seven seconds, exit zero. The first run and its failures remain in the temporary today-words-browser-20261114 folder; the correction is in today-words-browser-corrected-20261114. The passing theme sample includes both widths and both modes in two skins. Its screenshots show the revised words with the existing focus, contrast and geometry checks.

The browser checks cover Decisions reading, answer cards, attention links, context, broadcast, send recovery, races, saved drafts, errors, Set-aside Programs, update states and errors, packages, repairs, missing services, the help guide and chat recovery. The hidden native journey scripts were updated only where they assert the changed words. They were not rerun for this wording-only task. No live service or native window was used. Agent messages, release notes, user documents, saved historical records and errors supplied by external programs are shown as received and are not rewritten by this patch.

A further headless check used the actual revised What's New body from Go and the revised stand-alone Docs help, with invented service data. Their 16 combinations passed in Standard and Glass, light and dark, at both widths. Screenshots and measurements are in the temporary today-words-real-copy-20261114 folder.

The source comparison found 155 distinct production text changes. JavaScript and Go differ only inside string literals, and the HTML markup structure is unchanged. A separate comparison of the edited browser checks confirms that their executable tokens differ only in string literals. No assertion, timeout, numeric expectation or operation was relaxed. Syntax and diff whitespace checks passed.

No real-home folder was written, no credential file was read, and no push, merge or installation occurred. Front End retains ownership for findings. The requested next action is the lead's wording review and merge.
