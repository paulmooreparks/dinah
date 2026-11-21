package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/codexagent"
)

// threadConnection is a provider connection with the thread capabilities of
// agent_threads.go. It keeps one list of what it was asked, the session
// requests among it, so that a test can check their order.
type threadConnection struct {
	*testProviderConnection
	mu        sync.Mutex
	asked     []string
	fail      map[string]error // by "name", "archive" or "unarchive"
	ephemeral map[string]bool
}

func (c *threadConnection) note(line string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, line)
	return c.fail[strings.SplitN(line, " ", 2)[0]]
}

func (c *threadConnection) Call(ctx context.Context, method string, params, result any) error {
	_ = c.note(method)
	return c.testProviderConnection.Call(ctx, method, params, result)
}
func (c *threadConnection) SetSessionTitle(_ context.Context, id, title string) error {
	return c.note("name " + id + " " + title)
}
func (c *threadConnection) ArchiveSession(_ context.Context, id string) error {
	return c.note("archive " + id)
}
func (c *threadConnection) UnarchiveSession(_ context.Context, id string) error {
	return c.note("unarchive " + id)
}
func (c *threadConnection) SessionEphemeral(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ephemeral[id]
}

// lines are what the connection was asked that begins with one of the words
// given, in order.
func (c *threadConnection) lines(words ...string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []string{}
	for _, line := range c.asked {
		if slices.Contains(words, strings.SplitN(line, " ", 2)[0]) {
			out = append(out, line)
		}
	}
	return out
}

func (c *threadConnection) failing(what string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail == nil {
		c.fail = map[string]error{}
	}
	if err == nil {
		delete(c.fail, what)
		return
	}
	c.fail[what] = err
}

// threadTestApp is providerTestApp with the Codex provider's connection
// replaced by one that has the thread capabilities and can close a session.
func threadTestApp(t *testing.T) (*App, *threadConnection) {
	t.Helper()
	a, base := providerTestApp(t)
	c := &threadConnection{testProviderConnection: base}
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	a.runtime.PutProviderLocked("codex", newTestProvider(c, initial))
	a.sink = func(string, any) {}
	return a, c
}

func threadRecords(a *App, id string) (archived, waiting bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.ArchivedThreads[id], a.settings.ThreadsToArchive[id]
}

func equalLines(got []string, want ...string) bool { return slices.Equal(got, want) }

// Renaming a chat gives its thread the new title, open or closed. A failure
// to name the thread is not a failure of the rename, and a chat of a
// provider without the capability asks nothing.
func TestRenamingAChatNamesItsThread(t *testing.T) {
	a, c := threadTestApp(t)
	if _, err := a.RenameSession("codex:renamed", "Parser Work"); err != nil {
		t.Fatal(err)
	}
	c.failing("name", errors.New("stand-in refusal"))
	titles, err := a.RenameSession("codex:renamed", "Parser Review")
	if err != nil || titles["codex:renamed"] != "Parser Review" {
		t.Fatalf("a thread that could not be named failed the rename: %v %v", titles, err)
	}
	if _, err := a.RenameSession("claude:other", "Another Program"); err != nil {
		t.Fatal(err)
	}
	// A cleared name goes back to the agent's title, and the thread keeps
	// the name it has.
	if _, err := a.RenameSession("codex:renamed", ""); err != nil {
		t.Fatal(err)
	}
	if got := c.lines("name"); !equalLines(got, "name codex:renamed Parser Work", "name codex:renamed Parser Review") {
		t.Fatalf("the thread was named as %v", got)
	}
}

