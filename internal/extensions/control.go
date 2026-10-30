package extensions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type controlAddress struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}
type controlRequest struct {
	ID        string `json:"id"`
	Lease     string `json:"lease"`
	Operation string `json:"operation"`
}

var controlID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`)

// Pause marks the installation before waiting on backend shutdown. New calls
// cannot start another process while a package mutator waits for file locks.
func (m *Manager) Pause(id, lease string) {
	m.mu.Lock()
	if m.pauses[id] == nil {
		m.pauses[id] = map[string]bool{}
	}
	m.pauses[id][lease] = true
	cancels := []context.CancelFunc{}
	for _, s := range m.scopes {
		if s.extensionID == id && s.cancel != nil {
			cancels = append(cancels, s.cancel)
		}
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	m.Stop(id)
}
func (m *Manager) Resume(id, lease string) {
	m.mu.Lock()
	delete(m.pauses[id], lease)
	if len(m.pauses[id]) == 0 {
		delete(m.pauses, id)
	}
	m.mu.Unlock()
	if m.Hooks.Changed != nil {
		m.Hooks.Changed()
	}
}
func (m *Manager) publishControl() error {
	dir := filepath.Join(m.Store.Root, ".hosts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	m.controlToken = rand.Text()
	m.controlFile = filepath.Join(dir, fmt.Sprintf("%d-%s.json", os.Getpid(), rand.Text()))
	data, _ := json.Marshal(controlAddress{URL: "http://" + m.listener.Addr().String() + "/control", Token: m.controlToken, PID: os.Getpid()})
	f, err := os.CreateTemp(dir, ".host-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	closed := f.Close()
	if err != nil {
		return err
	}
	if closed != nil {
		return closed
	}
	return os.Rename(name, m.controlFile)
}
func (m *Manager) controlHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "POST" || r.Host != m.listener.Addr().String() || r.Header.Get("Origin") != "" || r.Header.Get("Authorization") != "Bearer "+m.controlToken {
		http.Error(w, "control request denied", 403)
		return
	}
	var request controlRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || !controlID.MatchString(request.ID) || len(request.ID) > 200 || len(request.Lease) < 16 || len(request.Lease) > 128 {
		http.Error(w, "invalid control request", 400)
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		http.Error(w, "trailing control request data", 400)
		return
	}
	if request.Operation == "pause" {
		m.Pause(request.ID, request.Lease)
	} else if request.Operation == "resume" {
		m.Resume(request.ID, request.Lease)
	} else {
		http.Error(w, "unknown control operation", 400)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WithPausedHosts coordinates CLI package mutations with running desktop hosts.
// The caller can read the private descriptor, so it already has local account
// authority. A separate token and Origin rejection exclude browser callers.
func WithPausedHosts(ctx context.Context, root, id string, mutate func() error) (result error) {
	files, err := os.ReadDir(filepath.Join(root, ".hosts"))
	if os.IsNotExist(err) {
		return mutate()
	}
	if err != nil {
		return err
	}
	lease := rand.Text()
	hosts := []controlAddress{}
	client := &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("control redirects are forbidden") }}
	send := func(host controlAddress, operation string) error {
		data, _ := json.Marshal(controlRequest{ID: id, Lease: lease, Operation: operation})
		callCtx := ctx
		if operation == "resume" {
			callCtx = context.WithoutCancel(ctx)
		}
		request, err := http.NewRequestWithContext(callCtx, "POST", host.URL, bytes.NewReader(data))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+host.Token)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 204 {
			return fmt.Errorf("extension host control returned %d", response.StatusCode)
		}
		return nil
	}
	defer func() {
		for _, host := range hosts {
			if err := send(host, "resume"); err != nil {
				result = errors.Join(result, fmt.Errorf("extension host resume failed: %w", err))
			}
		}
	}()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, ".hosts", file.Name()))
		if err != nil {
			return err
		}
		if len(data) > 4096 {
			return errors.New("extension host descriptor exceeds limit")
		}
		var host controlAddress
		if json.Unmarshal(data, &host) != nil {
			return errors.New("invalid extension host descriptor")
		}
		u, err := url.Parse(host.URL)
		if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/control" || u.User != nil || len(host.Token) < 16 || host.PID < 1 {
			return errors.New("invalid extension host control address")
		}
		alive, err := controlProcessAlive(host.PID)
		if err != nil {
			return err
		}
		if !alive {
			continue
		}
		hosts = append(hosts, host)
		if err = send(host, "pause"); err != nil {
			return fmt.Errorf("could not pause a running extension host; package was left unchanged: %w", err)
		}
	}
	return mutate()
}
