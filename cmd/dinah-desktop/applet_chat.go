package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/bushost"
	"studio/internal/layout"
	"studio/internal/sessionhost"
)

//go:embed applet_chat.go
var chatAppletSource []byte

//go:embed history.go
var chatHistorySource []byte

//go:embed session.go
var chatSessionSource []byte

//go:embed chat_delivery.go
var chatDeliverySource []byte

//go:embed steers.go
var chatSteeringSource []byte

//go:embed bus_chats.go
var chatBusSource []byte

const chatListAction = "dinah.chat/list@1"
const chatHistoryAction = "dinah.chat/prompt-history@1"
const legacyChatAskAction = "dinah.chat/ask@1"
const chatAskAction = sessionhost.ChatAskAction

// maxAskQuestion is the longest question one chat may put to another.
const maxAskQuestion = 32 * 1024

func (a *App) chatAppletBundle() (applet.Bundle, error) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"limit":{"type":"integer","minimum":1,"maximum":100}}}`)
	listOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["chats","truncated","coverage"],"properties":{"chats":{"type":"array","items":{"type":"object"}},"truncated":{"type":"boolean"},"coverage":{"const":"locally-known"}}}`)
	historyOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["prompts","truncated","coverage"],"properties":{"prompts":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["time","kind","text","textTruncated"],"properties":{"time":{"type":"string"},"kind":{"type":"string"},"text":{"type":"string"},"textTruncated":{"type":"boolean"}}}},"truncated":{"type":"boolean"},"coverage":{"const":"prompt-only"}}}`)
	askInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sessionId","question"],"properties":{"sessionId":{"type":"string","minLength":1,"maxLength":512},"question":{"type":"string","minLength":1,"maxLength":32768}}}`)
	askOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sessionId","title","reply","stopReason"],"properties":{"sessionId":{"type":"string"},"title":{"type":"string"},"reply":{"type":"string"},"stopReason":{"type":"string"}}}`)
	directoryInput := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string","maxLength":512},"workspace":{"type":"string","maxLength":512},"limit":{"type":"integer","minimum":1,"maximum":100}}}`)
	findInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":512},"workspace":{"type":"string","maxLength":512},"limit":{"type":"integer","minimum":1,"maximum":100}}}`)
	directoryOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["chats","truncated","coverage","match"],"properties":{"chats":{"type":"array","items":{"type":"object"}},"truncated":{"type":"boolean"},"coverage":{"const":"open-application"},"match":{"enum":["unique","ambiguous","not-found"]}}}`)
	manifest := applet.Manifest{ID: "dinah.chat", Version: "1.5.0", APIVersion: 1, Title: "Chat", Description: "Find and ask chats across the application's workspaces, open or closed. Prompt history remains scoped to the calling workspace.", Actions: []applet.Action{
		{Ref: chatDirectoryAction, Title: "List Chats Across Workspaces", Description: "List the chats this running application knows across its workspaces, open or closed, with titles, workspace labels, session IDs and each chat's state: open, open-in-background (opened without a window for a message) or closed. Open chats come first. An untitled chat is listed as \"Untitled Chat in\" its workspace. Optionally filter by name and workspace name or ID. Does not connect agents or read their conversation history.", InputSchema: directoryInput, OutputSchema: directoryOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: chatFindAction, Title: "Find a Chat by Name", Description: "Find a chat by its title across all workspaces in this running application, open or closed. Exact titles take precedence over partial matches. Returns unique, ambiguous or not-found with workspace labels, session IDs and states. For ambiguous names, clarify the workspace or target; never guess. Use the returned sessionId with dinah.chat/ask@2, which can ask a closed chat too.", InputSchema: findInput, OutputSchema: directoryOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: chatListAction, Title: "List Open Chats", Description: "List chats known to this running application in the calling workspace. This does not connect to an agent or load remote chat history.", InputSchema: input, OutputSchema: listOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: chatHistoryAction, Title: "Read Prompt History", Description: "Read recent user prompts in the calling workspace, newest first, with at most 8192 bytes per prompt. This is prompt-only history, not a conversation transcript, and does not include agent responses.", InputSchema: input, OutputSchema: historyOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: legacyChatAskAction, Title: "Ask Another Chat (Legacy)", Description: "Legacy status-based chat question. Use dinah.chat/ask@2 for new requests and automatic reply delivery. Version 1 asks over the bus as version 2 does, and its invocations retain their original status-based contract for the result.", InputSchema: askInput, OutputSchema: askOutput, Capabilities: []string{"chat.ask"}, Replay: "once"},
		{Ref: chatAskAction, Title: "Ask Another Chat", Description: "Ask another chat across this application's workspaces asynchronously, open or closed. Find its session ID with dinah.chat/find@1 when given a name. The question travels over the bus as a message that expects an answer: a closed chat is opened in the background to answer it. Invoke once, then continue independent work or end your turn. The returned invocation ID is a receipt. " + product.Name + " automatically delivers the reply or failure as a later prompt, immediately when idle, as steering when supported while busy, or after your active turn otherwise. Do not poll, sleep or repeat the question, and do not also send it with the bus's tools. A busy recipient receives steering when supported; otherwise it receives the question after its turn. A chat cannot ask itself or ask another chat while answering a question.", InputSchema: askInput, OutputSchema: askOutput, Capabilities: []string{"chat.ask"}, Replay: "once"},
	}}
	handlers := map[string]applet.Handler{}
	handlers[chatDirectoryAction] = a.chatDirectoryHandler(false)
	handlers[chatFindAction] = a.chatDirectoryHandler(true)
	handlers[chatListAction] = func(ctx context.Context, input string) (applet.Result, error) {
		ws, err := builtinWorkspace(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		limit := chatAppletLimit(input)
		a.mu.Lock()
		list := []SessionInfo{}
		for _, s := range a.runtime.ConversationsLocked() {
			if layout.SamePath(s.Cwd(), ws) && !a.hidden(s.ID()) {
				info := s.Info()
				info.Cwd, info.Queued = "", nil
				if title := a.settings.SessionTitles[s.ID()]; title != "" {
					info.Title = title
				}
				list = append(list, info)
			}
		}
		a.mu.Unlock()
		sort.Slice(list, func(i, j int) bool {
			if list[i].UpdatedAt == list[j].UpdatedAt {
				return list[i].ID < list[j].ID
			}
			return list[i].UpdatedAt > list[j].UpdatedAt
		})
		truncated := len(list) > limit
		if truncated {
			list = list[:limit]
		}
		return builtinJSON(map[string]any{"chats": list, "truncated": truncated, "coverage": "locally-known"})
	}
	handlers[chatHistoryAction] = func(ctx context.Context, input string) (applet.Result, error) {
		ws, err := builtinWorkspace(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		all, err := a.History()
		if err != nil {
			return applet.Result{}, err
		}
		limit := chatAppletLimit(input)
		list := []map[string]any{}
		truncated := false
		for i := len(all) - 1; i >= 0; i-- {
			if !layout.SamePath(all[i].Workspace, ws) {
				continue
			}
			if len(list) == limit {
				truncated = true
				break
			}
			text := all[i].Text
			textTruncated := len(text) > 8192
			if textTruncated {
				text = text[:8192]
				for !utf8.ValidString(text) {
					text = text[:len(text)-1]
				}
			}
			list = append(list, map[string]any{"time": all[i].Time, "kind": all[i].Kind, "text": text, "textTruncated": textTruncated})
		}
		return builtinJSON(map[string]any{"prompts": list, "truncated": truncated, "coverage": "prompt-only"})
	}
	handlers[chatAskAction] = func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			SessionID string `json:"sessionId"`
			Question  string `json:"question"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		if strings.TrimSpace(in.Question) == "" || len(in.Question) > maxAskQuestion {
			return applet.Result{}, errors.New("the question must have words, and at most 32768 bytes of them")
		}
		caller, target, label, err := a.askTarget(ctx, in.SessionID)
		if err != nil {
			return applet.Result{}, err
		}
		title := target.Title
		if !a.beginAsk(caller.ID(), target.SessionID) {
			return applet.Result{}, fmt.Errorf("the chat %q is itself waiting, directly or through other chats, for an answer from this chat, so both would wait for ever; answer from what you know, or ask again after this turn", title)
		}
		defer a.endAsk(caller.ID(), target.SessionID)
		reply, end := a.busCoordinator().Ask(ctx, caller, sessionhost.BusAskTarget{SessionID: target.SessionID, Cwd: target.cwd, Title: target.named}, label, in.Question)
		if end.Err != nil {
			return applet.Result{}, fmt.Errorf("the chat %q did not answer: %w", title, end.Err)
		}
		reply = strings.TrimSpace(reply)
		result, err := builtinJSON(map[string]any{"sessionId": target.SessionID, "title": title, "reply": reply, "stopReason": end.StopReason})
		result.Text = askReplyText(title, target.SessionID, reply)
		return result, err
	}
	// Version 1 asks over the bus as version 2 does. Only how its result is
	// read differs: its caller reads it with applet_status, and Dinah does
	// not deliver it (chat_delivery.go).
	handlers[legacyChatAskAction] = handlers[chatAskAction]
	sources := map[string][]byte{"applet_chat.go": chatAppletSource, "applet_workspace.go": workspaceAppletSource, "history.go": chatHistorySource, "session.go": chatSessionSource, "chat_delivery.go": chatDeliverySource, "steers.go": chatSteeringSource, "bus_chats.go": chatBusSource, "chat_directory.go": chatDirectorySource, "chat_ask.go": chatAskSource, "bus_wake.go": chatWakeSource, "bushost/scheduler.go": bushost.SchedulerSource, "bushost/service.go": bushost.ServiceSource, "sessionhost/bus_results.go": sessionhost.BusResultsSource, "sessionhost/bus_delivery.go": sessionhost.BusDeliverySource, "sessionhost/bus_chats.go": sessionhost.BusChatsSource, "sessionhost/values.go": sessionhost.ValuesSource, "sessionhost/transcript.go": sessionhost.TranscriptSource, "sessionhost/handoff.go": sessionhost.HandoffSource, "sessionhost/continuation.go": sessionhost.ContinuationSource, "sessionhost/transcript_capture.go": sessionhost.TranscriptCaptureSource, "sessionhost/copies.go": sessionhost.CopiesSource, "sessionhost/prompt.go": sessionhost.PromptSource, "sessionhost/turns.go": sessionhost.TurnsSource, "sessionhost/details.go": sessionhost.DetailsSource, "sessionhost/updates.go": sessionhost.UpdatesSource, "sessionhost/background.go": sessionhost.BackgroundSource, "sessionhost/subagents.go": sessionhost.SubagentsSource, "sessionhost/subagent_service.go": sessionhost.SubagentServiceSource, "sessionhost/history.go": sessionhost.HistorySource, "sessionhost/links.go": sessionhost.LinksSource, "sessionhost/conversation.go": sessionhost.ConversationSource, "sessionhost/conversation_ops.go": sessionhost.ConversationOpsSource, "sessionhost/conversation_state.go": sessionhost.ConversationStateSource, "sessionhost/runtime.go": sessionhost.RuntimeSource, "sessionhost/turn_io.go": sessionhost.TurnIOSource, "sessionhost/steering_io.go": sessionhost.SteeringIOSource, "sessionhost/turn_format.go": sessionhost.TurnFormatSource, "sessionhost/journal.go": sessionhost.JournalSource, "sessionhost/options.go": sessionhost.OptionsSource, "sessionhost/session_lifecycle.go": sessionhost.SessionLifecycleSource, "sessionhost/closed_sessions.go": sessionhost.ClosedSessionsSource, "sessionhost/option_restore.go": sessionhost.OptionRestoreSource, "sessionhost/option_substitutions.go": sessionhost.OptionSubstitutionsSource, "sessionhost/permissions.go": sessionhost.PermissionsSource, "sessionhost/steering_ledger.go": sessionhost.SteeringLedgerSource, "sessionhost/handler.go": sessionhost.HandlerSource, "sessionhost/handler_updates.go": sessionhost.HandlerUpdatesSource, "sessionhost/lifecycle.go": sessionhost.LifecycleSource, "sessionhost/provider.go": sessionhost.ProviderSource, "sessionhost/activity.go": sessionhost.ActivitySource, "sessionhost/chat_events.go": sessionhost.ChatEventsSource}
	for name, source := range sessionWiringDigestSources {
		sources[name] = source
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: sources}, nil
}

