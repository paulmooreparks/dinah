package requestsettlement

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSettlementInputPreservesUnicodeAndRejectsMalformedAuthority(t *testing.T) {
	for _, id := range []string{"x", strings.Repeat("😀", 128), `literal \uD800`} {
		raw, err := json.Marshal(map[string]any{"operationId": id, "id": "request-1", "revision": 1})
		if err != nil {
			t.Fatal(err)
		}
		var in Input
		if err := json.Unmarshal(raw, &in); err != nil || in.OperationID != id {
			t.Fatal("valid operation identity changed", in, err)
		}
	}
	valid := `{"operationId":"once","id":"request-1","revision":1}`
	for _, raw := range []string{
		`{"operationId":"\ud800","id":"request-1","revision":1}`,
		`{"operationId":"\udc00","id":"request-1","revision":1}`,
		`{"operationId":"once","id":"\ud800","revision":1}`,
		strings.Replace(valid, `"once"`, `null`, 1),
		strings.Replace(valid, `"once"`, `4`, 1),
		strings.Replace(valid, `"once"`, `""`, 1),
		strings.Replace(valid, `"once"`, `"`+strings.Repeat("😀", 129)+`"`, 1),
		strings.Replace(valid, `"revision":1`, `"revision":null`, 1),
		strings.Replace(valid, `"revision":1`, `"revision":1.0`, 1),
		strings.Replace(valid, `"revision":1`, `"revision":0`, 1),
		strings.Replace(valid, `"revision":1`, `"revision":1,"revision":2`, 1),
		strings.Replace(valid, `"revision":1`, `"revision":1,"person":true`, 1),
		`null`, `[]`, valid + `{}`, string([]byte{'"', 0xff, '"'}),
	} {
		original := Input{ID: "preserved", Revision: 8, OperationID: "preserved"}
		in := original
		if err := json.Unmarshal([]byte(raw), &in); err == nil || in != original {
			t.Fatalf("malformed input accepted or receiver changed: %q %+v %v", raw, in, err)
		}
	}
	var in Input
	if err := json.Unmarshal([]byte(`{"operationId":"\ud83d\ude00","id":"request-1","revision":1}`), &in); err != nil || in.OperationID != "😀" {
		t.Fatal("paired scalar refused", in, err)
	}
}