// A title a judge wrote is given to the thread when the person has given the
// chat no name, and the person's name comes first.
func TestAJudgedTitleNamesTheThread(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.JudgedTitles = map[string]JudgedTitle{"codex:judged": {Line: "please look at the parser", Title: "Parser Review"}, "codex:both": {Title: "Judged"}}
		s.SessionTitles = map[string]string{"codex:both": "The Person's Name"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a.nameAgentThread("codex:judged")
	a.nameAgentThread("codex:both")
	a.nameAgentThread("codex:untitled")
	if got := c.lines("name"); !equalLines(got, "name codex:judged Parser Review", "name codex:both The Person's Name") {
		t.Fatalf("the threads were named as %v", got)
	}
}

// A chat that is loaded again has its saved title given to the adapter after
// the load, which sends it when the thread's name differs
// (codexagent.SetSessionTitle). A chat with no title asks nothing.
func TestAReopenedChatHasItsTitleCheckedAgainstTheThread(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.SessionTitles = map[string]string{"codex:titled": "Parser Work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	c.failing("name", errors.New("stand-in refusal"))
	if _, err := a.OpenSessionAt(ws, "codex:titled"); err != nil {
		t.Fatalf("a thread that could not be named failed the open: %v", err)
	}
	if _, err := a.OpenSessionAt(ws, "codex:untitled"); err != nil {
		t.Fatal(err)
	}
	if got := c.lines("session/load", "name"); !equalLines(got, "session/load", "name codex:titled Parser Work", "session/load") {
		t.Fatalf("the connection was asked %v", got)
	}
}

// A chat that moves to a new thread takes its title there: the new thread is
// named when the chat is continued with the provider.
func TestAChatContinuedInANewThreadNamesThatThread(t *testing.T) {
	a, c := threadTestApp(t)
	s := continuationSource(t, a)
	preview, err := a.continuations().Preview(s, "codex")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.lines("name"); !equalLines(got, "name "+result.Session.ID+" Original Chat") {
		t.Fatalf("the new thread was named as %v", got)
	}
	// The earlier stage is another provider's, which archives nothing.
	if got := c.lines("archive"); len(got) != 0 {
		t.Fatalf("a thread was archived: %v", got)
	}
}

// The earlier stage of a chat that continues in a new thread is archived
// once the continuation is committed, and a refusal to archive it does not
// fail the continuation.
func TestTheEarlierStageOfAContinuedChatIsArchived(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "archived", true: "refused"}[refused], func(t *testing.T) {
			a, c := threadTestApp(t)
			other := newTestProviderConnection()
			other.newIDs = []string{"continued-elsewhere"}
			a.runtime.PutProviderLocked("devin", newTestProvider(other, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
			t.Cleanup(other.Close)
			s := newSessionState(&sessionSeed{ID: "codex:earlier", Provider: "codex", Cwd: t.TempDir(), Title: "Original Chat"}, sessionInitial{Open: true})
			a.runtime.PutConversationLocked(s.ID(), s)
			a.transcriptCapture().Initialize(s, false)
			a.transcriptCapture().RecordMessage(s, "you", "Create a report.", false, false)
			a.transcriptCapture().RecordMessage(s, "agent", "The report is saved.", false, false)
			a.transcriptCapture().FinishTurn(s, nil, "end_turn")
			if refused {
				c.failing("archive", errors.New("stand-in refusal"))
			}
			preview, err := a.continuations().Preview(s, "devin")
			if err != nil {
				t.Fatal(err)
			}
			result, err := a.ContinueSessionWithProvider(s.ID(), "devin", preview.Revision)
			if err != nil {
				t.Fatalf("the continuation failed: %v", err)
			}
			if result.Session.ID != "continued-elsewhere" {
				t.Fatalf("the chat continued as %q", result.Session.ID)
			}
			if got := c.lines("archive"); !equalLines(got, "archive codex:earlier") {
				t.Fatalf("the earlier stage was archived as %v", got)
			}
			if archived, waiting := threadRecords(a, "codex:earlier"); archived == refused || waiting {
				t.Fatalf("the record says archived %v, waiting %v", archived, waiting)
			}
		})
	}
}

// A chat that goes to the Recycle Bin has its thread archived, and restoring
// it takes the thread out of the archive. Neither a refused archive nor a
// refused unarchive stops the person's action.
func TestTheRecycleBinArchivesAndRestoresTheThread(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "done", true: "refused"}[refused], func(t *testing.T) {
			a, c := threadTestApp(t)
			ws, id := t.TempDir(), "codex:binned"
			cleanupFixtureChat(t, a, id, "codex", ws, "window-binned")
			if refused {
				c.failing("archive", errors.New("stand-in refusal"))
			}
			if err := a.RemoveChat(id); err != nil {
				t.Fatalf("the chat did not go to the Recycle Bin: %v", err)
			}
			if got := c.lines("archive", "unarchive"); !equalLines(got, "archive "+id) {
				t.Fatalf("the connection was asked %v", got)
			}
			if archived, _ := threadRecords(a, id); archived == refused {
				t.Fatalf("the record says archived %v", archived)
			}
			if refused {
				// The second half: an archived thread whose unarchive is refused.
				c.failing("archive", nil)
				a.recordThreadArchive(id, true, false)
				c.failing("unarchive", errors.New("stand-in refusal"))
			}
			if result := a.RestoreRemovedChat(id); !result.OK {
				t.Fatalf("the chat was not restored: %+v", result)
			}
			if got := c.lines("unarchive"); !equalLines(got, "unarchive "+id) {
				t.Fatalf("the connection was asked %v", got)
			}
			// A thread that stayed archived is taken out before the chat is loaded.
			if archived, _ := threadRecords(a, id); archived != refused {
				t.Fatalf("after the restore the record says archived %v", archived)
			}
			if refused {
				c.failing("unarchive", nil)
				if _, err := a.OpenSessionAt(ws, id); err != nil {
					t.Fatal(err)
				}
				if got := c.lines("unarchive", "session/load"); !equalLines(got, "unarchive "+id, "unarchive "+id, "session/load") {
					t.Fatalf("the connection was asked %v", got)
				}
				if archived, _ := threadRecords(a, id); archived {
					t.Fatal("the thread is still recorded as archived")
				}
			}
		})
	}
}

