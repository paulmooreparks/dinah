package fileaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"studio/internal/agents"
	"studio/internal/layout"
)

type Scope struct {
	SharedLinkRoots []string
	AuditAlways     bool
	Source          string
	LimitRoots      []string
	Limited         bool
	Chat            string
	ChatTitle       string
	Workspace       string
	Roots           []string
	OtherRoots      []string
	ManagedRoots    []string
	Layout          layout.Layout
}

type Check struct {
	UnverifiedRuleIDs []string `json:"unverifiedRuleIDs,omitempty"`
	Valid             bool     `json:"valid"`
	Error             string   `json:"error"`
	ResolvedPath      string   `json:"resolvedPath"`
	MatchedRuleIDs    []string `json:"matchedRuleIDs"`
	InsideWorkspace   bool     `json:"insideWorkspace"`
}

// Service snapshots policy before each read and commits required audit before returning content.
// Scope is supplied by the registered host session, never by the provider.
type Service struct {
	network  *networkProber
	mu       sync.Mutex
	Store    *Store
	Defaults []Rule
}

func Resolve(path string) (string, error) {
	if !filepath.IsAbs(path) || len(path) > 4096 || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("use an absolute path of at most 4096 characters without control characters")
	}
	return agents.ResolveFilePath(path)
}

func inside(roots []string, p string) bool {
	for _, root := range roots {
		resolved, err := Resolve(root)
		if err == nil && layout.Inside(resolved, p) {
			return true
		}
	}
	return false
}

func (s *Service) policy(scope Scope) (Policy, error) {
	policy, err := s.rawPolicy(scope)
	if err == nil {
		s.describeNetwork(&policy)
	}
	return policy, err
}
func (s *Service) rawPolicy(scope Scope) (Policy, error) {
	version, custom, switches, err := s.Store.Snapshot()
	if err != nil {
		return Policy{}, err
	}
	out := Policy{Version: version, Rules: []Rule{}}
	for i, root := range agents.ProtectedRoots(scope.Layout) {
		id := "protected-state"
		if i > 0 {
			id = fmt.Sprintf("protected-approvals-%d", i)
		}
		out.Rules = append(out.Rules, Rule{ID: id, Kind: "protected", Pattern: root.Path, Reason: root.Reason, Enabled: true, ResolvedPaths: []string{root.Path}, Examples: map[string]string{}, Actions: []string{}})
	}
	for _, r := range s.Defaults {
		r.ResolvedPaths = append([]string{}, r.ResolvedPaths...)
		if enabled, ok := switches[r.ID]; ok {
			r.Enabled = enabled
		}
		if r.ID == "other-workspaces" {
			r.ResolvedPaths = append(r.ResolvedPaths, scope.OtherRoots...)
		}
		if r.Enabled {
			r.Actions = []string{"disable"}
		} else {
			r.Actions = []string{"enable"}
		}
		out.Rules = append(out.Rules, r)
	}
	ids := []string{}
	for id := range custom {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := custom[id]
		out.Rules = append(out.Rules, Rule{ID: id, Kind: "custom", Pattern: p, Reason: "A path you chose to deny", Enabled: true, ResolvedPaths: []string{p}, Examples: map[string]string{}, Actions: []string{"remove"}})
	}
	for i := range out.Rules {
		r := &out.Rules[i]
		r.paths = append([]string{}, r.ResolvedPaths...)
		for j, path := range r.ResolvedPaths {
			if resolved, err := Resolve(path); err == nil {
				r.ResolvedPaths[j] = resolved
			}
		}
	}
	return out, nil
}

func (s *Service) Policy(scope Scope) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy(scope)
}

func evaluate(path string, scope Scope, policy Policy) Check {
	chain, _ := agents.PathIdentities(path)
	return evaluateIdentities(path, scope, policy, chain)
}

