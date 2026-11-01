package sessionhost

import "testing"

func TestWorkActivityProjectsHostWorkAndCounts(t *testing.T) {
	for _, s := range []State{
		{Info: Info{Open: true}, Switching: true},
		{Info: Info{Open: true, Queued: []string{"queued"}}},
		{Info: Info{Open: true}, HasSteer: true},
	} {
		activity := s.WorkActivity()
		if !activity.Busy() || activity.Foreground == "" {
			t.Fatal("host-owned work looked idle", activity)
		}
	}
	s := State{Info: Info{Open: true, Background: []BackgroundTask{{ID: "task", State: "paused"}}}, RunningSubagents: 1}
	activity := s.WorkActivity()
	if !activity.Busy() || activity.Tasks != 1 || activity.Subagents != 1 || activity.Detail != "1 background task, 1 subagent" {
		t.Fatal(activity)
	}
	counts := CountStates([]ChatState{{Life: LifeOpen, Exec: &ChatExec{State: ExecIdle}}, {Life: LifeOpen, Exec: &ChatExec{State: ExecBackground}}})
	if counts.Background != 1 || counts.Idle != 1 || counts.Top != ExecBackground {
		t.Fatal(counts)
	}
	counts.Merge(StateCounts{Working: 1})
	counts.Finish()
	if counts.Top != ExecWorking || counts.Background != 1 {
		t.Fatal(counts)
	}
}
