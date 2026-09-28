package durable

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// An act is the stretch during which one goroutine holds entity locks, or
// runs inside a scope BeginAct opened. Whether an operation may give up
// depends on whether its own act has written, and never on another act's, so
// the registry records which act every entry belongs to.
//
// Go gives a goroutine no identity of its own, so an act is tied to the
// operating-system thread its goroutine is pinned to. Every entry pins the
// goroutine that inserts it with runtime.LockOSThread, whose documentation
// says the goroutine "will always execute in that thread, and no other
// goroutine will execute in it" until it has unpinned as often as it pinned.
// While an act has an entry, therefore, its thread's identifier names that
// goroutine and nothing else, and an operation compares the thread it runs on
// against the entries' threads. A request in serve or ui, the terminal UI's
// reads off its event loop, and any other goroutine that holds no lock of its
// own find no act and may give up, whatever another goroutine's act has
// written.
//
// An entry is removed on the goroutine that inserted it. One removed
// elsewhere still leaves the registry, but the inserting goroutine stays
// pinned until it ends, which the runtime documents as terminating the thread.
type act struct {
	// wrote is set by the first mutating operation of the act that succeeds.
	wrote atomic.Bool
}

// binding is one thread's membership of an act, counted so that nested
// entries on one goroutine share one act and the act ends with the last of
// them.
type binding struct {
	act   *act
	count int
}

// Hold is an entry of the in-process registry of held locks. A lock's holder
// inserts one before it creates its lock file and removes it when it lets the
// lock go, and a judge or a reclaimer inserts one before it opens the file, so
// two goroutines of one process never hold, judge or reclaim one lock at once.
type Hold struct {
	// Dir is the directory the lock protects, which is the lock file's own
	// directory.
	Dir string

	key    string
	owner  any
	thread uint64
	act    *act
	bound  bool
}

// Wrote reports whether the act this entry belongs to has written. Until it
// has, an operation of that act may give up; after it, an operation of that
// act waits until it succeeds.
func (h *Hold) Wrote() bool {
	return h.act != nil && h.act.wrote.Load()
}

// Owner answers what inserted the entry, or what it was handed to since.
func (h *Hold) Owner() any {
	registry.Lock()
	defer registry.Unlock()
	return h.owner
}

// Hand makes owner the entry's owner, which is how a judge's entry becomes
// the reclaiming lock's own.
func (h *Hold) Hand(owner any) {
	registry.Lock()
	defer registry.Unlock()
	h.owner = owner
}

// Unregister removes the entry, but only while it is still the one standing
// under its path, so a failed acquisition can never remove the entry of the
// acquisition that beat it. The entry leaves its act either way, once.
func (h *Hold) Unregister() {
	if h == nil {
		return
	}
	registry.Lock()
	if registry.entries[h.key] == h {
		delete(registry.entries, h.key)
	}
	bound := h.bound
	h.bound = false
	registry.Unlock()
	if bound {
		leave(h.thread)
	}
}

// registry is the process-wide map from a lock file's path, compared under
// pathKey, to the entry that owns it, and from a thread to the act the
// goroutine pinned to it belongs to.
var registry = struct {
	sync.Mutex
	entries map[string]*Hold
	threads map[uint64]*binding
}{entries: map[string]*Hold{}, threads: map[uint64]*binding{}}

// join pins the calling goroutine to its thread and makes the thread a member
// of the act it already belongs to, or of into when it belongs to none, or of
// a new act when into is nil too. It answers the thread and the act.
func join(into *act) (uint64, *act) {
	thread, known := currentThread()
	if known {
		runtime.LockOSThread()
		// The identifier read before the pin may name a thread the
		// goroutine has since left; read it again now that it cannot move.
		thread, _ = currentThread()
	}
	registry.Lock()
	defer registry.Unlock()
	b := registry.threads[thread]
	if b == nil {
		if into == nil {
			into = &act{}
		}
		b = &binding{act: into}
		registry.threads[thread] = b
	}
	b.count++
	return thread, b.act
}

// leave ends one membership join began on thread, and unpins the calling
// goroutine when it is the one pinned there.
func leave(thread uint64) {
	registry.Lock()
	if b := registry.threads[thread]; b != nil {
		b.count--
		if b.count <= 0 {
			delete(registry.threads, thread)
		}
	}
	registry.Unlock()
	if current, known := currentThread(); known && current == thread {
		runtime.UnlockOSThread()
	}
}

