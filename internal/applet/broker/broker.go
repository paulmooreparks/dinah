// Package broker carries out applet action invocations for a program, the
// desktop app or the command line: it starts an action from the registry,
// keeps each invocation as a durable record in the state folder, recovers or
// fails invocations a stopped program left running, holds a human request
// until a person answers it, and keeps artifacts. Both programs keep their
// invocations in the same place, and an invocation is held by the program
// executing it, so neither marks the other's invocation as failed or runs it
// a second time.
package broker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"studio/internal/applet"
	"studio/internal/keylock"
	"studio/internal/layout"
	"studio/internal/records"
)

// ErrRecovery means a saved invocation cannot be recovered safely.
var ErrRecovery = errors.New("applet invocation could not be recovered safely")

// ErrElsewhere means another program is executing the invocation.
var ErrElsewhere = errors.New("another program is running this applet action")

var recordID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var fieldName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// Artifact is a file an action produced, kept with its invocation.
type Artifact struct {
	ID           string `json:"id"`
	InvocationID string `json:"invocationId"`
	Name         string `json:"name"`
	MIME         string `json:"mime"`
	SHA256       string `json:"sha256"`
	Size         int    `json:"size"`
	DataBase64   string `json:"dataBase64,omitempty"`
}

// Invocation is one invocation of an action, as it is saved.
type Invocation struct {
	ID              string          `json:"id"`
	Workspace       string          `json:"workspace"`
	OwnerKind       string          `json:"ownerKind"`
	OwnerID         string          `json:"ownerId"`
	Action          string          `json:"action"`
	Digest          string          `json:"digest"`
	InputJSON       string          `json:"inputJSON"`
	OutputJSON      string          `json:"outputJSON"`
	Text            string          `json:"text,omitempty"` // the result for a reader, when the action wrote one (applet.Result)
	Status          string          `json:"status"`
	Error           string          `json:"error,omitempty"`
	Deadline        time.Time       `json:"deadline"`
	Created         time.Time       `json:"created"`
	Updated         time.Time       `json:"updated"`
	Version         int             `json:"version"`
	Request         *applet.Request `json:"request,omitempty"`
	Artifacts       []Artifact      `json:"artifacts,omitempty"`
	ResultDismissed bool            `json:"resultDismissed,omitempty"`
}

type execution struct {
	replay string
	effect string
	cancel context.CancelFunc
	done   chan struct{}
	// unrecorded marks the run of a pane's read, which has no saved record
	// (runUnrecorded); owner is who may cancel it.
	unrecorded       bool
	ownerKind, owner string
}

// Broker invokes a registry's actions. Emit, when set, is told
// "applet:invocation" with the public view of each invocation that changes,
// and "applet:error" when one cannot be saved.
type Broker struct {
	Registry *applet.Registry
	// State is the state folder the invocations are kept under.
	State string
	Emit  func(event string, data any)
	// Consent, when set, is asked before an action runs, and the action
	// runs only if it answers nil; its error becomes the invocation's. It
	// lets the host ask the person before an action has any effect. It may
	// wait for the person, until the invocation's deadline.
	Consent func(ctx context.Context, v Invocation, action applet.Action) error
	// Logf, when set, takes the entries of a run that is not recorded, which
	// have no invocation event to be logged from. source is the applet's.
	Logf func(source, level, format string, args ...any)

	// records has a lock per invocation, held while its record is read,
	// changed and written, so that invocations never wait on each other's
	// disk work.
	records keylock.Set
	// mu guards the maps below and is never held while a file is read or
	// written. A key's entries change only while its record lock is held.
	mu       sync.Mutex
	active   map[string]*execution
	failures map[string]error
	// holds are the records this broker holds while it executes them.
	holds map[string]func() error
}

// lock locks an invocation's record for a read, change and write.
func (b *Broker) lock(workspace, id string) (unlock func()) {
	return b.records.Lock(b.key(workspace, id))
}

func (b *Broker) failure(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures[key]
}

func (b *Broker) running(key string) *execution {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active[key]
}

func (b *Broker) holding(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.holds[key] != nil
}

func (b *Broker) emit(event string, data any) {
	if b.Emit != nil {
		b.Emit(event, data)
	}
}

