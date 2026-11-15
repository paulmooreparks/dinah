// Command startup-peer is an invented protocol peer for the startup audit.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func record(kind string, value any) {
	f, err := os.OpenFile(os.Getenv("DINAH_STARTUP_PEER_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "provider": strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe"), "kind": kind, "value": value})
}
func main() {
	record("process", os.Args[1:])
	if os.Getenv("DINAH_STARTUP_PEER_REFUSE") == "1" {
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("v24.0.0")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "models" {
		fmt.Println(`{"models":[]}`)
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 2<<20)
	encode := json.NewEncoder(os.Stdout)
	notify := func(method string, params any) {
		_ = encode.Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	}
	options := []any{map[string]any{"id": "model", "category": "model", "name": "Model", "type": "select", "currentValue": "fixture", "options": []any{map[string]string{"value": "fixture", "name": "Invented Model"}}}}
	next := 0
	for scan.Scan() {
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scan.Bytes(), &q) != nil {
			continue
		}
		record("request", q.Method)
		record("request-params", map[string]any{"method": q.Method, "params": q.Params})
		if len(q.ID) == 0 {
			continue
		}
		var result any = map[string]any{}
		switch q.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": 1, "agentInfo": map[string]string{"name": "invented-peer", "version": "1.0.0"}, "agentCapabilities": map[string]any{"loadSession": true, "sessionCapabilities": map[string]any{"list": map[string]any{}, "close": map[string]any{}}}}
		case "session/list":
			result = map[string]any{"sessions": []any{}}
		case "thread/list":
			result = map[string]any{"data": []any{}}
		case "model/list":
			result = map[string]any{"data": []any{map[string]any{"id": "fixture", "model": "fixture", "displayName": "Invented Model", "defaultReasoningEffort": "medium", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "medium", "description": "Medium"}}}}}
		case "skills/list":
			result = map[string]any{"data": []any{map[string]any{"cwd": os.Getenv("DINAH_STARTUP_CONTROL_WORKSPACE"), "skills": []any{map[string]any{"name": "invented-skill", "description": "An invented receiving skill.", "path": filepath.Join(os.Getenv("DINAH_STARTUP_CONTROL_WORKSPACE"), ".fixture-skills", "invented-skill", "SKILL.md"), "enabled": true}}, "errors": []any{}}}}
		case "session/new":
			next++
			result = map[string]any{"sessionId": fmt.Sprintf("fixture-temporary-%d", next), "configOptions": options}
		case "session/load", "session/set_config_option":
			result = map[string]any{"configOptions": options}
		case "session/prompt":
			var params map[string]any
			_ = json.Unmarshal(q.Params, &params)
			notify("session/update", map[string]any{"sessionId": params["sessionId"], "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Invented answer."}}})
			result = map[string]string{"stopReason": "end_turn"}
		case "thread/start":
			next++
			result = map[string]any{"thread": map[string]any{"id": fmt.Sprintf("fixture-temporary-%d", next), "cwd": os.Getenv("DINAH_STARTUP_CONTROL_WORKSPACE"), "turns": []any{}}, "model": "fixture", "reasoningEffort": "medium"}
		case "turn/start":
			var params map[string]any
			_ = json.Unmarshal(q.Params, &params)
			turnID := "fixture-turn"
			_ = encode.Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": map[string]any{"turn": map[string]string{"id": turnID, "status": "inProgress"}}})
			notify("item/agentMessage/delta", map[string]any{"threadId": params["threadId"], "itemId": "fixture-message", "delta": "Invented answer."})
			notify("item/completed", map[string]any{"threadId": params["threadId"], "item": map[string]string{"type": "agentMessage", "id": "fixture-message", "text": "Invented answer."}})
			notify("turn/completed", map[string]any{"threadId": params["threadId"], "turn": map[string]string{"id": turnID, "status": "completed"}})
			continue
		case "thread/read", "thread/resume":
			var params map[string]any
			_ = json.Unmarshal(q.Params, &params)
			result = map[string]any{"thread": map[string]any{"id": params["threadId"], "cwd": os.Getenv("DINAH_STARTUP_CONTROL_WORKSPACE"), "turns": []any{}}, "model": "", "reasoningEffort": ""}
		case "account/read":
			result = map[string]any{"account": nil, "requiresOpenaiAuth": false}
		}
		_ = encode.Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}
}
