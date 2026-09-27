//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/httphead"
	"dinah/internal/resident"
	"dinah/internal/verb"
	"golang.org/x/sys/windows"
)

// eventList is the one ordered list the resident's hooks and the head's
// observers append to, so a test can assert which of a pass's end and a
// request's answer came first.
type eventList struct {
	mu     sync.Mutex
	events []string
}

func (l *eventList) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

// since answers the events from index from on, and the index after them.
func (l *eventList) since(from int) ([]string, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events[from:]...), len(l.events)
}

// before reports whether the first event beginning with first stands before
// the first event beginning with second, both present.
func before(events []string, first, second string) bool {
	i, j := indexOf(events, first), indexOf(events, second)
	return i >= 0 && j >= 0 && i < j
}

// heldOnThePlatform is a serveBench option that opens a resident with the
// platform's own watcher and hooks appending to events, and does not warm it:
// the first request is the one that builds. The head's two observers append
// to the same list.
func heldOnThePlatform(t *testing.T, events *eventList) func(*httphead.Config) {
	return func(cfg *httphead.Config) {
		t.Helper()
		hooks := &resident.Hooks{
			BeforePass: func(p resident.Pass) { events.add(fmt.Sprintf("before-pass rebuild=%v", p.Rebuild)) },
			AfterPass: func(p resident.Pass, published bool) {
				events.add(fmt.Sprintf("after-pass rebuild=%v published=%v", p.Rebuild, published))
			},
			AfterPublish: func(p resident.Published) {
				events.add(fmt.Sprintf("publish rebuilt=%v paths=%s", p.Rebuilt, strings.Join(p.Paths, ",")))
			},
		}
		w, err := resident.Open(cfg.Root, resident.Options{Hooks: hooks})
		if err != nil {
			t.Fatalf("open the resident with the platform's watcher: %v", err)
		}
		t.Cleanup(func() { w.Close() })
		cfg.Resident = w
		cfg.ObserveSource = func(fromResident bool) {
			if fromResident {
				events.add("source resident")
			} else {
				events.add("source disk")
			}
		}
		cfg.ObserveFirstWrite = func() { events.add("first-write") }
	}
}

// initIn makes a workbench in container, as newBench does, and answers the
// workbench directory. newBench has set the environment.
func initIn(t *testing.T, container string) string {
	t.Helper()
	if err := os.MkdirAll(container, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, container, "init", "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init in %s: %d %s", container, got.code, got.errw)
	}
	return soleBenchDir(t, container)
}

// getFully sends GET to a URL and reads the whole response, answering its
// status and body. The caller's next statement runs after the response has
// been read in full.
func getFully(t *testing.T, target string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("read GET %s: %v", target, err)
	}
	return response.StatusCode, string(body)
}

// diskAnswer answers what a head with no resident answers for GET / at root,
// with the port and host a served head of the same configuration carries.
func diskAnswer(t *testing.T, root, served string) (int, string) {
	t.Helper()
	parsed, err := url.Parse(served)
	if err != nil {
		t.Fatal(err)
	}
	port := 0
	fmt.Sscanf(parsed.Port(), "%d", &port)
	h := httphead.Handler(httphead.Config{Root: root, Home: os.Getenv("DINAH_HOME"), DefaultActor: "alka", Host: "127.0.0.1", Port: port})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = parsed.Host
	request.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

// childEnvDiscovering is childEnv with DINAH_WORKBENCH removed, so the child
// finds its workbench by climbing from its working directory.
func childEnvDiscovering() []string {
	var env []string
	for _, entry := range childEnv("") {
		if name, _, _ := strings.Cut(entry, "="); !strings.EqualFold(name, "DINAH_WORKBENCH") {
			env = append(env, entry)
		}
	}
	return env
}

// realServeFrom runs the real binary's init and then serve with dir as the
// working directory and no DINAH_WORKBENCH, reads the startup line, sends one
// GET /, and answers the line and a function that stops the child through its
// own process handle. The caller renames right after this returns.
func realServeFrom(t *testing.T, dir string) (string, func()) {
	t.Helper()
	binary, err := dinahBinary()
	if err != nil {
		t.Fatalf("%v", err)
	}
	env := childEnvDiscovering()
	initCommand := exec.Command(binary, "init", "--slug", "fx", "--operator", "alka")
	initCommand.Dir, initCommand.Env = dir, env
	if out, err := initCommand.CombinedOutput(); err != nil {
		t.Fatalf("init from %s: %v %s", dir, err, out)
	}
	serve := exec.Command(binary, "serve", "--listen", "127.0.0.1:0")
	serve.Dir, serve.Env = dir, env
	stdout, err := serve.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := serve.Start(); err != nil {
		t.Fatalf("start serve from %s: %v", dir, err)
	}
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			serve.Process.Kill()
			serve.Wait()
		}
	}
	t.Cleanup(stop)
	lines := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		lines <- line
	}()
	var line string
	select {
	case line = <-lines:
	case <-time.After(60 * time.Second):
		t.Fatalf("serve from %s printed no startup line", dir)
	}
	at := strings.Index(line, " at http://")
	if at < 0 {
		t.Fatalf("the startup line names no address: %q", line)
	}
	address := strings.Fields(line[at+len(" at "):])[0]
	if status, body := getFully(t, address); status != http.StatusOK {
		t.Fatalf("GET / from the real binary answered %d %s", status, body)
	}
	return line, stop
}

