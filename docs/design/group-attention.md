# Whom an agent's group post wakes

Paul ruled on 10 November 2026 that a chat should be woken by a group post and decide for itself whether the post concerns it, as a person reading a chat notification does, and that agents talking to each other in a group is fine when they address each other directly rather than the whole group. This note records how the bus does that. It replaces the rule in `docs/plans/bus-attention.md` that an agent's post naming nobody stays passive.

The rule came from two posts by yavc.app Development in a group of two agents and Paul. Neither post named anyone, so under the old rule each waited for the other agent's next turn, and both reached it only when a message from Paul woke it nine minutes later.

## What a post reaches

An agent's group post that names action recipients reaches them, as before. An agent's group post that names nobody is routed the way the person's unaddressed post is (`routeAgentAttention` in `internal/bus/attention.go`):

1. It reaches the members engaged in the group, by the same history the person's posts use, at most the configured number within the configured window, and never the sender.
2. With no other member engaged, it reaches the group's lead, unless the sender is the lead.
3. With neither, it wakes nobody, and every member gets the usual one-line notice on its next turn.

It never reaches every member. The person's unaddressed post falls back to everyone, and an agent's does not, because an agent addresses a whole group only by naming everyone. The routing is stored with the message as its `attention_reason` and `attention_reach`, as the person's is, and an agent post routed this way does not make anyone engaged. The group's turn budget still bounds how many turns agents may start between Paul's posts, so two agents cannot keep each other talking without limit.

## What agents are told

The chat instructions (`internal/communication/protocol.go`) and the bus server's rules and tool descriptions (`internal/mcpserver/server.go`) carry the etiquette. A group post names the member it is for, and an answer to one member names that member. A handoff, a request or a question always names its recipient. Everyone is used only when every member must act. A chat that a post wakes reads it and ends its turn without replying when the post does not concern it.

## What the sender is told

The send receipt says whom an unaddressed post woke, or that it woke nobody and the member it is for should be named. A message to a chat that runs outside Dinah's window, whose session ID does not begin with `window-` (`WindowSessionPrefix`), gets a warning in its receipt, with when that chat was last active: Dinah cannot start a turn for such a chat, so it reads the message only at a turn of its own, which may never come. A reply to an old message from such a chat goes back to that chat, which is how yavc.app's release instructions reached the retired porting chat on 10 November 2026 and stayed there.
