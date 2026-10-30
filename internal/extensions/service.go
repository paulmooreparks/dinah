package extensions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

// Service owns host orchestration and pending reviews and terminal tickets.
// Manager owns extension processes and closes them with its existing semantics.
type Service struct {
	*Manager
	mu                         sync.Mutex
	extensionReviews           map[string]*extensionReview
	reviewed                   int
	preparedExtensionTerminals map[string]preparedExtensionTerminal
	emit                       func(string, any)
}

// NewService wires review and terminal hooks without changing host permissions.
// emit and hooks.Changed are required and run outside the pending-state lock.
func NewService(root string, hooks Hooks, emit func(string, any)) *Service {
	s := &Service{emit: emit}
	hooks.Review = s.reviewExtensionCommand
	hooks.Terminal = func(ctx context.Context, input extension.TerminalInput) error {
		s.prepareExtensionTerminal(input)
		return nil
	}
	s.Manager = New(root, hooks)
	return s
}

type extensionReview struct {
	// id is the extension that asks, and key the review's own name. They
	// and input are what the extension:review event carries, kept so that a
	// page that missed the event can read them (PendingReviews).
	id, key  string
	input    extension.RunInput
	answer   chan extensionReviewAnswer
	order    int
	answered bool
}

// Review is a command review waiting for the person, as the
// extension:review event carries it.
type Review struct {
	Key       string             `json:"key"`
	Extension string             `json:"extension"`
	Input     extension.RunInput `json:"input"`
}

// PendingReviews returns the reviews not yet answered, oldest first. A page
// that missed an extension:review event reads them here.
func (a *Service) PendingReviews() []Review {
	a.mu.Lock()
	defer a.mu.Unlock()
	open := make([]*extensionReview, 0, len(a.extensionReviews))
	for _, r := range a.extensionReviews {
		if !r.answered {
			open = append(open, r)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].order < open[j].order })
	out := make([]Review, 0, len(open))
	for _, r := range open {
		out = append(out, Review{Key: r.key, Extension: r.id, Input: r.input})
	}
	return out
}

type extensionReviewAnswer struct {
	decision string
	args     []string
}

