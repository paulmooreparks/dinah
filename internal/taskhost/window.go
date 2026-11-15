package taskhost

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"studio/internal/records"
)

type taskRecord struct {
	Questions map[string]QuestionRecord `json:"questions,omitempty"`
	ID        string                    `json:"id"`
	Sequence  uint64                    `json:"sequence"`
	Receipts  map[string]Receipt        `json:"receipts"`
}

type lease struct {
	plan    Plan
	binding Binding
	ctx     context.Context
	cancel  context.CancelFunc
	release func() error
	active  int
	revoked bool
	poison  error
	targets map[string]string
}

type Window struct {
	questions *Questions
	mu        sync.Mutex
	options   Options
	store     records.Store
	reviews   map[string]Review
	bindings  map[string]Binding
	leases    map[string]*lease
	workers   sync.WaitGroup
	closed    bool
	targets   map[string]map[string]string
	events    []Receipt
	emitting  bool
	claims    map[string]bool
	root      context.Context
	stop      context.CancelFunc
}

func New(o Options) (*Window, error) {
	if o.State == "" || o.Resolve == nil || o.Check == nil || o.Admit == nil || o.Confirm == nil || o.Backend == nil || o.Track == nil {
		return nil, refusal("unavailable", "the task host is missing a required authority or execution port")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	root, stop := context.WithCancel(context.Background())
	return &Window{options: o, store: records.Store{Dir: filepath.Join(o.State, "task-operations")}, reviews: map[string]Review{}, bindings: map[string]Binding{}, leases: map[string]*lease{}, targets: map[string]map[string]string{}, claims: map[string]bool{}, root: root, stop: stop}, nil
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func taskKey(s Scope) string { return digest([]string{s.Service, s.Workspace, s.Task}) }

func validScope(s Scope) bool {
	return s.Service != "" && s.Workspace != "" && s.Task != "" && s.Generation != "" && len(s.Service)+len(s.Workspace)+len(s.Task)+len(s.Generation)+len(s.OwnerChat) <= 2048
}

func roomKind(k string) bool {
	return slices.Contains([]string{"EnsureRoom", "ClaimRoom", "PostRoom", "ObserveRoom", "ReleaseRoom", "RecoverRoom"}, k)
}

func (w *Window) roomsAvailable() bool {
	if w.options.RoomsAvailable != nil {
		return w.options.RoomsAvailable()
	}
	return w.options.Rooms
}

func validKind(k string) bool {
	return roomKind(k) || slices.Contains([]string{"OpenChat", "TakeTurn", "ReadOptions", "UseModel", "CancelTurn", "ReleaseChat", "InvokeAction", "CancelAction", "AskPerson"}, k)
}

func clone[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}

func (w *Window) check(ctx context.Context, s Scope) (Binding, error) {
	if !validScope(s) {
		return Binding{}, refusal("not_authorized", "a complete task identity is required")
	}
	if err := w.options.Check(ctx, s); err != nil {
		return Binding{}, err
	}
	return w.options.Resolve(ctx, s)
}

func (w *Window) Review(ctx context.Context, p Plan) (Review, error) {
	p = clone(p)
	b, err := w.check(ctx, p.Scope)
	if err != nil {
		return Review{}, err
	}
	now := w.options.Now()
	if !p.Deadline.After(now) || p.Deadline.After(now.Add(24*time.Hour)) {
		return Review{}, refusal("deadline_exceeded", "task deadline must be in the next 24 hours")
	}
	if len(p.Permits) == 0 || len(p.Permits) > 256 {
		return Review{}, refusal("not_authorized", "a bounded reviewed operation set is required")
	}
	targets := map[string]string{}
	for _, a := range p.Permits {
		if len(a.Input) > 1<<20 || len(a.Input) > 0 && !json.Valid(a.Input) {
			return Review{}, refusal("stale", "the reviewed input is invalid")
		}
		if !validKind(a.Kind) {
			return Review{}, refusal("unsupported", "the requested operation is not supported")
		}
		if roomKind(a.Kind) && !w.roomsAvailable() {
			return Review{}, refusal("unsupported", "the complete room protocol is unavailable")
		}
		if err := w.options.Admit(ctx, p.Scope, Operation{Kind: a.Kind, Target: a.Target, Program: a.Program, Digest: a.Digest}); err != nil {
			return Review{}, err
		}
		if w.options.TargetRevision != nil {
			stamp, err := w.options.TargetRevision(ctx, p.Scope, a)
			if err != nil {
				return Review{}, err
			}
			targets[digest(a)] = stamp
		}
	}
	if p.Rooms && !w.roomsAvailable() {
		return Review{}, refusal("unsupported", "the complete room protocol is unavailable")
	}
	r := Review{ID: rand.Text(), Plan: p, Expires: now.Add(5 * time.Minute)}
	if r.Expires.After(p.Deadline) {
		r.Expires = p.Deadline
	}
	w.mu.Lock()
	defer w.unlock()
	if w.closed {
		return Review{}, refusal("unavailable", "task admission is closed")
	}
	w.reviews[r.ID] = r
	w.bindings[r.ID] = b
	w.targets[r.ID] = targets
	return clone(r), nil
}

// Begin never accepts the copy of a plan supplied by a caller as authority.
func (w *Window) Begin(ctx context.Context, reviewID string) (*Handle, error) {
	w.mu.Lock()
	r, ok := w.reviews[reviewID]
	b := w.bindings[reviewID]
	targets := w.targets[reviewID]
	closed := w.closed
	w.unlock()
	if !ok || closed || !w.options.Now().Before(r.Expires) {
		return nil, refusal("not_authorized", "the review is absent, expired or revoked")
	}
	current, err := w.check(ctx, r.Plan.Scope)
	if err != nil {
		return nil, err
	}
	if current != b {
		return nil, refusal("workspace_changed", "the reviewed workspace binding changed")
	}
	confirmation, cancelConfirmation := context.WithCancel(ctx)
	stopConfirmation := context.AfterFunc(w.root, cancelConfirmation)
	defer stopConfirmation()
	defer cancelConfirmation()
	if err = w.options.Confirm(confirmation, clone(r)); err != nil {
		return nil, err
	}
	current, err = w.check(ctx, r.Plan.Scope)
	if err != nil {
		return nil, err
	}
	if err = w.checkTargets(ctx, r.Plan, targets); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.unlock()
	if w.closed || w.reviews[reviewID].ID == "" || !w.options.Now().Before(r.Expires) || current != b {
		return nil, refusal("stale", "review admission changed while confirmation was pending")
	}
	key := taskKey(r.Plan.Scope)
	if old := w.leases[key]; old != nil {
		if !old.revoked || old.active > 0 {
			return nil, refusal("busy", "this task is already admitted")
		}
		if err = old.release(); err != nil {
			return nil, err
		}
		delete(w.leases, key)
	}
	release, err := w.store.Hold(key)
	if err != nil {
		return nil, refusal("held_elsewhere", err.Error())
	}
	record, err := w.read(key)
	if err != nil {
		_ = release()
		return nil, err
	}
	for _, receipt := range record.Receipts {
		if receipt.State == "prepared" || receipt.State == "in_progress" {
			_ = release()
			return nil, refusal("unknown_outcome", "interrupted receipts require recovery before renewed execution")
		}
	}
	workCtx, cancel := context.WithDeadline(context.Background(), r.Plan.Deadline)
	l := &lease{plan: r.Plan, binding: b, ctx: workCtx, cancel: cancel, release: release, targets: targets}
	w.leases[key] = l
	delete(w.reviews, reviewID)
	delete(w.bindings, reviewID)
	delete(w.targets, reviewID)
	return &Handle{window: w, lease: l}, nil
}

func (w *Window) read(key string) (taskRecord, error) {
	b, err := w.store.Read(key)
	if errors.Is(err, os.ErrNotExist) {
		return taskRecord{ID: key, Receipts: map[string]Receipt{}}, nil
	}
	if err != nil {
		return taskRecord{}, refusal("persistence_failed", err.Error())
	}
	var r taskRecord
	if err = json.Unmarshal(b, &r); err != nil || r.ID != key || r.Receipts == nil {
		return taskRecord{}, refusal("persistence_failed", "invalid task receipt record")
	}
	return r, nil
}

// write is also the private cleanup path: it does not check execution handles.
// All callers hold mu. Closing execution admission cannot prevent checkpoints.
func (w *Window) write(record taskRecord, r Receipt) error {
	for _, old := range record.Receipts {
		if old.Sequence > record.Sequence {
			record.Sequence = old.Sequence
		}
	}
	record.Sequence++
	r.Sequence = record.Sequence
	r.Updated = w.options.Now()
	if w.options.Save != nil {
		if err := w.options.Save(clone(r)); err != nil {
			return refusal("persistence_failed", err.Error())
		}
	}
	record.Receipts[r.Operation] = r
	b, err := json.Marshal(record)
	if err == nil {
		err = w.store.Write(record.ID, b)
	}
	if err != nil {
		return refusal("persistence_failed", err.Error())
	}
	if w.options.AfterSave != nil {
		if err := w.options.AfterSave(clone(r)); err != nil {
			return refusal("outcome_unknown", "The write result is unknown. Reconcile the original operation before taking further action: "+err.Error())
		}
	}
	if w.options.Emit != nil {
		w.events = append(w.events, clone(r))
	}
	return nil
}

// unlock delivers committed notifications outside the store lock. Reentrant
// observation only queues further notifications behind the current delivery.
func (w *Window) unlock() {
	w.mu.Unlock()
	w.mu.Lock()
	if w.emitting || len(w.events) == 0 {
		w.mu.Unlock()
		return
	}
	w.emitting = true
	for len(w.events) > 0 {
		event := w.events[0]
		w.events = w.events[1:]
		w.mu.Unlock()
		func() {
			defer func() { _ = recover() }()
			w.options.Emit(event)
		}()
		w.mu.Lock()
	}
	w.emitting = false
	w.mu.Unlock()
}

func permits(p Plan, o Operation) bool {
	return slices.ContainsFunc(p.Permits, func(a Permit) bool {
		return a.Kind == o.Kind && a.Target == o.Target && a.Program == o.Program && a.Digest == o.Digest && digest(a.Input) == digest(o.Input)
	})
}

func (w *Window) checkTargets(ctx context.Context, p Plan, expected map[string]string) error {
	if w.options.TargetRevision == nil {
		return nil
	}
	for _, permit := range p.Permits {
		current, err := w.options.TargetRevision(ctx, p.Scope, permit)
		if err != nil {
			return err
		}
		if current != expected[digest(permit)] {
			return refusal("stale", "a reviewed target changed ownership or binding")
		}
	}
	return nil
}

func (w *Window) handle(h *Handle) (*lease, error) {
	if h == nil || h.window != w || h.lease == nil {
		return nil, refusal("not_authorized", "this window did not issue the handle")
	}
	l := h.lease
	if w.closed || l.revoked || l.poison != nil || !w.options.Now().Before(l.plan.Deadline) || l.ctx.Err() != nil {
		l.revoked = true
		l.cancel()
		return nil, refusal("not_authorized", "execution authority has been revoked")
	}
	return l, nil
}

// Start persists an intent and returns its receipt without tying the work's
// lifetime to a short service request. Wait observes it without cancelling it.
func (w *Window) Start(ctx context.Context, h *Handle, o Operation) (Receipt, error) {
	o = clone(o)
	if o.ID == "" || len(o.ID) > 128 || len(o.Input) > 1<<20 || (len(o.Input) > 0 && !json.Valid(o.Input)) {
		return Receipt{}, refusal("stale", "invalid operation identity or input")
	}
	w.mu.Lock()
	defer w.unlock()
	l, err := w.handle(h)
	if err != nil {
		return Receipt{}, err
	}
	if !permits(l.plan, o) {
		return Receipt{}, refusal("not_authorized", "the operation is outside the reviewed set")
	}
	if err = w.checkTargets(ctx, l.plan, l.targets); err != nil {
		l.revoked = true
		l.cancel()
		return Receipt{}, err
	}
	b, err := w.check(ctx, l.plan.Scope)
	if err != nil || b != l.binding {
		l.revoked = true
		l.cancel()
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{}, refusal("workspace_changed", "the reviewed workspace binding changed")
	}
	key := taskKey(l.plan.Scope)
	record, err := w.read(key)
	if err != nil {
		l.poison = err
		l.cancel()
		return Receipt{}, err
	}
	fingerprint := digest(struct {
		Scope     Scope
		Operation Operation
	}{l.plan.Scope, o})
	if old, ok := record.Receipts[o.ID]; ok {
		if old.Fingerprint != fingerprint {
			return Receipt{}, refusal("stale", "operation ID already describes different work")
		}
		return clone(old), nil
	}
	if err = w.options.Admit(ctx, l.plan.Scope, o); err != nil {
		return Receipt{}, err
	}
	resource := ""
	if w.options.Resource != nil {
		resource, err = w.options.Resource(ctx, l.plan.Scope, o)
		if err != nil {
			return Receipt{}, err
		}
		if resource != "" && w.claims[resource] {
			return Receipt{}, refusal("busy", "another task holds this chat's turn or configuration")
		}
	}
	done, err := w.options.Track(l.plan.Scope, o)
	if err != nil {
		return Receipt{}, err
	}
	if done == nil {
		return Receipt{}, refusal("unavailable", "the exit guard supplied no completion registration")
	}
	r := Receipt{ID: digest([]string{key, o.ID}), Scope: l.plan.Scope, Operation: o.ID, Kind: o.Kind, Target: o.Target, Fingerprint: fingerprint, State: "prepared"}
	if err = w.write(record, r); err != nil {
		done()
		l.poison = err
		l.cancel()
		return Receipt{}, err
	}
	r = record.Receipts[o.ID]
	l.active++
	if resource != "" {
		w.claims[resource] = true
	}
	w.workers.Add(1)
	go w.run(l, o, resource, done)
	return clone(r), nil
}

func (w *Window) run(l *lease, o Operation, resource string, done func()) {
	defer w.workers.Done()
	defer done()
	w.mu.Lock()
	key := taskKey(l.plan.Scope)
	record, err := w.read(key)
	r := record.Receipts[o.ID]
	if err == nil && l.ctx.Err() != nil {
		r.State = "cancelled"
		r.Code = "cancelled"
		err = w.write(record, r)
	} else if err == nil {
		r.State = "in_progress"
		err = w.write(record, r)
	}
	if err != nil {
		l.poison = err
		l.cancel()
	}
	shouldRun := err == nil && r.State == "in_progress"
	w.unlock()
	out := Outcome{State: "cancelled", Code: "cancelled"}
	if shouldRun {
		// Admission is repeated at the external-work boundary, after intent save.
		var current Binding
		current, err = w.check(l.ctx, l.plan.Scope)
		if err == nil && current != l.binding {
			err = refusal("workspace_changed", "the reviewed workspace binding changed before dispatch")
		}
		if err == nil {
			err = w.options.Admit(l.ctx, l.plan.Scope, o)
		}
		if err == nil {
			err = w.checkTargets(l.ctx, l.plan, l.targets)
		}
		if err != nil {
			out = Outcome{State: "failed", Code: "not_authorized", Reason: err.Error()}
		} else {
			out = executeBackend(w.options.Backend, context.WithValue(l.ctx, questionExecutionKey{}, questionExecution{w, o.ID}), l.plan.Scope, o, func(progress Progress) error {
				w.mu.Lock()
				defer w.unlock()
				rec, e := w.read(key)
				if e != nil {
					return e
				}
				r := rec.Receipts[o.ID]
				id := progress.Identity
				if len(progress.Result) > 1<<20 || len(progress.Result) > 0 && !json.Valid(progress.Result) {
					return refusal("stale", "invalid operation result checkpoint")
				}
				if (id == "" && !progress.Dispatched && !progress.SetupFinished && len(progress.Result) == 0) || (id != "" && r.Identity != "" && r.Identity != id) {
					return refusal("stale", "native receipt identity changed")
				}
				if id != "" {
					r.Identity = id
				}
				r.Dispatched = r.Dispatched || progress.Dispatched
				if progress.SetupFinished {
					r.SetupFinished = true
					r.SetupError = progress.SetupError
				}
				if progress.Result != nil {
					r.Result = append(json.RawMessage(nil), progress.Result...)
				}
				e = w.write(rec, r)
				if e != nil {
					l.poison = e
					l.cancel()
				}
				return e
			})
		}
	}
	w.mu.Lock()
	record, err = w.read(key)
	if err == nil {
		r = record.Receipts[o.ID]
		// A question settlement checkpoint already includes its final operation
		// receipt and completion obligation. The returning worker cannot replace it.
		if r.Disposition == "" {
			if !slices.Contains([]string{"succeeded", "failed", "cancelled", "unknown"}, out.State) {
				out = Outcome{State: "unknown", Code: "unknown_outcome", Reason: "the adapter did not establish an outcome"}
			}
			if l.poison != nil {
				out.State, out.Code, out.Reason = "unknown", "persistence_failed", l.poison.Error()
			}
			if out.Identity != "" {
				if r.Identity != "" && out.Identity != r.Identity {
					out.State = "unknown"
					out.Code = "unknown_outcome"
				} else {
					r.Identity = out.Identity
				}
			}
			r.State, r.Code, r.Reason = out.State, out.Code, out.Reason
			if out.Result != nil {
				r.Result = out.Result
			}
			err = w.write(record, r)
		}
	}
	if err != nil {
		l.poison = err
		l.cancel()
	}
	l.active--
	delete(w.claims, resource)
	w.unlock()
}

func executeBackend(backend Backend, ctx context.Context, scope Scope, operation Operation, capture Capture) (out Outcome) {
	defer func() {
		if recover() != nil {
			out = Outcome{State: "unknown", Code: "unknown_outcome", Reason: "the task adapter stopped before establishing an outcome"}
		}
	}()
	return backend.Execute(ctx, scope, operation, capture)
}

// Rebind grants no execution handle. It reconciles interrupted receipts only.
func (w *Window) Rebind(ctx context.Context, s Scope) (Recovery, error) {
	if w.options.HoldTask == nil {
		return Recovery{}, refusal("unavailable", "task record ownership cannot be established")
	}
	unlockTask, err := w.options.HoldTask(ctx, s)
	if err != nil {
		return Recovery{}, err
	}
	if unlockTask == nil {
		return Recovery{}, refusal("held_elsewhere", "the consumer supplied no record ownership")
	}
	defer unlockTask()
	if _, err := w.check(ctx, s); err != nil {
		return Recovery{}, err
	}
	w.mu.Lock()
	defer w.unlock()
	if w.closed {
		return Recovery{}, refusal("unavailable", "task admission is closed")
	}
	key := taskKey(s)
	if l := w.leases[key]; l != nil && (!l.revoked || l.active > 0) {
		return Recovery{}, refusal("held_elsewhere", "the task is still held")
	}
	if l := w.leases[key]; l != nil {
		_ = l.release()
		delete(w.leases, key)
	}
	release, err := w.store.Hold(key)
	if err != nil {
		return Recovery{}, refusal("held_elsewhere", err.Error())
	}
	defer release()
	record, err := w.read(key)
	if err != nil {
		return Recovery{}, err
	}
	out := Recovery{Scope: s, Receipts: []Receipt{}}
	for _, r := range record.Receipts {
		if r.State == "prepared" || r.State == "in_progress" {
			r.State = "unknown"
			r.Code = "unknown_outcome"
			r.Reason = "execution was interrupted; no replay was attempted"
			if err = w.write(record, r); err != nil {
				return Recovery{}, err
			}
			r = record.Receipts[r.Operation]
		}
		out.Receipts = append(out.Receipts, clone(r))
	}
	slices.SortFunc(out.Receipts, func(a, b Receipt) int {
		if a.Operation < b.Operation {
			return -1
		}
		if a.Operation > b.Operation {
			return 1
		}
		return 0
	})
	return out, nil
}

func (w *Window) Read(ctx context.Context, s Scope, operation string) (Receipt, error) {
	if _, err := w.check(ctx, s); err != nil {
		return Receipt{}, err
	}
	w.mu.Lock()
	defer w.unlock()
	r, err := w.read(taskKey(s))
	if err != nil {
		return Receipt{}, err
	}
	v, ok := r.Receipts[operation]
	if !ok {
		return Receipt{}, os.ErrNotExist
	}
	return clone(v), nil
}

func (w *Window) Cancel(h *Handle) error {
	w.mu.Lock()
	defer w.unlock()
	if h == nil || h.window != w || h.lease == nil {
		return refusal("not_authorized", "invalid handle")
	}
	h.lease.revoked = true
	h.lease.cancel()
	return nil
}

func (w *Window) RevokeChat(chat string) {
	w.mu.Lock()
	defer w.unlock()
	for _, l := range w.leases {
		if l.plan.Scope.OwnerChat == chat || slices.ContainsFunc(l.plan.Permits, func(p Permit) bool { return p.Target == chat }) {
			l.revoked = true
			l.cancel()
		}
	}
}

func (w *Window) CloseAdmission() {
	w.mu.Lock()
	if w.closed {
		w.unlock()
		return
	}
	w.closed = true
	clear(w.reviews)
	clear(w.bindings)
	clear(w.targets)
	var cancellations []context.CancelFunc
	for _, l := range w.leases {
		l.revoked = true
		cancellations = append(cancellations, l.cancel)
	}
	w.unlock()
	if w.options.Lifecycle != nil {
		w.options.Lifecycle("admission-closed")
	}
	w.stop()
	for _, cancel := range cancellations {
		cancel()
	}
}

func (w *Window) Shutdown(ctx context.Context) error {
	w.CloseAdmission()
	done := make(chan struct{})
	go func() { w.workers.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		w.mu.Lock()
		var checkpointErrors []error
		for key, l := range w.leases {
			if l.active == 0 {
				continue
			}
			record, err := w.read(key)
			if err != nil {
				checkpointErrors = append(checkpointErrors, err)
				continue
			}
			for _, receipt := range record.Receipts {
				if receipt.State == "prepared" || receipt.State == "in_progress" {
					receipt.State = "unknown"
					receipt.Code = "unknown_outcome"
					receipt.Reason = "shutdown ended before settlement; no replay is permitted"
					checkpointErrors = append(checkpointErrors, w.write(record, receipt))
				}
			}
		}
		w.unlock()
		if len(checkpointErrors) == 0 && w.options.Lifecycle != nil {
			w.options.Lifecycle("receipts-persisted")
		}
		return errors.Join(append(checkpointErrors, ctx.Err())...)
	case <-done:
	}
	w.mu.Lock()
	var errs []error
	for key, l := range w.leases {
		errs = append(errs, l.poison, l.release())
		delete(w.leases, key)
	}
	w.unlock()
	err := errors.Join(errs...)
	if err == nil && w.options.Lifecycle != nil {
		w.options.Lifecycle("receipts-persisted")
	}
	return err
}

func (w *Window) Snapshot() Snapshot {
	w.mu.Lock()
	defer w.unlock()
	s := Snapshot{AdmissionClosed: w.closed}
	for _, l := range w.leases {
		if !l.revoked {
			s.Handles++
		}
		s.Workers += l.active
	}
	return s
}
