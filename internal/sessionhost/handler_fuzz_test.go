package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"studio/internal/acp"
	"testing"
)

func FuzzReverseRequestParameters(f *testing.F) {
	for _, method := range []string{"fs/read_text_file", "fs/write_text_file", "session/request_permission"} {
		for _, raw := range []string{"null", "[]", "{", `{"sessionId":"fake","path":"bad","line":-1,"limit":-5}`, `{"line":9223372036854775808}`} {
			f.Add(method, []byte(raw))
		}
	}
	f.Fuzz(func(t *testing.T, method string, raw []byte) {
		if len(raw) > 64<<10 {
			t.Skip()
		}
		h := NewHandler(HandlerHost{
			ProductName: "test", RedactContexts: func(s string) string { return s },
			ResolveRequestChat: func(string) (*Conversation, *SubagentInfo) { return nil, nil },
			AskPermission: func(context.Context, string, json.RawMessage, []acp.PermissionOption) (string, bool) {
				return "", false
			},
			WriteTextFile: func(string, string) error { return errors.New("denied") },
		})
		_, _ = h.Request(context.Background(), method, json.RawMessage(raw))
	})
}