func (a *Service) SetCommandReview(id string, enabled bool) error {
	if err := a.Manager.Store.Review(id, enabled); err != nil {
		return err
	}
	a.emit("extensions:changed", map[string]bool{"changed": true})
	return nil
}
func (a *Service) reviewExtensionCommand(ctx context.Context, id string, input extension.RunInput) ([]string, error) {
	key := "review-" + rand.Text()
	request := &extensionReview{id: id, key: key, input: input, answer: make(chan extensionReviewAnswer, 1)}
	a.mu.Lock()
	if a.extensionReviews == nil {
		a.extensionReviews = map[string]*extensionReview{}
	}
	a.reviewed++
	request.order = a.reviewed
	a.extensionReviews[key] = request
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.extensionReviews, key)
		a.mu.Unlock()
		a.emit("extension:review-ended", key)
	}()
	a.emit("extension:review", map[string]any{"key": key, "extension": id, "input": input})
	select {
	case answer := <-request.answer:
		if answer.decision == "run" {
			return answer.args, nil
		}
		if answer.decision == "terminal" {
			a.prepareExtensionTerminal(extension.TerminalInput{Program: input.Program, Args: answer.args, Cwd: input.Cwd})
		}
		return nil, errors.New("command was not run; the action stopped at the person's request")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (a *Service) AnswerCommand(key, decision, arguments string) error {
	if decision != "run" && decision != "cancel" && decision != "terminal" {
		return errors.New("unknown command review decision")
	}
	var args []string
	if len(arguments) > 128<<10 || json.Unmarshal([]byte(arguments), &args) != nil || len(args) > 256 {
		return errors.New("enter a JSON array of at most 256 arguments")
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return errors.New("arguments cannot contain NUL")
		}
	}
	a.mu.Lock()
	request := a.extensionReviews[key]
	a.mu.Unlock()
	if request == nil {
		return errors.New("this command review has ended")
	}
	// A review round trip displays replacement characters for invalid UTF-8.
	// Preserve each original byte argument unless the person edited its display.
	for i := range args {
		if i < len(request.input.Args) {
			encoded, _ := json.Marshal(request.input.Args[i])
			var displayed string
			json.Unmarshal(encoded, &displayed)
			if args[i] == displayed {
				args[i] = request.input.Args[i]
			}
		}
	}
	if decision == "terminal" && request.input.Stdin != "" {
		return errors.New("this command uses standard input; run or cancel it here to preserve the patch")
	}
	select {
	case request.answer <- extensionReviewAnswer{decision, args}:
		a.mu.Lock()
		request.answered = true
		a.mu.Unlock()
		return nil
	default:
		return errors.New("this command review was already answered")
	}
}
func (a *Service) LibraryRevision() string {
	list, err := a.Manager.Store.List()
	if err != nil {
		return "invalid:" + err.Error()
	}
	rows := []any{}
	for _, v := range list {
		rows = append(rows, []any{v.Manifest.ID, v.Manifest.Signature, v.Enabled})
	}
	data, _ := json.Marshal(rows)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (a *Service) Preview(source string) (extension.Preview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	folder, cleanup, err := PrepareSource(ctx, source)
	if err != nil {
		return extension.Preview{}, err
	}
	defer cleanup()
	return a.Manager.Store.Inspect(folder)
}
func (a *Service) Install(source string, approval extension.Approval) (extension.Installation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	folder, cleanup, err := PrepareSource(ctx, source)
	if err != nil {
		return extension.Installation{}, err
	}
	defer cleanup()
	manager := a.Manager
	preview, err := manager.Store.Inspect(folder)
	if err != nil {
		return extension.Installation{}, err
	}
	lease := rand.Text()
	manager.Pause(preview.Manifest.ID, lease)
	defer manager.Resume(preview.Manifest.ID, lease)
	installed, err := manager.Store.Install(folder, source, approval)
	if err != nil {
		return installed, err
	}
	a.Hooks.Changed()
	return installed, nil
}
func (a *Service) Enable(id string, enabled bool) error {
	manager := a.Manager
	lease := rand.Text()
	manager.Pause(id, lease)
	defer manager.Resume(id, lease)
	if err := manager.Store.Enable(id, enabled); err != nil {
		return err
	}
	a.Hooks.Changed()
	return nil
}
func (a *Service) Remove(id string) error {
	manager := a.Manager
	lease := rand.Text()
	manager.Pause(id, lease)
	defer manager.Resume(id, lease)
	if err := manager.Store.Remove(id); err != nil {
		return err
	}
	a.Hooks.Changed()
	return nil
}

type preparedExtensionTerminal struct {
	Input   extension.TerminalInput
	Created time.Time
}

func (a *Service) prepareExtensionTerminal(input extension.TerminalInput) {
	ticket := "prepared-" + rand.Text()
	a.mu.Lock()
	if a.preparedExtensionTerminals == nil {
		a.preparedExtensionTerminals = map[string]preparedExtensionTerminal{}
	}
	for id, entry := range a.preparedExtensionTerminals {
		if time.Since(entry.Created) > 15*time.Minute {
			delete(a.preparedExtensionTerminals, id)
		}
	}
	if len(a.preparedExtensionTerminals) >= 64 {
		oldest := ""
		var created time.Time
		for id, entry := range a.preparedExtensionTerminals {
			if oldest == "" || entry.Created.Before(created) {
				oldest = id
				created = entry.Created
			}
		}
		delete(a.preparedExtensionTerminals, oldest)
	}
	a.preparedExtensionTerminals[ticket] = preparedExtensionTerminal{Input: input, Created: time.Now()}
	a.mu.Unlock()
	a.emit("extension:terminal", map[string]any{"ticket": ticket, "command": input})
}
func (a *Service) TakeTerminal(ticket string) (extension.TerminalInput, error) {
	a.mu.Lock()
	prepared, ok := a.preparedExtensionTerminals[ticket]
	delete(a.preparedExtensionTerminals, ticket)
	a.mu.Unlock()
	if !ok || time.Since(prepared.Created) > 15*time.Minute {
		return extension.TerminalInput{}, errors.New("this prepared command was already run or expired")
	}
	return prepared.Input, nil
}
