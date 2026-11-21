package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"studio/internal/bus"
)

// An agent that keeps one store of threads for every program that uses it
// shows Dinah's chats in its own app too. This file keeps Dinah's chats tidy
// there, with what the agent's adapter offers (docs/design/
// codex-thread-housekeeping.md):
//
//   - A chat's thread has the chat's title (nameAgentThread).
//   - The thread of a chat Dinah no longer uses is archived: the earlier
//     stage of a chat that continues in a new thread, and a chat in the
//     Recycle Bin (archiveAgentThread). It is taken out of the archive
//     before the chat is loaded again (unarchiveBeforeLoad).
//   - With the person's setting, the thread of a chat whose session was
//     ended is archived as well (chatSessionEnded), and the session of an
//     idle chat is ended when its last window is closed (chatWindowClosed).
//
// Each of these is an optional capability of a provider's connection, which
// shared code asks for by interface, as it does for a receipt of a new
// session (allocation_native.go). A provider without the capability is left
// alone. A failure is logged and changes nothing else, since none of this is
// needed for a chat to work.

// threadNamer is a connection that can give a session's thread a title.
type threadNamer interface {
	SetSessionTitle(ctx context.Context, sessionID, title string) error
}

// threadArchiver is a connection that can archive a session's thread and
// take it out of the archive.
type threadArchiver interface {
	ArchiveSession(ctx context.Context, sessionID string) error
	UnarchiveSession(ctx context.Context, sessionID string) error
}

// ephemeralSessions is a connection that says which of its sessions it keeps
// in memory only.
type ephemeralSessions interface {
	SessionEphemeral(sessionID string) bool
}

// agentThreadPatience is how long the agent is given for one of these calls.
const agentThreadPatience = 10 * time.Second

// liveThreadConnection is the running connection of a provider, or nil. None
// of the work here starts an agent.
func (a *App) liveThreadConnection(provider string) agentConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.providerConnLocked(provider)
}

// chatThreadTitle is the title a chat's thread is given: the name the person
// gave the chat, or else the title a judge wrote for it. A chat with neither
// is shown by the agent's own title, so there is nothing to give it.
func (a *App) chatThreadTitle(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if title := a.settings.SessionTitles[id]; title != "" {
		return title
	}
	return a.settings.JudgedTitles[id].Title
}

// nameAgentThread gives the chat's thread the chat's title, where the chat's
// agent is running and can name a thread. It is called when the title is
// given or changed and when the chat has been loaded. The adapter sends
// nothing for a loaded thread that already has the title.
func (a *App) nameAgentThread(id string) {
	title := a.chatThreadTitle(id)
	if title == "" {
		return
	}
	namer, ok := a.liveThreadConnection(providerForSession(id)).(threadNamer)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentThreadPatience)
	defer cancel()
	if err := namer.SetSessionTitle(ctx, id, title); err != nil {
		logf("chat", "warning", "the thread of chat %s could not be given the chat's title: %v", id, err)
	}
}

// archiveAgentThread archives the thread of a chat Dinah no longer uses, and
// records that it did. why completes the log line "the thread of chat X,
// which ...". With the agent not running, the chat is recorded as waiting for
// it, and archivePendingThreads does the work once the agent runs.
func (a *App) archiveAgentThread(provider, id, why string) {
	provider = normalizedProvider(provider)
	if id == "" || !providerModules[provider].ArchivesThreads {
		return
	}
	a.agentThreadMu.Lock()
	defer a.agentThreadMu.Unlock()
	a.mu.Lock()
	done := a.settings.ArchivedThreads[id]
	conn := a.providerConnLocked(provider)
	a.mu.Unlock()
	if done {
		return
	}
	if conn == nil {
		a.recordThreadArchive(id, false, true)
		logf("chat", "info", "the thread of chat %s, which %s, is archived once %s runs", id, why, providerName(provider))
		return
	}
	archiver, ok := conn.(threadArchiver)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentThreadPatience)
	defer cancel()
	if err := archiver.ArchiveSession(ctx, id); err != nil {
		// A request that was not answered in time may still be carried out,
		// so the thread is recorded as archived and is taken out of the
		// archive before its chat is loaded. A request the agent refused
		// left the thread as it was.
		a.recordThreadArchive(id, errors.Is(err, context.DeadlineExceeded), false)
		logf("chat", "warning", "the thread of chat %s, which %s, could not be archived: %v", id, why, err)
		return
	}
	a.recordThreadArchive(id, true, false)
	logf("chat", "info", "archived the thread of chat %s, which %s", id, why)
}

