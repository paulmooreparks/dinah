package sessionhost

import (
	_ "embed"
	"slices"

	"studio/internal/acp"
)

//go:embed copies.go
var CopiesSource []byte

func copyContent(in []acp.ContentBlock) []acp.ContentBlock {
	out := slices.Clone(in)
	for i := range out {
		if out[i].Resource != nil {
			resource := *out[i].Resource
			out[i].Resource = &resource
		}
	}
	return out
}
func copyAttachments(in []Attachment) []Attachment {
	out := slices.Clone(in)
	for i := range out {
		if out[i].Ref != nil {
			ref := *out[i].Ref
			out[i].Ref = &ref
		}
	}
	return out
}
func copyMessages(in []ConversationMessage) []ConversationMessage {
	out := slices.Clone(in)
	for i := range out {
		out[i].Images = copyContent(out[i].Images)
		if out[i].Delivered != nil {
			delivered := *out[i].Delivered
			out[i].Delivered = &delivered
		}
	}
	return out
}
