// Command decisions-agent exercises the real bus MCP server in an isolated home.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"studio/internal/acp"
	"studio/internal/bus"
)

type server struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("devin 3000.1.1")
		return
	}
	var mu sync.Mutex
	record := func(value any) {
		mu.Lock()
		defer mu.Unlock()
		f, err := os.OpenFile(filepath.Join(os.Getenv("DECISIONS_FIXTURE"), "agent.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err = json.NewEncoder(f).Encode(value); err != nil {
			panic(err)
		}
	}
	reply := func(id json.RawMessage, result any) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	servers := map[string]server{}
	next := 0
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 4<<20)
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
			var p struct {
				Servers []server `json:"mcpServers"`
			}
			json.Unmarshal(req.Params, &p)
			next++
			id := fmt.Sprintf("decision-%d-%d", os.Getpid(), next)
			for _, s := range p.Servers {
				if s.Name == bus.WindowServer {
					servers[id] = s
				}
			}
			reply(req.ID, map[string]any{"sessionId": id, "configOptions": []any{}})
		case "session/list":
			reply(req.ID, map[string]any{"sessions": []any{}})
		case "session/load":
			var p struct {
				Session string `json:"sessionId"`
			}
			json.Unmarshal(req.Params, &p)
			record(map[string]any{"loaded": p.Session, "at": time.Now().UTC()})
			reply(req.ID, map[string]any{"configOptions": []any{}})
		case "session/prompt":
			var p struct {
				Session string `json:"sessionId"`
				Prompt  []struct {
					Text string `json:"text"`
				} `json:"prompt"`
			}
			json.Unmarshal(req.Params, &p)
			text := ""
			if len(p.Prompt) > 0 {
				text = p.Prompt[0].Text
			}
			s := servers[p.Session]
			record(map[string]any{"session": p.Session, "prompt": text})
			go func(id json.RawMessage, session, text string, s server) {
				if text == "fixture ask" || strings.HasPrefix(text, "fixture mention") {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					cmd := exec.Command(s.Command, s.Args...)
					acp.HideWindow(cmd)
					client, err := mcp.NewClient(&mcp.Implementation{Name: "decisions-stand-in", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
					if err != nil {
						record(map[string]any{"error": err.Error()})
						reply(id, map[string]string{"stopReason": "end_turn"})
						return
					}
					defer client.Close()
					name := "ask_person"
					args := map[string]any{"operationId": "fixture-two-questions", "questions": []any{
						map[string]any{"number": 1, "text": "Which release should we use?", "recommendation": "Use the reviewed release.", "choices": []string{"Reviewed", "Wait"}},
						map[string]any{"number": 2, "text": "When should we proceed?", "recommendation": "Proceed tomorrow."}}}
					if strings.HasPrefix(text, "fixture mention") {
						name = "send"
						body := "Fixture plain decision for the person."
						if text == "fixture mention second" {
							body = "Second fixture request has a newer first line.\nThis second line must not appear on its card."
						}
						args = map[string]any{"to": "#Fixture Review", "text": body, "actionRecipients": []string{"paul"}}
					}
					result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
					if err != nil {
						record(map[string]any{"error": err.Error()})
					} else {
						var contents []string
						for _, c := range result.Content {
							if v, ok := c.(*mcp.TextContent); ok {
								contents = append(contents, v.Text)
							}
						}
						record(map[string]any{"session": session, "tool": name, "isError": result.IsError, "text": strings.Join(contents, "\n")})
					}
				}
				if text == "fixture hold" {
					for {
						if _, err := os.Stat(filepath.Join(os.Getenv("DECISIONS_FIXTURE"), "finish")); err == nil {
							break
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
				if strings.Contains(text, "Fixture held broadcast to all.") {
					for {
						if _, err := os.Stat(filepath.Join(os.Getenv("DECISIONS_FIXTURE"), "release-"+session)); err == nil {
							break
						}
						time.Sleep(20 * time.Millisecond)
					}
					record(map[string]any{"released": session, "at": time.Now().UTC()})
				}
				reply(id, map[string]string{"stopReason": "end_turn"})
			}(req.ID, p.Session, text, s)
		default:
			reply(req.ID, map[string]any{})
		}
	}
}