// recordThreadArchive keeps what is known of a chat's thread: archived by
// Dinah, waiting to be archived, or neither.
func (a *App) recordThreadArchive(id string, archived, waiting bool) {
	err := a.updateSettings(func(s *Settings) error {
		if archived {
			if s.ArchivedThreads == nil {
				s.ArchivedThreads = map[string]bool{}
			}
			s.ArchivedThreads[id] = true
		} else {
			delete(s.ArchivedThreads, id)
		}
		if waiting {
			if s.ThreadsToArchive == nil {
				s.ThreadsToArchive = map[string]bool{}
			}
			s.ThreadsToArchive[id] = true
		} else {
			delete(s.ThreadsToArchive, id)
		}
		return nil
	})
	if err != nil {
		logf("chat", "warning", "what was done with the thread of chat %s could not be recorded: %v", id, err)
	}
}

// unarchiveAgentThread takes a chat's thread out of the archive when Dinah
// archived it, and drops a wait to archive it. A thread that could not be
// taken out stays recorded, so the next load asks again; that includes a
// call with no connection, when the agent is not running.
func (a *App) unarchiveAgentThread(conn agentConnection, id string) {
	a.agentThreadMu.Lock()
	defer a.agentThreadMu.Unlock()
	a.mu.Lock()
	archived, waiting := a.settings.ArchivedThreads[id], a.settings.ThreadsToArchive[id]
	a.mu.Unlock()
	if !archived {
		if waiting {
			a.recordThreadArchive(id, false, false)
		}
		return
	}
	archiver, ok := conn.(threadArchiver)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentThreadPatience)
	defer cancel()
	if err := archiver.UnarchiveSession(ctx, id); err != nil {
		logf("chat", "warning", "the thread of chat %s could not be taken out of the archive, and the chat is loaded all the same: %v", id, err)
		return
	}
	a.recordThreadArchive(id, false, false)
	logf("chat", "info", "took the thread of chat %s out of the archive", id)
}

// unarchiveBeforeLoad is the session engine's BeforeLoad: a thread Dinah
// archived is taken out of the archive before its chat is loaded, whatever
// the load is for, so a chat with messages waiting opens as any other.
func (a *App) unarchiveBeforeLoad(conn agentConnection, provider, id string) {
	a.unarchiveAgentThread(conn, id)
}

// chatUnused says why Dinah no longer uses a chat's thread, or "" when it
// does: the chat continues in a new thread, or it is in the Recycle Bin.
func chatUnused(s *Settings, id string) string {
	if r, ok := s.RemovedChats[id]; ok && (r.Stage == "retaining" || r.Stage == "retained") {
		return "is in the Recycle Bin"
	}
	if s.ChatBindings[id] != "" {
		return "continues in a new thread"
	}
	return ""
}

// archivePendingThreads archives the threads that waited for the provider to
// run. A chat that is in use again meanwhile is dropped from the wait.
func (a *App) archivePendingThreads(provider string) {
	provider = normalizedProvider(provider)
	a.mu.Lock()
	waiting := map[string]string{}
	for id := range a.settings.ThreadsToArchive {
		if providerForSession(id) == provider {
			waiting[id] = chatUnused(&a.settings, id)
		}
	}
	a.mu.Unlock()
	for id, why := range waiting {
		if why == "" {
			a.recordThreadArchive(id, false, false)
			continue
		}
		a.archiveAgentThread(provider, id, why)
	}
}

// chatSessionEnded is called when a chat's session has been closed at the
// agent and let go here (sessionhost.ClosedSessions). The thread is archived
// when Dinah no longer uses the chat, or when the person's setting for the
// provider archives the thread of every chat that was closed.
func (a *App) chatSessionEnded(s *Session) {
	provider, id := normalizedProvider(s.Provider()), s.ID()
	m := providerModules[provider]
	a.mu.Lock()
	why := chatUnused(&a.settings, id)
	if why == "" && m.ArchiveClosed != nil && m.ArchiveClosed(a.settings) {
		why = "was closed"
	}
	a.mu.Unlock()
	if why != "" {
		a.archiveAgentThread(provider, id, why)
	}
}

// windowCloseSettleDefault is how long a chat whose last window was closed is
// left before its session is ended early. The page reports its windows at
// every change, and a page that is loading again reports them one by one as
// it restores them, so a chat can be missing from one report and back in the
// next. The wait keeps such a chat from being ended and loaded again.
const windowCloseSettleDefault = 5 * time.Second

// archivesClosed says the person's settings archive the thread of a closed
// chat of this provider (ArchiveClosed in the provider's module).
func (a *App) archivesClosed(provider string) bool {
	m := providerModules[normalizedProvider(provider)]
	if !m.ArchivesThreads || m.ArchiveClosed == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return m.ArchiveClosed(a.settings)
}