// StoreRoot is the folder a workspace's invocations are kept under. It is
// in the state folder rather than the workspace, where agents could write.
func (b *Broker) StoreRoot(workspace string) string {
	key := filepath.Clean(workspace)
	// Paths are compared without case, as Windows compares them.
	sum := sha256.Sum256([]byte(strings.ToLower(key)))
	return filepath.Join(b.State, "applets", hex.EncodeToString(sum[:]))
}

func (b *Broker) store(workspace string) records.Store {
	return layout.Layout{State: b.State}.Runs(b.StoreRoot(workspace))
}

func (b *Broker) key(workspace, id string) string { return b.StoreRoot(workspace) + ":" + id }

// Read reads a saved invocation.
func (b *Broker) Read(workspace, id string) (Invocation, error) {
	if !recordID.MatchString(id) {
		return Invocation{}, fmt.Errorf("%w: invalid invocation identity: use the id the invoke returned, letters and digits only, and not the requestId", ErrRecovery)
	}
	data, err := b.store(workspace).Read(id)
	if err != nil {
		return Invocation{}, err
	}
	var v Invocation
	if err := json.Unmarshal(data, &v); err != nil || v.ID != id || !layout.SamePath(v.Workspace, workspace) {
		return Invocation{}, fmt.Errorf("%w: invalid saved invocation", ErrRecovery)
	}
	return v, nil
}

// save writes an invocation. A record another program holds is not written.
// The caller holds the record's lock.
func (b *Broker) save(v *Invocation) error {
	key := b.key(v.Workspace, v.ID)
	if !b.holding(key) {
		release, err := b.store(v.Workspace).Hold(v.ID)
		if errors.Is(err, records.ErrHeld) {
			return ErrElsewhere
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRecovery, err)
		}
		defer release()
	}
	v.Updated = time.Now().UTC()
	v.Version++
	data, err := json.Marshal(v)
	if err == nil {
		err = b.store(v.Workspace).Write(v.ID, data)
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecovery, err)
	}
	b.index().set(b.indexKey(v.Workspace, v.ID), Summarize(*v))
	return nil
}

// Save writes an invocation as it is given, for tools and tests that set up
// or repair a record by hand.
func (b *Broker) Save(v *Invocation) error {
	defer b.lock(v.Workspace, v.ID)()
	return b.save(v)
}

// persist saves an invocation; a failed save blocks this program from
// consuming the result, since what is on disk is unknown. The caller holds
// the record's lock.
func (b *Broker) persist(v *Invocation) error {
	err := b.save(v)
	if err != nil && !errors.Is(err, ErrElsewhere) {
		b.mu.Lock()
		if b.failures == nil {
			b.failures = map[string]error{}
		}
		b.failures[b.key(v.Workspace, v.ID)] = err
		b.mu.Unlock()
	}
	return err
}

// elsewhere reports whether another program executes an invocation. The
// caller holds the record's lock.
func (b *Broker) elsewhere(workspace, id string) bool {
	if b.holding(b.key(workspace, id)) {
		return false
	}
	return b.store(workspace).Held(id)
}