// Register inserts an entry for the lock file at path, owned by owner, and
// makes it part of the calling goroutine's act. When an entry already stands
// under that path it inserts nothing and answers a nil entry together with
// the owner of the one that stands.
func Register(path string, owner any) (*Hold, any) {
	key := pathKey(path)
	registry.Lock()
	if present, ok := registry.entries[key]; ok {
		registry.Unlock()
		return nil, present.owner
	}
	hold := &Hold{Dir: filepath.Dir(filepath.Clean(path)), key: key, owner: owner}
	registry.entries[key] = hold
	registry.Unlock()
	thread, a := join(nil)
	registry.Lock()
	hold.thread, hold.act, hold.bound = thread, a, true
	registry.Unlock()
	return hold, nil
}

// Registered answers the owner of the entry standing under path, and whether
// one stands.
func Registered(path string) (any, bool) {
	registry.Lock()
	defer registry.Unlock()
	hold, ok := registry.entries[pathKey(path)]
	if !ok {
		return nil, false
	}
	return hold.owner, true
}

// Act is a scope that makes everything the calling goroutine does until End
// one act, whatever locks it takes and gives back inside it. A verb whose
// writes span several lock acquisitions opens one, so that once its first
// write has landed, no later operation of it gives up and leaves the verb half
// done.
type Act struct {
	thread uint64
	ended  atomic.Bool
}

// BeginAct opens an act scope on the calling goroutine. Nested scopes and the
// entries of locks taken inside them all belong to one act.
func BeginAct() *Act {
	thread, _ := join(nil)
	return &Act{thread: thread}
}

// End closes the scope. It must be called on the goroutine that opened it,
// and a second call does nothing.
func (a *Act) End() {
	if a == nil || !a.ended.CompareAndSwap(false, true) {
		return
	}
	leave(a.thread)
}

// ActRef names the act a goroutine belongs to, so that work it hands to other
// goroutines can join it.
type ActRef struct {
	act *act
}

// CurrentAct answers the act the calling goroutine belongs to, or nil when it
// belongs to none.
func CurrentAct() *ActRef {
	if a := classify(false).act; a != nil {
		return &ActRef{act: a}
	}
	return nil
}

// Enter makes the calling goroutine part of the act ref names until the
// answer is called, which must happen on the same goroutine. A nil ref enters
// nothing.
func (r *ActRef) Enter() (exit func()) {
	if r == nil {
		return func() {}
	}
	thread, _ := join(r.act)
	var once sync.Once
	return func() { once.Do(func() { leave(thread) }) }
}

// pathKey is the form a path is compared in: cleaned, and folded to lower
// case on the two platforms whose file systems compare names without regard
// to case by default. Folding where a volume is case-sensitive only makes two
// distinct names collide, which refuses an acquisition and never grants one.
func pathKey(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

// class is where an operation falls in its act: whether it may give up once
// RetryBudget has passed, and the act whose record of a write it sets on
// success.
type class struct {
	act      *act
	mayGive  bool
	mutating bool
}

// classify reads the calling goroutine's act. An operation outside any act,
// or in an act that has not yet written, may give up; one in an act that has
// written may not. The goroutine is pinned while it reads its thread, so the
// identifier it compares cannot name a thread another goroutine has since
// been pinned to.
func classify(mutating bool) class {
	var a *act
	thread, known := currentThread()
	if known {
		runtime.LockOSThread()
		thread, _ = currentThread()
	}
	registry.Lock()
	if b := registry.threads[thread]; b != nil {
		a = b.act
	}
	registry.Unlock()
	if known {
		runtime.UnlockOSThread()
	}
	return class{act: a, mayGive: a == nil || !a.wrote.Load(), mutating: mutating}
}

// alwaysGiveUp is the class of an operation that gives up at RetryBudget
// wherever it falls and never touches an act: a directory move, a tree
// removal, a lock creation, and the removal of a lock file on release.
var alwaysGiveUp = class{mayGive: true}

// done records a successful operation of this class on its act.
func (c class) done() {
	if c.mutating && c.act != nil {
		c.act.wrote.Store(true)
	}
}
