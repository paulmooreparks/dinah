---
title: The MCP root has a flag and an environment variable but no configuration key
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The root that bounds an MCP server resolves through two layers: the `--root` flag and the `DINAH_MCP_ROOT` environment variable, joined by `bench.Resolve` in `runMCP`. There is no configuration layer, though `SourceConfig` is an established rung of that same ladder everywhere else in the tool, including for the workbench pointer itself.

So somebody who wants a boundary and does not want to retype it has an environment variable and nothing else. That is the one spelling of a persistent setting this tool otherwise expects to live in configuration.

The operator asked for this on 2026-08-27, and was explicit about the shape: "I'm okay with a configuration key, but as an option, not a requirement. It's just another way of doing what `--root` does, isn't it?"

It is, and that is the whole card. Add the rung, in its documented place in the precedence order, and nothing else.

**This card is downstream of dinah-307 and must not start before it lands.** That card removes the requirement to give a root at all. Adding a configuration key first would make the requirement easier to live with instead of removing it, which is precisely what the operator rejected. Once the requirement is gone, a configuration key is a convenience for someone who wants a boundary, and nothing depends on it.

Two things a spec has to settle rather than assume.

Where the rung sits in the order. The existing ladder puts the flag above the environment variable. Read what the other settings do, since the tool already resolves several values across flag, config, editor and environment layers and the order is not the same for all of them; whatever this card does should match the sibling rather than invent a precedence.

What `status` reports. Every other resolved setting names the rung it came from, and a reader who has set a root three ways needs to be told which one won. Confirm the MCP root appears there at all today, because it may not.

Worth knowing for whoever specs it: dinah-285 will give a workbench an identifier, and if an identifier ever becomes acceptable where a path is accepted now, a configured root stops being a path and this card's shape changes with it.
