# Group message context

The page query is `BusGroupMessageContext(group string, messageId int64) ([]BusMessage, error)`. JavaScript calls `app().BusGroupMessageContext(group, messageId)` and receives the existing `BusMessage[]` shape, with an empty array when no preceding message exists.

The result contains up to five messages strictly before the target in its own group, oldest first, including membership lines as ordinary group history does. The target and all later messages are excluded. The call refuses an unknown group, a missing target or a target belonging to another group. It changes no delivery, read receipt or change counter. `pageMethods` classifies it as `pageQuery`.

The target check reads only its group key by primary key. Context uses the existing `(room,id)` index to select the five newest IDs below the target before reading their bodies and attachments. No unbounded tail is fetched, even for an old target. The existing `BusGroupMessagesFrom` contract remains unchanged.

Tests cover an old target followed by hundreds of messages, two preceding messages, no preceding messages, missing and wrong-group targets, unchanged read state and change counter, indexed selection, the App bridge and its page classification. No browser or live state is needed.

The implementation is frozen at `13079771b8f8e5b4c8f9bd9e4762cacea8583c4b` on `engine/group-context`. Through FIFO, the focused bus tests passed in 0.340 seconds, the focused App and page-method tests in 0.636 seconds, and the complete bus package in 45.771 seconds. The first focused run exposed fixture defects: its chat lacked the title required for group membership, and its plan assertion expected `rowid` where SQLite displayed the declared `id`. The corrected assertion still requires the group-history index and the upper ID bound. No full desktop suite, visible interaction, push, merge or installation occurred.
