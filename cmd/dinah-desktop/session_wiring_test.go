package main

import (
	"reflect"
	"testing"
	"time"
)

func TestSessionOwnerFactoriesStayLazyAndStable(t *testing.T) {
	factories := []struct {
		field string
		get   func(*App) any
	}{
		{"providerLifecycle", func(a *App) any { return a.lifecycle() }},
		{"turnService", func(a *App) any { return a.turnIO() }},
		{"closedService", func(a *App) any { return a.closedSessions() }},
		{"subagentsService", func(a *App) any { return a.subagentService() }},
		{"continuationService", func(a *App) any { return a.continuations() }},
		{"captureService", func(a *App) any { return a.transcriptCapture() }},
		{"sessionsService", func(a *App) any { return a.sessionsLifecycle() }},
		{"optionsService", func(a *App) any { return a.optionService() }},
		{"permissionService", func(a *App) any { return a.permissions() }},
		{"busChatsService", func(a *App) any { return a.busCoordinator() }},
		{"ledger", func(a *App) any { return a.steeringLedger() }},
	}
	for _, factory := range factories {
		t.Run(factory.field, func(t *testing.T) {
			a := &App{}
			done := make(chan any, 1)
			go func() { done <- factory.get(a) }()
			var first any
			select {
			case first = <-done:
			case <-time.After(testWait):
				t.Fatal("owner construction did not finish")
			}
			if first == nil || first != factory.get(a) {
				t.Fatal("owner identity changed")
			}
			value := reflect.ValueOf(a).Elem()
			for _, other := range factories {
				if other.field != factory.field && !value.FieldByName(other.field).IsNil() {
					t.Fatalf("construction eagerly initialized %s", other.field)
				}
			}
			if a.busService != nil || a.appletHost != nil || a.appletBridge != nil || a.terminals != nil {
				t.Fatal("construction started external services")
			}
		})
	}
}
