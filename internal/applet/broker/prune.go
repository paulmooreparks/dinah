package broker

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"studio/internal/records"
)

// Retention is how long a finished invocation's record is kept after its
// last change. A caller has an invocation's result at once, and a chat
// retries a request only until the request's deadline, which is at most a
// day. What a person may come back to later is an artifact, opened from
// Notifications or from a window left open, or a posted notification not
// yet dismissed, and a month allows for someone who is away. Notifications
// reports are kept a week because they only say what happened; an artifact
// is something made that may be wanted again.
const Retention = 30 * 24 * time.Hour

// Prune deletes the records of invocations that finished more than
// Retention before now, with the artifacts kept in them, and returns how
// many it deleted. It keeps every invocation still running or waiting, one
// another program holds, and one whose last save failed. It deletes only
// records in the state folder's applets folder.
func (b *Broker) Prune(now time.Time) (int, error) {
	ix, err := b.ready()
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-Retention)
	removed := 0
	var errs []error
	for _, s := range ix.snapshot() {
		if !TerminalStatus(s.Status) || !s.Updated.Before(cutoff) {
			continue
		}
		ok, err := b.prune(s, cutoff)
		if err != nil {
			errs = append(errs, err)
		}
		if ok {
			removed++
		}
	}
	if removed > 0 {
		b.emit("applet:invocation", nil)
	}
	return removed, errors.Join(errs...)
}

func (b *Broker) prune(s Summary, cutoff time.Time) (bool, error) {
	root := b.StoreRoot(s.Workspace)
	if !recordID.MatchString(s.ID) || filepath.Dir(root) != b.appletsDir() {
		return false, nil
	}
	defer b.lock(s.Workspace, s.ID)()
	key := b.key(s.Workspace, s.ID)
	if b.failure(key) != nil || b.running(key) != nil || b.holding(key) {
		return false, nil
	}
	store := b.store(s.Workspace)
	release, err := store.Hold(s.ID)
	if errors.Is(err, records.ErrHeld) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	v, err := b.Read(s.Workspace, s.ID)
	if err != nil {
		release()
		if !store.Exists(s.ID) {
			b.index().remove(b.indexKey(s.Workspace, s.ID))
		}
		return false, nil
	}
	if !TerminalStatus(v.Status) || !v.Updated.Before(cutoff) {
		release()
		return false, nil
	}
	err = store.Remove(s.ID)
	release()
	if err != nil {
		return false, err
	}
	// The hold file goes too, now that there is no record to hold. A
	// program that took it in the moment since the release would be acting
	// on an invocation that no longer exists, so it would start a new one.
	_ = os.Remove(store.HoldPath(s.ID))
	b.index().remove(b.indexKey(s.Workspace, s.ID))
	return true, nil
}