// NewID makes an invocation identity.
func NewID() (string, error) {
	var x [16]byte
	if _, err := rand.Read(x[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(x[:]), nil
}

// Start starts an invocation with a new identity.
func (b *Broker) Start(workspace, ownerKind, ownerID, ref, inputJSON, expectedDigest string, deadline time.Time) (Invocation, error) {
	id, err := NewID()
	if err != nil {
		return Invocation{}, err
	}
	return b.StartID(workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest, deadline)
}

// StartID starts an invocation with the identity given, or recovers the one
// already saved under it: a finished one is returned as it is, one this or
// another program is executing is left to it, and one a stopped program left
// running is run again if it is pure and failed if it may have had effects.
func (b *Broker) StartID(workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest string, deadline time.Time) (Invocation, error) {
	if (workspace == "" && ownerKind != "applet") || (workspace != "" && !filepath.IsAbs(workspace)) || ownerID == "" || !recordID.MatchString(id) {
		return Invocation{}, fmt.Errorf("%w: invalid invocation context", ErrRecovery)
	}
	if ownerKind != "chat" && ownerKind != "applet" {
		return Invocation{}, fmt.Errorf("%w: unsupported invocation owner", ErrRecovery)
	}
	action, err := b.Registry.Describe(ref)
	if err != nil {
		return Invocation{}, err
	}
	digest, err := b.Registry.Digest(ref)
	if err != nil {
		return Invocation{}, err
	}
	if expectedDigest != "" && digest != expectedDigest {
		return Invocation{}, fmt.Errorf("%w: the bundled action changed since you discovered it, so discover it again with applet_discover and invoke it with its new digest", ErrRecovery)
	}
	for _, c := range action.Capabilities {
		if !applet.SupportedCapability(c) {
			return Invocation{}, fmt.Errorf("the action requires unavailable capability %q", c)
		}
	}
	if deadline.IsZero() || deadline.After(time.Now().Add(24*time.Hour+time.Minute)) {
		return Invocation{}, errors.New("an applet deadline must be within 24 hours")
	}
	if err := applet.Validate(action.InputSchema, inputJSON); err != nil {
		return Invocation{}, err
	}
	defer b.lock(workspace, id)()
	if failure := b.failure(b.key(workspace, id)); failure != nil {
		return Invocation{}, failure
	}
	v, err := b.Read(workspace, id)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Invocation{}, fmt.Errorf("%w: %v", ErrRecovery, err)
	}
	if err == nil {
		if v.OwnerKind != ownerKind || v.OwnerID != ownerID || v.Action != ref || v.Digest != digest || v.InputJSON != inputJSON || !v.Deadline.Equal(deadline) {
			return Invocation{}, fmt.Errorf("%w: invocation identity was reused with different context", ErrRecovery)
		}
		if v.Status != "running" {
			return PublicView(v), nil
		}
		if b.running(b.key(workspace, id)) != nil || b.elsewhere(workspace, id) {
			return PublicView(v), nil
		}
		if action.Replay == "once" {
			return b.interrupted(v)
		}
		if err := b.execute(v); err != nil {
			return Invocation{}, err
		}
		return PublicView(v), nil
	}
	v = Invocation{ID: id, Workspace: workspace, OwnerKind: ownerKind, OwnerID: ownerID, Action: ref, Digest: digest, InputJSON: inputJSON, Status: "running", Deadline: deadline, Created: time.Now().UTC()}
	if err := b.execute(v); err != nil {
		return Invocation{}, err
	}
	return PublicView(v), nil
}

// execute holds an invocation's record, saves it and runs the action. A
// record another program holds is left to it. The caller holds the record's
// lock.
func (b *Broker) execute(v Invocation) error {
	key := b.key(v.Workspace, v.ID)
	release, err := b.store(v.Workspace).Hold(v.ID)
	if errors.Is(err, records.ErrHeld) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecovery, err)
	}
	b.mu.Lock()
	if b.holds == nil {
		b.holds = map[string]func() error{}
	}
	b.holds[key] = release
	b.mu.Unlock()
	if v.Version == 0 {
		if err := b.persist(&v); err != nil {
			b.mu.Lock()
			delete(b.holds, key)
			b.mu.Unlock()
			release()
			return err
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), v.Deadline)
	ex := &execution{cancel: cancel, done: make(chan struct{})}
	if action, err := b.Registry.Describe(v.Action); err == nil {
		ex.replay, ex.effect = action.Replay, action.Effect
	}
	b.mu.Lock()
	if b.active == nil {
		b.active = map[string]*execution{}
	}
	b.active[key] = ex
	b.mu.Unlock()
	go b.run(v, ex, ctx)
	return nil
}

// Executing is how many invocations this broker is executing now.
func (b *Broker) Executing() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.active)
}

// NonResumableExecuting counts invocations without a safe replay contract.
// An action declared "read" changes nothing, so stopping it loses nothing
// and it is not counted: the Git pane always has one waiting for its
// repository to change, and counting it made every exit and restart ask
// (Paul, 26 October 2026).
func (b *Broker) NonResumableExecuting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, ex := range b.active {
		if ex.replay != "pure" && ex.replay != "human" && ex.effect != "read" {
			count++
		}
	}
	return count
}

// PublicView is an invocation without its artifacts' data.
func PublicView(v Invocation) Invocation {
	v.Artifacts = slices.Clone(v.Artifacts)
	for i := range v.Artifacts {
		v.Artifacts[i].DataBase64 = ""
	}
	return v
}

