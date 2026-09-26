package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/httphead"
	"dinah/internal/resident"
	"dinah/internal/resident/residenttest"
	"dinah/internal/verb"
)

// serveRun is one dinah serve started in process through runCLI, with the
// context it stops on and what its listen function saw.
type serveRun struct {
	cancel context.CancelFunc
	// done carries the invocation once the command returns.
	done chan invocation
	// listening carries the address actually bound, once something is.
	listening chan string
	mu        sync.Mutex
	// bound are the addresses the listen function was handed.
	bound []string
}

// startServe runs dinah serve with argv through runCLI on a goroutine,
// handing it a context the test cancels and a listen function that records
// each address before doing what listen says. The two seams are restored
// when the test ends.
func startServe(t *testing.T, dir string, listen listenFunc, argv ...string) *serveRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &serveRun{cancel: cancel, done: make(chan invocation, 1), listening: make(chan string, 1)}
	previousBase, previousListen := serveBase, serveListen
	serveBase = func() context.Context { return ctx }
	serveListen = func(network, address string) (net.Listener, error) {
		run.mu.Lock()
		run.bound = append(run.bound, address)
		run.mu.Unlock()
		listener, err := listen(network, address)
		if err == nil {
			run.listening <- listener.Addr().String()
		}
		return listener, err
	}
	t.Cleanup(func() {
		cancel()
		serveBase, serveListen = previousBase, previousListen
	})
	go func() {
		run.done <- runCLI(t, dir, append([]string{"serve"}, argv...)...)
	}()
	return run
}

// addresses is what the listen function was handed.
func (r *serveRun) addresses() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bound...)
}

// wait waits for the command to return.
func (r *serveRun) wait(t *testing.T) invocation {
	t.Helper()
	select {
	case got := <-r.done:
		return got
	case <-time.After(30 * time.Second):
		t.Fatal("dinah serve did not return")
	}
	return invocation{}
}

// address waits for the server to bind and returns the bound address.
func (r *serveRun) address(t *testing.T) string {
	t.Helper()
	select {
	case address := <-r.listening:
		return address
	case got := <-r.done:
		t.Fatalf("dinah serve returned %d before it listened: %s", got.code, got.errw)
	case <-time.After(30 * time.Second):
		t.Fatal("dinah serve did not listen")
	}
	return ""
}

// refuseListen stands in for net.Listen where the test wants to see the
// address and bind nothing.
func refuseListen(string, string) (net.Listener, error) {
	return nil, errors.New("the test binds nothing")
}

// TestServeListensOnLoopbackAlone is dinah-152/criteria/1.
func TestServeListensOnLoopbackAlone(t *testing.T) {
	root := newBench(t)
	refused := []string{"0.0.0.0:0", ":0", "[::]:0", "192.0.2.1:0", "[::ffff:127.0.0.1]:0", "[fe80::1%1]:0", "example.com:0"}
	for _, address := range refused {
		run := startServe(t, root, refuseListen, "--listen", address)
		got := run.wait(t)
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("--listen %s: wanted exit 2, got %d", address, got.code)
		}
		if first, _, _ := strings.Cut(strings.TrimSpace(got.errw), " "); first != contract.NotLoopback {
			t.Errorf("--listen %s: wanted %s leading stderr, got %q", address, contract.NotLoopback, got.errw)
		}
		if bound := run.addresses(); len(bound) != 0 {
			t.Errorf("--listen %s reached the listen function with %v", address, bound)
		}
	}
	admitted := map[string]string{
		"127.0.0.1:0":           "127.0.0.1:0",
		"127.0.0.5:0":           "127.0.0.5:0",
		"localhost:0":           "127.0.0.1:0",
		"[::1]:0":               "[::1]:0",
		"[0:0:0:0:0:0:0:1]:0":   "[::1]:0",
		"LocalHost:0":           "127.0.0.1:0",
		"127.255.255.254:65535": "127.255.255.254:65535",
	}
	for address, want := range admitted {
		run := startServe(t, root, refuseListen, "--listen", address)
		if got := run.wait(t); got.code != contract.ExitCode(contract.OutcomeUnreachable) {
			t.Errorf("--listen %s: the stand-in listen refuses, so wanted exit 4, got %d %s", address, got.code, got.errw)
		}
		if bound := run.addresses(); len(bound) != 1 || bound[0] != want {
			t.Errorf("--listen %s: wanted the listen function handed %s, got %v", address, want, bound)
		}
	}
	t.Logf("%d addresses refused before listening and %d handed to it", len(refused), len(admitted))
}

