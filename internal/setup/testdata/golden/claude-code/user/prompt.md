Setup has written the user-scope configuration for Claude Code under <BASE>. Close any Claude Code session that is open before you run setup, because Claude Code writes some of these files itself when you change a setting. Four things remain, and they are yours to do.

1. At project scope, open Claude Code in <BASE> and approve the `dinah` server when Claude Code asks about the servers in `.mcp.json`. Run `/mcp` in Claude Code to confirm it is connected.
2. At project scope, make sure `.claude/settings.local.json` is ignored by version control, because it carries this machine's agent name and model. Decide whether `.mcp.json` should be committed: it carries this machine's absolute path to the workbench, so commit it only where every checkout sits at that path.
3. At user scope, register the MCP server yourself, because Claude Code keeps user-scope servers in a file setup does not write:

       claude mcp add --env DINAH_ACTOR=helper --env DINAH_HARNESS=claude-code --env DINAH_PROVIDER=anthropic --transport stdio --scope user dinah -- dinah mcp --tools all

   When every session of yours runs one model, add `--env DINAH_MODEL=<model>` beside the other `--env` pairs, before `--transport`.
4. Where sessions run different models, which Claude Code subagents usually do, a session declares its own model: set `DINAH_MODEL` on the command, or pass `model` on the MCP call. A workbench that gates columns by tier refuses a claim from a caller that declared none, as `dinah.undeclared-model`.

Start Claude Code again afterwards, because it reads these files when a session starts.