func (b *Broker) run(v Invocation, ex *execution, ctx context.Context) {
	defer ex.cancel()
	defer close(ex.done)
	ctx = applet.WithInvocation(ctx, applet.Invocation{Workspace: v.Workspace, OwnerKind: v.OwnerKind, OwnerID: v.OwnerID, ID: v.ID})
	var result applet.Result
	var actionErr error
	if b.Consent != nil {
		action, err := b.Registry.Describe(v.Action)
		if err == nil {
			err = b.Consent(ctx, v, action)
		}
		actionErr = err
	}
	if actionErr == nil {
		result, actionErr = b.Registry.Execute(ctx, v.Action, v.InputJSON)
	}
	defer b.lock(v.Workspace, v.ID)()
	key := b.key(v.Workspace, v.ID)
	defer func() {
		b.mu.Lock()
		delete(b.active, key)
		release := b.holds[key]
		delete(b.holds, key)
		b.mu.Unlock()
		if release != nil {
			release()
		}
	}()
	current, err := b.Read(v.Workspace, v.ID)
	if err != nil || current.Status != "running" {
		return
	}
	v = current
	if actionErr == nil {
		actionErr = ctx.Err()
	}
	if actionErr != nil {
		v.Status, v.Error = "failed", actionErr.Error()
		if errors.Is(actionErr, context.Canceled) {
			v.Status = "cancelled"
		}
		if errors.Is(actionErr, context.DeadlineExceeded) {
			v.Status = "timeout"
		}
		if action, err := b.Registry.Describe(v.Action); err == nil && action.Replay == "once" && (v.Status == "cancelled" || v.Status == "timeout") {
			v.Error += "; changes may already be saved and are not rolled back; inspect the resource before retrying"
		}
	} else if actionErr = b.prepare(&v, result); actionErr != nil {
		v.Status, v.Error = "failed", actionErr.Error()
		v.Artifacts, v.Request, v.OutputJSON, v.Text = nil, nil, "", ""
	}
	if err := b.persist(&v); err != nil {
		b.emit("applet:error", map[string]string{"id": v.ID, "error": err.Error()})
		return
	}
	b.emit("applet:invocation", PublicView(v))
}

func (b *Broker) prepare(v *Invocation, result applet.Result) error {
	action, err := b.Registry.Describe(v.Action)
	if err != nil {
		return err
	}
	has := func(c string) bool { return slices.Contains(action.Capabilities, c) }
	if len(result.Artifacts) > 0 && !has("artifact.create") {
		return errors.New("the action did not declare artifact creation")
	}
	if result.Request != nil && !has("attention.request") {
		return errors.New("the action did not declare human requests")
	}
	if len(result.OutputJSON) > 4<<20 || len(result.Text) > 4<<20 || len(result.Artifacts) > 8 {
		return errors.New("the action result exceeds host bounds")
	}
	if !utf8.ValidString(result.Text) {
		return errors.New("the action's text is not valid UTF-8")
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.OutputJSON), &values); err != nil || values == nil {
		return errors.New("the action output must be a JSON object")
	}
	totalBytes := 0
	for i, artifact := range result.Artifacts {
		totalBytes += len(artifact.Data)
		if totalBytes > 4<<20 || artifact.Key == "" || len(artifact.Name) > 128 || filepath.Base(artifact.Name) != artifact.Name || strings.ContainsAny(artifact.Name, "\\/:\x00\r\n") {
			return errors.New("the artifact has invalid metadata or exceeds 4 MiB")
		}
		if _, exists := values[artifact.Key]; exists {
			return errors.New("the artifact output key is already occupied")
		}
		if artifact.MIME != "image/svg+xml" && artifact.MIME != "text/plain" && artifact.MIME != "text/markdown" && artifact.MIME != "application/json" {
			return errors.New("the artifact MIME type is not supported")
		}
		if artifact.MIME == "image/svg+xml" {
			if err := ValidateSVG(string(artifact.Data)); err != nil {
				return err
			}
		}
		sum := sha256.Sum256(artifact.Data)
		stored := Artifact{ID: fmt.Sprintf("%s_%d", v.ID, i), InvocationID: v.ID, Name: artifact.Name, MIME: artifact.MIME, Size: len(artifact.Data), SHA256: hex.EncodeToString(sum[:]), DataBase64: base64.StdEncoding.EncodeToString(artifact.Data)}
		v.Artifacts = append(v.Artifacts, stored)
		stored.DataBase64 = ""
		values[artifact.Key], _ = json.Marshal(stored)
	}
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	v.OutputJSON = string(data)
	v.Text = result.Text
	v.Request = result.Request
	if v.Request != nil {
		if len(v.Request.Fields) > 32 || len(v.Request.Title) > 1024 || len(v.Request.Body) > 80000 {
			return errors.New("the human request exceeds host bounds")
		}
		names := map[string]bool{}
		for _, field := range v.Request.Fields {
			if !fieldName.MatchString(field.Name) || field.Name == "constructor" || field.Name == "prototype" || names[field.Name] || len(field.Label) > 2048 || len(field.Choices) > 64 {
				return errors.New("the request has invalid or duplicate fields")
			}
			names[field.Name] = true
			if field.Type != "string" && field.Type != "boolean" && field.Type != "choice" {
				return errors.New("the request uses an unsupported field type")
			}
		}
		v.Status = "waiting"
		return nil
	}
	if err := applet.Validate(action.OutputSchema, v.OutputJSON); err != nil {
		return fmt.Errorf("invalid action output: %w", err)
	}
	v.Status = "done"
	return nil
}

