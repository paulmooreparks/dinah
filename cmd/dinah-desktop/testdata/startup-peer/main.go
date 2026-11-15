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
		case "thread/list", "model/list":
			result = map[string]any{"data": []any{}}
		case "session/load", "session/new":
			result = map[string]any{"sessionId": "fixture-open", "configOptions": []any{}}
		case "thread/read", "thread/resume":
			var params map[string]any
			_ = json.Unmarshal(q.Params, &params)
			result = map[string]any{"thread": map[string]any{"id": params["threadId"], "cwd": os.Getenv("DINAH_STARTUP_CONTROL_WORKSPACE"), "turns": []any{}}, "model": "", "reasoningEffort": ""}
		case "account/read":
			result = map[string]any{"account": nil, "requiresOpenaiAuth": false}
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}
}
