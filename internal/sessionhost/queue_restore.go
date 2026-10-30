package sessionhost

import (
	"strings"
)

// Restoring the queued messages that were kept on disk (queue_store.go). When a chat
// is opened its file goes back in its queue ahead of anything new; the hold of a
// full window is not kept, since the first restored message goes through the same
// check as any send. A chat that was continued while Dinah was down sends its
// messages to its continuation, once. The messages of a deleted chat are discarded
// and the person is told what they said. A file that cannot be read is set aside
// and the person is told messages may have been lost.

// QueueRestoreHost is what restoring needs of the window.
type QueueRestoreHost struct {
	// Head is the chat a chat was continued as, or its own ID, or "" when that cannot
	// be found.
	Head func(id string) string
	// Deleted says the chat's saved history is marked deleted.
	Deleted func(id string) bool
	// Started says the chat's saved history records that the queued message with the
	// id has started its turn, which is how a message carried to a chat and run there
	// is known when the earlier chat's file was not yet settled.
	Started func(chat, queueID string) bool
	// Session is the open chat with the ID, or nil.
	Session func(id string) *Conversation
	// Send gives an open chat a prompt as a message sent to it would be.
	Send func(id string, p Prompt) error
	// Discarded tells the person what the messages of a deleted chat said.
	Discarded func(id string, texts []string)
	// Damaged tells the person a chat's file could not be read, so messages may
	// have been lost.
	Damaged func(id string, err error)
	Log     func(level, format string, args ...any)
}

// QueueRestorer restores kept queues.
type QueueRestorer struct {
	store *QueueStore
	host  QueueRestoreHost
}

// NewQueueRestorer restores from store.
func NewQueueRestorer(store *QueueStore, host QueueRestoreHost) *QueueRestorer {
	return &QueueRestorer{store: store, host: host}
}

func (r *QueueRestorer) log(level, format string, args ...any) {
	if r.host.Log != nil {
		r.host.Log(level, format, args...)
	}
}

// Restore puts a chat's kept messages back in its queue, when the chat has just been
// opened and before it takes a turn.
func (r *QueueRestorer) Restore(s *Conversation) {
	id := s.ID()
	stored, err := r.store.Load(id)
	if err != nil {
		r.log("error", "the queued messages of chat %s could not be read, so the file was set aside and the chat opens with none: %v", id, err)
		if r.host.Damaged != nil {
			r.host.Damaged(id, err)
		}
		r.store.Arm(id)
		return
	}
	if len(stored) == 0 {
		r.store.Arm(id)
		return
	}
	if head := r.host.Head(id); head != "" && head != id {
		r.carryOn(id, head, stored)
		return
	}
	prompts := make([]Prompt, len(stored))
	for i, sp := range stored {
		prompts[i] = PromptFromStored(sp)
	}
	s.RestoreQueued(prompts)
	r.store.Arm(id)
	r.log("info", "%d queued messages of chat %s are back in its queue", len(prompts), id)
}

// Sweep deals with the files of chats that are not open and cannot wait to be opened:
// those continued and those deleted. A closed chat's file otherwise waits until the
// person opens it.
func (r *QueueRestorer) Sweep() {
	for _, id := range r.store.Chats() {
		if r.host.Session(id) != nil {
			continue
		}
		switch {
		case r.host.Deleted(id):
			r.discard(id)
		default:
			if head := r.host.Head(id); head != "" && head != id {
				stored, err := r.store.Load(id)
				if err != nil {
					r.log("error", "the queued messages of chat %s could not be read, so the file was set aside: %v", id, err)
					if r.host.Damaged != nil {
						r.host.Damaged(id, err)
					}
					continue
				}
				r.carryOn(id, head, stored)
			}
		}
	}
}

// discard drops the messages of a deleted chat, and says what they said.
func (r *QueueRestorer) discard(id string) {
	stored, err := r.store.Load(id)
	if err != nil {
		r.log("error", "the queued messages of deleted chat %s could not be read: %v", id, err)
		return
	}
	var texts []string
	for _, sp := range stored {
		if t := strings.TrimSpace(sp.Text); t != "" {
			texts = append(texts, t)
		}
	}
	r.store.Forget(id)
	if len(stored) == 0 {
		return
	}
	r.log("warning", "%d queued messages of deleted chat %s were discarded", len(stored), id)
	if r.host.Discarded != nil {
		r.host.Discarded(id, texts)
	}
}

// started says a message has already started its turn in the chat.
func (r *QueueRestorer) started(chat, queueID string) bool {
	return queueID != "" && r.host.Started != nil && r.host.Started(chat, queueID)
}

// carryOn sends the messages of a chat that was continued to its continuation, those
// it does not already have, and ends the old chat's file.
func (r *QueueRestorer) carryOn(id, head string, stored []StoredPrompt) {
	if open := r.host.Session(head); open != nil {
		have := open.QueuedIDs()
		for _, sp := range stored {
			if have[sp.ID] || r.started(head, sp.ID) {
				continue
			}
			if err := r.host.Send(head, PromptFromStored(sp)); err != nil {
				// The continuation could not take it now; it is kept in its file.
				r.store.AppendFile(head, []StoredPrompt{sp})
			}
		}
	} else {
		var fresh []StoredPrompt
		for _, sp := range stored {
			if !r.started(head, sp.ID) {
				fresh = append(fresh, sp)
			}
		}
		r.store.AppendFile(head, fresh)
	}
	r.store.Forget(id)
	r.log("info", "%d queued messages of chat %s went to its continuation %s", len(stored), id, head)
}
