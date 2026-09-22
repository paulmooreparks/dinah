Setup has written Dinah's section into AGENTS.md under C:\work\o'neil "x". Codex keeps its own configuration in TOML, which setup does not edit, so four things remain, and they are yours to do. Close any Codex session that is open first.

1. Register the `dinah` MCP server for this project. Add these lines to `.codex/config.toml` in C:\work\o'neil "x", creating the file and its directory if they do not exist:

       [mcp_servers.dinah]
       command = "dinah"
       args = ["mcp", "--tools", "all", "--workbench", "C:\\work\\o'neil \"x\"\\.dinah\\0123"]

       [mcp_servers.dinah.env]
       DINAH_ACTOR = "codex"
       DINAH_HARNESS = "codex"
       DINAH_PROVIDER = "openai"

   If the file already defines `mcp_servers.dinah` in any form, compare it with these lines and correct it rather than adding a second definition, which TOML refuses. When every session of yours runs one model, add `DINAH_MODEL = "<model>"` under `[mcp_servers.dinah.env]`. Codex also documents a `codex mcp add` command, but its documentation does not say which file the command writes, so this checklist uses the project file instead.
2. Give the agent's own `dinah` commands the same identity. Add these lines to the same file:

       [shell_environment_policy.set]
       DINAH_ACTOR = "codex"
       DINAH_HARNESS = "codex"
       DINAH_PROVIDER = "openai"

   If the file already has a `[shell_environment_policy.set]` table, or a `set = { ... }` line under `[shell_environment_policy]`, add these keys to it and change any it already carries, because TOML refuses a table or a key defined twice. Check the rest of your shell environment policy too, in this file and in `~/.codex/config.toml`, because Codex applies include filters after `set` and they can drop these three variables without a word. If the policy has a `[shell_environment_policy.filters]` table with any pattern set to `"include"`, add `"DINAH_*" = "include"` to that table. If it uses the older `include_only` array instead, add `"DINAH_*"` to the array. Do not add a `filters` table beside an `include_only` array in one file, because Codex rejects that combination.
3. Trust this project in Codex. Codex loads a project's `.codex/config.toml` only in a trusted project, so the lines above do nothing until you do.
4. Check that Codex reads the section. Where a directory holds both `AGENTS.override.md` and `AGENTS.md`, Codex reads the override, so if C:\work\o'neil "x" has an `AGENTS.override.md` the section in `AGENTS.md` is not read, and you must copy it into the override yourself. Codex also limits how much instruction text it reads, by `project_doc_max_bytes` (32 KiB unless configured). One of its pages applies that limit to each `AGENTS.md` file and another to all the instruction files it reads combined, so the section, which setup appended at the end, can be cut when this `AGENTS.md` alone, or this file together with the instruction files Codex reads before it (your Codex home's `AGENTS.md` and any above the project root), comes near 32 KiB. In either case, move the whole marked block, markers included, nearer the top. Setup finds and updates it wherever it stands in the file.

Start Codex again in C:\work\o'neil "x" afterwards, and run `/mcp` to confirm the `dinah` server is listed. Where sessions run different models, a session declares its own: set `DINAH_MODEL` on the command, or pass `model` on the MCP call, because a workbench that gates columns by tier refuses a claim from a caller that declared none, as `dinah.undeclared-model`.
