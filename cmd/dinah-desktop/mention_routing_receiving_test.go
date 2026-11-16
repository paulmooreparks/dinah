//go:build org_chart_acceptance && startup_page_acceptance

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/bus"
)

type MentionReceivingSeed struct {
	Kind   string `json:"kind"`
	ChatID string `json:"chatId"`
	Group  string `json:"group"`
	Text   string `json:"text"`
}

func (a *App) OrgChartHostFixtureMentionSeed(in MentionReceivingSeed) (map[string]any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := f.manifest["chats"].(map[string]string)
	if in.ChatID == "" {
		in.ChatID = ids["free-a"]
	}
	known := false
	for _, id := range ids {
		known = known || id == in.ChatID
	}
	if !known {
		return nil, errors.New("seed requires an isolated fixture chat")
	}
	groups := f.manifest["groups"].(map[string]string)
	if in.Group == "" {
		in.Group = groups["Invented overlapping group"]
	}
	known = false
	for _, id := range groups {
		known = known || id == in.Group
	}
	if !known {
		return nil, errors.New("seed requires an isolated fixture group")
	}
	if in.Text == "" {
		in.Text = "Invented notification routing message"
	}
	var sent bus.Message
	var request bus.Decision
	err = f.a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		f.a.noticeMentions(ctx, b)
		sender, err := b.Session(ctx, f.a.knownBusID(in.ChatID))
		if err != nil {
			return err
		}
		switch in.Kind {
		case "direct":
			sent, err = b.Send(ctx, bus.Sender{Session: &sender}, bus.Human, in.Text, nil, 0)
		case "group":
			sent, err = b.SendMentioning(ctx, bus.Sender{Session: &sender}, "#"+in.Group, in.Text, nil, 0, []string{bus.Human})
		case "nonmention":
			sent, err = b.SendMentioning(ctx, bus.Sender{Session: &sender}, "#"+in.Group, in.Text, nil, 0, nil)
		case "request":
			request, err = b.AskPerson(ctx, sender, bus.AskPersonInput{OperationID: fmt.Sprintf("routing-request-%d", len(f.mentionIDs)), Group: "#" + in.Group, Questions: []bus.PersonQuestion{{Number: 1, Text: in.Text, Recommendation: "Keep the request pending."}}})
			if err == nil {
				sent, err = b.Message(ctx, request.MessageID)
			}
		default:
			return errors.New("unknown mention seed kind")
		}
		if err == nil {
			f.a.noticeMentions(ctx, b)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	f.mentionIDs = append(f.mentionIDs, sent.ID)
	return map[string]any{"message": sent, "request": request}, nil
}

// These receiving ports keep the current App after the existing restart
// control. Original arguments reach the same production window entry points.
func (a *App) OrgChartHostFixtureAcknowledgeMention(id int64) error {
	f, err := orgChartFixture(a)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.a.AcknowledgeMention(id)
}

func (a *App) OrgChartHostFixtureMentionCall(name string, args []json.RawMessage) (any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	allowed := map[string]bool{}
	for _, method := range []string{"Notifications", "DecisionEntries", "Decisions", "DecisionByID", "AcknowledgeMention", "BusGroup", "BusThread", "BusGroupMessages", "BusGroupMessagesFrom", "BusGroupMessageContext", "BusMarkInboxRead", "BusMarkGroupRead", "BusSend", "BusSendToGroup", "BusReplyInThread", "PendingTaskPermissions", "Sessions", "SavedChats", "BusOverview", "BusInboxMessage", "BusInboxPage"} {
		allowed[method] = true
	}
	if !allowed[name] {
		return nil, errors.New("method is outside the mention receiving seam")
	}
	method := reflect.ValueOf(f.a).MethodByName(name)
	if !method.IsValid() || method.Type().NumIn() != len(args) {
		return nil, errors.New("wrong original argument count")
	}
	inputs := []reflect.Value{}
	for i, raw := range args {
		value := reflect.New(method.Type().In(i))
		if err := json.Unmarshal(raw, value.Interface()); err != nil {
			return nil, err
		}
		inputs = append(inputs, value.Elem())
	}
	results := method.Call(inputs)
	if len(results) > 0 && results[len(results)-1].Type().Implements(reflect.TypeFor[error]()) {
		last := results[len(results)-1]
		if !last.IsNil() {
			return nil, last.Interface().(error)
		}
		results = results[:len(results)-1]
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0].Interface(), nil
}

func (a *App) OrgChartHostFixtureMentionState() (map[string]any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	messages := []bus.Message{}
	err = f.a.busHost().With(func(b *bus.Bus) error {
		f.a.noticeMentions(context.Background(), b)
		for _, id := range f.mentionIDs {
			m, err := b.Message(context.Background(), id)
			if err != nil {
				return err
			}
			messages = append(messages, m)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	notices, err := f.a.Notifications()
	if err != nil {
		return nil, err
	}
	entries, err := f.a.DecisionEntries()
	if err != nil {
		return nil, err
	}
	decisions, err := f.a.Decisions(0)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(configDir(), "notifications.json"))
	if errors.Is(err, os.ErrNotExist) {
		raw = []byte{}
	} else if err != nil {
		return nil, err
	}
	return map[string]any{"notifications": notices, "entries": entries, "decisions": decisions, "sourceMessages": messages, "notificationBytes": raw, "pendingTaskQuestions": f.a.PendingTaskPermissions()}, nil
}

func TestMentionReceivingSeedsAndRestartsActualStore(t *testing.T) {
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	newOrgChartHostFixture(t, a)
	initial, err := a.OrgChartHostFixtureMentionState()
	if err != nil {
		t.Fatal("fresh notification baseline failed", err)
	}
	if len(initial["notifications"].([]Notification)) != 0 || len(initial["notificationBytes"].([]byte)) != 0 {
		t.Fatal("fresh notification baseline was not empty", initial)
	}
	for _, kind := range []string{"group", "direct", "request", "nonmention"} {
		if _, err := a.OrgChartHostFixtureMentionSeed(MentionReceivingSeed{Kind: kind}); err != nil {
			t.Fatal(err)
		}
	}
	state, err := a.OrgChartHostFixtureMentionState()
	if err != nil {
		t.Fatal(err)
	}
	if state["entries"].(bus.DecisionEntryState).Count != 1 || len(state["notifications"].([]Notification)) != 3 {
		t.Fatal("seed did not enter actual stores", state)
	}
	messages := state["sourceMessages"].([]bus.Message)
	if err := a.OrgChartHostFixtureAcknowledgeMention(messages[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.OrgChartHostFixtureAcknowledgeMention(messages[2].ID); err == nil {
		t.Fatal("request acknowledgement was allowed")
	}
	if err := a.OrgChartHostFixtureAcknowledgeMention(messages[3].ID); err == nil {
		t.Fatal("nonmention acknowledgement was allowed")
	}
	if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "restart"}); err != nil {
		t.Fatal(err)
	}
	after, err := a.OrgChartHostFixtureMentionState()
	if err != nil {
		t.Fatal(err)
	}
	if len(after["notifications"].([]Notification)) != 2 || after["entries"].(bus.DecisionEntryState).Count != 1 {
		t.Fatal("restart lost acknowledgement or request", after)
	}
	forwarded, err := a.OrgChartHostFixtureMentionCall("DecisionEntries", nil)
	if err != nil || forwarded.(bus.DecisionEntryState).Count != 1 {
		t.Fatal("forwarding did not reach the restarted actual App", forwarded, err)
	}
}
