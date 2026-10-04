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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/httphead"
	"dinah/internal/msg"
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
	// chdirs are the directories serveChdir was handed.
	chdirs []string
	// dir is the directory runCLI runs the command from.
	dir string
}

// sameDirectory reports whether two paths name one directory.
func sameDirectory(a, b string) bool {
	x, errX := os.Stat(a)
	y, errY := os.Stat(b)
	return errX == nil && errY == nil && os.SameFile(x, y)
}

// startServe runs dinah serve with argv through runCLI on a goroutine,
// handing it a context the test cancels and a listen function that records
// each address before doing what listen says. serveChdir is replaced by a
// recorder, so no test moves the test binary's working directory. The seams
// are restored when the test ends.
func startServe(t *testing.T, dir string, listen listenFunc, argv ...string) *serveRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &serveRun{cancel: cancel, done: make(chan invocation, 1), listening: make(chan string, 1), dir: dir}
	previousBase, previousListen, previousChdir := serveBase, serveListen, serveChdir
	serveChdir = func(dir string) error {
		run.mu.Lock()
		defer run.mu.Unlock()
		run.chdirs = append(run.chdirs, dir)
		return nil
	}
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
		serveBase, serveListen, serveChdir = previousBase, previousListen, previousChdir
	})
	go func() {
		run.done <- runCLI(t, dir, append([]string{"serve"}, argv...)...)
	}()
	return run
}

// readsClause is the clause the startup line carries when serve opens the
// platform's own resident: from memory on Windows, and from disk, naming the
// platform, everywhere else.
func readsClause() string {
	if runtime.GOOS == "windows" {
		return " (reads answered from memory)"
	}
	return " (reads answered from disk: this system has no file-change watcher Dinah uses)"
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

// address waits for the server to bind and returns the bound address. By
// then serveUntil has passed the step that moves its working directory, so it
// also checks that the test binary still stands where runCLI put it: a
// serveChdir that moved the process would have moved the test binary, and
// runCLI puts the directory back when the run returns, so TestMain's own
// comparison at the end of the run cannot see it.
func (r *serveRun) address(t *testing.T) string {
	t.Helper()
	select {
	case address := <-r.listening:
		if wd, err := os.Getwd(); err == nil && r.dir != "" && !sameDirectory(wd, r.dir) {
			t.Errorf("the test binary stands in %s while dinah serve runs from %s, so serveUntil moved the test binary's working directory", wd, r.dir)
		}
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
	want := "Serving " + absolute + " at http://" + address + "/" + readsClause() + "\n"
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
		Reads     string `json:"reads"`
	}
	err = json.Unmarshal([]byte(got.out), &announced)
	if err != nil || got.code != 0 || strings.Count(got.out, "\n") != 1 || announced.Workbench != absolute || announced.URL != "http://"+address+"/" || announced.Reads == "" {
		t.Errorf("--json: wanted exit 0 and one object with url, workbench and reads, got %d %q (%v)", got.code, got.out, err)
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
// resident is the platform watcher's. The resident reads the disk only while
// a request asks, so a goroutine sends GET / in a loop for the whole run,
// which keeps the resident reading as a page left open keeps a server busy.
// Each round adds a card, archives it, restores it and deletes it, and the
// test fails on the first act refused, naming the round, the act and the
// refusal.
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
	var stop atomic.Bool
	polled := make(chan int, 1)
	go func() {
		n := 0
		for !stop.Load() {
			if response, err := http.Get(f.url + "/"); err == nil {
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				n++
			}
		}
		polled <- n
	}()
	defer func() {
		if !stop.Load() {
			stop.Store(true)
			<-polled
		}
	}()
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
	stop.Store(true)
	requests := <-polled
	if publishes == 0 {
		t.Fatal("the resident published nothing while the acts ran, so it read nothing and the test proves nothing")
	}
	status, _, body := f.do(http.MethodGet, "/cards", "", "")
	if status != http.StatusOK {
		t.Fatalf("GET /cards after the rounds: %d %v", status, body)
	}
	t.Logf("%d rounds of add, archive, restore and delete from another process, %d requests and %d publishes read by the resident meanwhile", structuralRounds, requests, publishes)
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
		warmResident(t, w)
		cfg.Resident = w
	}
}

// warmResident is the first request after resident.Open, which builds the
// snapshot inside it; it fails the test unless that answers a snapshot and
// Ready is then closed.
func warmResident(t *testing.T, w *resident.Workbench) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if pick := w.Current(ctx, time.Now()); pick.Snapshot == nil {
		t.Fatal("the first request answered no snapshot")
	}
	select {
	case <-w.Ready():
	default:
		t.Fatal("the first request returned and Ready is not closed")
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
// until the resident has received a batch whose changes include the card's
// anchor, as Windows reports the write, and one GET then shows the new
// column, since that request asks for the pass that applies the change. The
// wait is on the received batch, against a deadline that bounds the test
// only, and nothing polls the head.
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
	received := make(chan resident.Batch, 1024)
	f := serveBench(t, root, withPlatformResident(t, &resident.Hooks{AfterReceive: func(b resident.Batch) {
		select {
		case received <- b:
		default:
		}
	}}))
	if code, stderr := cliMove(t, workbench, card, "review"); code != 0 {
		t.Fatalf("the CLI move: %d %s", code, stderr)
	}
	timeout := time.After(30 * time.Second)
	for waiting := true; waiting; {
		select {
		case b := <-received:
			for _, c := range b.Changes {
				if filepath.Clean(c.Path) == anchor {
					waiting = false
				}
			}
		case <-timeout:
			t.Fatalf("no notification naming %s was received within 30 seconds of the CLI move; Windows documents that the change will be reported and not how soon, so this is a finding", anchor)
		}
	}
	status, _, shown := f.do(http.MethodGet, "/cards/"+card, "", "")
	detail, _ := shown["detail"].(map[string]any)
	view, _ := detail["card"].(map[string]any)
	if status != http.StatusOK || view["column_title"] != "Review" {
		t.Errorf("GET /cards/%s after the publish: wanted the card in Review, got %d %v", card, status, view)
	}
}

// stepLog is one ordered list of what serveUntil's seams were handed.
type stepLog struct {
	mu    sync.Mutex
	steps []string
}

func (l *stepLog) add(step string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.steps = append(l.steps, step)
}

func (l *stepLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.steps...)
}