// TestServeStartsPrintsAndStops is dinah-152/criteria/2.
func TestServeStartsPrintsAndStops(t *testing.T) {
	root := newBench(t)
	// The root serve prints is the one discovery found from the directory it
	// ran in, so the test asks discovery the same question rather than
	// spelling the fixture's path itself: on macOS the temporary directory
	// lies behind a symbolic link, and the working directory discovery climbs
	// from is the resolved side of it.
	anchor := runCLI(t, root, "path", "workbench")
	if anchor.code != 0 {
		t.Fatalf("path workbench: %d %s", anchor.code, anchor.errw)
	}
	absolute, err := filepath.Abs(filepath.Dir(strings.TrimSpace(anchor.out)))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	run := startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	address := run.address(t)
	if response, err := http.Get("http://" + address + "/whoami"); err != nil || response.StatusCode != http.StatusOK {
		t.Errorf("the bound address does not answer: %v %v", response, err)
	}
	run.cancel()
	got := run.wait(t)
	want := "Serving " + absolute + " at http://" + address + "/\n"
	if got.code != 0 || got.out != want || got.errw != "" || strings.HasSuffix(address, ":0") {
		t.Errorf("wanted exit 0 and the one line %q, got %d, stdout %q, stderr %q", want, got.code, got.out, got.errw)
	}

	run = startServe(t, root, net.Listen, "--json", "--listen", "127.0.0.1:0")
	address = run.address(t)
	run.cancel()
	got = run.wait(t)
	var announced struct {
		URL       string `json:"url"`
		Workbench string `json:"workbench"`
	}
	err = json.Unmarshal([]byte(got.out), &announced)
	if err != nil || got.code != 0 || strings.Count(got.out, "\n") != 1 || announced.Workbench != absolute || announced.URL != "http://"+address+"/" {
		t.Errorf("--json: wanted exit 0 and one object with url and workbench, got %d %q (%v)", got.code, got.out, err)
	}

	run = startServe(t, root, refuseListen)
	run.wait(t)
	if bound := run.addresses(); len(bound) != 1 || bound[0] != defaultListen || defaultListen != "127.0.0.1:7340" {
		t.Errorf("no --listen: wanted the listen function handed 127.0.0.1:7340, got %v", bound)
	}

	empty := t.TempDir()
	run = startServe(t, empty, refuseListen, "--workbench", empty, "--listen", "127.0.0.1:0")
	if got := run.wait(t); got.code != contract.ExitCode(contract.OutcomeRefused) || len(run.addresses()) != 0 {
		t.Errorf("no workbench: wanted exit 2 and nothing bound, got %d and %v; stderr %q", got.code, run.addresses(), got.errw)
	}

	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold a port: %v", err)
	}
	defer held.Close()
	run = startServe(t, root, net.Listen, "--listen", held.Addr().String())
	if got := run.wait(t); got.code != contract.ExitCode(contract.OutcomeUnreachable) || !strings.Contains(got.errw, held.Addr().String()) {
		t.Errorf("an address in use: wanted exit 4 naming the address, got %d %q", got.code, got.errw)
	}
}

// httpFixture serves a newBench workbench through the HTTP head in process.
type httpFixture struct {
	t    *testing.T
	root string
	url  string
}

// serveBench serves the workbench beneath root with the handler's own
// configuration, edited by each option.
func serveBench(t *testing.T, root string, options ...func(*httphead.Config)) *httpFixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	cfg := httphead.Config{
		Root:         soleBenchDir(t, root),
		Home:         os.Getenv("DINAH_HOME"),
		DefaultActor: "alka",
		Host:         "127.0.0.1",
		Port:         listener.Addr().(*net.TCPAddr).Port,
	}
	for _, option := range options {
		option(&cfg)
	}
	server := &http.Server{Handler: httphead.Handler(cfg)}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return &httpFixture{t: t, root: root, url: "http://" + listener.Addr().String()}
}

