---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 24
note: "TreeNode.Kind already documents itself as carrying one value that names no entity of the format, which is NodeGroup, so a second such value is a shape the payload already has rather than a new one. Declaring it in internal/verb rather than in internal/bench is deliberate: a collection is not an entity kind of the format, and a constant sitting beside KindCard and KindComment would be read as one and would be handed to Contains and MountOf by somebody who never checked. The name is KindCollection rather than NodeCollection because two payloads carry it and only one of them has nodes."
---
The token a payload gives a collection is "collection", declared once as verb.KindCollection beside NodeGroup, and it appears in the contents tree's root node and in an attachments listing.