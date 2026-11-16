//go:build org_chart_acceptance && startup_page_acceptance

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
		case "request", "two-question-request", "partial-request":
			questions := []bus.PersonQuestion{{Number: 1, Text: in.Text, Recommendation: "Keep the request pending."}}
			if in.Kind != "request" {
				questions = append(questions, bus.PersonQuestion{Number: 2, Text: "Keep the invented record?", Recommendation: "Keep it."})
			}
			request, err = b.AskPerson(ctx, sender, bus.AskPersonInput{OperationID: fmt.Sprintf("routing-request-%d", len(f.mentionIDs)), Group: "#" + in.Group, Questions: questions})
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
	if in.Kind == "partial-request" {
		receipt, err := f.a.AnswerDecision(bus.AnswerDecisionInput{ID: request.ID, Revision: request.Revision, OperationID: fmt.Sprintf("routing-partial-%d", sent.ID), Answers: []bus.DecisionAnswer{{Number: 1, Text: "Keep this existing invented answer."}}})
		if err != nil {
			return nil, err
		}
		if receipt.State != "committed" {
			return nil, errors.New("partial request seed did not commit its original answer")
		}
		detail, err := f.a.DecisionByID(request.ID)
		if err != nil {
			return nil, err
		}
		request = detail.Decision
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
	for _, method := range []string{"Notifications", "DecisionEntries", "Decisions", "DecisionByID", "AnswerDecision", "ReadUIState", "SaveUIState", "AcknowledgeMention", "BusGroup", "BusThread", "BusGroupMessages", "BusGroupMessagesFrom", "BusGroupMessageContext", "BusMarkInboxRead", "BusMarkGroupRead", "BusSend", "BusSendToGroup", "BusReplyInThread", "PendingTaskPermissions", "Sessions", "SavedChats", "BusOverview", "BusInboxMessage", "BusInboxPage"} {
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

func TestMentionReceivingPreservesActualPartialAnswerAndSavedDraft(t *testing.T) {
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	newOrgChartHostFixture(t, a)
	seed, err := a.OrgChartHostFixtureMentionSeed(MentionReceivingSeed{Kind: "two-question-request"})
	if err != nil {
		t.Fatal(err)
	}
	request := seed["request"].(bus.Decision)
	input, _ := json.Marshal(bus.AnswerDecisionInput{ID: request.ID, Revision: request.Revision, OperationID: "retained-partial", Answers: []bus.DecisionAnswer{{Number: 1, Text: "Keep the existing answer."}}})
	result, err := a.OrgChartHostFixtureMentionCall("AnswerDecision", []json.RawMessage{input})
	if err != nil || result.(bus.DecisionReceipt).State != "committed" {
		t.Fatal("production partial answer failed", result, err)
	}
	key := "dinah-decisions:" + url.QueryEscape("request:"+request.ID)
	saved := UIState{Version: 1, Values: map[string]json.RawMessage{key: json.RawMessage(`{"version":1,"answers":{"2":{"text":"Invented retained draft","asRecommended":false,"ownText":"Invented retained draft","selection":"own"}},"reviewed":"","operation":null}`)}}
	encoded, _ := json.Marshal(saved)
	if _, err := a.OrgChartHostFixtureMentionCall("SaveUIState", []json.RawMessage{encoded}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OrgChartHostFixtureMentionState(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "restart"}); err != nil {
		t.Fatal(err)
	}
	state, err := a.OrgChartHostFixtureMentionCall("ReadUIState", nil)
	actualJSON, encodeErr := json.Marshal(state)
	if err != nil || encodeErr != nil || string(actualJSON) != string(encoded) {
		t.Fatal("saved nonempty draft changed", state, err)
	}
	id, _ := json.Marshal(request.ID)
	detail, err := a.OrgChartHostFixtureMentionCall("DecisionByID", []json.RawMessage{id})
	if err != nil {
		t.Fatal(err)
	}
	actual := detail.(bus.DecisionDetail)
	if actual.Closed || len(actual.Decision.Questions) != 2 || !actual.Decision.Questions[0].Answered || actual.Decision.Questions[0].Answer != "Keep the existing answer." || actual.Decision.Questions[1].Answered {
		t.Fatal("actual partial answer changed", actual)
	}
}
