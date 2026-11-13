// Command restart-agent is a local ACP stand-in for the hidden restart check.
// It never calls a model or executes a tool.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("devin 3000.1.1")
		return
	}
	var mu sync.Mutex
	reply := func(id json.RawMessage, result any) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 4*1024*1024)
	for scan.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scan.Bytes(), &req) != nil || len(req.ID) == 0 {
			continue
		}
		switch req.Method {
		case "initialize":
			reply(req.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true}, "agentInfo": map[string]string{"name": "devin", "version": "3000.1.1"}})
		case "session/new":
			reply(req.ID, map[string]any{"sessionId": "restart-fixture", "configOptions": []any{}})
		case "session/list":
			reply(req.ID, map[string]any{"sessions": []any{}})
		case "session/prompt":
			f, err := os.OpenFile(filepath.Join(os.Getenv("RESTART_FIXTURE"), "prompts.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				panic(err)
			}
			// Keep only the prompt's first text block. Host authorization context
			// appended to a prompt must not enter a test receipt.
			var p struct {
				Prompt []struct {
					Text string `json:"text"`
				} `json:"prompt"`
			}
			json.Unmarshal(req.Params, &p)
			if len(p.Prompt) > 0 {
				json.NewEncoder(f).Encode(map[string]string{"text": p.Prompt[0].Text})
			}
			f.Close()
			go func(id json.RawMessage, p json.RawMessage) {
				if strings.Contains(string(p), "fixture original work") {
					for {
						if _, err := os.Stat(filepath.Join(os.Getenv("RESTART_FIXTURE"), "finish")); err == nil {
							break
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
				reply(id, map[string]string{"stopReason": "end_turn"})
			}(req.ID, req.Params)
		default:
			reply(req.ID, map[string]any{})
		}
	}
}
