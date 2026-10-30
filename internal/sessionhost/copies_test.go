package sessionhost

import (
	"reflect"
	"testing"

	"studio/internal/acp"
	"studio/internal/workspaces"
)

func TestNestedConversationCopiesAreIsolated(t *testing.T) {
	content := []acp.ContentBlock{{Type: "resource", Resource: &acp.EmbeddedResource{Text: "original"}}}
	messages := []ConversationMessage{{Text: "original", Images: content, Delivered: &DeliveredMessage{Prompt: "original"}}}
	original := NewPrompt("question", nil).WithDelivery("label", messages, nil)
	copied := original.Delivered()
	copied[0].Text = "changed"
	copied[0].Images[0].Resource.Text = "changed"
	copied[0].Delivered.Prompt = "changed"
	if messages[0].Text != "original" || messages[0].Images[0].Resource.Text != "original" || messages[0].Delivered.Prompt != "original" {
		t.Fatal("copy changed original delivery", messages)
	}
	messages[0].Images[0].Resource.Text = "source changed"
	if copied[0].Images[0].Resource.Text != "changed" {
		t.Fatal("source changed copy")
	}
	if got := original.Delivered()[0]; got.Text != "original" || got.Images[0].Resource.Text != "original" || got.Delivered.Prompt != "original" {
		t.Fatal("source or view changed private prompt storage", got)
	}
	attachments := []Attachment{{Ref: &workspaces.Ref{Path: "original"}}}
	picturePrompt := NewPrompt("picture", nil).WithDelivery("label", nil, attachments)
	copiedAttachments := picturePrompt.Pictures()
	copiedAttachments[0].Ref.Path = "changed"
	if attachments[0].Ref.Path != "original" {
		t.Fatal("copy changed attachment reference")
	}
	attachments[0].Ref.Path = "source changed"
	if picturePrompt.Pictures()[0].Ref.Path != "original" {
		t.Fatal("attachment aliases private prompt storage")
	}
}

func TestCopiesPreserveNilAndEmptySlices(t *testing.T) {
	for _, empty := range []bool{false, true} {
		var content []acp.ContentBlock
		var attachments []Attachment
		var messages []ConversationMessage
		if empty {
			content = []acp.ContentBlock{}
			attachments = []Attachment{}
			messages = []ConversationMessage{}
		}
		if !reflect.DeepEqual(content, copyContent(content)) || !reflect.DeepEqual(attachments, copyAttachments(attachments)) || !reflect.DeepEqual(messages, copyMessages(messages)) {
			t.Fatalf("slice representation changed (empty=%v)", empty)
		}
	}
}
