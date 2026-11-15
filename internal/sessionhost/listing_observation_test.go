package sessionhost

import (
	"errors"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestListingObservationCompleteness(t *testing.T) {
	for _, mode := range []string{"complete", "error", "repeated", "bound"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var runtime Runtime
			var init acp.InitializeResult
			init.AgentCapabilities.SessionCapabilities.List = &struct{}{}
			calls, finished := 0, 0
			host := SessionLifecycleHost{
				ListProviders: func() []string { return []string{"devin"} },
				Live:          func(string) (Connection, acp.InitializeResult, error) { return nil, init, nil },
				Hidden:        func(string) bool { return false }, Title: func(_, title string) string { return title }, FilterAncestors: func(rows []Info, _ string) []Info { return rows },
				BeginListing: func(string, string) (uint64, error) { return 7, nil },
				CallWithin: func(_ Connection, _ time.Duration, _ string, _ any, out any) error {
					calls++
					if mode == "error" && calls == 2 {
						return errors.New("invented page failure")
					}
					next := "same"
					if mode == "complete" {
						next = ""
					}
					if mode == "bound" {
						next = time.Duration(calls).String()
					}
					return acp.Result(acp.ListSessionsResult{Sessions: []acp.SessionInfo{{SessionID: "one", Cwd: t.TempDir()}}, NextCursor: next}, out)
				},
				FinishListing: func(_, _ string, sequence uint64, rows []acp.SessionInfo, complete bool) error {
					finished++
					if sequence != 7 || len(rows) == 0 || complete != (mode == "complete") {
						t.Fatalf("wrong completion %d %+v %v", sequence, rows, complete)
					}
					return nil
				},
			}
			if _, err := NewSessionLifecycle(&mu, &runtime, host).List(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if finished != 1 || (mode == "bound" && calls != 5) || (mode == "repeated" && calls != 2) {
				t.Fatalf("calls %d finish %d", calls, finished)
			}
		})
	}
}
