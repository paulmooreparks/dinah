# Questions and failed commands in chats

Paul asked for readable questions above the composer and quieter handling of failed commands on 24 October 2026.

The question area shares the available output space with the scrolling history. It grows to fit pending questions, up to half that space, and then scrolls internally. Its cards keep their natural height. An empty area has no height, border or padding. Answering uses the same request as Notifications and preserves the existing cancellation and retry behavior.

Failed tool steps no longer create toasts or Notifications entries. Each chat's Actions menu offers Show Failed Command Summaries, which is off by default and is saved as a UI preference for that chat. Enabling it shows one compact review link in the scrolling conversation. Further failures update that summary without moving it out of the history. Dismiss removes the summary and its retained review details. Later failures can start a new summary while the preference remains enabled.

Review Failed Commands opens a separate PUDL window for that chat. Each retained step has a disclosure with its command and output. The review keeps at most 100 steps and 256 Ki characters of text per chat, across at most 32 chats, for the current application run. It drops the oldest details first and explains when its step list is incomplete. Full records remain in the original chat logs. Closing and reopening a review window preserves the retained details; restarting the application clears this temporary review cache. Restored review windows explain when their details are unavailable.

Connection errors and unsuccessful turns retain their existing error notifications. Historical notifications remain available for dismissal. Replayed steps and steps the person rejected do not generate failure summaries.