// do sends one request and decodes the body as an object.
func (f *httpFixture) do(method, path, contentType, body string, header ...string) (int, http.Header, map[string]any) {
	f.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.url+path, reader)
	if err != nil {
		f.t.Fatalf("build: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	read, _ := io.ReadAll(response.Body)
	var object map[string]any
	if err := json.Unmarshal(read, &object); err != nil {
		f.t.Fatalf("%s %s answered %d with no JSON object: %s", method, path, response.StatusCode, read)
	}
	return response.StatusCode, response.Header, object
}

// TestTheHeadAnswersWhatTheCLIWroteAMomentAgo is dinah-152/criteria/13.
func TestTheHeadAnswersWhatTheCLIWroteAMomentAgo(t *testing.T) {
	root := newBench(t)
	f := serveBench(t, root)
	status, _, first := f.do(http.MethodGet, "/changes", "", "")
	cursor, _ := first["cursor"].(string)
	if status != http.StatusOK || cursor == "" {
		t.Fatalf("GET /changes: wanted a cursor, got %d %v", status, first)
	}
	if got := runCLI(t, root, "add", "Added while the head runs"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	status, _, later := f.do(http.MethodGet, "/changes?since="+cursor, "", "")
	events, _ := later["events"].([]any)
	if status != http.StatusOK || later["changed"] != true || len(events) == 0 {
		t.Errorf("GET /changes?since: wanted changed true with the event, got %d %v", status, later)
	}
	if got := runCLI(t, root, "column", "new", "Parking"); got.code != 0 {
		t.Fatalf("column new: %d %s", got.code, got.errw)
	}
	_, _, columns := f.do(http.MethodGet, "/columns", "", "")
	encoded, _ := json.Marshal(columns)
	if !strings.Contains(string(encoded), `"Parking"`) {
		t.Errorf("GET /columns does not show the column the CLI added: %s", encoded)
	}
	status, _, waited := f.do(http.MethodGet, "/changes?wait", "", "")
	if status != http.StatusBadRequest || waited["refusal"] != contract.Usage {
		t.Errorf("GET /changes?wait: wanted 400 %s, got %d %v", contract.Usage, status, waited)
	}
}

// dinahBinary is the real binary, built once per run for the tests that need
// a writer in another operating-system process.
var dinahBinary = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "dinah-serve-")
	if err != nil {
		return "", err
	}
	name := "dinah"
	if os.PathSeparator == '\\' {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", path, ".")
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build dinah: %v\n%s", err, out)
	}
	return path, nil
})

// childEnv is the environment a child process runs in: the three variables
// set explicitly and every variable isolatedEnv names cleared.
func childEnv(workbench string) []string {
	cleared := map[string]bool{"DINAH_ACTOR": true, "DINAH_HOME": true, "DINAH_WORKBENCH": true}
	for _, name := range isolatedEnv {
		cleared[name] = true
	}
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !cleared[strings.ToUpper(name)] {
			env = append(env, entry)
		}
	}
	return append(env, "DINAH_ACTOR=alka", "DINAH_HOME="+os.Getenv("DINAH_HOME"), "DINAH_WORKBENCH="+workbench)
}

// cliMove runs the real binary's move in another process and returns its exit
// code and stderr.
func cliMove(t *testing.T, workbench, card, column string) (int, string) {
	t.Helper()
	binary, err := dinahBinary()
	if err != nil {
		t.Fatalf("%v", err)
	}
	command := exec.Command(binary, "move", card, column)
	command.Env = childEnv(workbench)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err = command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatalf("run the CLI: %v", err)
	}
	return 0, stderr.String()
}

// cliRun runs the real binary in another process with the given arguments
// and returns its exit code and stderr.
func cliRun(t *testing.T, workbench string, argv ...string) (int, string) {
	t.Helper()
	binary, err := dinahBinary()
	if err != nil {
		t.Fatalf("%v", err)
	}
	command := exec.Command(binary, argv...)
	command.Env = childEnv(workbench)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err = command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatalf("run the CLI: %v", err)
	}
	return 0, stderr.String()
}

