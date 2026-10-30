package judge

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// WhyPaused is why a call used the rule when the provider's judge is paused after repeated errors, so no request was sent.
const WhyPaused = "paused"

// A provider that errors repeatedly is left alone for a while, so that a judge that cannot work (an account out of
// allowance, a model that is gone) does not spend a request on every call. After pauseAfter errors in a row the provider
// is paused for pauseFirst, then for twice as long each time the one request let through at the end of a pause fails
// too, up to pauseLongest. A call that gets an answer, from the judge's side of the provider, ends the pause. The pause is
// kept in memory: a restart tries the provider afresh.
const (
	pauseAfter   = 3
	pauseFirst   = 10 * time.Minute
	pauseLongest = 4 * time.Hour
)

// Pause is a provider that is not being asked.
type Pause struct {
	Provider string    `json:"provider"`
	Until    time.Time `json:"until"`
	// Reason is the provider's own words for the last failure, shortened.
	Reason string `json:"reason"`
}

type providerHealth struct {
	errors  int
	delay   time.Duration
	until   time.Time
	probing bool
	reason  string
}

type health struct {
	mu sync.Mutex
	by map[string]*providerHealth
}

// admit says whether a call to the provider may go ahead now. While a pause lasts it says no, with the reason. When
// the pause has run out it lets one call through, the probe, and says no to the others until the probe has finished.
func (h *health) admit(provider string, now time.Time) (ok, probe bool, p Pause) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ph := h.by[provider]
	if ph == nil || ph.until.IsZero() {
		return true, false, Pause{}
	}
	p = Pause{Provider: provider, Until: ph.until, Reason: ph.reason}
	if now.Before(ph.until) || ph.probing {
		return false, false, p
	}
	ph.probing = true
	return true, true, p
}

// failed records an error from the provider, and returns the pause it starts, if any.
func (h *health) failed(provider, reason string, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.by == nil {
		h.by = map[string]*providerHealth{}
	}
	ph := h.by[provider]
	if ph == nil {
		ph = &providerHealth{}
		h.by[provider] = ph
	}
	ph.errors++
	ph.probing = false
	ph.reason = reason
	if ph.errors < pauseAfter {
		return
	}
	switch {
	case ph.delay == 0:
		ph.delay = pauseFirst
	default:
		ph.delay = min(ph.delay*2, pauseLongest)
	}
	ph.until = now.Add(ph.delay)
}

// answered ends any pause and forgets the errors: the provider's judge answered.
func (h *health) answered(provider string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.by, provider)
}

// released ends a probe that came to nothing either way (the call was late or its answer was not in form), so the next
// call may probe again.
func (h *health) released(provider string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ph := h.by[provider]; ph != nil {
		ph.probing = false
	}
}

// reset forgets every error and ends every pause.
func (h *health) reset() {
	h.mu.Lock()
	h.by = nil
	h.mu.Unlock()
}

// pauses are the providers that are paused at now.
func (h *health) pauses(now time.Time) []Pause {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []Pause
	for name, ph := range h.by {
		if !ph.until.IsZero() && now.Before(ph.until) {
			out = append(out, Pause{Provider: name, Until: ph.until, Reason: ph.reason})
		}
	}
	return out
}

var refPattern = regexp.MustCompile(`\s*\(ref: [^)]*\)`)

// ShortReason is a provider's failure as a person reads it: its own words without a request reference, on one line, at
// most 160 characters.
func ShortReason(text string) string {
	text = refPattern.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > 160 {
		r := []rune(text)
		text = string(r[:159]) + "…"
	}
	return text
}

// Pauses are the providers whose judge is paused now, with why.
func (j *Judges) Pauses() []Pause { return j.health.pauses(j.now()) }