func evaluateIdentities(path string, scope Scope, policy Policy, chain []os.FileInfo) Check {
	return evaluateNetwork(path, scope, policy, chain, processNetworkProber)
}
func evaluateNetwork(path string, scope Scope, policy Policy, chain []os.FileInfo, network *networkProber) Check {
	networkContext, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
	defer cancel()
	out := Check{MatchedRuleIDs: []string{}}
	resolved, err := Resolve(path)
	if err != nil {
		out.Error = err.Error()
		if _, boundary := agents.NormalizeFilePath(path); boundary != nil {
			out.MatchedRuleIDs = append(out.MatchedRuleIDs, "path-boundary")
		}
		return out
	}
	out.Valid, out.ResolvedPath, out.InsideWorkspace = true, resolved, inside(scope.Roots, resolved)
	for _, p := range []string{path, resolved} {
		if reason := agents.ProtectedPath(scope.Layout, p, scope.ManagedRoots...); reason != "" {
			id := "protected-state"
			for _, rule := range policy.Rules {
				if rule.Kind == "protected" && inside(rule.paths, p) {
					id = rule.ID
					break
				}
			}
			out.MatchedRuleIDs = append(out.MatchedRuleIDs, id)
			out.Error = reason
			return out
		}
	}
	// A protected root may itself be configured through a symbolic link.
	// Compare its canonical identity as well as the requested path spelling.
	for _, rule := range policy.Rules {
		if rule.Kind != "protected" {
			continue
		}
		matched := inside(rule.paths, resolved)
		for _, root := range rule.paths {
			same, err := agents.IdentityContains(root, chain)
			if err != nil {
				out.Valid = false
				out.Error = "A protected-rule identity could not be checked"
				return out
			}
			matched = matched || same
		}
		if !matched {
			continue
		}
		if rule.ID == "protected-state" && inside(scope.ManagedRoots, resolved) {
			continue
		}
		out.MatchedRuleIDs = append(out.MatchedRuleIDs, rule.ID)
		out.Error = rule.Reason
		return out
	}
	if scope.Limited && !inside(scope.LimitRoots, resolved) {
		out.MatchedRuleIDs = append(out.MatchedRuleIDs, "provider-scope")
		out.Error = "This path is outside the folders permitted for this agent"
		return out
	}
	for _, r := range policy.Rules {
		if !r.Enabled || r.Kind == "protected" {
			continue
		}
		if r.ID == "other-workspaces" && (out.InsideWorkspace || inside(scope.SharedLinkRoots, resolved)) {
			continue
		}
		roots := r.paths
		if roots == nil {
			roots = r.ResolvedPaths
		}
		for _, root := range roots {
			if runtime.GOOS == "windows" && uncRoot(root) {
				result := network.probe(networkContext, root, chain)
				if !result.verified {
					out.UnverifiedRuleIDs = append(out.UnverifiedRuleIDs, r.ID)
				}
				if result.match || strings.EqualFold(root, path) || strings.EqualFold(root, resolved) {
					out.MatchedRuleIDs = append(out.MatchedRuleIDs, r.ID)
					out.Error = r.Reason
					break
				}
				continue
			}
			if runtime.GOOS == "windows" {
				if _, err := agents.NormalizeFilePath(root); err != nil {
					if strings.EqualFold(root, path) {
						out.MatchedRuleIDs = append(out.MatchedRuleIDs, r.ID)
						out.Error = r.Reason
					}
					continue
				}
			}
			if !filepath.IsAbs(root) {
				continue
			}
			canonical, err := Resolve(root)
			same, identityErr := agents.IdentityContains(root, chain)
			if identityErr != nil {
				out.Valid = false
				out.Error = "A deny-rule identity could not be checked"
				return out
			}
			if same || layout.Inside(root, path) || (err == nil && layout.Inside(canonical, resolved)) {
				out.MatchedRuleIDs = append(out.MatchedRuleIDs, r.ID)
				if out.Error == "" {
					out.Error = r.Reason
				}
				break
			}
		}
	}
	return out
}

func (s *Service) Check(path string, scope Scope) (Check, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy, err := s.rawPolicy(scope)
	if err != nil {
		return Check{}, err
	}
	chain, _ := agents.PathIdentities(path)
	return evaluateNetwork(path, scope, policy, chain, s.networkProber()), nil
}