// indexOf answers where a step first stands in a list, or -1.
func indexOf(steps []string, prefix string) int {
	for i, step := range steps {
		if strings.HasPrefix(step, prefix) {
			return i
		}
	}
	return -1
}

// TestServeClosesItsResident is part of dinah-619/criteria/16. With
// serveWorkDir, serveChdir, residentOpen and openURL replaced by recorders
// appending to one list, serveUntil records one serveWorkDir call whose
// argument is the workbench root, then one serveChdir call whose argument is
// what serveWorkDir answered, both before the one residentOpen call; it opens
// once, hands the head the resident, and closes it after shutdown. Run as
// dinah ui, the list also shows serveChdir before openURL, which is the order
// that keeps the browser out of the folder the server was started in.
//
// Arming: moving the serveChdir call into afterListen puts it after
// residentOpen, and after openURL for dinah ui.
func TestServeClosesItsResident(t *testing.T) {
	for _, command := range []string{"serve", "ui"} {
		t.Run(command, func(t *testing.T) { serveClosesItsResident(t, command) })
	}
}

func serveClosesItsResident(t *testing.T, command string) {
	root := newBench(t)
	workbench := soleBenchDir(t, root)
	log := &stepLog{}
	var opened []*resident.Workbench
	ctx, cancel := context.WithCancel(context.Background())
	listening := make(chan string, 1)
	previousBase, previousListen, previousChdir := serveBase, serveListen, serveChdir
	previousWorkDir, previousOpen, previousURL := serveWorkDir, residentOpen, openURL
	serveBase = func() context.Context { return ctx }
	serveListen = func(network, address string) (net.Listener, error) {
		listener, err := net.Listen(network, address)
		if err == nil {
			listening <- listener.Addr().String()
		}
		return listener, err
	}
	answered := ""
	serveWorkDir = func(root string) (string, error) {
		dir, err := resident.WorkingDirectory(root)
		answered = dir
		log.add("workdir " + root)
		return dir, err
	}
	serveChdir = func(dir string) error {
		log.add("chdir " + dir)
		return nil
	}
	residentOpen = func(dir string) (*resident.Workbench, error) {
		log.add("open")
		w, err := resident.Open(dir, resident.Options{Notifier: residenttest.NewManual()})
		if err == nil {
			// The snapshot is built now, before the CLI adds a card below,
			// so a head reading the resident cannot draw that card.
			warmResident(t, w)
			opened = append(opened, w)
		}
		return w, err
	}
	openURL = func(url string) error {
		log.add("openURL")
		return nil
	}
	t.Cleanup(func() {
		cancel()
		serveBase, serveListen, serveChdir = previousBase, previousListen, previousChdir
		serveWorkDir, residentOpen, openURL = previousWorkDir, previousOpen, previousURL
	})
	done := make(chan invocation, 1)
	go func() { done <- runCLI(t, root, command, "--listen", "127.0.0.1:0") }()
	var address string
	select {
	case address = <-listening:
	case got := <-done:
		t.Fatalf("%s returned %d before it listened: %s", command, got.code, got.errw)
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not listen", command)
	}
	if len(opened) != 1 {
		t.Fatalf("%s opened %d residents, wanted one", command, len(opened))
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
		t.Errorf("%s: the head drew a card only the disk holds, so serve did not hand it the resident", command)
	}
	cancel()
	var got invocation
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not return", command)
	}
	if got.code != 0 || got.errw != "" || strings.Count(got.out, "\n") != 1 {
		t.Errorf("%s wanted exit 0 and its one line, got %d, stdout %q, stderr %q", command, got.code, got.out, got.errw)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if pick := opened[0].Current(ctx2, time.Now()); pick.Snapshot != nil {
		t.Errorf("%s: the resident still serves a snapshot after the command returned, so it was not closed", command)
	}
	steps := log.all()
	workdir, chdir, open := indexOf(steps, "workdir "), indexOf(steps, "chdir "), indexOf(steps, "open")
	switch {
	case workdir < 0 || !sameDirectory(strings.TrimPrefix(steps[workdir], "workdir "), workbench):
		// On macOS the temporary directory lies behind a symbolic link, and
		// serve resolves its root from the side discovery climbs, so the two
		// spellings are compared as directories.
		t.Errorf("%s: serveWorkDir was not handed the workbench root %s: %v", command, workbench, steps)
	case chdir < 0 || steps[chdir] != "chdir "+answered:
		t.Errorf("%s: serveChdir was not handed what serveWorkDir answered, %s: %v", command, answered, steps)
	case !(workdir < chdir && chdir < open):
		t.Errorf("%s: the steps ran as %v, wanted serveWorkDir, then serveChdir, then residentOpen", command, steps)
	}
	if command == "ui" {
		if url := indexOf(steps, "openURL"); url < 0 || url < chdir {
			t.Errorf("ui: the steps ran as %v, wanted serveChdir before openURL", steps)
		}
	}
}

