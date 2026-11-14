package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDecisionEntriesCompleteAndPartialLifecycle(t *testing.T) {
	b, s, first := personRequestFixture(t)
	ctx := context.Background()
	for i := 0; i < 102; i++ {
		_, err := b.AskPerson(ctx, s, AskPersonInput{OperationID: fmt.Sprintf("entry-%d", i), Questions: []PersonQuestion{{Number: 1, Text: "Long question omitted from the index", Recommendation: "Proceed"}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := b.DecisionEntries(ctx)
	if err != nil || entries.Count != 103 || len(entries.Items) != 103 || entries.Items[102].ID != first.ID || entries.Items[102].MessageID != first.MessageID {
		t.Fatalf("complete index %+v %v", entries, err)
	}
	encoded, _ := json.Marshal(entries)
	if strings.Contains(string(encoded), "question") || strings.Contains(string(encoded), "recommendation") {
		t.Fatal("index includes question payloads")
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 100 || page.NextBefore == 0 {
		t.Fatalf("paged fixture %+v %v", page, err)
	}
	_, err = b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "entry-part", ID: first.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, AsRecommended: true}}})
	if err != nil {
		t.Fatal(err)
	}
	entries, err = b.DecisionEntries(ctx)
	if err != nil || entries.Count != 103 {
		t.Fatalf("partial count %+v %v", entries, err)
	}
	_, err = b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "entry-final", ID: first.ID, Revision: 2, Answers: []DecisionAnswer{{Number: 7, AsRecommended: true}}})
	if err != nil {
		t.Fatal(err)
	}
	entries, err = b.DecisionEntries(ctx)
	if err != nil || entries.Count != 102 {
		t.Fatalf("closed count %+v %v", entries, err)
	}
	for _, item := range entries.Items {
		if item.ID == first.ID {
			t.Fatal("closed request stayed indexed")
		}
	}
}

func TestDecisionEntriesMentionAndEmpty(t *testing.T) {
	b, s, first := personRequestFixture(t)
	ctx := context.Background()
	_, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "close-entry", ID: first.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, AsRecommended: true}, {Number: 7, AsRecommended: true}}})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := b.DecisionEntries(ctx)
	if err != nil || entries.Count != 0 || entries.Items == nil {
		t.Fatalf("empty %+v %v", entries, err)
	}
	// A plain direct message to the person does not become a Decisions row.
	if _, err := b.Send(ctx, Sender{Session: &s}, Human, "A result", nil, 0); err != nil {
		t.Fatal(err)
	}
	entries, err = b.DecisionEntries(ctx)
	if err != nil || entries.Count != 0 {
		t.Fatalf("direct result indexed %+v %v", entries, err)
	}
}