// structuralRounds is how many archive, restore and delete cycles
// TestStructuralActsFromAnotherProcessSucceedWhileTheResidentReads runs. The
// review that found the defect saw four of five archives fail against the
// unfixed resident, so a round that passes by luck is rare, and forty in a
// row passing by luck is not a thing that happens.
const structuralRounds = 40

// TestStructuralActsFromAnotherProcessSucceedWhileTheResidentReads is the
// cross-process test dinah-619/comments/12 found missing. A structural act
// writes inside an entity's directory, records the act, then renames or
// removes the directory. The resident reads the directory at once, because
// Windows reports the writes, and Windows refuses a rename of a directory
// while any handle is open below it, so the act met a refusal nearly every
// time. The act now retries that refusal for a bounded time (the storage
// layer's retry of dinah-619's write-path section), and the resident opens as
// little as it can for as short as it can.
//
// Every act is the real binary in another operating-system process, and the
// resident is the platform watcher's, reading as the server's does. Each
// round adds a card, archives it, restores it and deletes it, and the test
// fails on the first act refused, naming the round, the act and the refusal.
func TestStructuralActsFromAnotherProcessSucceedWhileTheResidentReads(t *testing.T) {
	root := newBench(t)
	workbench := soleBenchDir(t, root)
	published := make(chan resident.Published, 1024)
	f := serveBench(t, root, withPlatformResident(t, &resident.Hooks{AfterPublish: func(p resident.Published) {
		select {
		case published <- p:
		default:
		}
	}}))
	publishes := 0
	for round := 1; round <= structuralRounds; round++ {
		card := "fx-" + strconv.Itoa(round)
		steps := [][]string{
			{"add", "Structural round " + strconv.Itoa(round)},
			{"archive", card},
			{"restore", card},
			{"delete", card, "--yes"},
		}
		for _, argv := range steps {
			if code, stderr := cliRun(t, workbench, argv...); code != 0 {
				t.Fatalf("round %d of %d: %v answered %d: %s", round, structuralRounds, argv, code, strings.TrimSpace(stderr))
			}
		}
	drain:
		for {
			select {
			case <-published:
				publishes++
			default:
				break drain
			}
		}
	}
	if publishes == 0 {
		t.Fatal("the resident published nothing while the acts ran, so it read nothing and the test proves nothing")
	}
	status, _, body := f.do(http.MethodGet, "/cards", "", "")
	if status != http.StatusOK {
		t.Fatalf("GET /cards after the rounds: %d %v", status, body)
	}
	t.Logf("%d rounds of add, archive, restore and delete from another process, %d publishes read by the resident meanwhile", structuralRounds, publishes)
}

// lockHelperVar turns this test binary into a process that holds one card's
// lock: it names the card's directory.
const lockHelperVar = "DINAH_SERVE_LOCK_HELPER"

// TestServeLockHelper is not a test. Started as a child with lockHelperVar
// set, it takes the card's lock through the library's own acquisition, says
// so on stdout, and releases when its stdin closes.
func TestServeLockHelper(t *testing.T) {
	dir := os.Getenv(lockHelperVar)
	if dir == "" {
		t.Skip("run as a child by TestAWriterInAnotherProcessIsSerialised")
	}
	lock, err := bench.Acquire(dir, "helper", bench.Stamp(time.Now()))
	if err != nil {
		fmt.Println("refused", err)
		os.Exit(1)
	}
	fmt.Println("held")
	io.Copy(io.Discard, os.Stdin)
	lock.Release()
	os.Exit(0)
}

// TestAWriterInAnotherProcessIsSerialised is dinah-152/criteria/20. Each
// writer is a separate operating-system process: the real binary for a CLI
// move, and this test binary re-executed for a held lock.
func TestAWriterInAnotherProcessIsSerialised(t *testing.T) {
	writersAreSerialised(t, nil)
}

