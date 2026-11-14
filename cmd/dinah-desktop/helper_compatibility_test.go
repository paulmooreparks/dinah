package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/helpercompat"
)

func TestWindowHelperCompatibilityEndpoint(t *testing.T) {
	s := &shellServer{addr: "127.0.0.1:47616"}
	for _, channel := range []helpercompat.Channel{helpercompat.Bus, helpercompat.Steering, helpercompat.SlideShow} {
		for _, offer := range []string{"1", "2", ""} {
			r := httptest.NewRequest(http.MethodGet, "http://"+s.addr+helpercompat.EndpointPath, nil)
			r.Header.Set(helpercompat.ChannelHeader, string(channel))
			r.Header.Set(helpercompat.OfferHeader, offer)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			want := 204
			if offer == "2" {
				want = 412
			}
			if w.Code != want || w.Header().Get(helpercompat.OfferHeader) != "1" {
				t.Fatalf("%s %q: %d %s", channel, offer, w.Code, w.Body.String())
			}
		}
	}
	r := httptest.NewRequest(http.MethodGet, "http://"+s.addr+helpercompat.EndpointPath, nil)
	r.Header.Set(helpercompat.ChannelHeader, string(helpercompat.Bus))
	r.Header.Set("Origin", "https://other.invalid")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("browser reached helper endpoint")
	}
}

func TestBusHelperRefusesBeforeOpeningSQLite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		helpercompat.Policy{Channel: helpercompat.Bus, Numbers: []int{2}}.Accept(w, r)
	}))
	defer server.Close()
	t.Setenv(helpercompat.EndpointEnv(), server.URL+helpercompat.EndpointPath)
	dir := t.TempDir()
	err := runBusMCP([]string{"--state-dir", dir, "--session", "fixture"})
	if !helpercompat.IsRefusal(err) {
		t.Fatalf("expected compatibility refusal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bus.db")); !os.IsNotExist(err) {
		t.Fatal("incompatible helper opened the bus")
	}
}

func TestAppletCompatibilityChecksBeforeBodyOrDispatch(t *testing.T) {
	a := withTestApplets(t, NewApp())
	c, client := testAppletBridge(t, a, t.TempDir(), "chat")
	for _, offer := range []string{"1", "2", ""} {
		a.appletBridge.mu.Lock()
		c.connected = false
		a.appletBridge.mu.Unlock()
		req, _ := http.NewRequest(http.MethodPost, client.endpoint, strings.NewReader(`{"operation":"discover"}`))
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(helpercompat.OfferHeader, offer)
		res, err := client.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 200
		if offer == "2" {
			want = 412
		}
		if res.StatusCode != want {
			t.Fatalf("offer %q: %d", offer, res.StatusCode)
		}
		a.appletBridge.mu.Lock()
		connected := c.connected
		a.appletBridge.mu.Unlock()
		if connected != (offer != "2") {
			t.Fatalf("offer %q changed connection state incorrectly", offer)
		}
	}
	future := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Error("incompatible helper sent operation")
		}
		helpercompat.Policy{Channel: helpercompat.Applets, Numbers: []int{2}}.Accept(w, r)
	}))
	defer future.Close()
	other, err := newAppletBridgeClient(future.URL+"/rpc", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.call(context.Background(), appletBridgeRequest{Operation: "invoke"}); !helpercompat.IsRefusal(err) {
		t.Fatalf("future applet host: %v", err)
	}
}

func TestShowHelperRefusesBeforeCreatingAWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		helpercompat.Policy{Channel: helpercompat.SlideShow, Numbers: []int{2}}.Accept(w, r)
	}))
	defer server.Close()
	t.Setenv(helpercompat.EndpointEnv(), server.URL+helpercompat.EndpointPath)
	// Missing entry/place makes reaching the legacy path return 2 rather
	// than creating any UI, even if this regression ever fails.
	t.Setenv(showEntryVar(), "")
	t.Setenv(showPlaceVar(), "")
	if status := runSlideShow(); status != 1 {
		t.Fatalf("show did not refuse incompatible parent: %d", status)
	}
}
