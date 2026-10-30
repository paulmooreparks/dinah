package main

import (
	"context"
	"maps"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"studio/internal/bridge"
)

// Every method the bridge offers the page has a row in pageMethods, and
// every row is a method the App still has. A method added to the App
// without a row fails here, so nothing reaches the page, or is counted as
// unfinished work, by accident (docs/plans/cleanup.md, phase 0).
func TestEveryPageMethodIsListed(t *testing.T) {
	offered := bridge.New(&App{}).Methods()
	var missing, stale []string
	seen := map[string]bool{}
	for _, name := range offered {
		seen[name] = true
		_, ok := pageMethods[name]
		if !ok {
			missing = append(missing, name)
		}
	}
	for name := range pageMethods {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("App methods the bridge offers that pageMethods does not list:\n%s", strings.Join(missing, "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("pageMethods lists methods the App no longer offers: %s", strings.Join(stale, ", "))
	}
	b, err := newPageBridge(&App{}, pageMethods)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range b.Methods() {
		if pageMethods[name] == notForPage {
			t.Errorf("exposed excluded method %s", name)
		}
	}
}

func TestExitGuardReadsClassesFromPageMethods(t *testing.T) {
	a := &App{}
	if a.observeExitCall("State") != nil {
		t.Error("a query was counted as unfinished work")
	}
	if done := a.observeExitCall("Send"); done == nil {
		t.Error("work was not counted")
	} else {
		done()
	}
	if done := a.observeExitCall("NoSuchMethod"); done == nil {
		t.Error("an unlisted method was not counted as unfinished work")
	} else {
		done()
	}
	if a.exitCalls != 0 {
		t.Errorf("exitCalls left at %d", a.exitCalls)
	}
}

// Provider messages must not be callable through the window's App.
func TestAgentHandlersAreNotAppMethods(t *testing.T) {
	appType := reflect.TypeOf(&App{})
	for _, name := range []string{"Notify", "Request"} {
		if _, ok := appType.MethodByName(name); ok {
			t.Errorf("App exposes the agent handler %s", name)
		}
	}
}

type extendedPageApp struct {
	*App
	called bool
}

func (a *extendedPageApp) AccidentalPageMethod() { a.called = true }

func TestPageBridgeRequiresExplicitPermission(t *testing.T) {
	for _, denied := range []bool{false, true} {
		a := &extendedPageApp{App: &App{}}
		table := maps.Clone(pageMethods)
		if denied {
			table["AccidentalPageMethod"] = notForPage
		}
		b, err := newPageBridge(a, table)
		if err != nil {
			t.Fatal(err)
		}
		script := httptest.NewRecorder()
		b.Handler("/bridge").ServeHTTP(script, httptest.NewRequest("GET", "/bridge/app.js", nil))
		if strings.Contains(script.Body.String(), "AccidentalPageMethod") {
			t.Fatal("unapproved method appeared in bindings")
		}
		if _, err := b.Call(context.Background(), "AccidentalPageMethod", nil); err == nil || a.called {
			t.Fatal("unapproved method was callable")
		}
	}
	a := &extendedPageApp{App: &App{}}
	b, err := newPageBridge(a, map[string]pageMethodClass{"AccidentalPageMethod": pageWork})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call(context.Background(), "AccidentalPageMethod", nil); err != nil || !a.called {
		t.Fatalf("approved method: %v", err)
	}
}

func TestPageBridgeRejectsInvalidInventory(t *testing.T) {
	for _, table := range []map[string]pageMethodClass{
		{"State": pageMethodClass(-1)},
		{"Missing": pageQuery},
		{"Missing": notForPage},
	} {
		if b, err := newPageBridge(&App{}, table); err == nil || b != nil {
			t.Fatalf("accepted invalid inventory %v", table)
		}
	}
}
