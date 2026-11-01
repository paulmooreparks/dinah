package sessionhost

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"
)

// WakeRoute retains delivery ownership independently of continuation resolution.
type WakeRoute struct {
	Origin, Target, Recipient, TargetRecipient, Cwd string
	Provider, Title                                 string
	RoutingError                                    string
}

func (r WakeRoute) sameOwnership(other WakeRoute) bool {
	return r.Origin == other.Origin && r.Target == other.Target && r.Recipient == other.Recipient && r.TargetRecipient == other.TargetRecipient && r.Cwd == other.Cwd && r.Provider == other.Provider && r.RoutingError == other.RoutingError
}

func (r WakeRoute) needsRecovery() bool {
	return r.Origin == "" || r.Target == "" || r.Recipient == "" || r.TargetRecipient != r.Recipient || r.RoutingError != ""
}

// WakeFailure is also the write-ahead record for an opening whose result is not
// known after a crash. It never changes a bus delivery or recipient identity.
type WakeFailure struct {
	Route       WakeRoute  `json:"route"`
	Observed    *WakeRoute `json:"observed,omitempty"`
	Attempt     string     `json:"attempt"`
	Stage       string     `json:"stage"`
	FailureKind string     `json:"failureKind,omitempty"`
	Error       string     `json:"error"`
	Created     time.Time  `json:"created"`
}

type wakeAttempt struct {
	route   WakeRoute
	id      string
	started bool
	done    chan struct{}
	info    Info
	err     error
}

// WakeAttempts owns generation-bound page/fallback joins. Persistence runs
// before an open is admitted and before a failed attempt is reported complete.
// Callbacks must not re-enter this owner while its store callback is running.
type WakeAttempts struct {
	mu      sync.Mutex
	active  map[string]*wakeAttempt
	loaded  bool
	records map[string]WakeFailure
	loadErr error
	Read    func() (map[string]WakeFailure, error)
	Write   func(map[string]WakeFailure) error
	Now     func() time.Time
}

func (w *WakeAttempts) recordsLocked() (map[string]WakeFailure, error) {
	if !w.loaded {
		w.records, w.loadErr = w.Read()
		w.loaded = true
		if w.records == nil {
			w.records = map[string]WakeFailure{}
		}
	}
	return maps.Clone(w.records), w.loadErr
}

func (w *WakeAttempts) persistLocked(records map[string]WakeFailure) error {
	if err := w.Write(records); err != nil {
		return err
	}
	w.records = maps.Clone(records)
	return nil
}

func (w *WakeAttempts) Snapshot() (map[string]WakeFailure, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.recordsLocked()
}

// ReleaseCommitted retires only the held origin whose unchanged recipient has
// been committed to a prepared recovery by the host's binding authority.
func (w *WakeAttempts) ReleaseCommitted(origin, recipient string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	records, err := w.recordsLocked()
	if err != nil {
		return err
	}
	old, ok := records[origin]
	if !ok {
		return nil
	}
	if old.Route.Recipient != recipient {
		return errors.New("the held recipient differs from the committed recovery")
	}
	if a := w.active[origin]; a != nil {
		select {
		case <-a.done:
		default:
			return errors.New("the original opening is still active")
		}
	}
	delete(records, origin)
	return w.persistLocked(records)
}

func (w *WakeAttempts) RemoveRecipient(origin, recipient string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	records, err := w.recordsLocked()
	if err != nil {
		return err
	}
	for source, record := range records {
		if source != origin && (recipient == "" || record.Route.Recipient != recipient) {
			continue
		}
		if a := w.active[source]; a != nil {
			select {
			case <-a.done:
			default:
				return errors.New("this recipient still has an active opening")
			}
		}
		delete(records, source)
	}
	return w.persistLocked(records)
}

// Reload is explicit because a corrupt store must not be reread on every ring.
// An active attempt keeps its original write-ahead authority until completion.
func (w *WakeAttempts) Reload() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.active {
		select {
		case <-a.done:
		default:
			return errors.New("wait for the current chat opening to finish before reloading recovery state")
		}
	}
	w.loaded = false
	_, err := w.recordsLocked()
	return err
}