// askTarget checks that the calling chat may ask the chat given: only a
// chat's agent asks; the target is another chat this application knows,
// open or closed (chatDirectory); and a chat answering a question asks no
// one in that turn. It returns the calling chat, the target as the
// directory lists it, and the label the question has in the target's log.
func (a *App) askTarget(ctx context.Context, id string) (*Session, chatDirectoryEntry, string, error) {
	inv, ok := applet.InvocationFromContext(ctx)
	if !ok || inv.OwnerKind != "chat" || inv.OwnerID == "" {
		return nil, chatDirectoryEntry{}, "", errors.New("only a chat's agent can ask another chat")
	}
	ws, err := builtinWorkspace(ctx)
	if err != nil {
		return nil, chatDirectoryEntry{}, "", err
	}
	if id == inv.OwnerID {
		return nil, chatDirectoryEntry{}, "", errors.New("a chat cannot ask itself")
	}
	caller := a.session(inv.OwnerID)
	if caller == nil || !layout.SamePath(caller.Cwd(), ws) {
		return nil, chatDirectoryEntry{}, "", errors.New("the calling chat does not belong to the invocation workspace")
	}

	state := caller.State()
	asked := state.Asked || state.Reserved

	if asked {
		return nil, chatDirectoryEntry{}, "", errors.New("this chat is answering another chat's question, and a chat cannot ask another chat in that turn; answer from what you know, or say what the person should ask")
	}
	target, ok := a.directoryChat(id)
	if !ok {
		return nil, chatDirectoryEntry{}, "", fmt.Errorf("there is no chat %s in this application; find the chat by name for its session ID", id)
	}
	label := "Asked by " + a.chatTitle(caller)
	if !layout.SamePath(target.cwd, ws) {
		label += " (workspace " + a.chatWorkspaceName(ws) + ")"
	}
	return caller, target, label, nil
}

