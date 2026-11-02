package bus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/fileaccess"
)

func TestAttachmentPolicyRefusesDeniedSendAndLaterRead(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	workspace := workspace(t, root, "source")
	sender, err := b.RegisterSession(ctx, "sender", workspace, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := b.RegisterSession(ctx, "reader", workspace, "Reader")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "sensitive.txt")
	if err := os.WriteFile(path, []byte("synthetic-only"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := b.fileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	scope, err := b.attachmentScope(ctx, sender, "bus.attach")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := service.Policy(scope)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = service.Add(path, policy.Version, scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(ctx, Sender{Session: &sender}, reader.ID, "link", []string{path}, 0); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatal("denied path attached", err)
	}
	log, err := service.Store.Log(fileaccess.Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].Source != "bus.attach" {
		t.Fatalf("send audit: %+v %v", log, err)
	}
	custom := ""
	for _, rule := range policy.Rules {
		if rule.Kind == "custom" {
			custom = rule.ID
		}
	}
	policy, err = service.Remove(custom, policy.Version, scope)
	if err != nil {
		t.Fatal(err)
	}
	message, err := b.Send(ctx, Sender{Session: &sender}, reader.ID, "link", []string{path}, 0)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = service.Add(path, policy.Version, scope)
	if err != nil {
		t.Fatal(err)
	}
	if contents, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1); err == nil || len(contents.Data) != 0 {
		t.Fatal("previous live link bypassed updated policy")
	}
	log, err = service.Store.Log(fileaccess.Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) != 2 || log.Rows[0].Source != "read_attachment" || log.Rows[0].Chat != reader.ID {
		t.Fatalf("read audit: %+v %v", log, err)
	}
	if saved, err := b.Message(ctx, message.ID); err != nil || len(saved.Attachments) != 1 || saved.Attachments[0].Removed {
		t.Fatal("refusal changed original attachment")
	}
}

func TestAttachmentPolicyRejectsSymlinkEscapeAndKeepsLiveChanges(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	workspace := workspace(t, root, "source")
	sender, _ := b.RegisterSession(ctx, "sender", workspace, "Sender")
	reader, _ := b.RegisterSession(ctx, "reader", workspace, "Reader")
	path := filepath.Join(workspace, "live.txt")
	os.WriteFile(path, []byte("before"), 0600)
	message, err := b.Send(ctx, Sender{Session: &sender}, reader.ID, "link", []string{path}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1)
	if err != nil || !changed.Changed || changed.Text != "after" || changed.CurrentSHA256 == message.Attachments[0].SHA256 {
		t.Fatalf("live reporting changed: %+v %v", changed, err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")
	os.WriteFile(outside, []byte("outside"), 0600)
	link := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(ctx, Sender{Session: &sender}, reader.ID, "link", []string{link}, 0); err == nil {
		t.Fatal("symlink escape attached")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if contents, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1); err == nil || len(contents.Data) != 0 {
		t.Fatal("live link followed an escaped replacement")
	}
}

func TestAttachmentPolicyDefaultsProtectOtherWorkspaces(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	one := workspace(t, root, "one")
	two := workspace(t, root, "two")
	sender, _ := b.RegisterSession(ctx, "sender", one, "Sender")
	reader, _ := b.RegisterSession(ctx, "reader", two, "Reader")
	service, err := b.fileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	for i := range service.Defaults {
		if service.Defaults[i].ID == "other-workspaces" {
			service.Defaults[i].Enabled = true
		}
	}
	path := filepath.Join(one, "live.txt")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	message, err := b.Send(ctx, Sender{Session: &sender}, reader.ID, "link", []string{path}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1); err != nil || got.Text != "synthetic" {
		t.Fatalf("sender-owned link: %+v %v", got, err)
	}
	if got, err := b.readAttachmentWithPolicy(ctx, reader, message.Attachments[0]); err == nil || len(got) != 0 {
		t.Fatal("a raw pathname received the sharing exception")
	}
	log, err := service.Store.Log(fileaccess.Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].RuleID != "other-workspaces" || log.Rows[0].Chat != reader.ID {
		t.Fatalf("audit: %+v %v", log, err)
	}
	if got, err := b.ReadAttachment(ctx, sender.ID, message.ID, 1); err != nil || got.Text != "synthetic" {
		t.Fatalf("owner read: %+v %v", got, err)
	}
}

func TestAttachmentPolicyStoredCopiesStayInCatalogueAndAudit(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	reader, _ := b.RegisterSession(ctx, "reader", workspace(t, root, "one"), "Reader")
	message, err := b.SendFiles(ctx, "Reader", "copy", []PersonFile{{Name: "copy.txt", Data: []byte("supplied")}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	attachment := message.Attachments[0]
	if got, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1); err != nil || got.Text != "supplied" {
		t.Fatalf("copy: %+v %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("not supplied"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(attachment.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, attachment.Path); err != nil {
		t.Fatal(err)
	}
	if got, err := b.ReadAttachment(ctx, reader.ID, message.ID, 1); err == nil || len(got.Data) != 0 {
		t.Fatal("copy followed symlink")
	}
	attachment.Path = outside
	if got, err := b.readAttachmentWithPolicy(ctx, reader, attachment); err == nil || len(got) != 0 {
		t.Fatal("copy escaped catalogue")
	}
	service, _ := b.fileAccessPolicy()
	log, err := service.Store.Log(fileaccess.Filter{})
	if err != nil || len(log.Rows) != 3 || log.Rows[0].Outcome != "error" || log.Rows[1].Outcome != "error" || log.Rows[2].Outcome != "allowed" || log.Rows[2].Bytes != len("supplied") {
		t.Fatalf("copy audit: %+v %v", log, err)
	}
	for _, row := range log.Rows {
		if row.Source != "read_attachment" || row.Chat != reader.ID {
			t.Fatal("copy audit attribution", row)
		}
	}
}
