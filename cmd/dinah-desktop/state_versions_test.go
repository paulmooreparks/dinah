package main

import (
	"encoding/json"
	"testing"
)

// The window's own state events carry a generation and a version, taken under
// the lock that guards the state, and the calls the page rereads them with
// return numbers in the same sequence, later than any event before them.
func TestTheWindowsStateEventsAreNumbered(t *testing.T) {
	a, _ := providerTestApp(t)
	events := map[string][]map[string]any{}
	a.sink = func(name string, data any) {
		raw, _ := json.Marshal(data)
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			events[name] = append(events[name], m)
		}
	}
	number := func(m map[string]any) (string, float64) {
		gen, _ := m["stateGen"].(string)
		ver, _ := m["stateVer"].(float64)
		return gen, ver
	}

	a.emitSettingsChanged()
	a.emitSettingsChanged()
	settings := events["settings:changed"]
	g1, v1 := number(settings[0])
	g2, v2 := number(settings[1])
	if len(settings) != 2 || g1 == "" || g1 != g2 || v2 <= v1 || settings[0]["revision"] == nil || settings[0]["preferences"] == nil {
		t.Fatalf("settings events: %v", settings)
	}

	a.claudeAccountReport(json.RawMessage(`{"authStatus":{"kind":"account","label":"Claude Max"}}`))
	account := events["agent:account"]
	ga, va := number(account[0])
	if len(account) != 1 || ga != g1 || va <= v2 {
		t.Fatalf("account event %v, after settings %s/%v", account, g1, v2)
	}
	reread := a.ProviderAccounts()
	if len(reread) != 1 || reread[0].StateGen != ga || float64(reread[0].StateVer) <= va {
		t.Fatalf("a reread of the accounts %+v is not newer than the event %v", reread, account[0])
	}

	a.emitUpdate()
	a.emitUpdate()
	update := events["update:state"]
	gu1, vu1 := number(update[0])
	gu2, vu2 := number(update[1])
	if len(update) != 2 || gu1 == "" || gu1 != gu2 || vu2 <= vu1 {
		t.Fatalf("update events: %v", update)
	}
	if st := a.UpdateState(); st.StateGen != gu1 || float64(st.StateVer) <= vu2 {
		t.Fatalf("a reread of the update state %+v is not newer than the events", st)
	}
	// The update watcher's numbers are its own: another owner, another generation.
	if gu1 == g1 {
		t.Fatal("two owners share a generation")
	}
}
