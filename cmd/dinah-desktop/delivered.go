package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// Messages the bus delivers into a chat (replies to its questions, other
// agents' messages, Paul's bus messages, release notices and verdicts) are
// kept in the chat's saved copy as entries of their own, with the role
// "delivered", one for each message, rather than as the person's words
// (Paul, 26 October 2026). The agent still receives the prompt the bus
// formats for it (bus.FormatForAgent); only the record and the page change.
// Chats saved before this keep their rows as they were.

// DeliveredMessage is what a delivered entry keeps of its message, beside
// the readable text in the entry's Text (bus.Describe). From is the sender,
// a chat by the title it had when the message came, and Workspace its
// workspace's label; FromSession is the sending chat's session on the bus,
// by which the page shows the chat's title as it is now (app.js).
type DeliveredMessage = sessionhost.DeliveredMessage

// deliveredRole is the role of a delivered entry in a saved conversation.
const deliveredRole = "delivered"

// deliveredUpdate is the update the page draws a delivered entry from,
// whether it comes from the agent's replay or from the saved copy. It is
// Dinah's own, not the agent protocol's.
const deliveredUpdate = "dinah_delivered"

// busPictureTypes are the picture types that go to an agent as image blocks,
// those a single chat's page shows as pictures (app.js, drawMessageImages).
var busPictureTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// busPictures returns the pictures the person attached to messages being
// delivered, as the attachments a single chat's pasted pictures are, so
// that the prompt's blocks make them image blocks (blocks), and a sentence
// telling the agent which they are. It returns none when the chat's agent
// does not take images, and then every file, pictures too, is only listed
// with its name, type and size (bus.FormatOne) for read_attachment. A
// picture whose copy has been removed or cannot be read is left listed.
//
// The chat's provider's limit on the files in one message (file_limits.go)
// bounds the pictures given as images: past it, a picture is left listed,
// and the sentence says so. The group's box keeps a message within the
// smallest limit among its members, so this matters when several messages
// are delivered in one prompt or a limit changed after the message was sent.
func (a *App) busPictures(s *Session, msgs []bus.Message) ([]Attachment, string) {
	if !a.promptCaps(s).Image {
		return nil, ""
	}
	limit := a.providerFileLimit(s.Provider())
	var out []Attachment
	var which, over []string
	for _, m := range msgs {
		for _, at := range m.Attachments {
			if !at.Stored || at.Removed || !busPictureTypes[at.Mime] || at.Size > maxAttachmentFile {
				continue
			}
			if limit > 0 && len(out) >= limit {
				over = append(over, fmt.Sprintf("attachment %d of message %d (%s)", at.Index, m.ID, at.Rel))
				continue
			}
			data, err := os.ReadFile(at.Path)
			if err != nil {
				logf("bus", "warn", "reading the picture %s of message %d: %v", at.Rel, m.ID, err)
				continue
			}
			out = append(out, Attachment{Kind: "file", Name: at.Rel, MimeType: at.Mime, Data: base64.StdEncoding.EncodeToString(data)})
			which = append(which, fmt.Sprintf("attachment %d of message %d (%s)", at.Index, m.ID, at.Rel))
		}
	}
	if len(out) == 0 {
		return nil, ""
	}
	note := "The pictures " + a.agentPerson() + " attached are in this prompt as images, in this order: " + strings.Join(which, ", ") + "."
	if len(over) > 0 {
		note += " This chat's provider takes at most " + countOfFiles(limit) + " in one message, so these pictures are not in this prompt, and read_attachment reads them: " + strings.Join(over, ", ") + "."
	}
	return out, note
}

// deliveredHeading says what a delivered entry is and who it is from, as
// the page's card does (app.js, deliveredHeading).
func deliveredHeading(d *DeliveredMessage) string {
	if d == nil {
		return "Delivered message"
	}
	from := speakerName(d.From)
	if d.Workspace != "" {
		from += " (workspace " + d.Workspace + ")"
	}
	switch d.Kind {
	case "result":
		if d.From == "" {
			return "Completion report from " + product.Name
		}
		return "Reply from " + from
	case "release":
		return "Release notice from " + from
	case "verdict":
		return "Verdict from " + from
	}
	return "Message from " + from
}

// conversationSpeaker is who a saved message is from, as a handoff and its
// summary attribute it: a delivered message to its sender, never to the
// person.
func conversationSpeaker(m ConversationMessage) string {
	if m.Role == deliveredRole {
		who := "delivered by the bus, " + deliveredHeading(m.Delivered)
		if m.Delivered != nil {
			if t, err := time.Parse(time.RFC3339Nano, m.Delivered.Time); err == nil {
				who += ", sent " + t.UTC().Format("2 Jan 2006 15:04 UTC")
			}
		}
		return who
	}
	return m.Role + ", " + m.Provider
}

// holdReplayed keeps a replayed user update while its message is held, and
// reports whether it did.

// sentKind is the kind of the agent:sent event for a prompt: bus for a
// prompt that delivers bus messages, which the page draws as their cards,
// such as another chat's question (chat_ask.go), and otherwise fallback.
func sentKind(p *prompt, fallback string) string { return sessionhost.SentKind(p, fallback) }
func sentEvent(id, text, kind, label string, entries []ConversationMessage) any {
	return sessionhost.SentEvent(id, text, kind, label, entries)
}