// TestServeWithoutAResidentServesFromDisk is part of dinah-619/criteria/16.
// Where resident.Open answers an *Unsupported, serve serves from disk and
// writes nothing beyond its one line, which names the reason.
func TestServeWithoutAResidentServesFromDisk(t *testing.T) {
	root := newBench(t)
	previous := residentOpen
	calls := 0
	residentOpen = func(string) (*resident.Workbench, error) {
		calls++
		return nil, &resident.Unsupported{Why: resident.WhyMountedInFolder}
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
	if !strings.HasSuffix(got.out, " (reads answered from disk: the workbench's volume is mounted in a folder)\n") {
		t.Errorf("the startup line does not name the reason: %q", got.out)
	}
}

// TestServeStaysPutWhenNoVolumeRootIsConfirmed is part of
// dinah-619/criteria/16, and portable. With serveWorkDir answering a
// *resident.NoVolumeRoot and serveGetwd a fixed path, serveUntil calls no
// serveChdir, calls serveGetwd before residentOpen, serves, and ends the
// human startup line with the serve.workdir.stays clause naming the fixed
// path; the JSON line carries stays_in. With serveGetwd answering an error,
// serveUntil answers that error and nothing listens.
//
// Arming: answering a *NoVolumeRoot with reportError, as a refusal would,
// leaves no server to answer the GET /.
func TestServeStaysPutWhenNoVolumeRootIsConfirmed(t *testing.T) {
	root := newBench(t)
	const fixed = "/fixed/where/serve/started"
	log := &stepLog{}
	previousWorkDir, previousGetwd, previousOpen := serveWorkDir, serveGetwd, residentOpen
	serveWorkDir = func(string) (string, error) {
		return "", &resident.NoVolumeRoot{Tried: [2]resident.Refused{
			{Why: errors.New("the workbench's final path could not be read")},
			{Dir: `C:\`, Why: errors.New("its final path is not a bare volume GUID path")},
		}}
	}
	getwdErr := error(nil)
	serveGetwd = func() (string, error) {
		log.add("getwd")
		return fixed, getwdErr
	}
	residentOpen = func(string) (*resident.Workbench, error) {
		log.add("open")
		return nil, &resident.Unsupported{Why: resident.WhyPlatform}
	}
	t.Cleanup(func() { serveWorkDir, serveGetwd, residentOpen = previousWorkDir, previousGetwd, previousOpen })

	stays := " (working directory left at " + fixed + ": no volume root could be confirmed to move to, so " + fixed + " and the folders above it may stay locked while this runs)\n"
	run := startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	address := run.address(t)
	response, err := http.Get("http://" + address + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("GET / answered %d, wanted 200 from a server that stayed where it started", response.StatusCode)
	}
	run.cancel()
	got := run.wait(t)
	if got.code != 0 || !strings.HasSuffix(got.out, stays) {
		t.Errorf("the human startup line does not end with the stays clause: %d %q", got.code, got.out)
	}
	run.mu.Lock()
	chdirs := len(run.chdirs)
	run.mu.Unlock()
	if chdirs != 0 {
		t.Errorf("serveChdir was called %d times with no volume root confirmed", chdirs)
	}
	if steps := log.all(); len(steps) != 2 || steps[0] != "getwd" || steps[1] != "open" {
		t.Errorf("the steps ran as %v, wanted serveGetwd before residentOpen", steps)
	}

	run = startServe(t, root, net.Listen, "--json", "--listen", "127.0.0.1:0")
	run.address(t)
	run.cancel()
	got = run.wait(t)
	var announced map[string]any
	if err := json.Unmarshal([]byte(got.out), &announced); err != nil || announced["stays_in"] != fixed {
		t.Errorf("the JSON startup line does not carry stays_in %s: %q (%v)", fixed, got.out, err)
	}

	getwdErr = errors.New("getwd failed")
	run = startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	got = run.wait(t)
	if got.code == 0 || len(run.addresses()) != 0 || !strings.Contains(got.errw, "getwd failed") {
		t.Errorf("with serveGetwd failing, wanted a refusal naming it and nothing bound, got %d, %v, %q", got.code, run.addresses(), got.errw)
	}
}

// TestServeResolvesItsPathsBeforeLeavingItsDirectory is part of
// dinah-619/criteria/16. With DINAH_HOME set to a relative path, the
// Config.Home the head receives through serveHandler is absolute and names
// the directory the relative path named before the move.
//
// Arming: passing s.home unchanged hands the head the relative path.
func TestServeResolvesItsPathsBeforeLeavingItsDirectory(t *testing.T) {
	root := newBench(t)
	const relative = "relative-home"
	if err := os.MkdirAll(filepath.Join(root, relative), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DINAH_HOME", relative)
	var home string
	previous := serveHandler
	serveHandler = func(cfg httphead.Config) http.Handler {
		home = cfg.Home
		return httphead.Handler(cfg)
	}
	t.Cleanup(func() { serveHandler = previous })
	run := startServe(t, root, net.Listen, "--listen", "127.0.0.1:0")
	run.address(t)
	run.cancel()
	run.wait(t)
	if !filepath.IsAbs(home) {
		t.Fatalf("the head received the user base %q, which is not absolute", home)
	}
	got, err := os.Stat(home)
	if err != nil {
		t.Fatalf("the user base the head received, %s, does not exist: %v", home, err)
	}
	want, err := os.Stat(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(got, want) {
		t.Errorf("the head received the user base %s, which is not the directory %s named from %s", home, relative, root)
	}
}

// TestServeSaysWhereReadsAreAnswered is part of dinah-619/criteria/21. A table
// over announce's input: a resident opened, and residentOpen answering each
// of the four reasons and a plain error; and two cases where the server
// stayed where it started, one with a resident opened and one with the
// platform reason. Each human line is serve.listening followed by its clause,
// and each JSON line carries url, workbench, reads and, as the case needs,
// why, detail and stays_in. Every locale carries the new keys.
func TestServeSaysWhereReadsAreAnswered(t *testing.T) {
	const url, root, dir = "http://127.0.0.1:7340/", "/where/the/workbench/is", "/where/serve/started"
	stays := " (working directory left at " + dir + ": no volume root could be confirmed to move to, so " + dir + " and the folders above it may stay locked while this runs)"
	cases := []struct {
		name   string
		err    error
		stays  string
		clause string
		reads  string
		why    string
		detail string
	}{
		{"a resident opened", nil, "", " (reads answered from memory)", "memory", "", ""},
		{"no watcher on this platform", &resident.Unsupported{Why: resident.WhyPlatform}, "", " (reads answered from disk: this system has no file-change watcher Dinah uses)", "disk", "platform", ""},
		{"a volume that is not fixed", &resident.Unsupported{Why: resident.WhyVolumeType}, "", " (reads answered from disk: the workbench is not on a fixed local disk)", "disk", "volume-type", ""},
		{"a volume mounted in a folder", &resident.Unsupported{Why: resident.WhyMountedInFolder}, "", " (reads answered from disk: the workbench's volume is mounted in a folder)", "disk", "mounted-in-folder", ""},
		{"a volume with no drive path", &resident.Unsupported{Why: resident.WhyNoDOSPath}, "", " (reads answered from disk: Windows gives the workbench's volume no drive path)", "disk", "no-dos-path", ""},
		{"a plain error", errors.New("the volume root refused FILE_LIST_DIRECTORY"), "", " (reads answered from disk: the volume root refused FILE_LIST_DIRECTORY)", "disk", "error", "the volume root refused FILE_LIST_DIRECTORY"},
		{"stayed, with a resident", nil, dir, " (reads answered from memory)" + stays, "memory", "", ""},
		{"stayed, on this platform", &resident.Unsupported{Why: resident.WhyPlatform}, dir, " (reads answered from disk: this system has no file-change watcher Dinah uses)" + stays, "disk", "platform", ""},
	}
	if len(cases) != 8 {
		t.Fatalf("the table holds %d cases, wanted 8", len(cases))
	}
	for _, c := range cases {
		var human bytes.Buffer
		s := &session{r: msg.For(msg.Base), out: &human, format: formatHuman}
		s.announce(url, root, readsOf(c.err), c.stays)
		if want := "Serving " + root + " at " + url + c.clause + "\n"; human.String() != want {
			t.Errorf("%s: the human line is %q, wanted %q", c.name, human.String(), want)
		}
		var machine bytes.Buffer
		s = &session{r: msg.For(msg.Base), out: &machine, format: formatJSON}
		s.announce(url, root, readsOf(c.err), c.stays)
		var announced map[string]any
		if err := json.Unmarshal(machine.Bytes(), &announced); err != nil {
			t.Fatalf("%s: the JSON line does not parse: %q", c.name, machine.String())
		}
		want := map[string]any{"url": url, "workbench": root, "reads": c.reads}
		if c.why != "" {
			want["why"] = c.why
		}
		if c.detail != "" {
			want["detail"] = c.detail
		}
		if c.stays != "" {
			want["stays_in"] = c.stays
		}
		if fmt.Sprint(announced) != fmt.Sprint(want) {
			t.Errorf("%s: the JSON line is %v, wanted %v", c.name, announced, want)
		}
	}
	keys := []string{"serve.reads.memory", "serve.reads.disk", "serve.reads.why.platform", "serve.reads.why.volume-type",
		"serve.reads.why.mounted-in-folder", "serve.reads.why.no-dos-path", "serve.workdir.stays"}
	for _, key := range keys {
		if _, ok := msg.BaseEntry(key); !ok {
			t.Errorf("the base catalog carries no %s", key)
		}
	}
}
