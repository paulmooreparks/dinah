// Package helpercompat negotiates installed helpers independently of releases.
package helpercompat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"studio/internal/profile"
)

type Channel string

const (
	Bus              Channel = "bus"
	Steering         Channel = "steering"
	Applets          Channel = "applets"
	Documents        Channel = "documents"
	ExtensionControl Channel = "extension-control"
	ExtensionHost    Channel = "extension-host"
	SlideShow        Channel = "slide-show"
	OfferHeader              = "X-Dinah-Protocols"
	SelectedHeader           = "X-Dinah-Protocol"
	ChannelHeader            = "X-Dinah-Channel"
	EndpointPath             = "/helper-compatibility"
)

// Policy lists exchanges this side implements. Absence always denotes 1.
type Policy struct {
	Channel Channel
	Numbers []int
}

// These numbers advance separately, only with incompatible exchange changes.
const (
	BusProtocol              = 1
	SteeringProtocol         = 1
	AppletProtocol           = 1
	DocumentProtocol         = 1
	ExtensionControlProtocol = 1
	ExtensionHostProtocol    = 1
	SlideShowProtocol        = 1
)

func For(channel Channel) Policy {
	current := 0
	switch channel {
	case Bus:
		current = BusProtocol
	case Steering:
		current = SteeringProtocol
	case Applets:
		current = AppletProtocol
	case Documents:
		current = DocumentProtocol
	case ExtensionControl:
		current = ExtensionControlProtocol
	case ExtensionHost:
		current = ExtensionHostProtocol
	case SlideShow:
		current = SlideShowProtocol
	}
	return Policy{Channel: channel, Numbers: []int{current}}
}

func Known(channel Channel) bool {
	switch channel {
	case Bus, Steering, Applets, Documents, ExtensionControl, ExtensionHost, SlideShow:
		return true
	}
	return false
}

func parse(value string) ([]int, error) {
	if value == "" {
		return []int{1}, nil
	}
	if len(value) > 128 {
		return nil, errors.New("helper protocol offer is too long")
	}
	parts := strings.Split(value, ",")
	if len(parts) > 16 {
		return nil, errors.New("helper protocol offer has too many numbers")
	}
	out := []int{}
	seen := map[int]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 || part != strconv.Itoa(n) || seen[n] {
			return nil, errors.New("invalid helper protocol number; restart the window to start matching helpers")
		}
		seen[n] = true
		out = append(out, n)
	}
	return out, nil
}

func (p Policy) Offer() string {
	parts := []string{}
	for _, n := range p.Numbers {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ",")
}

// Refusal remains identifiable through callers that might otherwise restart a service.
type Refusal struct{ Message string }

func (e *Refusal) Error() string { return e.Message }

func IsRefusal(err error) bool {
	var refused *Refusal
	return errors.As(err, &refused)
}

func (p Policy) selectNumber(offer string, localIsWindow bool) (int, error) {
	peer, err := parse(offer)
	if err != nil {
		return 0, &Refusal{Message: err.Error()}
	}
	selected := 0
	localMax, peerMax := 0, 0
	for _, n := range p.Numbers {
		localMax = max(localMax, n)
		for _, candidate := range peer {
			if n == candidate {
				selected = max(selected, n)
			}
		}
	}
	if selected != 0 {
		return selected, nil
	}
	for _, n := range peer {
		peerMax = max(peerMax, n)
	}
	window, helper := peerMax, localMax
	if localIsWindow {
		window, helper = localMax, peerMax
	}
	newer := "The installed helper"
	if window > helper {
		newer = "The running window"
	}
	return 0, &Refusal{Message: fmt.Sprintf("%s uses a newer %s protocol (window %d, helper %d). Restart the window so it starts matching helpers. No operation was performed.", newer, p.Channel, window, helper)}
}

// Accept runs after authentication and before any operation or body decoding.
func (p Policy) Accept(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set(OfferHeader, p.Offer())
	n, err := p.selectNumber(r.Header.Get(OfferHeader), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
		return false
	}
	w.Header().Set(SelectedHeader, strconv.Itoa(n))
	return true
}

// Response checks the selected exchange, including an absent legacy header.
func (p Policy) Response(response *http.Response) (int, error) {
	n, err := p.selectNumber(response.Header.Get(OfferHeader), false)
	if err != nil {
		return 0, err
	}
	selected := response.Header.Get(SelectedHeader)
	if selected != "" {
		values, err := parse(selected)
		if err != nil || len(values) != 1 || values[0] != n {
			return 0, &Refusal{Message: "The window returned an invalid helper protocol selection. Restart the window so it starts matching helpers."}
		}
	} else if response.Header.Get(OfferHeader) != "" {
		return 0, &Refusal{Message: "The window did not select a helper protocol. Restart the window so it starts matching helpers."}
	}
	return n, nil
}

// Probe is read-only, even when an old server refuses OPTIONS. It never treats
// transport failure as a legacy peer and never changes a JSON body or descriptor.
func (p Policy) Probe(ctx context.Context, client *http.Client, endpoint string, headers http.Header) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodOptions, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set(OfferHeader, p.Offer())
	response, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	n, err := p.Response(response)
	if err != nil {
		return 0, err
	}
	switch response.StatusCode {
	case 200, 204, 400, 403, 404, 405:
		return n, nil
	}
	return 0, fmt.Errorf("helper compatibility probe returned HTTP %d", response.StatusCode)
}

func EndpointEnv() string { return profile.Current().EnvPrefix + "HELPER_ENDPOINT" }

// CheckEnvironment precedes SQLite work in helpers hosted by the window.
func CheckEnvironment(ctx context.Context, channel Channel) error {
	return For(channel).CheckEndpoint(ctx, os.Getenv(EndpointEnv()))
}

func (p Policy) CheckEndpoint(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		_, err := p.selectNumber("", false)
		return err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Port() == "" || u.Path != EndpointPath || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid window helper compatibility endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set(ChannelHeader, string(p.Channel))
	req.Header.Set(OfferHeader, p.Offer())
	client := &http.Client{Timeout: 500 * time.Millisecond, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("helper compatibility redirects are forbidden")
	}}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		if p.Channel == Bus || p.Channel == Steering {
			// These helpers also serve terminal sessions with only SQLite.
			// A missing or busy window must not take that service away.
			return nil
		}
		return fmt.Errorf("the running window could not be checked; restart the window before using helpers: %w", err)
	}
	defer response.Body.Close()
	if (p.Channel == Bus || p.Channel == Steering) && response.Header.Get(OfferHeader) == "" && response.Header.Get(SelectedHeader) == "" {
		return nil
	}
	if _, err := p.Response(response); err != nil {
		return err
	}
	if p.Channel == Bus || p.Channel == Steering {
		return nil
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("window helper compatibility returned HTTP %d", response.StatusCode)
	}
	return nil
}

// Channels returns a stable inventory for tests and the host's audit.
func Channels() []Channel {
	out := []Channel{Bus, Steering, Applets, Documents, ExtensionControl, ExtensionHost, SlideShow}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