func (s *Service) Add(path string, version int64, scope Scope) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolved, err := Resolve(path)
	if err != nil {
		return Policy{}, err
	}
	// Root-wide denial is allowed only as an explicit custom choice.
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return Policy{}, err
	}
	if err = s.Store.Change(version, "add", "custom-"+hex.EncodeToString(idBytes), resolved, true); err != nil {
		return Policy{}, err
	}
	return s.policy(scope)
}

func (s *Service) Remove(id string, version int64, scope Scope) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.HasPrefix(id, "custom-") {
		return Policy{}, errors.New("only your custom entries can be removed")
	}
	if err := s.Store.Change(version, "remove", id, "", false); err != nil {
		return Policy{}, err
	}
	return s.policy(scope)
}

func (s *Service) SetDefault(id string, enabled bool, version int64, scope Scope) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, r := range s.Defaults {
		if r.ID == id && r.Kind == "default" {
			found = true
			break
		}
	}
	if !found {
		return Policy{}, errors.New("that rule is not an optional default")
	}
	if err := s.Store.Change(version, "default", id, "", enabled); err != nil {
		return Policy{}, err
	}
	return s.policy(scope)
}

func (s *Service) Read(ctx context.Context, path string, line, limit *int, scope Scope) (string, error) {
	return s.read(ctx, path, line, limit, scope, func(check func(string, []os.FileInfo) error) (string, error) {
		return agents.ReadTextFileChecked(ctx, scope.Layout, path, scope.ManagedRoots, line, limit, nil, func(chain []os.FileInfo) error { return check(path, chain) })
	})
}

// ReadOpened applies the same policy and audit to a resource handle already
// opened through its confined source. Identity is checked before any content read.
func (s *Service) ReadOpened(ctx context.Context, path string, scope Scope, file *os.File, reader func(*os.File) (string, error)) (string, error) {
	return s.read(ctx, path, nil, nil, scope, func(check func(string, []os.FileInfo) error) (string, error) {
		if err := agents.CheckOpenedFile(path, file, func(chain []os.FileInfo) error { return check(path, chain) }); err != nil {
			return "", err
		}
		text, err := reader(file)
		if len(text) > agents.MaxProviderText {
			return "", errors.New("the resource exceeds the 1 MiB read limit")
		}
		return text, err
	})
}

func (s *Service) read(ctx context.Context, path string, line, limit *int, scope Scope, read func(func(string, []os.FileInfo) error) (string, error)) (string, error) {
	if scope.Chat == "" || scope.Workspace == "" {
		return "", errors.New("a registered chat and workspace are required for a file read")
	}
	if err := s.admitOperationPath(ctx, scope, path); err != nil {
		return "", err
	}
	if err := agents.ValidateLineRange(line, limit); err != nil {
		return "", err
	}
	s.mu.Lock()
	policy, err := s.rawPolicy(scope)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	chain, _ := agents.PathIdentities(path)
	check := evaluateNetwork(path, scope, policy, chain, s.networkProber())
	record := Record{UnverifiedRuleIDs: check.UnverifiedRuleIDs, Source: scope.Source, Chat: scope.Chat, ChatTitle: scope.ChatTitle, Workspace: scope.Workspace, Path: check.ResolvedPath}
	if record.Path == "" {
		record.Path = path
	}
	if len(check.MatchedRuleIDs) > 0 {
		record.Outcome, record.RuleID, record.Reason = "denied", check.MatchedRuleIDs[0], check.Error
		if err = s.recordRead(ctx, scope, record); err != nil {
			return "", fmt.Errorf("could not record refused file read: %w", err)
		}
		return "", errors.New("this read was refused: " + check.Error)
	}
	if !check.Valid {
		record.Outcome, record.Reason = "error", "The file identity could not be checked"
		if err := s.recordRead(ctx, scope, record); err != nil {
			return "", err
		}
		return "", errors.New(check.Error)
	}
	var refused Check
	text, readErr := read(func(canonical string, chain []os.FileInfo) error {
		final := evaluateNetwork(canonical, scope, policy, chain, s.networkProber())
		record.UnverifiedRuleIDs = append(record.UnverifiedRuleIDs, final.UnverifiedRuleIDs...)
		record.Path = final.ResolvedPath
		check.InsideWorkspace = final.InsideWorkspace
		if len(final.MatchedRuleIDs) > 0 {
			refused = final
			return errors.New("this read was refused: " + final.Error)
		}
		if !final.Valid {
			return errors.New(final.Error)
		}
		return nil
	})
	var protected *agents.ProtectedReadError
	if errors.As(readErr, &protected) {
		record.Path, record.Outcome, record.RuleID, record.Reason = protected.Path, "denied", "protected-state", protected.Reason
	} else if len(refused.MatchedRuleIDs) > 0 {
		record.Outcome, record.RuleID, record.Reason = "denied", refused.MatchedRuleIDs[0], refused.Error
	} else if readErr != nil {
		record.Outcome, record.Reason = "error", "The file could not be read"
	} else {
		record.Outcome, record.Bytes = "allowed", len([]byte(text))
	}
	if len(record.UnverifiedRuleIDs) > 0 || operationIn(ctx) != nil || scope.AuditAlways || !check.InsideWorkspace || record.Outcome == "denied" {
		if err = s.recordRead(ctx, scope, record); err != nil {
			return "", fmt.Errorf("could not commit file-read audit; no content returned: %w", err)
		}
	}
	return text, readErr
}

