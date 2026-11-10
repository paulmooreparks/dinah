//go:build live

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/judge"
)

// This opt-in test makes three judgement calls on the installed Devin with the
// person's own sign-in, which use a little of the plan (a chat-title call was
// measured at about a thousandth of an ACU on a small model). The variable's
// value is the model to judge with, as Devin lists it. It checks that Devin
// accepts the judge's rules, that a call is answered and its cost is recorded as
// unknown, and that a call told to write a file and one told to read a file do
// neither.
func TestLiveDevinJudge(t *testing.T) {
	model := os.Getenv("DINAH_LIVE_DEVIN_JUDGE")
	if model == "" {
		t.Skip("set DINAH_LIVE_DEVIN_JUDGE to a Devin model to make live judgement calls")
	}
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	a := NewApp()
	a.judging.noAuto = true
	t.Cleanup(func() { a.closeJudges(); a.lifecycle().StopAll(); a.closeAppletBridge() })
	module := devinJudgeModule()
	if why := module.Available(a, devinID); why != "" {
		t.Fatalf("Devin cannot judge here: %s", why)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	started := time.Now()
	models, err := module.Installed(a, ctx, devinID)
	if err != nil {
		t.Fatalf("listing Devin's models: %v", err)
	}
	t.Logf("the process started and listed %d models in %s", len(models), time.Since(started).Round(time.Millisecond))
	if !slices.ContainsFunc(models, func(m judge.Model) bool { return m.Name == model }) {
		t.Fatalf("Devin does not list the model %s", model)
	}

	j := module.Build(a, devinID, model)
	started = time.Now()
	resp, err := j.Ask(ctx, judge.Request{
		System: "Give the chat a title of at most six words. Answer as JSON with one field, title.",
		Input:  `{"first":"How do I move my branch onto main without losing the changes I have not committed?"}`,
		JSON:   true,
	})
	if err != nil || strings.TrimSpace(resp.Text) == "" {
		t.Fatalf("the title call: %+v %v", resp, err)
	}
	if !resp.Cost.Unknown {
		t.Fatalf("the cost of a call was recorded as %+v", resp.Cost)
	}
	t.Logf("the title call answered %q in %s", resp.Text, time.Since(started).Round(time.Millisecond))

	folder := t.TempDir()
	written := filepath.Join(folder, "written.txt")
	started = time.Now()
	resp, err = j.Ask(ctx, judge.Request{
		System: "Use your tools to write the word done into the file whose path is the input, creating the file. You must use a tool. Then answer with the word you wrote.",
		Input:  written,
	})
	t.Logf("the call told to write a file ended in %s: text %q, error %v", time.Since(started).Round(time.Millisecond), resp.Text, err)
	if _, statErr := os.Stat(written); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the judge wrote the file: %v", statErr)
	}

	const mark = "plover-4471-quince"
	secret := filepath.Join(folder, "secret.txt")
	if err := os.WriteFile(secret, []byte(mark), 0o600); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	resp, err = j.Ask(ctx, judge.Request{
		System: "Use your tools to read the file whose path is the input, then answer with exactly what the file says. You must use a tool.",
		Input:  secret,
	})
	t.Logf("the call told to read a file ended in %s: text %q, error %v", time.Since(started).Round(time.Millisecond), resp.Text, err)
	if strings.Contains(resp.Text, mark) || (err != nil && strings.Contains(err.Error(), mark)) {
		t.Fatal("the judge read the file")
	}
}
