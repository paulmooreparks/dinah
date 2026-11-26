package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"studio/internal/bus"
)

func TestSendReceiptExplainsOnlyPendingHopLimitedDirectMessages(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		hops, limit                int
		link, self, group, warning bool
	}{
		{name: "within-limit", hops: 3},
		{name: "past-limit", hops: 4, warning: true},
		{name: "custom-within-limit", hops: 4, link: true, limit: 4},
		{name: "custom-past-limit", hops: 4, link: true, limit: 2, warning: true},
		{name: "unlimited-link", hops: 4, link: true, limit: bus.NoHopLimit},
		{name: "self-link", hops: 4, link: true, self: true, limit: 10},
		{name: "group-post", hops: 4, group: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			b, err := bus.OpenAt(filepath.Join(root, "bus.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			alpha, beta := filepath.Join(root, "alpha"), filepath.Join(root, "beta")
			for _, p := range []string{alpha, beta} {
				if err := os.MkdirAll(p, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.self {
				beta = alpha
			}
			a, err := b.RegisterSession(ctx, "window-alpha", alpha, "Alpha")
			if err != nil {
				t.Fatal(err)
			}
			z, err := b.RegisterSession(ctx, "window-beta", beta, "Beta")
			if err != nil {
				t.Fatal(err)
			}
			if tc.link {
				if err := b.Trust(ctx, a.WorkspaceName, z.WorkspaceName, tc.limit); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := b.Send(ctx, bus.Sender{}, a.Address(), "Start", nil, 0); err != nil {
				t.Fatal(err)
			}
			pair := []bus.Session{a, z}
			var sent bus.Message
			var recipient bus.Session
			for hop := 1; hop <= tc.hops; hop++ {
				from, to := pair[(hop-1)%2], pair[hop%2]
				if _, _, err := b.Deliver(ctx, from.ID, "PostToolUse"); err != nil {
					t.Fatal(err)
				}
				target := to.Address()
				if tc.group && hop == tc.hops {
					g, err := b.CreateGroup(ctx, bus.Sender{}, "Review", []string{a.Address(), z.Address()})
					if err != nil {
						t.Fatal(err)
					}
					target = "#" + g.Name
				}
				sent, err = b.Send(ctx, bus.Sender{Session: &from}, target, "Result", nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				recipient = to
			}
			before, err := b.ChangeCounter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			got := New(b).sentReceipt(ctx, sent).Content[0].(*mcp.TextContent).Text
			if strings.Contains(got, "automatic handoffs") != tc.warning {
				t.Fatalf("receipt=%s; warning=%v", got, tc.warning)
			}
			if tc.warning {
				limit := bus.MaxHops
				if tc.link {
					limit = tc.limit
				}
				if !strings.Contains(got, fmt.Sprintf("limit of %d automatic handoffs", limit)) {
					t.Fatalf("incorrect limit: %s", got)
				}
				for _, text := range []string{recipient.Title + " will not be woken", "group chat you share", "ask_person", "trusted link", "Group Chats"} {
					if !strings.Contains(got, text) {
						t.Errorf("receipt omits %q: %s", text, got)
					}
				}
			}
			if after, err := b.ChangeCounter(ctx); err != nil || after != before {
				t.Fatalf("receipt mutated bus: %d %v", after, err)
			}
		})
	}
}
