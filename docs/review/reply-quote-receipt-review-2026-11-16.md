# Reply quote recipient evidence

This source review covers mockup 6216eb9e and Lead's question in message 10104. It changes no implementation and makes no runtime claim.

The stored delivery rows can establish historical addressing. insertMessage resolves and fans out recipients inside its transaction, then inserts one deliveries row per recipient. Group resolution supplies the members at that send. Joining the room later does not create a delivery row for an earlier message. A session recipient row for the original message and the canonical authenticated recipient therefore supplies the proposed stricter historical check without consulting today's membership.

The design should call this check "was sent to this chat". A waiting row proves addressing, rather than agent receipt. Deliver sets delivered_at and delivered_session when the bus hands the message over. Even those columns do not prove that the native agent consumed the prompt. A workspace row must never grant every chat in that workspace access: when delivery occurred, delivered_session identifies the actual selected chat. Human and human-room rows similarly cannot authorize an agent quote. Cancelled undelivered rows must not qualify as successful handover.

If the intended policy requires completed bus handover, require a nonempty delivered_at and an exact delivered_session match to the authenticated canonical bus session. That is narrower than historical addressing and can withhold a quote for an original still waiting in the same incoming batch. The implementation must make this choice explicit. The mockup's phrase "was not sent to you" fits historical addressing better than completed handover.

ReadMessage is unsuitable as this check. It permits a current group member to read the group's older messages. That behavior is broader than the new quote policy proposed in the mockup. Neither current membership nor a globally accessible Message lookup should populate a quote.

FormatOne is a pure formatter and has no recipient identity or database. Authorized quote metadata must be attached while the bus still has the authenticated recipient, before formatting, and must also govern attachment file names and sender metadata. The original message author may need an explicit authored-message rule because fanOut excludes the sender; that must be stated rather than inferred from a missing delivery row. Lookup errors and missing proof should withhold sender, text and attachment names.

The person-facing group, Inbox and thread representations need their own authenticated reader check. The agent's delivery restriction must not accidentally stand in for the person's existing access. This review grants no new read authority to either surface.
