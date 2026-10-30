package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"sync"
	"time"

	"studio/internal/applet"
)

//go:embed applet_window_commands.go
var windowsAppletSource []byte

/* The open windows' commands, for agents and flows.
   A window's commands are declared in the page with PUDL's commands
   (pudl-commands.js); an agent lists the windows of its workspace, lists one
   window's commands as data with their state now, and runs one by its id as
   pressing it would. The page answers: the Go side asks it with the event
   page:ask and the page replies through AnswerPage. A command that would
   destroy something is the person's to press and is refused, as is a
   command that cannot run now, with its reason. A tool window that is not
   open is opened by its tool's ID, as Tools would open it, so that a chat
   that was given a window can drive it. */

const windowsListAction = "dinah.windows/list@1"
const windowCommandsAction = "dinah.windows/commands@1"
const windowRunAction = "dinah.windows/run@1"
const windowOpenAction = "dinah.windows/open@1"

// windowToolID is the form of a tool's ID, the kind of window Tools opens.
var windowToolID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// pageAskTimeout is how long an action waits for the page to answer.
const pageAskTimeout = 15 * time.Second

type pageAnswer struct {
	result string
	err    string
}

type pageAsks struct {
	mu      sync.Mutex
	waiting map[string]*pageAsk
	asked   int
}

// pageAsk is a question waiting for the page's answer. Its first fields are
// what the page:ask event carries, kept so that a page that missed the event
// can read them (PendingPageAsks). answered is set when the page's answer has
// been handed on, so an ask the page has answered is no longer pending though
// askPage has not yet returned.
type pageAsk struct {
	ID       string `json:"id"`
	Op       string `json:"op"`
	Args     any    `json:"args"`
	ch       chan pageAnswer
	order    int
	answered bool
}

// PendingPageAsks is the questions the engine has asked the page and has not
// had answered, oldest first, each as the page:ask event carried it. A page
// that missed the event reads them here.
func (a *App) PendingPageAsks() []pageAsk {
	a.asks.mu.Lock()
	defer a.asks.mu.Unlock()
	out := make([]pageAsk, 0, len(a.asks.waiting))
	for _, ask := range a.asks.waiting {
		if !ask.answered {
			out = append(out, pageAsk{ID: ask.ID, Op: ask.Op, Args: ask.Args, order: ask.order})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].order < out[j].order })
	return out
}

// askPage sends the page a question and waits for its answer, which is the
// JSON the page gives, or its error.
func (a *App) askPage(ctx context.Context, op string, args any) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	ch := make(chan pageAnswer, 1)
	ask := &pageAsk{ID: id, Op: op, Args: args, ch: ch}
	a.asks.mu.Lock()
	if a.asks.waiting == nil {
		a.asks.waiting = map[string]*pageAsk{}
	}
	a.asks.asked++
	ask.order = a.asks.asked
	a.asks.waiting[id] = ask
	a.asks.mu.Unlock()
	defer func() {
		a.asks.mu.Lock()
		delete(a.asks.waiting, id)
		a.asks.mu.Unlock()
	}()
	a.emit("page:ask", map[string]any{"id": id, "op": op, "args": args})
	timer := time.NewTimer(pageAskTimeout)
	defer timer.Stop()
	select {
	case got := <-ch:
		if got.err != "" {
			return "", errors.New(got.err)
		}
		return got.result, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return "", errors.New(product.Name + "'s window did not answer; it may be busy or closed")
	}
}

// AnswerPage is the page's answer to a question askPage sent.
func (a *App) AnswerPage(id, result, errText string) {
	a.asks.mu.Lock()
	ask := a.asks.waiting[id]
	a.asks.mu.Unlock()
	if ask != nil {
		select {
		case ask.ch <- pageAnswer{result: result, err: errText}:
			a.asks.mu.Lock()
			ask.answered = true
			a.asks.mu.Unlock()
		default:
		}
	}
}