// TestAWriterInAnotherProcessIsSerialisedWithAResident is part of
// dinah-619/criteria/16: dinah-152's writer tests run a second time with a
// resident held by every head, and pass unchanged, which shows that the
// locks and the basis guard behave as they did.
func TestAWriterInAnotherProcessIsSerialisedWithAResident(t *testing.T) {
	writersAreSerialised(t, func(t *testing.T) func(*httphead.Config) {
		return withPlatformResident(t, nil)
	})
}

// withPlatformResident is a serveBench option that opens a resident with the
// platform's own watcher over the head's root and closes it when the test
// ends. It skips the test where the platform has no watcher.
func withPlatformResident(t *testing.T, hooks *resident.Hooks) func(*httphead.Config) {
	return func(cfg *httphead.Config) {
		t.Helper()
		w, err := resident.Open(cfg.Root, resident.Options{Hooks: hooks})
		if errors.Is(err, resident.ErrUnsupported) {
			t.Skip("this platform has no watcher, so the head reads the disk as the first run did")
		}
		if err != nil {
			t.Fatalf("open the resident: %v", err)
		}
		t.Cleanup(func() { w.Close() })
		<-w.Ready()
		cfg.Resident = w
	}
}

// writersAreSerialised is dinah-152/criteria/20's body. held, when set,
// answers an option each head is served with.
func writersAreSerialised(t *testing.T, held func(t *testing.T) func(*httphead.Config)) {
	serve := func(t *testing.T, root string, options ...func(*httphead.Config)) *httpFixture {
		t.Helper()
		if held != nil {
			options = append(options, held(t))
		}
		return serveBench(t, root, options...)
	}
	root := newBench(t)
	workbench := soleBenchDir(t, root)
	for _, argv := range [][]string{{"column", "new", "Review"}, {"add", "A card"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: %d %s", argv, got.code, got.errw)
		}
	}
	card := "fx-1"
	carryToDoing(t, root, card)
	if got := runCLI(t, root, "claim", card); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	opened, err := bench.Open(workbench)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	revision := func() string {
		detail, _, _, _, err := library.Show(&verb.Request{Verb: "show", Card: card})
		if err != nil {
			t.Fatalf("show: %v", err)
		}
		return detail.Card.Revision
	}
	column := func() string {
		detail, _, _, _, _ := library.Show(&verb.Request{Verb: "show", Card: card})
		return detail.Card.ColumnTitle
	}
	journal := func() []bench.Event {
		result, err := library.ListRef(&verb.Request{Verb: "list", Ref: card + "/journal"})
		if err != nil {
			t.Fatalf("journal: %v", err)
		}
		return result.History
	}
	move := func(f *httpFixture, to, basis string) (int, http.Header, map[string]any) {
		return f.do(http.MethodPatch, "/cards/"+card, "application/vnd.dinah.move+json", `{"column": "`+to+`"}`, "If-Match", strconv.Quote(basis))
	}

	t.Run("a lock held by another process", func(t *testing.T) {
		f := serve(t, root)
		found, err := opened.ResolveCard(card)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		helper := exec.Command(os.Args[0], "-test.run=^TestServeLockHelper$")
		helper.Env = append(childEnv(workbench), lockHelperVar+"="+found.Card.Dir)
		stdin, err := helper.StdinPipe()
		if err != nil {
			t.Fatalf("stdin: %v", err)
		}
		stdout, err := helper.StdoutPipe()
		if err != nil {
			t.Fatalf("stdout: %v", err)
		}
		if err := helper.Start(); err != nil {
			t.Fatalf("start the helper: %v", err)
		}
		if line, _ := bufio.NewReader(stdout).ReadString('\n'); strings.TrimSpace(line) != "held" {
			stdin.Close()
			helper.Wait()
			t.Fatalf("the helper did not take the lock: %q", line)
		}
		before, at := len(journal()), column()
		status, _, body := move(f, "review", revision())
		if status != http.StatusConflict || body["refusal"] != contract.Locked {
			t.Errorf("a move while another process holds the lock: wanted 409 %s, got %d %v", contract.Locked, status, body)
		}
		if len(journal()) != before || column() != at {
			t.Errorf("the refused move wrote: journal %d to %d lines, column %s to %s", before, len(journal()), at, column())
		}
		stdin.Close()
		if err := helper.Wait(); err != nil {
			t.Fatalf("the helper: %v", err)
		}
		if status, _, body := move(f, "review", revision()); status != http.StatusOK {
			t.Errorf("the same move once the helper released: wanted 200, got %d %v", status, body)
		}
	})

	t.Run("a write between the head's read and its write", func(t *testing.T) {
		once := sync.Once{}
		f := serve(t, root, func(cfg *httphead.Config) {
			cfg.BeforeRun = func() {
				once.Do(func() {
					if code, stderr := cliMove(t, workbench, card, "doing"); code != 0 {
						t.Errorf("the CLI move in BeforeRun: %d %s", code, stderr)
					}
				})
			}
		})
		earlier := revision()
		status, header, body := move(f, "review", earlier)
		if status != http.StatusPreconditionFailed || header.Get("ETag") != strconv.Quote(revision()) || column() != "Doing" {
			t.Errorf("a move carrying the revision read before the CLI move: wanted 412 with the current ETag and the card where the CLI put it, got %d %q %v at %s", status, header.Get("ETag"), body, column())
		}
	})

	t.Run("a write between a GET and a PATCH", func(t *testing.T) {
		f := serve(t, root)
		_, header, _ := f.do(http.MethodGet, "/cards/"+card, "", "")
		read, _ := strconv.Unquote(header.Get("ETag"))
		if code, stderr := cliMove(t, workbench, card, "review"); code != 0 {
			t.Fatalf("the CLI move: %d %s", code, stderr)
		}
		status, header, body := move(f, "doing", read)
		if status != http.StatusPreconditionFailed || header.Get("ETag") != strconv.Quote(revision()) || column() != "Review" {
			t.Errorf("a move carrying the GET's ETag after the CLI move: wanted 412 with the current ETag and the card where the CLI put it, got %d %q %v at %s", status, header.Get("ETag"), body, column())
		}
	})

	t.Run("a CLI writer arriving while the head holds the lock", func(t *testing.T) {
		code, stderr := -1, ""
		f := serve(t, root, func(cfg *httphead.Config) {
			cfg.Interleave = func() { code, stderr = cliMove(t, workbench, card, "done") }
		})
		before := len(journal())
		status, _, body := move(f, "doing", revision())
		if first, _, _ := strings.Cut(strings.TrimSpace(stderr), " "); code == 0 || first != contract.Locked {
			t.Errorf("the CLI move inside the head's lock: wanted a non-zero exit refused %s, got %d %q", contract.Locked, code, stderr)
		}
		if status != http.StatusOK || column() != "Doing" {
			t.Errorf("the HTTP move: wanted 200 and the card at Doing, got %d %v at %s", status, body, column())
		}
		events := journal()[before:]
		if len(events) != 1 || events[0].Event != "moved" || events[0].To != "Doing" && !strings.EqualFold(events[0].ToTitle, "Doing") {
			t.Errorf("wanted the journal to carry the HTTP move alone, got %+v", events)
		}
	})
}

