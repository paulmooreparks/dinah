package codexagent

import (
	"context"
)

// A bare thread.
//
// A judgement is one small question, and Codex gave it what it gives every
// thread: its own instructions, the person's memories, the texts of the
// bundled skills and the installed plugins, and the schemas of its tools.
// A one-word question cost about 26,600 tokens that way on 20 November 2026,
// and a judgement about 33,000, whatever was asked. A bare thread leaves all
// of that out by parameters and configuration keys that are documented, and
// the same question then cost about 9,600.
//
// The thread/start parameters are in the protocol's published schema
// (ThreadStartParams: baseInstructions, ephemeral, config):
// https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartParams.ts
// The configuration keys are in Codex's configuration reference
// (https://learn.chatgpt.com/docs/config-file/config-reference): web_search,
// project_doc_max_bytes, skills.include_instructions, skills.bundled.enabled,
// features.shell_tool, features.view_image, features.apps,
// features.multi_agent, features.remote_plugin, features.memories,
// plugins._default.enabled and plugins.<plugin>.enabled.
//
// What is left, about 9,600 tokens, is Codex's own and no documented key
// removes it.

// bareInstructions take the place of Codex's instructions for a coding agent.
const bareInstructions = "Answer the one message you are given, in the form it asks for. Use no tools and take no action."

// bare makes the thread/start request q, with its configuration, one for a
// bare thread. The installed plugins are read from the person's
// configuration, since each is turned off by its own name; if that cannot
// be read the thread is started with the rest.
func (c *Conn) bare(ctx context.Context, q, config map[string]any) {
	q["ephemeral"] = true
	q["baseInstructions"] = bareInstructions
	config["web_search"] = "disabled"
	config["project_doc_max_bytes"] = 0
	config["skills"] = map[string]any{"include_instructions": false, "bundled": map[string]any{"enabled": false}}
	config["features"] = map[string]any{"shell_tool": false, "view_image": false, "apps": false, "multi_agent": false, "remote_plugin": false, "memories": false}
	plugins := map[string]any{"_default": map[string]any{"enabled": false}}
	var effective struct {
		Config struct {
			Plugins map[string]any `json:"plugins"`
		} `json:"config"`
	}
	if err := c.Conn.Call(ctx, "config/read", map[string]any{"includeLayers": false}, &effective); err == nil {
		for name := range effective.Config.Plugins {
			plugins[name] = map[string]any{"enabled": false}
		}
	}
	config["plugins"] = plugins
}
