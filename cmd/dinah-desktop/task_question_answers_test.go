package main

import (
	"strings"
	"testing"
)

func TestTypedQuestionStrictWireDecoder(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"type":"text"}`, `{"type":"text","text":null}`,
		`{"type":"text","text":"x","person":true}`, `{"type":"text","text":"x","text":"y"}`,
		`{"type":"text","text":"\ud800"}`, `{"type":"text","text":"\udc00"}`,
		`{"type":"text","text":"\ud800\u0041"}`, `{"type":"text","text":"x"} {}`,
		`{"type":"multiple","optionIds":null}`, `{"type":"multiple","optionIds":[1]}`,
		`{"type":"multiple","optionIds":[],"text":"x"}`,
		string([]byte{'{', '"', 't', 'y', 'p', 'e', '"', ':', '"', 't', 'e', 'x', 't', '"', ',', '"', 't', 'e', 'x', 't', '"', ':', '"', 255, '"', '}'}),
		strings.Repeat(" ", 131073) + `{}`,
	} {
		if _, err := decodeTaskQuestionAnswer(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range []string{
		`{"type":"text","text":""}`, `{"type":"text","text":"\ud83d\ude00"}`,
		`{"type":"text","text":"\\ud800 \"ok\""}`,
		` { "type" : "multiple", "optionIds" : [] } `,
	} {
		if _, err := decodeTaskQuestionAnswer(raw); err != nil {
			t.Fatalf("refused %q: %v", raw, err)
		}
	}
	body := `{"type":"text","text":"ok"}`
	if _, err := decodeTaskQuestionAnswer(body + strings.Repeat(" ", 131072-len(body))); err != nil {
		t.Fatal(err)
	}
}

func TestTypedQuestionStrictDeclarationDecoder(t *testing.T) {
	for _, raw := range []string{
		`{"title":"Q","answer":{"type":"text","maxLength":4,"minSelections":0}}`,
		`{"title":"Q","answer":{"type":"text","maxLength":4,"extra":true}}`,
		`{"title":"Q","answer":{"type":"text","maxLength":4,"maxLength":5}}`,
		`{"title":"Q","answer":null}`, `{"title":"Q","choices":[],"answer":{"type":"text","maxLength":4}}`,
		`{"title":"Q","answer":{"type":"text","maxLength":1.5}}`,
	} {
		if _, err := decodeTypedTaskQuestion([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