func (w *WakeAttempts) Begin(route WakeRoute, retry bool) (string, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if a := w.active[route.Origin]; a != nil {
		select {
		case <-a.done:
		default:
			return a.id, false, nil
		}
	}
	records, err := w.recordsLocked()
	if err != nil {
		return "", false, fmt.Errorf("read waiting-chat recovery state: %w", err)
	}
	if records == nil {
		records = map[string]WakeFailure{}
	}
	if old, ok := records[route.Origin]; ok && old.Route.Recipient != route.Recipient {
		old.Stage, old.Error = "routing-recovery", "The original waiting recipient changed. Review explicit routing recovery before retrying; its pending deliveries have not been reassigned."
		old.Observed = &route
		records[route.Origin] = old
		if err := w.persistLocked(records); err != nil {
			return "", false, err
		}
		return "", false, errors.New(old.Error)
	}
	if old, ok := records[route.Origin]; ok && old.Route.sameOwnership(route) && (!retry || route.needsRecovery()) {
		return "", false, fmt.Errorf("waiting delivery for %s remains held for %s (%s): %s", route.Origin, route.Target, old.Stage, old.Error)
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", false, err
	}
	id := hex.EncodeToString(token[:])
	record := WakeFailure{Route: route, Attempt: id, Stage: "opening", Error: "The previous opening has no confirmed completion. Retry or review recovery before opening it again.", Created: w.Now().UTC()}
	if route.needsRecovery() {
		record.Stage = "routing-recovery"
		record.Error = fmt.Sprintf("Messages belong to recipient %s, while continuation %s owns %s. Review routing recovery; all messages remain waiting.", route.Recipient, route.Target, route.TargetRecipient)
		if route.RoutingError != "" {
			record.Error = route.RoutingError + " All messages remain waiting with their original recipient."
		}
	}
	records[route.Origin] = record
	if err := w.persistLocked(records); err != nil {
		return "", false, fmt.Errorf("save waiting-chat recovery state: %w", err)
	}
	if route.needsRecovery() {
		return "", false, errors.New(record.Error)
	}
	if w.active == nil {
		w.active = map[string]*wakeAttempt{}
	}
	w.active[route.Origin] = &wakeAttempt{route: route, id: id, done: make(chan struct{})}
	return id, true, nil
}

func (w *WakeAttempts) ActiveTarget(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.active {
		if a.route.Target != id && a.route.Origin != id {
			continue
		}
		select {
		case <-a.done:
		default:
			return true
		}
	}
	return false
}

func (w *WakeAttempts) Current(target string) (string, string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var origin, token string
	for _, a := range w.active {
		if a.route.Target != target && a.route.Origin != target {
			continue
		}
		select {
		case <-a.done:
			continue
		default:
		}
		if token != "" {
			return "", "", false
		}
		origin, token = a.route.Origin, a.id
	}
	return origin, token, token != ""
}

func (w *WakeAttempts) DeliveryHeld(target string, recipients ...string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	records, err := w.recordsLocked()
	if err != nil {
		return true
	}
	for _, record := range records {
		for _, recipient := range recipients {
			if recipient != "" && record.Route.Recipient == recipient {
				return true
			}
		}
		if len(recipients) > 0 {
			continue
		}
		if record.Route.Target == target || record.Route.Origin == target {
			return true
		}
		if record.Observed != nil && record.Observed.Target == target {
			return true
		}
	}
	return false
}

// Open rejects retired generations before calling a provider. Concurrent callers
// for one attempt share its result, including failures and persistence errors.
func (w *WakeAttempts) Open(origin, id string, current func(WakeRoute) (WakeRoute, error), open func(WakeRoute) (Info, error)) (Info, error, bool) {
	w.mu.Lock()
	a := w.active[origin]
	if a == nil || a.id != id {
		w.mu.Unlock()
		return Info{}, errors.New("this waiting-chat opening was retired; review the current recovery state"), false
	}
	if a.started {
		w.mu.Unlock()
		<-a.done
		return a.info, a.err, false
	}
	a.started = true
	w.mu.Unlock()
	route, err := current(a.route)
	changed := err == nil && !a.route.sameOwnership(route)
	var info Info
	if changed {
		err = errors.New("the waiting chat's routing changed before opening; review it again")
	}
	if err == nil {
		info, err = open(route)
	}
	if err == nil && info.ID != a.route.Target {
		err = errors.New("the provider opening returned a different target; review routing recovery")
	}
	if err == nil {
		after, checkErr := current(a.route)
		if checkErr != nil {
			err = checkErr
		} else if !a.route.sameOwnership(after) {
			route, changed = after, true
			err = errors.New("the waiting chat's routing changed during opening; messages remain held")
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	records, saveErr := w.recordsLocked()
	if saveErr == nil {
		if record, ok := records[origin]; !ok || record.Attempt != id {
			saveErr = errors.New("the durable waiting-chat attempt changed before completion")
		} else {
			if err == nil {
				delete(records, origin)
			} else {
				record.Stage = "failed"
				if changed {
					record.Stage = "routing-recovery"
					record.Observed = &route
				}
				if notFound(err) {
					record.FailureKind = "resource-unavailable"
				}
				record.Error = err.Error()
				records[origin] = record
			}
			saveErr = w.persistLocked(records)
		}
	}
	if saveErr != nil {
		err = errors.Join(err, fmt.Errorf("save waiting-chat completion: %w", saveErr))
	}
	a.info, a.err = info, err
	close(a.done)
	return info, err, true
}
