package main

import (
	"testing"

	"studio/internal/bus"
)

// The person sets a group's turn budget from the window. The group list
// reports it and the turns used, a group starts with none, and a value the
// bus refuses reaches the page as an error.
func TestThePersonSetsAGroupsTurnBudget(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Lead"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Lead"}, {ID: "closed-chat", Cwd: t.TempDir(), Title: "Timesheet Applet"}})
	if err != nil {
		t.Fatal(err)
	}
	listed := func() BusRoom {
		t.Helper()
		o, err := a.BusOverview()
		if err != nil || len(o.Rooms) != 1 {
			t.Fatalf("the overview: %+v %v", o.Rooms, err)
		}
		if o.NoTurnBudget != bus.NoTurnBudget {
			t.Fatalf("the value for no budget is %d", o.NoTurnBudget)
		}
		return o.Rooms[0]
	}
	if r := listed(); r.Budget != bus.NoTurnBudget || r.Used != 0 {
		t.Fatalf("a new group has budget %d, used %d", r.Budget, r.Used)
	}
	if err := a.BusGroupBudget(g.Name, 12); err != nil {
		t.Fatal(err)
	}
	if r := listed(); r.Budget != 12 || r.Used != 0 {
		t.Fatalf("after setting 12: budget %d, used %d", r.Budget, r.Used)
	}
	if got, err := a.BusGroup(g.Name); err != nil || got.Budget != 12 {
		t.Fatalf("the group itself: %+v %v", got, err)
	}
	if err := a.BusGroupBudget(g.Name, -5); err == nil {
		t.Fatal("a budget below -1 was accepted")
	}
	if err := a.BusGroupBudget("  ", 3); err == nil {
		t.Fatal("no group was accepted")
	}
	if err := a.BusGroupBudget(g.Name, bus.NoTurnBudget); err != nil {
		t.Fatal(err)
	}
	if r := listed(); r.Budget != bus.NoTurnBudget {
		t.Fatalf("after turning it off: budget %d", r.Budget)
	}
}

// The person renames a group chat from the window: the title changes, and the
// group's stable name, members and budget stay; a name the bus refuses reaches the
// page as an error, and the group list shows the new title.
func TestThePersonRenamesAGroupChat(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Lead"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Lead"}, {ID: "closed-chat", Cwd: t.TempDir(), Title: "Timesheet Applet"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.BusGroupBudget(g.Name, 9); err != nil {
		t.Fatal(err)
	}
	renamed, err := a.BusRenameGroup(g.Name, "Dinah Development")
	if err != nil || renamed.Name != g.Name || renamed.Title != "Dinah Development" || len(renamed.Members) != len(g.Members) || renamed.Budget != 9 {
		t.Fatalf("renamed %+v: %v", renamed, err)
	}
	o, err := a.BusOverview()
	if err != nil || len(o.Rooms) != 1 || o.Rooms[0].Title != "Dinah Development" || o.Rooms[0].Name != g.Name {
		t.Fatalf("the overview: %+v %v", o.Rooms, err)
	}
	if _, err := a.BusRenameGroup(g.Name, "   "); err == nil {
		t.Fatal("an empty name was accepted")
	}
	if _, err := a.BusRenameGroup("  ", "X"); err == nil {
		t.Fatal("no group was named")
	}
}