// A thread that could not be taken out of the archive does not keep its chat
// closed: the load is asked for all the same, and the record stays for the
// next load.
func TestAChatLoadsWhenItsThreadCouldNotBeUnarchived(t *testing.T) {
	a, c := threadTestApp(t)
	a.recordThreadArchive("codex:kept", true, false)
	c.failing("unarchive", errors.New("stand-in refusal"))
	if _, err := a.OpenSessionAt(t.TempDir(), "codex:kept"); err != nil {
		t.Fatalf("the chat did not open: %v", err)
	}
	if got := c.lines("unarchive", "session/load"); !equalLines(got, "unarchive codex:kept", "session/load") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, _ := threadRecords(a, "codex:kept"); !archived {
		t.Fatal("the record of the archived thread was dropped")
	}
}

// With the agent not running, the chat waits to be archived, and it is
// archived when a chat of that agent next opens. Restoring the chat first
// ends the wait.
func TestAThreadWaitsToBeArchivedUntilItsAgentRuns(t *testing.T) {
	a, c := threadTestApp(t)
	ws := t.TempDir()
	for _, id := range []string{"codex:waits", "codex:restored"} {
		cleanupFixtureChat(t, a, id, "codex", ws, "window-"+strings.TrimPrefix(id, "codex:"))
	}
	running := a.runtime.ProviderLocked("codex")
	a.mu.Lock()
	a.runtime.DetachProviderLocked("codex", running)
	a.mu.Unlock()
	for _, id := range []string{"codex:waits", "codex:restored"} {
		if err := a.RemoveChat(id); err != nil {
			t.Fatal(err)
		}
		if archived, waiting := threadRecords(a, id); archived || !waiting {
			t.Fatalf("%s: the record says archived %v, waiting %v", id, archived, waiting)
		}
	}
	if result := a.RestoreRemovedChat("codex:restored"); !result.OK {
		t.Fatalf("the chat was not restored: %+v", result)
	}
	if _, waiting := threadRecords(a, "codex:restored"); waiting {
		t.Fatal("a restored chat still waits to be archived")
	}
	a.mu.Lock()
	a.runtime.PutProviderLocked("codex", running)
	a.mu.Unlock()
	if _, err := a.OpenSessionAt(ws, "codex:another"); err != nil {
		t.Fatal(err)
	}
	if got := c.lines("archive", "unarchive"); !equalLines(got, "archive codex:waits") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, waiting := threadRecords(a, "codex:waits"); !archived || waiting {
		t.Fatalf("the record says archived %v, waiting %v", archived, waiting)
	}
}