func (a *App) windowsAppletBundle() (applet.Bundle, error) {
	windowInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["window"],"properties":{"window":{"type":"string","minLength":1,"maxLength":4096}}}`)
	command := `{"type":"object","required":["id","label"],"properties":{"id":{"type":"string"},"label":{"type":"string"},"glyph":{"type":"string"},"shortcut":{"type":"string"},"disabled":{"type":"boolean"},"reason":{"type":"string"},"checked":{"type":"boolean"},"choice":{"type":"string"},"value":{},"current":{"type":"string"},"badge":{},"hidden":{"type":"boolean"},"danger":{"type":"boolean"},"action":{"type":"string"},"shown":{"type":"string"},"items":{"type":"array"}}}`
	manifest := applet.Manifest{ID: "dinah.windows", Version: "1.1.0", APIVersion: 1, Title: "Windows",
		Description: "The open windows of the calling workspace and their commands: list them, open a tool's window, and run a command by its id as pressing it would.",
		Actions: []applet.Action{
			{Ref: windowsListAction, Title: "List the Windows", Replay: "pure", Capabilities: []string{"window.read"},
				Description: "List the open windows of the calling workspace: each window's key, kind, title and whether it is in front. Use a key with the window's commands.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["windows"],"properties":{"windows":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["window","kind","title","front"],` +
					`"properties":{"window":{"type":"string"},"kind":{"type":"string"},"title":{"type":"string"},"detail":{"type":"string"},"front":{"type":"boolean"}}}}}}`)},
			{Ref: windowCommandsAction, Title: "List a Window's Commands", Replay: "pure", Capabilities: []string{"window.read"},
				Description:  "List one window's commands as data, with their state now: label, key, whether each is disabled and why, checked, the item chosen, and whether a toolbar shows it. A command with items is a menu of them.",
				InputSchema:  windowInput,
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["commands"],"properties":{"commands":{"type":"array","items":` + command + `}}}`)},
			{Ref: windowRunAction, Title: "Run a Window's Command", Replay: "once", Capabilities: []string{"window.command"},
				Description:  "Run one of a window's commands by its id, as pressing it would. A command that cannot run now is refused with its reason; one that would destroy something is left to the person.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["window","command"],"properties":{"window":{"type":"string","minLength":1,"maxLength":4096},"command":{"type":"string","minLength":1,"maxLength":200},"value":{}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["ran"],"properties":{"ran":{"type":"string"}}}`)},
			{Ref: windowOpenAction, Title: "Open a Tool Window", Replay: "once", Capabilities: []string{"window.command"},
				Description:  "Open a tool's window, as choosing it in Tools would, and bring it to the front; a tool whose window is already open is brought to the front. The tool is its ID, such as tableaus for the Tableau Viewer, as a window reference in a chat says. Answers the window's key.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["tool"],"properties":{"tool":{"type":"string","pattern":"^[a-z][a-z0-9-]{0,63}$"}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["window"],"properties":{"window":{"type":"string"}}}`)},
		}}
	ask := func(op string) applet.Handler {
		return func(ctx context.Context, input string) (applet.Result, error) {
			var args map[string]any
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return applet.Result{}, err
			}
			if args == nil {
				args = map[string]any{}
			}
			// A window says its workspace by its folder or by its ID, so the
			// page is given both and shows only that workspace's windows.
			ws, err := builtinWorkspace(ctx)
			if err != nil {
				return applet.Result{}, err
			}
			args["workspace"], args["workspaceId"] = ws, ""
			if vfs, err := a.scopedWorkspaceVFS(ctx); err == nil {
				if records := vfs.records(); len(records) == 1 {
					args["workspaceId"] = records[0].ID
				}
			}
			out, err := a.askPage(ctx, op, args)
			if err != nil {
				return applet.Result{}, err
			}
			return applet.Result{OutputJSON: out}, nil
		}
	}
	handlers := map[string]applet.Handler{windowsListAction: ask("windows"), windowCommandsAction: ask("commands"), windowRunAction: ask("run"), windowOpenAction: ask("open")}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_window_commands.go": windowsAppletSource}}, nil
}