func owner(v Invocation, ownerKind, ownerID string) error {
	if v.OwnerKind != ownerKind || v.OwnerID != ownerID {
		return errors.New("the invocation belongs to another caller")
	}
	return nil
}

// Get reads an invocation for its owner, timing it out once its deadline has
// passed and recovering it when a stopped program left it running.
func (b *Broker) Get(workspace, ownerKind, ownerID, id string) (Invocation, error) {
	defer b.lock(workspace, id)()
	key := b.key(workspace, id)
	v, err := b.Read(workspace, id)
	if err != nil {
		return v, fmt.Errorf("%w: %v", ErrRecovery, err)
	}
	if err := owner(v, ownerKind, ownerID); err != nil {
		return Invocation{}, err
	}
	if failure := b.failure(key); failure != nil {
		return Invocation{}, failure
	}
	if v.Status == "running" && b.running(key) == nil && b.elsewhere(workspace, id) {
		return PublicView(v), nil
	}
	if (v.Status == "running" || v.Status == "waiting") && !time.Now().Before(v.Deadline) {
		v.Status, v.Error = "timeout", "the applet deadline expired"
		if action, err := b.Registry.Describe(v.Action); err == nil && action.Replay == "once" {
			v.Error += "; changes may already be saved and are not rolled back; inspect the resource before retrying"
		}
		if err := b.persist(&v); err != nil {
			return Invocation{}, err
		}
		if active := b.running(key); active != nil {
			active.cancel()
		}
	}
	if v.Status == "running" && b.running(key) == nil {
		action, err := b.Registry.Describe(v.Action)
		if err != nil {
			return Invocation{}, err
		}
		if action.Replay == "once" {
			return b.interrupted(v)
		}
		digest, err := b.Registry.Digest(v.Action)
		if err != nil || digest != v.Digest {
			return Invocation{}, fmt.Errorf("%w: the bundled action changed while this invocation was saved, so discover it again with applet_discover and invoke it anew with its new digest", ErrRecovery)
		}
		if err := b.execute(v); err != nil {
			return Invocation{}, err
		}
	}
	return PublicView(v), nil
}

// interrupted fails an invocation that may have had effects before the
// program executing it stopped, since it cannot be repeated safely. The
// caller holds the record's lock.
func (b *Broker) interrupted(v Invocation) (Invocation, error) {
	v.Status = "failed"
	v.Error = "The program running this action stopped before the action recorded its result. Its changes may already be saved. Inspect the resource before starting a new action; this invocation will not be repeated."
	if err := b.persist(&v); err != nil {
		return Invocation{}, err
	}
	b.emit("applet:invocation", PublicView(v))
	return PublicView(v), nil
}

// Cancel cancels an invocation for its owner. One another program executes
// is refused with ErrElsewhere.
func (b *Broker) Cancel(workspace, ownerKind, ownerID, id string) error {
	// A run that is not recorded is cancelled where it runs.
	b.mu.Lock()
	ex := b.active[b.key(workspace, id)]
	b.mu.Unlock()
	if ex != nil && ex.unrecorded {
		if ex.ownerKind != ownerKind || ex.owner != ownerID {
			return errors.New("the invocation belongs to another caller")
		}
		ex.cancel()
		return nil
	}
	defer b.lock(workspace, id)()
	v, err := b.Read(workspace, id)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecovery, err)
	}
	if err := owner(v, ownerKind, ownerID); err != nil {
		return err
	}
	if v.Status != "running" && v.Status != "waiting" {
		return nil
	}
	v.Status, v.Error = "cancelled", "the caller cancelled this invocation"
	if action, err := b.Registry.Describe(v.Action); err == nil && action.Replay == "once" {
		v.Error += "; changes may already be saved and are not rolled back; inspect the resource before retrying"
	}
	if err := b.persist(&v); err != nil {
		return err
	}
	if active := b.running(b.key(workspace, id)); active != nil {
		active.cancel()
	}
	b.emit("applet:invocation", PublicView(v))
	return nil
}

