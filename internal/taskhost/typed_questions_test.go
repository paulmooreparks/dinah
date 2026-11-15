package taskhost

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func textAnswer(s string) QuestionAnswer { return QuestionAnswer{Type: "text", Text: &s} }

func TestTypedQuestionBoundsAndSingleSettlement(t *testing.T) {
	for _, typ := range []string{"text", "multiple"} {
		t.Run(typ, func(t *testing.T) {
			q, asked, ended, scope := questionFixture(t)
			spec := QuestionSpec{Type: "text", MinLength: 1, MaxLength: 2}
			answer := textAnswer("猫😀")
			bad := []QuestionAnswer{textAnswer(""), textAnswer("abc"), textAnswer(string([]byte{255})), {Type: "multiple", OptionIDs: []string{"a"}}}
			if typ == "multiple" {
				spec = QuestionSpec{Type: "multiple", Options: []QuestionOption{{OptionID: "a", Name: "A"}, {OptionID: "b", Name: "B"}}, MinSelections: 1, MaxSelections: 2}
				answer = QuestionAnswer{Type: "multiple", OptionIDs: []string{"b", "a"}}
				bad = []QuestionAnswer{{Type: "multiple"}, {Type: "multiple", OptionIDs: []string{"a", "a"}}, {Type: "multiple", OptionIDs: []string{"foreign"}}, textAnswer("a")}
			}
			done := make(chan QuestionAnswer, 1)
			go func() {
				a, err := q.AskTyped(context.Background(), scope, "Invented question", spec)
				if err != nil {
					t.Error(err)
				}
				done <- a
			}()
			view := <-asked
			if view.AnswerType != typ {
				t.Fatal(view)
			}
			if err := q.Answer(view.Key, "a"); err == nil {
				t.Fatal("legacy entry accepted typed answer")
			}
			for _, invalid := range bad {
				if err := q.AnswerTyped(view.Key, invalid); err == nil || len(q.Pending()) != 1 {
					t.Fatal("invalid answer consumed the question", invalid, err)
				}
			}
			copy := q.Pending()
			if typ == "multiple" {
				copy[0].Options[0].OptionID = "forged"
			}
			var wg sync.WaitGroup
			var mu sync.Mutex
			accepted := 0
			for range 3 {
				wg.Go(func() {
					if q.AnswerTyped(view.Key, answer) == nil {
						mu.Lock()
						accepted++
						mu.Unlock()
					}
				})
			}
			wg.Wait()
			if accepted != 1 || len(q.Pending()) != 0 {
				t.Fatal("answer did not settle once", accepted)
			}
			got := <-done
			if typ == "text" && (got.Text == nil || *got.Text != "猫😀") {
				t.Fatal(got)
			}
			if typ == "multiple" && strings.Join(got.OptionIDs, ",") != "a,b" {
				t.Fatal(got)
			}
			if end := <-ended; end.Key != view.Key {
				t.Fatal(end)
			}
			select {
			case extra := <-ended:
				t.Fatal("duplicate settlement", extra)
			default:
			}
		})
	}
}

func TestTypedQuestionSchemasAndEmptyAnswers(t *testing.T) {
	for _, spec := range []QuestionSpec{{Type: "text"}, {Type: "text", MaxLength: MaxQuestionTextLength + 1}, {Type: "text", MinLength: 3, MaxLength: 2}, {Type: "text", MaxLength: 2, Options: []QuestionOption{{OptionID: "a", Name: "A"}}}, {Type: "multiple"}, {Type: "multiple", Options: []QuestionOption{{OptionID: "a", Name: "A"}}, MaxSelections: 2}} {
		q, _, _, scope := questionFixture(t)
		if _, err := q.AskTyped(context.Background(), scope, "Question", spec); err == nil {
			t.Fatal("invalid schema admitted", spec)
		}
	}
	for _, typ := range []string{"text", "multiple"} {
		q, asked, _, scope := questionFixture(t)
		spec, answer := QuestionSpec{Type: "text", MaxLength: 2}, textAnswer("")
		if typ == "multiple" {
			spec = QuestionSpec{Type: typ, Options: []QuestionOption{{OptionID: "a", Name: "A"}}, MaxSelections: 1}
			answer = QuestionAnswer{Type: typ}
		}
		done := make(chan QuestionAnswer, 1)
		go func() {
			got, err := q.AskTyped(context.Background(), scope, "Optional question", spec)
			if err != nil {
				t.Error(err)
			}
			done <- got
		}()
		view := <-asked
		if err := q.AnswerTyped(view.Key, answer); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(<-done)
		if err != nil || (typ == "multiple" && !strings.Contains(string(encoded), `"optionIds":[]`)) {
			t.Fatal(string(encoded), err)
		}
	}
}
