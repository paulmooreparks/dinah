package helpercompat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEveryChannelNegotiatesBeforeDispatch(t *testing.T) {
	for _, channel := range Channels() {
		t.Run(string(channel), func(t *testing.T) {
			for _, tc := range []struct {
				name                                   string
				window, helper                         []int
				legacyRequest, legacyResponse, refused bool
			}{
				{name: "equal", window: []int{1}, helper: []int{1}},
				{name: "newer-helper", window: []int{1}, helper: []int{2}, refused: true},
				{name: "older-helper", window: []int{2}, helper: []int{1}, refused: true},
				{name: "supported-older-helper", window: []int{2, 1}, helper: []int{1}},
				{name: "supported-older-window", window: []int{1}, helper: []int{2, 1}},
				{name: "absent-request", window: []int{1}, helper: []int{1}, legacyRequest: true},
				{name: "absent-response", window: []int{1}, helper: []int{1}, legacyResponse: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					window := Policy{Channel: channel, Numbers: tc.window}
					helper := Policy{Channel: channel, Numbers: tc.helper}
					dispatched := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if tc.legacyResponse && r.Method == http.MethodOptions {
							http.Error(w, "legacy server accepts POST only", http.StatusForbidden)
							return
						}
						if !tc.legacyResponse && !window.Accept(w, r) {
							return
						}
						if r.Method == http.MethodOptions {
							w.WriteHeader(204)
							return
						}
						dispatched++
						w.WriteHeader(200)
					}))
					defer server.Close()
					selected, err := helper.Probe(context.Background(), server.Client(), server.URL, nil)
					if tc.refused {
						if !IsRefusal(err) || !strings.Contains(err.Error(), "Restart the window") || dispatched != 0 {
							t.Fatalf("refusal: %v, dispatches %d", err, dispatched)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
					if !tc.legacyRequest {
						req.Header.Set(OfferHeader, Policy{Numbers: []int{selected}}.Offer())
					}
					res, err := server.Client().Do(req)
					if err != nil {
						t.Fatal(err)
					}
					defer res.Body.Close()
					if _, err := helper.Response(res); err != nil || res.StatusCode != 200 || dispatched != 1 {
						t.Fatalf("response: %v, status %d, dispatches %d", err, res.StatusCode, dispatched)
					}
				})
			}
		})
	}
}

func TestMalformedOffersNeverBecomeLegacy(t *testing.T) {
	for _, offer := range []string{"0", "-1", "01", "1,1", "1,", "garbage", strings.Repeat("1", 129)} {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/", nil)
		r.Header.Set(OfferHeader, offer)
		w := httptest.NewRecorder()
		if For(Applets).Accept(w, r) || w.Code != 412 {
			t.Fatalf("accepted %q", offer)
		}
	}
}

func TestDocumentRefusalNamesTheIndependentService(t *testing.T) {
	policy := For(Documents)
	_, err := policy.selectNumber("2", false)
	if !IsRefusal(err) || !strings.Contains(err.Error(), "The document service uses a newer") || !strings.Contains(err.Error(), "save and close the document windows") {
		t.Fatalf("document recovery instruction: %v", err)
	}
}

func TestTerminalHelpersSurviveAMissingOrUnresponsiveWindow(t *testing.T) {
	for _, channel := range []Channel{Bus, Steering} {
		t.Run(string(channel), func(t *testing.T) {
			policy := For(channel)
			if err := policy.CheckEndpoint(context.Background(), ""); err != nil {
				t.Fatal(err)
			}
			closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			address := closed.URL + EndpointPath
			closed.Close()
			if err := policy.CheckEndpoint(context.Background(), address); err != nil {
				t.Fatal(err)
			}
			slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
			defer slow.Close()
			if err := policy.CheckEndpoint(context.Background(), slow.URL+EndpointPath); err != nil {
				t.Fatal(err)
			}
			incompatible := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Policy{Channel: channel, Numbers: []int{2}}.Accept(w, r) }))
			defer incompatible.Close()
			if err := policy.CheckEndpoint(context.Background(), incompatible.URL+EndpointPath); !IsRefusal(err) {
				t.Fatalf("incompatible answered window was accepted: %v", err)
			}
		})
	}
}