// Run invokes an action and waits for its output: the value of a done
// invocation, its error for a failed one, and context.Canceled or
// context.DeadlineExceeded for one cancelled or timed out. Cancelling ctx
// cancels the invocation.
func (b *Broker) Run(ctx context.Context, workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest string, deadline time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if b.unrecorded(ownerKind, ownerID, ref) {
		return b.runUnrecorded(ctx, workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest, deadline)
	}
	if _, err := b.StartID(workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest, deadline); err != nil {
		return "", err
	}
	// The wait ends when this broker's execution of the invocation finishes,
	// and otherwise on the next tick, for one another program executes or
	// one that waits for a person.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	key := b.key(workspace, id)
	for {
		var finished chan struct{}
		if ex := b.running(key); ex != nil {
			finished = ex.done
		}
		v, err := b.Get(workspace, ownerKind, ownerID, id)
		if err != nil {
			return "", err
		}
		switch v.Status {
		case "done":
			return v.OutputJSON, nil
		case "failed":
			return "", errors.New(v.Error)
		case "cancelled":
			return "", fmt.Errorf("%w: %s", context.Canceled, v.Error)
		case "timeout":
			return "", fmt.Errorf("%w: %s", context.DeadlineExceeded, v.Error)
		}
		select {
		case <-ctx.Done():
			if err := b.Cancel(workspace, ownerKind, ownerID, id); err != nil {
				return "", err
			}
			return "", ctx.Err()
		case <-finished:
		case <-ticker.C:
		}
	}
}

// unrecorded says whether a run is a pane's read, which is not recorded. A
// pane, an extension's own, is waiting for the answer and has the invocation
// only for as long as the call lasts; a durable record exists so that a chat
// can poll a result across restarts and so that effects are not repeated, and
// neither applies to an action declared to change nothing. A chat's invocation
// keeps its record whatever its effect, because the chat polls it, and an
// action that makes an artifact or asks the person needs one too. The Git
// pane, which waits on a change in its repository and refreshes itself, once
// left some 15,000 records in four days (docs/plans/group-chat.md).
func (b *Broker) unrecorded(ownerKind, ownerID, ref string) bool {
	if ownerKind != "applet" || !strings.HasPrefix(ownerID, "extension:") {
		return false
	}
	action, err := b.Registry.Describe(ref)
	if err != nil || action.Effect != "read" {
		return false
	}
	return !slices.Contains(action.Capabilities, "artifact.create") && !slices.Contains(action.Capabilities, "attention.request")
}

