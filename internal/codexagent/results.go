package codexagent

import (
	"bytes"
	"encoding/json"
	"fmt"

	"studio/internal/acp"
)

// toolResultContent exposes documented model-visible results through ACP.
// Arguments and private MCP _meta are excluded from this visible content.
func toolResultContent(b json.RawMessage) []any {
	var p struct {
		Type   string `json:"type"`
		Output string `json:"aggregatedOutput"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			Content    []json.RawMessage `json:"content"`
			Structured json.RawMessage   `json:"structuredContent"`
		} `json:"result"`
		Changes []struct {
			Path string `json:"path"`
			Diff string `json:"diff"`
			Kind struct {
				Type     string `json:"type"`
				MovePath string `json:"move_path"`
			} `json:"kind"`
		} `json:"changes"`
	}
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	var content []any
	add := func(block acp.ContentBlock) {
		content = append(content, map[string]any{"type": "content", "content": block})
	}
	text := func(value string) {
		if value != "" {
			add(acp.ContentBlock{Type: "text", Text: value})
		}
	}
	text(p.Output)
	if p.Type == "mcpToolCall" {
		if p.Error != nil {
			text("Tool failed: " + p.Error.Message)
		}
		if p.Result != nil {
			for _, part := range p.Result.Content {
				var block acp.ContentBlock
				if json.Unmarshal(part, &block) == nil && block.Type != "" {
					add(block)
				}
			}
			if len(p.Result.Structured) > 0 && !bytes.Equal(p.Result.Structured, []byte("null")) {
				var formatted bytes.Buffer
				if json.Indent(&formatted, p.Result.Structured, "", "  ") == nil {
					text("Structured Result:\n" + formatted.String())
				}
			}
		}
	}
	if p.Type == "fileChange" {
		for _, change := range p.Changes {
			value := fmt.Sprintf("File: %s\nChange: %s\n", change.Path, change.Kind.Type)
			if change.Kind.MovePath != "" {
				value += "Moved to: " + change.Kind.MovePath + "\n"
			}
			// Codex supplies a unified diff, not complete old and new text.
			text(value + "Diff:\n" + change.Diff)
		}
	}
	return content
}