// endClosedChat closes a chat with no window for long enough that its
// session is ended, and waits for that end.
func endClosedChat(t *testing.T, a *App, id string) {
	t.Helper()
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	a.wakeNow = func() time.Time { return now }
	s := newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: t.TempDir()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.ChatWindows(nil)
	a.closedSessions().EndClosed()
	now = now.Add(closedSessionEnd)
	a.closedSessions().EndClosed()
	a.closedSessions().Wait()
	if a.session(id) != nil {
		t.Fatalf("the session of chat %s was not ended", id)
	}
}

// The thread of a chat that was closed and had its session ended is left as
// it is unless the person turned the setting on.
func TestAClosedChatIsNotArchivedWithTheSettingOff(t *testing.T) {
	a, c := threadTestApp(t)
	if a.State().CodexArchiveClosed {
		t.Fatal("the setting is on in a new installation")
	}
	endClosedChat(t, a, "codex:closed")
	if got := c.lines("session/close", "archive"); !equalLines(got, "session/close") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, waiting := threadRecords(a, "codex:closed"); archived || waiting {
		t.Fatalf("the record says archived %v, waiting %v", archived, waiting)
	}
	if _, err := a.OpenSessionAt(t.TempDir(), "codex:closed"); err != nil {
		t.Fatal(err)
	}
	if got := c.lines("unarchive"); len(got) != 0 {
		t.Fatalf("a thread that was not archived was unarchived: %v", got)
	}
}

// With the setting on, the thread is archived after the session is closed,
// and it is taken out of the archive before the chat is loaded again, as it
// is for a chat that a waiting message opens.
func TestAClosedChatIsArchivedAndUnarchivedWithTheSettingOn(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	if !a.State().CodexArchiveClosed {
		t.Fatal("the setting was not kept")
	}
	if class, listed := pageMethods["SetCodexArchiveClosed"]; !listed || class != pageWork {
		t.Fatal("the page cannot call SetCodexArchiveClosed")
	}
	endClosedChat(t, a, "codex:closed")
	if got := c.lines("session/close", "archive"); !equalLines(got, "session/close", "archive codex:closed") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, _ := threadRecords(a, "codex:closed"); !archived {
		t.Fatal("the archived thread was not recorded")
	}
	if _, err := a.OpenSessionAt(t.TempDir(), "codex:closed"); err != nil {
		t.Fatalf("the archived chat did not open: %v", err)
	}
	if got := c.lines("archive", "unarchive", "session/load"); !equalLines(got, "archive codex:closed", "unarchive codex:closed", "session/load") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, _ := threadRecords(a, "codex:closed"); archived {
		t.Fatal("the thread is still recorded as archived")
	}
	// Turning the setting off leaves a later closed chat alone.
	if err := a.SetCodexArchiveClosed(false); err != nil {
		t.Fatal(err)
	}
	endClosedChat(t, a, "codex:later")
	if got := c.lines("archive"); !equalLines(got, "archive codex:closed") {
		t.Fatalf("the connection was asked %v", got)
	}
}

