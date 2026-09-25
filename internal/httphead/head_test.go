package httphead

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// fixtureDefinition is the workbench these tests serve. Its columns cover the
// three answers a ready card's take-up act can give: Intake, where no work is
// taken up and a pull carries a card on, so a ready card is offered pull;
// Build and Review, which hold active cards, so a ready card is offered claim;
// and Waiting, which waits on somebody outside, so a ready card there is
// offered neither. Finished is terminal.
const fixtureDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "HTTP",
  "instructions": "Standing text.\n",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake", "instructions": "Intake text.\n" },
    { "id": "c00000000002", "title": "Build", "kind": "work", "instructions": "Build text.\n" },
    { "id": "c00000000003", "title": "Waiting", "kind": "work", "awaiting_outside": true },
    { "id": "c00000000004", "title": "Review", "kind": "work", "instructions": "Review text.\n" },
    { "id": "c00000000005", "title": "Finished", "kind": "done" }
  ]
}`

// fixtureID is the fixture workbench's identifier.
const fixtureID = "0199a1b2c3d47abc8000000000000152"

// fixture is a workbench served by the head on a real loopback listener.
type fixture struct {
	t *testing.T
	// root is the workbench's directory.
	root string
	// home is the user base.
	home string
	// url is the server's base URL, ending without a slash.
	url string
	// port is the bound port.
	port int
	// cfg is what the handler was built with.
	cfg Config
}

// newFixture builds the workbench and serves it with the default actor
// alka. Each option edits the handler's configuration before it is built.
func newFixture(t *testing.T, options ...func(*Config)) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, bench.UserBaseName, fixtureID)
	read, err := bench.ReadDefinition([]byte(fixtureDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, "ht", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	cfg := Config{
		Root:         root,
		Home:         filepath.Join(base, "home"),
		DefaultActor: "alka",
		Agent:        bench.Agent{Harness: "fixture", Model: "fixture-model"},
		Host:         "127.0.0.1",
		Port:         port,
	}
	for _, option := range options {
		option(&cfg)
	}
	server := &http.Server{Handler: Handler(cfg)}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return &fixture{t: t, root: root, home: cfg.Home, url: "http://127.0.0.1:" + strconv.Itoa(port), port: port, cfg: cfg}
}

// origin is this server's own origin, as a browser serialises it.
func (f *fixture) origin() string {
	return f.url
}

// reply is one response as a test reads it.
type reply struct {
	status int
	header http.Header
	body   string
}

// object decodes the body as a JSON object, failing the test when it is not
// one.
func (a reply) object(t *testing.T) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(a.body), &decoded); err != nil {
		t.Fatalf("the body is not a JSON object: %v\n%s", err, a.body)
	}
	return decoded
}

// refusal is the refusal name the body carries, empty on any other reply.
func (a reply) refusal(t *testing.T) string {
	t.Helper()
	name, _ := a.object(t)["refusal"].(string)
	return name
}

// detail is the detail the body carries.
func (a reply) detail(t *testing.T) string {
	t.Helper()
	detail, _ := a.object(t)["detail"].(string)
	return detail
}

// request is one request a test sends.
type request struct {
	method string
	path   string
	header map[string]string
	body   string
	// chunked sends the body with no declared length.
	chunked bool
}

// send sends one request and reads the whole reply.
func (f *fixture) send(r request) reply {
	f.t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	if r.chunked {
		body = io.MultiReader(strings.NewReader(r.body))
	}
	req, err := http.NewRequest(r.method, f.url+r.path, body)
	if err != nil {
		f.t.Fatalf("build %s %s: %v", r.method, r.path, err)
	}
	if r.chunked {
		req.ContentLength = -1
	}
	for name, value := range r.header {
		if name == "Host" {
			req.Host = value
			continue
		}
		req.Header.Set(name, value)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		f.t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer response.Body.Close()
	read, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatalf("read %s %s: %v", r.method, r.path, err)
	}
	return reply{status: response.StatusCode, header: response.Header, body: string(read)}
}

// get sends a GET.
func (f *fixture) get(path string, header ...string) reply {
	f.t.Helper()
	return f.send(request{method: http.MethodGet, path: path, header: pairs(header)})
}

// json sends an unsafe request with a body of the given type.
func (f *fixture) json(method, path, contentType, body string, header ...string) reply {
	f.t.Helper()
	headers := pairs(header)
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return f.send(request{method: method, path: path, header: headers, body: body})
}

// form posts a form body with this server's own Origin unless the header
// list says otherwise.
func (f *fixture) form(path, body string, header ...string) reply {
	f.t.Helper()
	headers := map[string]string{"Content-Type": typeForm, "Origin": f.origin()}
	for name, value := range pairs(header) {
		headers[name] = value
	}
	if headers["Origin"] == "-" {
		delete(headers, "Origin")
	}
	return f.send(request{method: http.MethodPost, path: path, header: headers, body: body})
}

// pairs reads alternating header names and values.
func pairs(header []string) map[string]string {
	headers := map[string]string{}
	for i := 0; i+1 < len(header); i += 2 {
		headers[header[i]] = header[i+1]
	}
	return headers
}

// library opens the fixture workbench outside the head, the way another
// process would.
func (f *fixture) library() *verb.Library {
	f.t.Helper()
	opened, err := bench.Open(f.root)
	if err != nil {
		f.t.Fatalf("open: %v", err)
	}
	return verb.New(opened, f.home)
}

// add files a card through the library and returns its reference.
func (f *fixture) add(title, column string) string {
	f.t.Helper()
	response := f.library().Add(&verb.Request{Verb: "add", Actor: "alka", Title: title, Column: column})
	if response.Card == nil {
		f.t.Fatalf("add %q: %+v", title, response)
	}
	return response.Card.Ref
}

// act runs one act through the library, failing the test unless it answers ok.
func (f *fixture) act(req *verb.Request) *verb.Response {
	f.t.Helper()
	response := f.library().Do(req)
	if response.Outcome != "ok" {
		f.t.Fatalf("%s %s: %s %s", req.Verb, req.Card, response.Refusal, response.Detail)
	}
	return response
}

// card reads a card as it stands.
func (f *fixture) card(ref string) *verb.CardView {
	f.t.Helper()
	detail, _, _, _, err := f.library().Show(&verb.Request{Verb: "show", Card: ref})
	if err != nil || detail == nil {
		f.t.Fatalf("show %s: %v", ref, err)
	}
	return &detail.Card
}

// journal reads a card's journal.
func (f *fixture) journal(ref string) []bench.Event {
	f.t.Helper()
	result, err := f.library().ListRef(&verb.Request{Verb: "list", Ref: ref + "/journal"})
	if err != nil {
		f.t.Fatalf("journal %s: %v", ref, err)
	}
	return result.History
}

// quoted is a revision as an entity-tag.
func quoted(revision string) string {
	return strconv.Quote(revision)
}