// runUnrecorded runs a pane's read as Run runs any action and waits for it,
// without saving a record of it, logging it above debug, or telling the page
// of an invocation. It checks what StartID checks, and can be cancelled with
// ctx or Cancel as a recorded one can.
func (b *Broker) runUnrecorded(ctx context.Context, workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest string, deadline time.Time) (string, error) {
	if (workspace == "" && ownerKind != "applet") || (workspace != "" && !filepath.IsAbs(workspace)) || ownerID == "" || !recordID.MatchString(id) {
		return "", fmt.Errorf("%w: invalid invocation context", ErrRecovery)
	}
	action, err := b.Registry.Describe(ref)
	if err != nil {
		return "", err
	}
	digest, err := b.Registry.Digest(ref)
	if err != nil {
		return "", err
	}
	if expectedDigest != "" && digest != expectedDigest {
		return "", fmt.Errorf("%w: the bundled action changed since you discovered it, so discover it again with applet_discover and invoke it with its new digest", ErrRecovery)
	}
	for _, c := range action.Capabilities {
		if !applet.SupportedCapability(c) {
			return "", fmt.Errorf("the action requires unavailable capability %q", c)
		}
	}
	if deadline.IsZero() || deadline.After(time.Now().Add(24*time.Hour+time.Minute)) {
		return "", errors.New("an applet deadline must be within 24 hours")
	}
	if err := applet.Validate(action.InputSchema, inputJSON); err != nil {
		return "", err
	}
	v := Invocation{ID: id, Workspace: workspace, OwnerKind: ownerKind, OwnerID: ownerID, Action: ref, Digest: digest, InputJSON: inputJSON, Status: "running", Deadline: deadline, Created: time.Now().UTC()}
	runCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	key := b.key(workspace, id)
	ex := &execution{replay: action.Replay, effect: action.Effect, cancel: cancel, done: make(chan struct{}), unrecorded: true, ownerKind: ownerKind, owner: ownerID}
	b.mu.Lock()
	if b.active == nil {
		b.active = map[string]*execution{}
	}
	if b.active[key] != nil {
		b.mu.Unlock()
		return "", fmt.Errorf("%w: invocation identity is already in use", ErrRecovery)
	}
	b.active[key] = ex
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.active, key)
		b.mu.Unlock()
		close(ex.done)
	}()
	runCtx = applet.WithInvocation(runCtx, applet.Invocation{Workspace: workspace, OwnerKind: ownerKind, OwnerID: ownerID, ID: id})
	var result applet.Result
	var actionErr error
	if b.Consent != nil {
		actionErr = b.Consent(runCtx, v, action)
	}
	if actionErr == nil {
		result, actionErr = b.Registry.Execute(runCtx, ref, inputJSON)
	}
	if actionErr == nil {
		actionErr = runCtx.Err()
	}
	if actionErr == nil {
		actionErr = b.prepare(&v, result)
	}
	source := "applet"
	if i := strings.Index(ref, "/"); i > 0 {
		source = "applet:" + ref[:i]
	}
	took := time.Since(v.Created).Round(time.Millisecond)
	switch {
	case actionErr == nil:
		b.debug(source, "%s done for %s %s in %s", ref, ownerKind, ownerID, took)
		return v.OutputJSON, nil
	case errors.Is(actionErr, context.Canceled):
		b.debug(source, "%s cancelled for %s %s", ref, ownerKind, ownerID)
		return "", fmt.Errorf("%w: %s", context.Canceled, actionErr)
	case errors.Is(actionErr, context.DeadlineExceeded):
		b.debug(source, "%s timed out for %s %s", ref, ownerKind, ownerID)
		return "", fmt.Errorf("%w: %s", context.DeadlineExceeded, actionErr)
	}
	b.debug(source, "%s failed for %s %s: %v", ref, ownerKind, ownerID, actionErr)
	return "", actionErr
}

func (b *Broker) debug(source, format string, args ...any) {
	if b.Logf != nil {
		b.Logf(source, "debug", format, args...)
	}
}

// Artifact returns an artifact of an invocation, with its data, for the
// invocation's owner.
func (b *Broker) Artifact(workspace, ownerKind, ownerID, id string) (Artifact, error) {
	split := strings.LastIndex(id, "_")
	if split < 1 {
		return Artifact{}, errors.New("invalid artifact identity")
	}
	v, err := b.Get(workspace, ownerKind, ownerID, id[:split])
	if err != nil {
		return Artifact{}, err
	}
	defer b.lock(workspace, v.ID)()
	if failure := b.failure(b.key(workspace, v.ID)); failure != nil {
		return Artifact{}, failure
	}
	stored, err := b.Read(workspace, v.ID)
	if err != nil {
		return Artifact{}, err
	}
	if err := owner(stored, ownerKind, ownerID); err != nil {
		return Artifact{}, err
	}
	for _, artifact := range stored.Artifacts {
		if artifact.ID == id {
			return artifact, nil
		}
	}
	return Artifact{}, errors.New("the artifact is unavailable")
}

