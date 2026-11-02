package fileaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const MaxOperationPaths = 20000
const MaxSummaryDeniedPaths = 24
const MaxSummaryDeniedPathBytes = 4096

type operationKey struct{}
type operation struct {
	mu         sync.Mutex
	store      *Store
	chat       string
	workspace  string
	paths      map[string]bool
	denied     map[string]bool
	errors     map[string]bool
	fatal      error
	rules      map[string]bool
	unverified map[string]bool
}

func operationIn(ctx context.Context) *operation {
	if ctx == nil {
		return nil
	}
	op, _ := ctx.Value(operationKey{}).(*operation)
	return op
}
func InOperation(ctx context.Context) bool { return operationIn(ctx) != nil }

func (s *Service) admitOperationPath(ctx context.Context, scope Scope, path string) error {
	op := operationIn(ctx)
	if op == nil {
		return nil
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	if op.store != s.Store || op.chat != scope.Chat || op.workspace != scope.Workspace {
		return errors.New("a file operation cannot change its caller or store")
	}
	if resolved, err := Resolve(path); err == nil {
		path = resolved
	}
	if !op.paths[path] && len(op.paths) >= MaxOperationPaths {
		op.fatal = errors.New("the file operation exceeds 20000 paths; choose a smaller folder")
		return op.fatal
	}
	op.paths[path] = true
	return nil
}

func (s *Service) recordRead(ctx context.Context, scope Scope, record Record) error {
	op := operationIn(ctx)
	if op == nil {
		ids := map[string]bool{}
		for _, id := range record.UnverifiedRuleIDs {
			ids[id] = true
		}
		record.UnverifiedRuleIDs = nil
		for id := range ids {
			record.UnverifiedRuleIDs = append(record.UnverifiedRuleIDs, id)
		}
		sort.Strings(record.UnverifiedRuleIDs)
		_, err := s.Store.Append(record)
		return err
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	if op.store != s.Store || op.chat != scope.Chat || op.workspace != scope.Workspace {
		return errors.New("a file operation cannot change its caller or store")
	}
	for _, id := range record.UnverifiedRuleIDs {
		op.unverified[id] = true
	}
	switch record.Outcome {
	case "denied":
		op.denied[record.Path] = true
		if record.RuleID != "" {
			op.rules[record.RuleID] = true
		}
	case "error":
		op.errors[record.Path] = true
	}
	return nil
}

// RunOperation commits one bounded summary, including every visited and denied
// candidate, before returning any tool result. Candidate checks never write.
func (s *Service) RunOperation(ctx context.Context, path string, scope Scope, work func(context.Context) (string, bool)) (string, bool) {
	if InOperation(ctx) {
		return "nested file operations are refused", false
	}
	if scope.Chat == "" || scope.Workspace == "" {
		return "a registered file-operation caller is required", false
	}
	op := &operation{store: s.Store, chat: scope.Chat, workspace: scope.Workspace, paths: map[string]bool{}, denied: map[string]bool{}, errors: map[string]bool{}, rules: map[string]bool{}, unverified: map[string]bool{}}
	text, ok := work(context.WithValue(ctx, operationKey{}, op))
	op.mu.Lock()
	visited, denied, failed, fatal := len(op.paths), len(op.denied), len(op.errors), op.fatal
	ruleIDs := []string{}
	for id := range op.rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	unverified := []string{}
	for id := range op.unverified {
		unverified = append(unverified, id)
	}
	sort.Strings(unverified)
	paths := []string{}
	for path := range op.denied {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	retained := []string{}
	used := 0
	for _, path := range paths {
		encoded, _ := json.Marshal(path)
		if len(retained) >= MaxSummaryDeniedPaths || used+len(encoded) > MaxSummaryDeniedPathBytes {
			continue
		}
		retained = append(retained, path)
		used += len(encoded)
	}
	ruleID := ""
	if len(op.rules) == 1 {
		for id := range op.rules {
			ruleID = id
		}
	}
	op.mu.Unlock()
	if fatal != nil {
		text, ok = fatal.Error(), false
	}
	if ctx.Err() != nil {
		text, ok = "the file operation was cancelled", false
	}
	resolved, err := Resolve(path)
	if err != nil {
		resolved = path
	}
	if ok && denied > 0 && (scope.Source == "find_files" || scope.Source == "search_text" || scope.Source == "list_directory") {
		text = strings.ReplaceAll(text, "No file matches.", "No file matches among permitted entries.")
		text = strings.ReplaceAll(text, "No line matches.", "No line matches among permitted entries.")
		text += fmt.Sprintf("\n[%d paths visited; %d denied by file policy and omitted.]", visited, denied)
	}
	record := Record{Operation: true, RuleID: ruleID, RuleIDs: ruleIDs, DeniedPaths: retained, DeniedPathsOmitted: denied - len(retained), UnverifiedRuleIDs: unverified, Chat: scope.Chat, ChatTitle: scope.ChatTitle, Workspace: scope.Workspace, Source: scope.Source, Path: resolved, Visited: visited, Denied: denied, Errors: failed, Outcome: "allowed", Reason: "File-operation summary"}
	if ok {
		record.Bytes = len(text)
	} else {
		record.Outcome = "error"
		record.Errors++
		record.Reason = "The file operation failed or was cancelled"
	}
	if denied > 0 {
		record.Outcome = "denied"
		record.Count = denied
		zero := 0
		record.DeniedBytes = &zero
	}
	if _, err := s.Store.Append(record); err != nil {
		return "could not commit file-operation audit; no result returned", false
	}
	return text, ok
}
