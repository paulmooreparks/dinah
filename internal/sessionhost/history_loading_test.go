package sessionhost

import "testing"

func TestInfoDistinguishesBackgroundOpenFromReadyHistory(t *testing.T) {
	c := NewConversation(Identity{ID: "background", Provider: "codex", Cwd: t.TempDir()}, InitialActivity{Open: true}, true, true)
	if !c.Info().HistoryLoading {
		t.Fatal("an admitted open must report its unfinished history")
	}
	c.EndReplay()
	if !c.Info().HistoryLoading {
		t.Fatal("provider settings are still restoring")
	}
	c.SetRestoring(false)
	if c.Info().HistoryLoading {
		t.Fatal("completed restore must release the window's checkpoint read")
	}
}