// chatWindowClosed is called when the person has closed a chat's last window.
// Where the person's settings archive the thread of a closed chat, the chat's
// session is ended now instead of after closedSessionEnd, when the chat is
// idle, and the end archives the thread (chatSessionEnded). This is the
// trial of docs/design/codex-thread-housekeeping.md, section 3b: it gives up
// the quick reopening of a chat that was just closed, which keeping its
// session loaded is for, so that the thread leaves the agent's list at once.
//
// With the setting off this does nothing. With it on the work runs in a
// goroutine, so closing a window never waits for it, and a failure is logged
// only. A chat that is working, or that anything else keeps loaded, is left
// to the ordinary path, which ends its session when it has been closed for
// closedSessionEnd.
func (a *App) chatWindowClosed(id string) {
	s := a.session(id)
	if s == nil || !a.archivesClosed(s.Provider()) {
		return
	}
	a.windowCloseMu.Lock()
	if a.windowCloseStop == nil {
		a.windowCloseStop = make(chan struct{})
	}
	stop := a.windowCloseStop
	select {
	case <-stop:
		a.windowCloseMu.Unlock()
		return
	default:
	}
	a.windowCloses.Add(1)
	a.windowCloseMu.Unlock()
	settle := a.windowCloseSettle
	if settle == 0 {
		settle = windowCloseSettleDefault
	}
	go func() {
		defer a.windowCloses.Done()
		if settle > 0 {
			timer := time.NewTimer(settle)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-stop:
				return
			}
		}
		a.endClosedChatNow(id)
	}()
}

// endClosedChatNow ends the session of a chat whose window was closed, when
// the setting still asks for it and nothing waits for the chat. The rules for
// what keeps a chat loaded are the session engine's (ClosedSessions.EndNow);
// the two looked at here are the ones it cannot see, which are a message on
// the bus that would wake the chat and a delivery of such a message that is
// under way.
func (a *App) endClosedChatNow(id string) {
	s := a.session(id)
	if s == nil || !a.archivesClosed(s.Provider()) {
		return
	}
	if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(id); active {
		return
	}
	waits, err := a.messageWaitsFor(id)
	if err != nil {
		logf("chat", "warning", "chat %s keeps its session after its window was closed, because the messages waiting could not be read: %v", id, err)
		return
	}
	if waits {
		return
	}
	a.closedSessions().EndNow(id, "whose window was closed while its provider's closed chats are archived")
}

// messageWaitsFor says whether a message that would wake the chat waits for
// it on the bus (bus.Waking).
func (a *App) messageWaitsFor(id string) (bool, error) {
	waits := false
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx, cancel := context.WithTimeout(context.Background(), agentThreadPatience)
		defer cancel()
		wakes, err := b.Waking(ctx)
		if err != nil {
			return err
		}
		for _, w := range wakes {
			if a.chatOfBus(w.Session) == id {
				waits = true
			}
		}
		return nil
	})
	return waits, err
}

// stopWindowCloses keeps any further look at a closed window from beginning,
// ends the ones that are still waiting, and waits for the ones at work.
func (a *App) stopWindowCloses() {
	a.windowCloseMu.Lock()
	if a.windowCloseStop == nil {
		a.windowCloseStop = make(chan struct{})
	}
	select {
	case <-a.windowCloseStop:
	default:
		close(a.windowCloseStop)
	}
	a.windowCloseMu.Unlock()
	a.windowCloses.Wait()
}

// chatThreadOpened is the session engine's AfterOpen work for the agent's threads:
// the loaded thread is given the chat's title when it has another name, and
// the threads that waited for this agent are archived.
func (a *App) chatThreadOpened(s *Session) {
	a.nameAgentThread(s.ID())
	a.archivePendingThreads(s.Provider())
}

// forgetAgentThread drops what is recorded of a chat's thread, for a chat
// that was deleted.
func forgetAgentThread(s *Settings, id string) {
	delete(s.ArchivedThreads, id)
	delete(s.ThreadsToArchive, id)
}

// openFailureAdvice is the sentence the page adds to its note when the chat
// with this ID could not be opened, from the module of the provider that
// would have opened it, and empty for a provider that gives none.
func (a *App) openFailureAdvice(id string) string {
	if id == "" {
		return ""
	}
	if head, err := a.ResolveSessionContinuation(id); err == nil && head != "" {
		id = head
	}
	return providerModules[providerForSession(id)].OpenFailureAdvice
}

// chatOpenTarget is the chat an opening method of the page is for, from the
// arguments the page gave (chatOpenCall): the first for the two methods that
// open a chat a message waits for, and the second, after the workspace, for
// the others.
func chatOpenTarget(method string, args []json.RawMessage) string {
	at := 1
	if method == "RetryWaitingSession" || method == "OpenWaitingSession" {
		at = 0
	}
	var id string
	if at < len(args) {
		_ = json.Unmarshal(args[at], &id)
	}
	return id
}
