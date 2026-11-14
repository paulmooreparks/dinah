# Restart without a waiting update

Pause Chats and Restart is always available in the application menu. It uses the existing pause record, turn/write admission, idle wait, two-minute notification and continuation delivery. Restart When Chats Finish remains an update command. Restart Now can finish either kind of pending restart after the existing confirmation.

The shared requestRestart function selects its target when the restart begins. If an update is ready, it selects the watched installed program. Otherwise it uses os.Executable. A later update notification cannot change that pending target. Update watching fills its metadata into the existing state so that starting the watcher cannot erase a pending same-program restart. Cancelling releases the holds and preserves the existing cancellation continuation rules.

The durable pause record adds withoutUpdate. An absent field retains the interpretation of older update records. A same-program continuation says the program restarted without claiming an update. The steering request is valid for either kind of restart. The current update state and bridge methods remain shared; no second restart protocol is introduced.

The program is restarted by its executable path with the existing --after process wait. This does not promise preservation of arbitrary launch arguments or of an old executable image if somebody replaces its file while it runs. Neither was part of the existing restart contract. Native storage persistence remains outside this change.

## Verification

Verification is pending on engine/restart-any-time, based on master 5db16def. Only Go tests and hidden headless checks are permitted. The implementation has not been merged, pushed or installed.
