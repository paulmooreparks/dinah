package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type counter struct{ n int }

type pair struct {
	A string `json:"a"`
	B int    `json:"b"`
}

func (c *counter) Add(n int) int         { c.n += n; return c.n }
func (c *counter) Fail(why string) error { return errors.New(why) }
func (c *counter) Pair(ctx context.Context, p pair, times int) (pair, error) {
	if ctx == nil {
		return pair{}, errors.New("no context")
	}
	return pair{A: strings.Repeat(p.A, times), B: p.B * times}, nil
}
func (c *counter) Nothing() {}
func (c *counter) hidden()  {}

func TestObjectCallsExportedMethodsWithJSONArguments(t *testing.T) {
	s := Object(&counter{})
	ctx := context.Background()
	got, err := s.Call(ctx, "Add", json.RawMessage(`[3]`))
	if err != nil || got != 3 {
		t.Fatalf("Add: %v %v", got, err)
	}
	got, err = s.Call(ctx, "Add", json.RawMessage(`[4]`))
	if err != nil || got != 7 {
		t.Fatalf("Add again: %v %v", got, err)
	}
	if _, err := s.Call(ctx, "Fail", json.RawMessage(`["no"]`)); err == nil || err.Error() != "no" {
		t.Fatalf("Fail: %v", err)
	}
	got, err = s.Call(ctx, "Pair", json.RawMessage(`[{"a":"x","b":2},3]`))
	if err != nil || got.(pair) != (pair{A: "xxx", B: 6}) {
		t.Fatalf("Pair: %+v %v", got, err)
	}
	// A missing argument is zero; a surplus one is refused.
	got, err = s.Call(ctx, "Pair", json.RawMessage(`[]`))
	if err != nil || got.(pair) != (pair{}) {
		t.Fatalf("Pair with nothing: %+v %v", got, err)
	}
	if _, err := s.Call(ctx, "Add", json.RawMessage(`[1,2]`)); err == nil {
		t.Fatal("a surplus argument was accepted")
	}
	if _, err := s.Call(ctx, "Add", json.RawMessage(`["x"]`)); err == nil {
		t.Fatal("a wrong argument was accepted")
	}
	if got, err := s.Call(ctx, "Nothing", nil); err != nil || got != nil {
		t.Fatalf("Nothing: %v %v", got, err)
	}
	if _, err := s.Call(ctx, "hidden", nil); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("an unexported method was offered: %v", err)
	}
	if _, err := s.Call(ctx, "Missing", nil); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("a missing method: %v", err)
	}
}

func TestRegistryRefusesBadNamesAndDuplicates(t *testing.T) {
	Register("test-counter", Object(&counter{}))
	if _, ok := Lookup("test-counter"); !ok {
		t.Fatal("the service was not registered")
	}
	for _, bad := range []string{"Bad", "has space", "", "-x"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%q was accepted", bad)
				}
			}()
			Register(bad, Object(&counter{}))
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a duplicate was accepted")
			}
		}()
		Register("test-counter", Object(&counter{}))
	}()
	found := false
	for _, name := range Names() {
		found = found || name == "test-counter"
	}
	if !found {
		t.Fatal("Names leaves the service out")
	}
}
