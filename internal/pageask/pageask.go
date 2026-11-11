// Package pageask lets an applet's action, written outside the window
// program, put a question to the page and wait for its answer
// (docs/design/product-applets.md). The window program has always asked its
// page things on behalf of its own applets, such as which windows are open
// or what a document's text is. A product built from this source brings
// applets of its own in packages of its own, and an action of one that
// needs something only the page can make, such as a picture of what a
// window draws, asks through here.
//
// The window program gives its way of asking with Set when it starts. The
// page answers a kind of question that one of its files registered for
// (dinahWorkbench.answerAsks), and the answer comes back as the JSON text
// the page gave. Nothing in this package knows a question: the product's Go
// code and its page file agree on the kind and on what is asked and
// answered.
package pageask

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Asker puts a question of a kind to the page, with its arguments, and
// waits up to within for the answer, which is JSON text.
type Asker func(ctx context.Context, kind string, args any, within time.Duration) (string, error)

var (
	mu    sync.RWMutex
	asker Asker
)

// ErrNoPage means no window program has given a way to ask, as in a program
// with no page, or a test.
var ErrNoPage = errors.New("there is no window to ask")

// Set gives the way to ask the page. The window program calls it once.
func Set(a Asker) {
	mu.Lock()
	defer mu.Unlock()
	asker = a
}

// Ask puts a question of a kind to the page and returns its answer.
func Ask(ctx context.Context, kind string, args any, within time.Duration) (string, error) {
	mu.RLock()
	a := asker
	mu.RUnlock()
	if a == nil {
		return "", ErrNoPage
	}
	return a(ctx, kind, args, within)
}
