package httphead

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestAdmissionRefusesWhatAnotherSiteCouldSend is dinah-152/criteria/3. It
// drives the admission steps over the form path itself, as a plain act (the
// claim) and as the tunnel (a block), and holds each refusal against the
// card's journal, which must not gain a line. The accepting cases stand
// beside the refusing ones, so a head that refused every form would fail.
func TestAdmissionRefusesWhatAnotherSiteCouldSend(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	var seen []reply
	record := func(a reply) reply {
		seen = append(seen, a)
		return a
	}

	t.Run("a Host naming anything but this server answers 421", func(t *testing.T) {
		for _, host := range []string{"evil.example:" + strconv.Itoa(f.port), "127.0.0.1:1", "evil.example"} {
			got := record(f.get("/cards/"+card, "Host", host))
			if got.status != http.StatusMisdirectedRequest || got.refusal(t) != contract.ForeignHost {
				t.Errorf("Host %s: wanted 421 %s, got %d %s", host, contract.ForeignHost, got.status, got.body)
			}
		}
		for _, host := range []string{"localhost:" + strconv.Itoa(f.port), "LocalHost:" + strconv.Itoa(f.port), "127.0.0.1:" + strconv.Itoa(f.port)} {
			if got := record(f.get("/cards/"+card, "Host", host)); got.status != http.StatusOK {
				t.Errorf("Host %s: wanted 200, got %d %s", host, got.status, got.body)
			}
		}
	})

	shapes := []struct {
		name    string
		path    string
		body    string
		success int
		undo    func()
	}{
		{
			name: "the claim", path: "/cards/" + card + "/claim", body: "expires=1h", success: http.StatusCreated,
			undo: func() { f.act(&verb.Request{Verb: verb.Release, Actor: "alka", Card: card}) },
		},
		{
			name: "the tunnelled block", path: "/cards/" + card,
			body:    "_method=PATCH&_type=" + strings.ReplaceAll(typeBlock, "+", "%2B") + "&_basis=*&reason=An+obstacle",
			success: http.StatusOK,
			undo:    func() { f.act(&verb.Request{Verb: verb.Unblock, Actor: "alka", Card: card}) },
		},
	}
	otherPort := "http://127.0.0.1:" + strconv.Itoa(f.port+1)
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			refusals := []struct {
				header []string
				want   string
			}{
				{[]string{"Origin", "http://evil.example"}, contract.ForeignOrigin},
				{[]string{"Origin", otherPort}, contract.ForeignOrigin},
				{[]string{"Origin", "null"}, contract.ForeignOrigin},
				{[]string{"Origin", "-"}, contract.OriginRequired},
				{[]string{"Sec-Fetch-Site", "cross-site"}, contract.ForeignOrigin},
				{[]string{"Sec-Fetch-Site", "same-site"}, contract.ForeignOrigin},
			}
			for _, refusal := range refusals {
				before := len(f.journal(card))
				got := record(f.form(shape.path, shape.body, refusal.header...))
				if got.status != http.StatusForbidden || got.refusal(t) != refusal.want {
					t.Errorf("%v: wanted 403 %s, got %d %s", refusal.header, refusal.want, got.status, got.body)
				}
				if after := len(f.journal(card)); after != before {
					t.Errorf("%v: the journal went from %d lines to %d on a refused request", refusal.header, before, after)
				}
			}
			for _, admitted := range [][]string{{}, {"Origin", "-", "Sec-Fetch-Site", "same-origin"}} {
				got := record(f.form(shape.path, shape.body, admitted...))
				if got.status != shape.success {
					t.Errorf("%v: wanted %d, got %d %s", admitted, shape.success, got.status, got.body)
					continue
				}
				shape.undo()
			}
		})
	}

	t.Run("an empty POST needs the proof a form needs", func(t *testing.T) {
		got := record(f.send(request{method: http.MethodPost, path: "/cards/" + card + "/claim"}))
		if got.status != http.StatusForbidden || got.refusal(t) != contract.OriginRequired {
			t.Errorf("an empty claim with no Origin: wanted 403 %s, got %d %s", contract.OriginRequired, got.status, got.body)
		}
		got = record(f.send(request{method: http.MethodPost, path: "/cards/" + card + "/claim", header: map[string]string{"Origin": f.origin()}}))
		if got.status != http.StatusCreated {
			t.Errorf("an empty claim with this server's Origin: wanted 201, got %d %s", got.status, got.body)
		}
		f.act(&verb.Request{Verb: verb.Release, Actor: "alka", Card: card})
	})

	t.Run("a JSON act needs no proof and refuses a foreign Origin", func(t *testing.T) {
		got := record(f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}"))
		if got.status != http.StatusCreated {
			t.Errorf("a JSON claim with no Origin: wanted 201, got %d %s", got.status, got.body)
		}
		f.act(&verb.Request{Verb: verb.Release, Actor: "alka", Card: card})
		got = record(f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}", "Origin", "null"))
		if got.status != http.StatusForbidden || got.refusal(t) != contract.ForeignOrigin {
			t.Errorf("a JSON claim with Origin null: wanted 403 %s, got %d %s", contract.ForeignOrigin, got.status, got.body)
		}
	})

	t.Run("a type the route does not take answers 415", func(t *testing.T) {
		for _, contentType := range []string{"multipart/form-data; boundary=x", "text/plain"} {
			got := record(f.json(http.MethodPost, "/cards/"+card+"/claim", contentType, "x", "Origin", f.origin()))
			if got.status != http.StatusUnsupportedMediaType || got.refusal(t) != contract.UnsupportedMediaType {
				t.Errorf("%s: wanted 415, got %d %s", contentType, got.status, got.body)
			}
		}
		got := record(f.json(http.MethodPatch, "/cards/"+card, typeForm, "column=review", "If-Match", "*", "Origin", f.origin()))
		if got.status != http.StatusUnsupportedMediaType || got.header.Get("Accept-Patch") != strings.Join(patchTypes, ", ") {
			t.Errorf("a real PATCH with a form body: wanted 415 with Accept-Patch, got %d %q %s", got.status, got.header.Get("Accept-Patch"), got.body)
		}
		for _, raw := range []string{"", "text/;"} {
			got := record(f.json(http.MethodPost, "/cards/"+card+"/claim", "", "{}", "Content-Type", raw))
			if got.status != http.StatusUnsupportedMediaType {
				t.Errorf("Content-Type %q: wanted 415, got %d %s", raw, got.status, got.body)
			}
		}
		got = record(f.send(request{method: http.MethodPost, path: "/cards/" + card + "/claim", body: "{}", chunked: true, header: map[string]string{"Origin": f.origin()}}))
		if got.status != http.StatusUnsupportedMediaType {
			t.Errorf("a chunked body with no Content-Type: wanted 415, got %d %s", got.status, got.body)
		}
	})

	t.Run("a body over the limit answers 413", func(t *testing.T) {
		big := `{"text": "` + strings.Repeat("x", maxBody) + `"}`
		got := record(f.json(http.MethodPost, "/cards/"+card+"/comments", typeJSON, big))
		if got.status != http.StatusRequestEntityTooLarge || got.refusal(t) != contract.BodyTooLarge {
			t.Errorf("wanted 413 %s, got %d %.200s", contract.BodyTooLarge, got.status, got.body)
		}
		// A body of unknown length declares nothing to refuse early, so the
		// limit is met while the body is read.
		for _, contentType := range []string{typeJSON, typeForm} {
			body := big
			if contentType == typeForm {
				body = "text=" + strings.Repeat("x", maxBody)
			}
			got := record(f.send(request{method: http.MethodPost, path: "/cards/" + card + "/comments", body: body, chunked: true, header: map[string]string{"Content-Type": contentType, "Origin": f.origin()}}))
			if got.status != http.StatusRequestEntityTooLarge || got.refusal(t) != contract.BodyTooLarge {
				t.Errorf("a chunked %s body over the limit: wanted 413 %s, got %d %.200s", contentType, contract.BodyTooLarge, got.status, got.body)
			}
		}
	})

	t.Run("route and method are answered before the body", func(t *testing.T) {
		cases := []struct {
			req    request
			status int
			name   string
		}{
			{request{method: http.MethodOptions, path: "/cards/" + card}, http.StatusMethodNotAllowed, contract.MethodNotAllowed},
			{request{method: http.MethodOptions, path: "/nowhere"}, http.StatusNotFound, contract.UnknownResource},
			{request{method: http.MethodPut, path: "/cards/" + card + "/claim"}, http.StatusMethodNotAllowed, contract.MethodNotAllowed},
			{request{method: http.MethodPost, path: "/nowhere", body: "{}", header: map[string]string{"Content-Type": typeJSON}}, http.StatusNotFound, contract.UnknownResource},
		}
		for _, c := range cases {
			got := record(f.send(c.req))
			if got.status != c.status || got.refusal(t) != c.name {
				t.Errorf("%s %s: wanted %d %s, got %d %s", c.req.method, c.req.path, c.status, c.name, got.status, got.body)
			}
		}
	})

	if len(seen) == 0 {
		t.Fatal("no reply was read, so the header check below read nothing")
	}
	for _, got := range seen {
		for name := range got.header {
			if strings.HasPrefix(http.CanonicalHeaderKey(name), "Access-Control-Allow-") {
				t.Errorf("an reply carried %s, which the admission argument depends on no reply carrying", name)
			}
		}
	}
	t.Logf("read %d answers for Access-Control-Allow- headers", len(seen))
}