// ReadDirectory audits the bounded metadata returned by a pathname listing.
func (s *Service) ReadDirectory(ctx context.Context, path string, scope Scope) ([]fs.DirEntry, error) {
	if !InOperation(ctx) {
		var entries []fs.DirEntry
		text, ok := s.RunOperation(ctx, path, scope, func(ctx context.Context) (string, bool) {
			var err error
			entries, err = s.ReadDirectory(ctx, path, scope)
			if err != nil {
				return err.Error(), false
			}
			names := []string{}
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			data, err := json.Marshal(names)
			return string(data), err == nil
		})
		if !ok {
			return nil, errors.New(text)
		}
		return entries, nil
	}
	var entries []fs.DirEntry
	_, err := s.read(ctx, path, nil, nil, scope, func(check func(string, []os.FileInfo) error) (string, error) {
		var err error
		entries, err = agents.ReadDirectoryChecked(ctx, path, func(chain []os.FileInfo) error { return check(path, chain) })
		if err != nil {
			return "", err
		}
		parentInfo, statErr := os.Stat(path)
		if statErr != nil {
			return "", statErr
		}
		permitted := []fs.DirEntry{}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			if !parentInfo.IsDir() {
				child = path
			}
			if err := s.admitOperationPath(ctx, scope, child); err != nil {
				return "", err
			}
			file, openErr := agents.OpenIdentityPath(child)
			if openErr != nil {
				if err := s.recordRead(ctx, scope, Record{Source: scope.Source, Chat: scope.Chat, ChatTitle: scope.ChatTitle, Workspace: scope.Workspace, Path: child, Outcome: "error", Reason: "Metadata could not be opened"}); err != nil {
					return "", err
				}
				continue
			}
			_, readErr := s.ReadMetadata(ctx, child, scope, file)
			file.Close()
			if readErr == nil {
				permitted = append(permitted, entry)
			}
		}
		entries = permitted
		metadata := []map[string]any{}
		for _, e := range entries {
			metadata = append(metadata, map[string]any{"name": e.Name(), "directory": e.IsDir(), "type": e.Type().String()})
		}
		data, err := json.Marshal(metadata)
		if err != nil {
			return "", err
		}
		if len(data) > agents.MaxProviderText {
			return "", errors.New("the directory metadata exceeds 1 MiB; choose a smaller directory")
		}
		return string(data), nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ReadMetadata applies policy to an already confined source entry without
// reading a child directory or file body.
func (s *Service) ReadMetadata(ctx context.Context, path string, scope Scope, file *os.File) (string, error) {
	return s.read(ctx, path, nil, nil, scope, func(check func(string, []os.FileInfo) error) (string, error) {
		if err := agents.CheckOpenedPath(path, file, func(chain []os.FileInfo) error { return check(path, chain) }); err != nil {
			return "", err
		}
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(map[string]any{"name": info.Name(), "directory": info.IsDir(), "size": info.Size(), "modified": info.ModTime()})
		return string(data), err
	})
}