// TestAFolderAboveAServedWorkbenchStaysRenamable is the operator's ruling on
// dinah-619/questions/1 as a test (dinah-619/criteria/18): folders above a
// served workbench stay renamable, movable and deletable while the server
// runs. Every rename and removal is called once, with no retry, because a
// retry would hide exactly the refusal the ruling forbids. "Immediately"
// means that the rename is the test's next statement after the response has
// been read in full.
//
// Arming, each recorded: the watch opened on the workbench root rather than
// the volume root makes step 1's rename fail with "Access is denied"; the
// third revision's Current, answering nil at once and leaving the rebuild to
// run after the request, fails step 1's order assertion; no serveChdir call
// fails step 6's rename where the working directory pins its folder; the
// lapse settle run after the mux returns fails step 7's order assertion; and
// the working directory taken from filepath.VolumeName(root) fails step 8's
// rename where the working directory pins its folder.
func TestAFolderAboveAServedWorkbenchStaysRenamable(t *testing.T) {
	newBench(t)
	tmp := t.TempDir()
	events := &eventList{}
	container := filepath.Join(tmp, "a", "b", "wb")
	root := initIn(t, container)
	if got := runCLI(t, container, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	f := serveBench(t, container, heldOnThePlatform(t, events))
	mark := 0

	// Step 1: the first request builds, and nothing stays open after it.
	status, _ := getFully(t, f.url+"/")
	renameErr := os.Rename(filepath.Join(tmp, "a"), filepath.Join(tmp, "a2"))
	got, next := events.since(mark)
	mark = next
	if status != http.StatusOK {
		t.Fatalf("step 1: the first GET / answered %d", status)
	}
	if !before(got, "after-pass rebuild=true", "source resident") {
		t.Errorf("step 1: the build's AfterPass did not precede the first request's answer from the resident: %v", got)
	}
	if renameErr != nil {
		t.Fatalf("step 1: renaming the folder above the workbench after the first request failed: %v", renameErr)
	}

	// Step 2: the path names nothing, and the request answers trunk's refusal.
	status, body := getFully(t, f.url+"/")
	wantStatus, wantBody := diskAnswer(t, root, f.url)
	got, next = events.since(mark)
	mark = next
	if status != wantStatus || body != wantBody {
		t.Errorf("step 2: GET / answered %d %s, and a head with no resident answers %d %s", status, body, wantStatus, wantBody)
	}
	if indexOf(got, "source disk") < 0 || indexOf(got, "before-pass") >= 0 {
		t.Errorf("step 2: wanted the request to read the disk with no pass started for it: %v", got)
	}

	// Step 3: the folder comes back, and the request that finds it rebuilds.
	moved := filepath.Join(tmp, "a2", "b", "wb", strings.TrimPrefix(root, container))
	opened, err := bench.Open(moved)
	if err != nil {
		t.Fatalf("open the workbench where it was moved: %v", err)
	}
	if response := verb.New(opened, os.Getenv("DINAH_HOME")).Do(&verb.Request{Verb: verb.Move, Actor: "alka", Card: "fx-1", Column: "doing"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("step 3: the second library's move: %+v", response)
	}
	if err := os.Rename(filepath.Join(tmp, "a2"), filepath.Join(tmp, "a")); err != nil {
		t.Fatalf("step 3: renaming the folder back failed: %v", err)
	}
	status, _ = getFully(t, f.url+"/")
	got, next = events.since(mark)
	mark = next
	if status != http.StatusOK || !before(got, "after-pass rebuild=true", "source resident") {
		t.Errorf("step 3: GET / answered %d, and the rebuild's AfterPass did not precede its answer from the resident: %v", status, got)
	}
	_, card := getFully(t, f.url+"/cards/fx-1")
	var shown struct {
		Detail struct {
			Card struct {
				ColumnTitle string `json:"column_title"`
			} `json:"card"`
		} `json:"detail"`
	}
	if err := json.Unmarshal([]byte(card), &shown); err != nil || shown.Detail.Card.ColumnTitle != "Doing" {
		t.Errorf("step 3: the card is not drawn in the column the second library moved it to (%v): %s", err, card)
	}

	// Step 4: a move into another folder, and back.
	if err := os.MkdirAll(filepath.Join(tmp, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(tmp, "a"), filepath.Join(tmp, "elsewhere", "a")); err != nil {
		t.Fatalf("step 4: moving the folder above the workbench into another folder failed: %v", err)
	}
	status, body = getFully(t, f.url+"/")
	if status != wantStatus || body != wantBody {
		t.Errorf("step 4: GET / answered %d %s after the move, wanted the refusal of step 2", status, body)
	}
	if err := os.Rename(filepath.Join(tmp, "elsewhere", "a"), filepath.Join(tmp, "a")); err != nil {
		t.Fatalf("step 4: moving the folder back failed: %v", err)
	}
	getFully(t, f.url+"/")
	// Step 5: immediately after the rebuilding request, a removal.
	removeErr := os.RemoveAll(filepath.Join(tmp, "a"))
	if removeErr != nil {
		t.Fatalf("step 5: removing the folder above the workbench failed: %v", removeErr)
	}
	status, body = getFully(t, f.url+"/")
	if status != wantStatus || body != wantBody {
		t.Errorf("step 5: GET / answered %d %s after the removal, wanted the refusal of step 2", status, body)
	}

	// Step 6: the real binary, standing where the operator stands.
	dir := filepath.Join(tmp, "c", "d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line, stop := realServeFrom(t, dir)
	renameErr = os.Rename(filepath.Join(tmp, "c"), filepath.Join(tmp, "c2"))
	stop()
	if !strings.Contains(line, "(reads answered from memory)") {
		t.Errorf("step 6: the startup line does not say reads are answered from memory: %q", line)
	}
	if renameErr != nil {
		t.Errorf("step 6: renaming the folder the real binary was started in, after its first request, failed: %v", renameErr)
	}

	t.Run("a lapse settle ends before the response begins", func(t *testing.T) {
		lapseSettleEndsBeforeTheResponse(t, tmp)
	})
	t.Run("a subst drive", func(t *testing.T) {
		substDriveStaysRenamable(t, tmp)
	})
}

// lapseSettleEndsBeforeTheResponse is step 7 of
// TestAFolderAboveAServedWorkbenchStaysRenamable.
func lapseSettleEndsBeforeTheResponse(t *testing.T, tmp string) {
	container := filepath.Join(tmp, "e", "f", "wb")
	root := initIn(t, container)
	if got := runCLI(t, container, "add", "A leased card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, container, "fx-1")
	home := os.Getenv("DINAH_HOME")
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if response := verb.New(opened, home).Do(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: "fx-1", Expires: time.Hour}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("claim: %+v", response)
	}
	opened, _ = bench.Open(root)
	resolved, err := opened.ResolveCard("fx-1")
	if err != nil {
		t.Fatal(err)
	}
	anchor := resolved.Card.AnchorPath()
	text, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(text), "claim_expires: "+resolved.Card.Expires, "claim_expires: "+bench.Stamp(time.Now().Add(-time.Hour)), 1)
	if rewritten == string(text) {
		t.Fatalf("the anchor carries no claim_expires to move into the past:\n%s", text)
	}
	if err := os.WriteFile(anchor, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	cardDir, err := filepath.Rel(root, resolved.Card.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cardDir = filepath.ToSlash(cardDir)
	events := &eventList{}
	f := serveBench(t, container, heldOnThePlatform(t, events))
	status, body := getFully(t, f.url+"/")
	renameErr := os.Rename(filepath.Join(tmp, "e"), filepath.Join(tmp, "e2"))
	got, _ := events.since(0)
	settled := -1
	for i, e := range got {
		if strings.HasPrefix(e, "publish rebuilt=false") && strings.Contains(e, cardDir) {
			settled = i
			break
		}
	}
	switch {
	case status != http.StatusOK:
		t.Errorf("step 7: GET / answered %d %s", status, body)
	case !before(got, "after-pass rebuild=true", "source disk"):
		t.Errorf("step 7: the build's AfterPass did not precede the request's choice of the disk: %v", got)
	case settled < 0 || settled < indexOf(got, "source disk"):
		t.Errorf("step 7: no publish naming the card's directory, %s, followed the request's choice of the disk: %v", cardDir, got)
	case indexOf(got[settled:], "after-pass rebuild=false") < 0 || indexOf(got, "first-write") < settled+indexOf(got[settled:], "after-pass rebuild=false"):
		t.Errorf("step 7: the settle's AfterPass did not precede the first byte of the response: %v", got)
	}
	if renameErr != nil {
		t.Errorf("step 7: renaming the folder above the workbench after the lapsing request failed: %v", renameErr)
	}
	reopened, err := bench.Open(filepath.Join(tmp, "e2", "f", "wb", strings.TrimPrefix(root, container)))
	if err != nil {
		t.Fatal(err)
	}
	card, err := reopened.ResolveCard("fx-1")
	if err != nil {
		t.Fatal(err)
	}
	if card.Card.State != contract.StateReady {
		t.Errorf("step 7: the card reads %s after the lapsing request, wanted ready", card.Card.State)
	}
}

// substDriveStaysRenamable is step 8 of
// TestAFolderAboveAServedWorkbenchStaysRenamable.
func substDriveStaysRenamable(t *testing.T, tmp string) {
	target := filepath.Join(tmp, "p", "g")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	used, err := windows.GetLogicalDrives()
	if err != nil {
		t.Skipf("GetLogicalDrives failed: %v", err)
	}
	letter := ""
	for c := 'Z'; c >= 'D'; c-- {
		if used&(1<<uint(c-'A')) == 0 {
			letter = string(c) + ":"
			break
		}
	}
	if letter == "" {
		t.Skip("no drive letter is free for subst")
	}
	if out, err := exec.Command("subst", letter, target).CombinedOutput(); err != nil {
		t.Skipf("subst %s %s was refused: %v %s", letter, target, err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		if out, err := exec.Command("subst", letter, "/D").CombinedOutput(); err != nil {
			t.Errorf("remove the subst drive %s: %v %s", letter, err, strings.TrimSpace(string(out)))
		}
	})
	// The control: with no server running, the folder the subst drive maps
	// renames away and back. If the mapping itself held it, this step could
	// say nothing about Dinah.
	for _, pair := range [][2]string{{"p", "p2"}, {"p2", "p"}} {
		if err := os.Rename(filepath.Join(tmp, pair[0]), filepath.Join(tmp, pair[1])); err != nil {
			t.Fatalf("step 8's control: renaming %s to %s with no server running failed, so the subst mapping holds the folder: %v", pair[0], pair[1], err)
		}
	}
	dir := filepath.Join(target, "h")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line, stop := realServeFrom(t, letter+`\h`)
	renameErr := os.Rename(filepath.Join(tmp, "p"), filepath.Join(tmp, "p2"))
	stop()
	if !strings.Contains(line, "(reads answered from memory)") {
		t.Errorf("step 8: the startup line does not say reads are answered from memory: %q", line)
	}
	if renameErr != nil {
		t.Errorf("step 8: renaming the folder above the subst drive's target, after the first request, failed: %v", renameErr)
	}
}