// TestTheResidentAnswersWhatTheCLIWroteOnceItsRecordArrives is
// dinah-619/criteria/6: dinah-152/criteria/13's cross-process shape on the
// resident. The real binary moves a card in another process; the test waits
// for the publish whose paths include the card's anchor, as Windows reports
// the write, and one GET then shows the new column. The wait is on the
// publish, against a deadline that bounds the test only, and nothing polls
// the head.
func TestTheResidentAnswersWhatTheCLIWroteOnceItsRecordArrives(t *testing.T) {
	root := newBench(t)
	workbench := soleBenchDir(t, root)
	for _, argv := range [][]string{{"column", "new", "Review"}, {"add", "A card"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: %d %s", argv, got.code, got.errw)
		}
	}
	card := "fx-1"
	carryToDoing(t, root, card)
	opened, err := bench.Open(workbench)
	if err != nil {
		t.Fatal(err)
	}
	found, err := opened.ResolveCard(card)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := filepath.Rel(workbench, found.Card.AnchorPath())
	if err != nil {
		t.Fatal(err)
	}
	anchor = filepath.ToSlash(anchor)
	published := make(chan resident.Published, 256)
	f := serveBench(t, root, withPlatformResident(t, &resident.Hooks{AfterPublish: func(p resident.Published) {
		select {
		case published <- p:
		default:
		}
	}}))
	if code, stderr := cliMove(t, workbench, card, "review"); code != 0 {
		t.Fatalf("the CLI move: %d %s", code, stderr)
	}
	timeout := time.After(30 * time.Second)
	for waiting := true; waiting; {
		select {
		case p := <-published:
			for _, path := range p.Paths {
				if path == anchor {
					waiting = false
				}
			}
		case <-timeout:
			t.Fatalf("no publish naming %s arrived within 30 seconds of the CLI move; Windows documents that the change will be reported and not how soon, so this is a finding", anchor)
		}
	}
	status, _, shown := f.do(http.MethodGet, "/cards/"+card, "", "")
	detail, _ := shown["detail"].(map[string]any)
	view, _ := detail["card"].(map[string]any)
	if status != http.StatusOK || view["column_title"] != "Review" {
		t.Errorf("GET /cards/%s after the publish: wanted the card in Review, got %d %v", card, status, view)
	}
}