// An earlier stage whose archive did not go through when its chat continued
// is archived when its session is ended, whatever the setting.
func TestAnEarlierStageIsArchivedWhenItsSessionEnds(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.ChatBindings = map[string]string{"codex:stage": "codex:head"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	endClosedChat(t, a, "codex:stage")
	if got := c.lines("archive"); !equalLines(got, "archive codex:stage") {
		t.Fatalf("the connection was asked %v", got)
	}
}

// A judgement call's session that the agent keeps in memory only is closed,
// since it has nothing stored to delete. Any other is deleted where the
// agent can delete one, as before.
func TestAJudgeSessionKeptInMemoryIsClosed(t *testing.T) {
	_, c := threadTestApp(t)
	c.ephemeral = map[string]bool{"codex:bare": true}
	var caps acp.InitializeResult
	caps.AgentCapabilities.SessionCapabilities.Delete = &struct{}{}
	caps.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	endJudgeSession(c, caps, "codex:bare")
	endJudgeSession(c, caps, "codex:stored")
	if got := c.lines("session/close", "session/delete"); !equalLines(got, "session/close", "session/delete") {
		t.Fatalf("the sessions were ended with %v", got)
	}
	calls := c.calls.Snapshot()
	if closed, ok := calls[0].params.(acp.CloseSessionParams); !ok || closed.SessionID != "codex:bare" {
		t.Fatalf("the session kept in memory was ended with %+v", calls[0])
	}
	if deleted, ok := calls[1].params.(acp.DeleteSessionParams); !ok || deleted.SessionID != "codex:stored" {
		t.Fatalf("the stored session was ended with %+v", calls[1])
	}
}

// Every failed open of a Codex chat that the page started comes with the
// Codex module's advice, whatever the failure was, and a chat of a provider
// whose module gives none comes with none.
func TestAFailedOpenComesWithTheProvidersAdvice(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	release := heldLoads(t, c)
	a.sessionsLifecycle().LoadLimit = 40 * time.Millisecond
	if _, err := a.BeginChatOpen("page-1", "OpenSessionAt", rawArgs(t, t.TempDir(), "codex:held")); err != nil {
		t.Fatal(err)
	}
	if o := events.outcome(t); o.Error == "" || o.Advice != codexagent.OpenFailureAdvice {
		t.Fatalf("the failed open came with the advice %q: %+v", o.Advice, o)
	}
	release()
	until(t, "the late answer has been taken", func() bool { return !a.sessionsLifecycle().LoadOutstanding("codex:held") })
	// A relative workspace fails before the agent is asked anything.
	if _, err := a.BeginChatOpen("page-2", "OpenSessionAt", rawArgs(t, "relative", "codex:other")); err != nil {
		t.Fatal(err)
	}
	if o := events.outcome(t); o.Error == "" || o.Advice != codexagent.OpenFailureAdvice {
		t.Fatalf("the failed open came with the advice %q: %+v", o.Advice, o)
	}
	if _, err := a.BeginChatOpen("page-3", "OpenSessionAt", rawArgs(t, "relative", "claude:other")); err != nil {
		t.Fatal(err)
	}
	if o := events.outcome(t); o.Error == "" || o.Advice != "" {
		t.Fatalf("a chat of another provider came with the advice %q: %+v", o.Advice, o)
	}
	if got := chatOpenTarget("RetryWaitingSession", rawArgs(t, "codex:waiting")); got != "codex:waiting" {
		t.Fatalf("the chat of a retried open was read as %q", got)
	}
	if strings.ContainsRune(codexagent.OpenFailureAdvice, '\n') || strings.Contains(codexagent.OpenFailureAdvice, ". ") || !strings.HasSuffix(codexagent.OpenFailureAdvice, "may free it.") {
		t.Fatalf("the advice is not the one plain sentence agreed: %q", codexagent.OpenFailureAdvice)
	}
}

// A request to archive that was not answered in time may still be carried
// out, so its thread is recorded as archived and is taken out of the archive
// before the chat is loaded. A request the agent refused leaves no record.
func TestAnUnansweredArchiveIsRecordedAsArchived(t *testing.T) {
	a, c := threadTestApp(t)
	c.failing("archive", context.DeadlineExceeded)
	a.archiveAgentThread("codex", "codex:slow", "is in the Recycle Bin")
	c.failing("archive", errors.New("stand-in refusal"))
	a.archiveAgentThread("codex", "codex:refused", "is in the Recycle Bin")
	if archived, _ := threadRecords(a, "codex:slow"); !archived {
		t.Fatal("an unanswered archive left no record")
	}
	if archived, waiting := threadRecords(a, "codex:refused"); archived || waiting {
		t.Fatal("a refused archive left a record")
	}
	if _, err := a.OpenSessionAt(t.TempDir(), "codex:slow"); err != nil {
		t.Fatal(err)
	}
	if got := c.lines("unarchive", "session/load"); !equalLines(got, "unarchive codex:slow", "session/load") {
		t.Fatalf("the connection was asked %v", got)
	}
}