func (b *Broker) ids(root string) ([]string, error) {
	files, err := os.ReadDir(layout.Layout{State: b.State}.RunDir(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, file := range files {
		id := records.ID(file.Name())
		if seen[id] || !recordID.MatchString(id) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// Saved reads a workspace's invocations as they are saved, without
// recovering or timing out any, in no particular order.
func (b *Broker) Saved(workspace string) ([]Invocation, error) {
	ids, err := b.ids(b.StoreRoot(workspace))
	if err != nil {
		return nil, err
	}
	var out []Invocation
	for _, id := range ids {
		v, err := b.Read(workspace, id)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// List lists a workspace's invocations, newest first.
func (b *Broker) List(workspace string) ([]Invocation, error) {
	if workspace != "" && !filepath.IsAbs(workspace) {
		return []Invocation{}, nil
	}
	ids, err := b.ids(b.StoreRoot(workspace))
	if err != nil {
		return nil, err
	}
	list := []Invocation{}
	for _, id := range ids {
		v, err := b.Read(workspace, id)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRecovery, err)
		}
		if v, err = b.Get(workspace, v.OwnerKind, v.OwnerID, id); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
	return list, nil
}

// Answer completes a waiting human request with the person's response.
func (b *Broker) Answer(workspace, id string, version int, responseJSON string) (Invocation, error) {
	defer b.lock(workspace, id)()
	if failure := b.failure(b.key(workspace, id)); failure != nil {
		return Invocation{}, failure
	}
	v, err := b.Read(workspace, id)
	if err != nil {
		return Invocation{}, err
	}
	if v.Status != "waiting" || v.Request == nil || v.Version != version {
		return Invocation{}, errors.New("this request changed or was already completed; refresh it")
	}
	if !time.Now().Before(v.Deadline) {
		return Invocation{}, errors.New("the request deadline expired")
	}
	if len(responseJSON) > 65536 {
		return Invocation{}, errors.New("the response exceeds 64 KiB")
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil || response == nil {
		return Invocation{}, errors.New("the response must be a JSON object")
	}
	allowed := map[string]bool{}
	for _, field := range v.Request.Fields {
		allowed[field.Name] = true
		value, exists := response[field.Name]
		if !exists {
			if field.Required {
				return Invocation{}, fmt.Errorf("%s is required", field.Label)
			}
			continue
		}
		switch field.Type {
		case "boolean":
			var x bool
			if string(value) == "null" || json.Unmarshal(value, &x) != nil {
				return Invocation{}, fmt.Errorf("%s must be true or false", field.Label)
			}
		case "string", "choice":
			var s string
			if json.Unmarshal(value, &s) != nil || (field.Required && strings.TrimSpace(s) == "") {
				return Invocation{}, fmt.Errorf("%s requires text", field.Label)
			}
			if field.Type == "choice" && !slices.Contains(field.Choices, s) {
				return Invocation{}, fmt.Errorf("%s is not an offered choice", field.Label)
			}
		default:
			return Invocation{}, errors.New("the request uses an unsupported field type")
		}
	}
	for key := range response {
		if !allowed[key] {
			return Invocation{}, fmt.Errorf("unknown response field %q", key)
		}
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.OutputJSON), &values); err != nil {
		return Invocation{}, fmt.Errorf("%w: invalid pending output", ErrRecovery)
	}
	values["response"] = json.RawMessage(responseJSON)
	data, _ := json.Marshal(values)
	action, err := b.Registry.Describe(v.Action)
	if err != nil {
		return Invocation{}, err
	}
	digest, err := b.Registry.Digest(v.Action)
	if err != nil || digest != v.Digest {
		return Invocation{}, fmt.Errorf("%w: the bundled action changed while this invocation ran, so discover it again with applet_discover and invoke it anew with its new digest", ErrRecovery)
	}
	if err := applet.Validate(action.OutputSchema, string(data)); err != nil {
		return Invocation{}, err
	}
	v.OutputJSON = string(data)
	v.Status = "done"
	if err := b.persist(&v); err != nil {
		return Invocation{}, err
	}
	b.emit("applet:invocation", PublicView(v))
	return PublicView(v), nil
}

// TerminalStatus reports whether an invocation's status is final.
func TerminalStatus(status string) bool {
	switch status {
	case "done", "failed", "cancelled", "timeout":
		return true
	}
	return false
}

// Dismiss marks a finished invocation's result as seen.
func (b *Broker) Dismiss(workspace, id string) error {
	defer b.lock(workspace, id)()
	if failure := b.failure(b.key(workspace, id)); failure != nil {
		return failure
	}
	v, err := b.Read(workspace, id)
	if err != nil {
		return err
	}
	if !TerminalStatus(v.Status) {
		return errors.New("only completed applet results can be dismissed")
	}
	if v.ResultDismissed {
		return nil
	}
	v.ResultDismissed = true
	if err := b.persist(&v); err != nil {
		return err
	}
	b.emit("applet:invocation", PublicView(v))
	return nil
}