// TestServeClosesItsResident is part of dinah-619/criteria/16. serve opens
// the resident once, serves from it, and closes it after shutdown.
func TestServeClosesItsResident(t *testing.T) {
	root := newBench(t)
	var opened []*resident.Workbench
	previous := residentOpen
	residentOpen = func(dir string) (*resident.Workbench, error) {
		w, err := resident.Open(dir, resident.Options{Notifier: residenttest.NewManual()})
		if err == nil {
			<-w.Ready()
			opened = append(opened, w)
		}
		return w, err
	}
	t.Cleanup(func() { residentOpen = previous })
	run := startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	address := run.address(t)
	if len(opened) != 1 {
		t.Fatalf("serve opened %d residents, wanted one", len(opened))
	}
	// The notifier never reports, so a card the CLI adds now is drawn only by
	// a head that reads the disk.
	if got := runCLI(t, root, "add", "Added under a resident that is never told"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	response, err := http.Get("http://" + address + "/cards")
	if err != nil {
		t.Fatal(err)
	}
	read, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(read), "Added under a resident that is never told") {
		t.Error("the head drew a card only the disk holds, so serve did not hand it the resident")
	}
	run.cancel()
	got := run.wait(t)
	if got.code != 0 || got.errw != "" || strings.Count(got.out, "\n") != 1 {
		t.Errorf("serve wanted exit 0 and its one line, got %d, stdout %q, stderr %q", got.code, got.out, got.errw)
	}
	if pick := opened[0].Current(time.Now()); pick.Snapshot != nil {
		t.Error("the resident still serves a snapshot after serve returned, so it was not closed")
	}
}

// TestServeWithoutAResidentServesFromDisk is part of dinah-619/criteria/16.
// Where resident.Open answers ErrUnsupported, serve serves from disk and
// writes nothing beyond its one line.
func TestServeWithoutAResidentServesFromDisk(t *testing.T) {
	root := newBench(t)
	previous := residentOpen
	calls := 0
	residentOpen = func(string) (*resident.Workbench, error) {
		calls++
		return nil, resident.ErrUnsupported
	}
	t.Cleanup(func() { residentOpen = previous })
	run := startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	address := run.address(t)
	if got := runCLI(t, root, "add", "Added with no resident"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	response, err := http.Get("http://" + address + "/cards")
	if err != nil {
		t.Fatal(err)
	}
	read, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !strings.Contains(string(read), "Added with no resident") {
		t.Error("a head with no resident did not draw what the disk holds")
	}
	run.cancel()
	got := run.wait(t)
	if calls != 1 || got.code != 0 || got.errw != "" || strings.Count(got.out, "\n") != 1 {
		t.Errorf("wanted one attempt, exit 0 and the one line, got %d attempts, %d, stdout %q, stderr %q", calls, got.code, got.out, got.errw)
	}
}
