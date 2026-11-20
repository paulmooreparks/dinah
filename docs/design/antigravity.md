# Antigravity as an agent program

Paul asked on 20 November 2026 for Google Gemini in the product, and for whichever route is newest, has support ahead of it, and is most like the other integrations. That is Google's Antigravity command line. Google's developer blog said on 19 May 2026 that the Antigravity CLI is available to everyone and that the Gemini CLI stops serving Google AI Pro and Ultra on 18 June 2026, and the Gemini CLI's README says it is not optimised for Gemini 4 and later.

## How it is hosted

Antigravity is hosted as Devin is: the product starts the command line's Agent Client Protocol server, `agy acp`, which Antigravity documents at https://antigravity.google/docs/cli/acp/, and talks to it on standard input and output. It reuses the sign-in the person made with `agy` itself, and the product never signs anybody in.

The pieces are these.

- `internal/antigravityagent` starts the server. The program's session IDs have no prefix of their own, so they get `antigravity:` on Dinah's side, by the code Claude's adapter uses for `claude:` (`internal/acpprefix`).
- The provider table has its row (`internal/providers`), and the window has its module (`cmd/dinah-desktop/provider_antigravity.go`): finding the command line, starting it, and the words for signing in.
- The command line is looked for on the search path and in `%LOCALAPPDATA%\agy\bin`, where its install script puts it without adding the folder to the search path of running programs. The person can choose another in Settings.
- The page needed no change: it draws providers from what the engine reports.

## What the installed program offers

Release 1.5.13 was asked on 20 November 2026. At `initialize` it offers loading, resuming, closing and listing sessions, pictures in a prompt, and four ways to sign in. A new session reports its models, four modes (Default, Accept edits, Auto, Plan) and three options: Model, Thinking Level and Mode. A prompt's result has its token use. These are the protocol's own shapes, which the host already reads.

An opt-in live test (`TestLiveAntigravityChatAndReopen`, with `DINAH_LIVE_ANTIGRAVITY=1` and `-tags live`) sends two short turns: a new chat answers, and after its agent is stopped and the chat reopened it still has the conversation.

## What is not done

- Judgements through Antigravity use the general protocol judge. Whether a call can be made bare, as for Claude and Codex, has not been looked at.
- Steering a running turn: the documentation read so far names no way, so a message sent while a turn runs is queued.
- The applet tools and the message bus reach an agent as MCP servers given with a session. Antigravity's documentation describes its own `mcp_config.json`; whether it takes servers given with a session has not been checked, so an Antigravity chat may be without Dinah's tools for now.
- No thinking text arrived in the turns measured, although thinking tokens were counted.
- Removing a chat hides it, since the program offers no way to delete a session.
