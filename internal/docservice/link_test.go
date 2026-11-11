package docservice

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// A window may answer a command the moment it sees it, so the place for
// the answer has to exist by the time the command can be seen.
func TestLinkKeepsAnAnswerThatComesAtOnce(t *testing.T) {
	for i := 0; i < 200; i++ {
		l := newLink()
		got := make(chan string, 1)
		go func() {
			raw, err := l.ask(context.Background(), "pick-open", nil)
			if err != nil {
				got <- "error: " + err.Error()
				return
			}
			got <- string(raw)
		}()
		commands := l.after(context.Background(), 0, 5*time.Second, 0)
		if len(commands) != 1 {
			t.Fatalf("the window saw %d commands, not one", len(commands))
		}
		l.mu.Lock()
		waiting := l.replies[commands[0].Seq] != nil
		l.mu.Unlock()
		if !waiting {
			t.Fatal("the command could be seen before there was a place for its answer")
		}
		l.reply(commands[0].Seq, json.RawMessage(`"a.md"`))
		select {
		case answer := <-got:
			if answer != `"a.md"` {
				t.Fatalf("the answer was %s", answer)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the answer was lost")
		}
	}
}