// beginAsk records that the chat from waits on the chat to, unless to already
// waits on from through some chain of asks. Questions may queue behind
// active work, so a cycle could leave every caller waiting until timeout.
func (a *App) beginAsk(from, to string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runtime.BeginAskLocked(from, to)
}

func (a *App) endAsk(from, to string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runtime.EndAskLocked(from, to)
}

// chatTitle is a chat's title as the person sees it, and as the bus shows
// an untitled chat: "Untitled Chat in" its workspace.
func (a *App) chatTitle(s *Session) string {
	return bus.DisplayTitle(a.shownTitle(s), a.chatWorkspaceName(s.Cwd()))
}

// shownTitle is the title the person sees for a chat: the name they gave
// it, else its agent's title, or "" when it has neither.
func (a *App) shownTitle(s *Session) string {
	a.mu.Lock()
	title := a.settings.SessionTitles[s.ID()]
	a.mu.Unlock()
	if title != "" {
		return title
	}
	return s.Title()
}

// askPrompt is what the asked chat's agent is told. The question reaches it
// over the bus, but the answer is the turn's own words, so the agent is
// told not to answer through the bus's tools as well.
func askPrompt(label, question string) string {
	from := strings.TrimPrefix(label, "Asked by ")
	return "Another chat, \"" + from + "\", asks you the question below over the bus. Answer it from your own work in this chat, in this turn; " + product.Name + " returns your reply to that chat word for word, so make it complete in itself, and do not also send it with the bus's tools. Do not ask any other chat in this turn.\n\n" + question
}

// askReplyText is a chat's reply as the asking chat reads it: which chat
// answered, and the reply as that chat wrote it (applet.Result's Text).
func askReplyText(title, session, reply string) string {
	if reply == "" {
		return fmt.Sprintf("The chat \"%s\" (session %s) ended its turn without a reply.", title, session)
	}
	return fmt.Sprintf("Reply from the chat \"%s\" (session %s):\n\n%s", title, session, reply)
}

func chatAppletLimit(input string) int {
	var in struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal([]byte(input), &in)
	if in.Limit < 1 || in.Limit > 100 {
		return 25
	}
	return in.Limit
}
