package main

import (
	"slices"
	"strings"
	"testing"
)

func assistantDesk() []AssistantWindow {
	return []AssistantWindow{
		{ID: "w1", Kind: "chat", Title: "Pricing Rules for Bundles", State: "working"},
		{ID: "w2", Kind: "document", Title: "Pricing Design.md"},
		{ID: "w3", Kind: "chat", Title: "Release Notes Draft", State: "idle"},
		{ID: "w4", Kind: "diagram", Title: "Refund Flow.d2", Away: true},
		{ID: "w5", Kind: "document", Title: "Meeting Notes.md", State: "unsaved"},
		{ID: "w6", Kind: "chat", Title: "Tax Rule Question", State: "waiting"},
	}
}

// With no judge the rule reads every task of the starter set as what it is
// called, and a subject as something to find, which it never refuses.
func TestTheAssistantsRuleReadsTheStarterTasksAndFindsBySubject(t *testing.T) {
	desk := assistantDesk()
	want := map[string]string{
		"Tile the Windows That Are Showing":   assistantTile,
		"Show Only Windows with Work in Them": assistantWorking,
		"Tidy Up":                             assistantTidy,
		"Documents Side by Side, Chat Under":  assistantSide,
		"Bring Everything Back":               assistantAll,
		"What Needs Me?":                      assistantSay,
	}
	for _, task := range assistantStarterTasks() {
		got := assistantRule(assistantIn{Ask: task.Ask, Windows: desk})
		if got.Do != want[task.Name] || got.Say == "" {
			t.Errorf("%s: the rule answered %+v, want %s", task.Name, got, want[task.Name])
		}
	}
	side := assistantRule(assistantIn{Ask: "put the documents side by side and the chat under them", Windows: desk})
	if !slices.Equal(side.Windows, []string{"w2", "w5"}) || side.Under != "w1" {
		t.Errorf("side by side has %v over %q", side.Windows, side.Under)
	}
	needs := assistantRule(assistantIn{Ask: "which windows are waiting for me?", Windows: desk})
	for _, title := range []string{"Release Notes Draft", "Meeting Notes.md", "Tax Rule Question"} {
		if !strings.Contains(needs.Say, title) {
			t.Errorf("what needs the person leaves out %s: %s", title, needs.Say)
		}
	}
	found := assistantRule(assistantIn{Ask: "Find all the windows for the pricing work", Windows: desk})
	if found.Do != assistantGather || !slices.Equal(found.Windows, []string{"w1", "w2"}) || !strings.Contains(found.Say, "not closed") {
		t.Errorf("finding by subject answered %+v", found)
	}
	none := assistantRule(assistantIn{Ask: "Find all the windows for the payroll migration", Windows: desk})
	if none.Do != assistantSay || len(none.Windows) != 0 || !strings.Contains(none.Say, "payroll migration") {
		t.Errorf("a subject no window has answered %+v", none)
	}
	// A subject with the word "finished" in it is found, and closes nothing.
	if got := assistantRule(assistantIn{Ask: "find the windows about finished goods", Windows: desk}); got.Do == assistantTidy {
		t.Errorf("a subject was read as tidying: %+v", got)
	}
}

// A request is answered with judgements off, and with a judge whose answer
// names a window that is not open the rule's answer stands.
func TestTheAssistantAnswersWithoutAJudgeAndRefusesAWindowThatIsNotOpen(t *testing.T) {
	a, _ := calibrationApp(t)
	got, err := a.AskAssistant("find the windows for the pricing work", assistantDesk())
	if err != nil || got.Judged || got.Do != assistantGather || len(got.Windows) != 2 {
		t.Fatalf("with no judge: %+v, %v", got, err)
	}
	if _, err := a.AskAssistant("  ", nil); err == nil {
		t.Fatal("an empty request was taken")
	}
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	set, err := a.judges()
	if err != nil {
		t.Fatal(err)
	}
	set.SetJudge(&answering{`{"do":"gather","windows":["w1","w4"],"say":"Gathered the pricing work with its diagram."}`}, "test", "tokens")
	if got, err = a.AskAssistant("find the windows for the pricing work", assistantDesk()); err != nil || !got.Judged || !slices.Equal(got.Windows, []string{"w1", "w4"}) {
		t.Fatalf("with a judge: %+v, %v", got, err)
	}
	set.SetJudge(&answering{`{"do":"gather","windows":["w99"],"say":"Gathered."}`}, "test", "tokens")
	if got, err = a.AskAssistant("find the windows for the pricing work", assistantDesk()); err != nil || got.Judged || !slices.Equal(got.Windows, []string{"w1", "w2"}) {
		t.Fatalf("with a judge that invents a window: %+v, %v", got, err)
	}
}

// The list begins as the starter set. The person's own list is kept in its
// order, an emptied list stays empty, and the starter set can be had back.
func TestTheAssistantsTasksAreThePersonsToChangeAndRestore(t *testing.T) {
	a, _ := calibrationApp(t)
	if got := a.AssistantTasks(); len(got) != 6 || got[0].Name != "Tile the Windows That Are Showing" {
		t.Fatalf("the list begins as %+v", got)
	}
	mine := []AssistantTask{{Name: " Pricing ", Ask: "find all the windows for the pricing work"}, {Name: "Tidy Up", Ask: "close what is finished"}}
	got, err := a.SetAssistantTasks(mine)
	if err != nil || len(got) != 2 || got[0].Name != "Pricing" {
		t.Fatalf("the saved list is %+v, %v", got, err)
	}
	if _, err := a.SetAssistantTasks([]AssistantTask{{Name: "No Words"}}); err == nil {
		t.Fatal("a task that asks nothing was kept")
	}
	if got, err = a.SetAssistantTasks(nil); err != nil || len(got) != 0 {
		t.Fatalf("an emptied list is %+v, %v", got, err)
	}
	if got, err = a.RestoreAssistantTasks(); err != nil || len(got) != 6 {
		t.Fatalf("the restored list is %+v, %v", got, err)
	}
}
