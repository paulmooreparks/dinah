package lsp

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dinah/internal/bench"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// defaultPollSeconds is how often the server rereads the workbench for a
// change when nobody names an interval. It is a count of seconds rather than
// a duration, so the package declares exactly the two durations section 3.1
// permits and a reader counting them is not fooled by a third in a constant.
const defaultPollSeconds = 2

// minimumPollSeconds is the floor an interval is clamped to.
const minimumPollSeconds = 1

// completionCap is how many card candidates one completion answer carries.
// A workbench with thousands of cards must not send them all, and a list cut
// short says so through isIncomplete rather than pretending it is whole.
const completionCap = 200

// Options are what the command hands the server: the explicit pointers at a
// workbench, the two settings a client may also send, and the seams a test
// drives the poll loop through.
type Options struct {
	// Workbench is the --workbench value, naming a workbench directory
	// outright.
	Workbench string
	// Root is the --root value, a directory to search for a workbench
	// beneath.
	Root string
	// AnnotateProse is the --annotate-prose marker, the command-line
	// equivalent of the client setting for an editor that sends none.
	AnnotateProse bool
	// PollSeconds is the --poll-seconds value, zero when none was named.
	PollSeconds int
	// Getenv reads an environment variable, so a test drives the ladder
	// without touching the process environment.
	Getenv func(string) string
	// Wd is the process working directory, the last rung of the ladder.
	Wd string
	// Messages renders every person-facing string.
	Messages *msg.Renderer
	// Version is what the serverInfo member reports.
	Version string
	// Discover searches a directory for a workbench, which is the rung the
	// last three steps of the ladder share.
	Discover func(start string) (string, error)
	// Sleep is how the poll loop waits, so a test drives it on a clock of
	// its own.
	Sleep func(time.Duration)
	// Since reports the wall time a walk took, given when it started. It is
	// an observation of something that already happened and never a policy
	// about how long a value stays good.
	Since func(started time.Time) time.Duration
	// Now is the clock a walk is timed against.
	Now func() time.Time
	// Walk runs one change checkpoint, answering whether the workbench
	// moved. It is filled by the server and replaced by a test driving the
	// loop without a workbench under it.
	Walk func() bool
}

// document is one file the client has open: the text as last sent, and the
// annotation model computed from it.
type document struct {
	uri        string
	path       string
	languageID string
	text       string
	slots      []slot
	model      []annotation
}

// Server is one workbench served to one editor. Everything it holds is
// bounded by what the editor has open: the resolved workbench root, the text
// of each open document, the model computed from that text, and one change
// cursor.
//
// It holds no *bench.Bench across a change, re-opening on every tick that
// reports one, and it holds no lock, no open handle inside an entity
// directory and no working directory. It is a reader.
type Server struct {
	opts     Options
	conn     *conn
	messages *msg.Renderer

	// mu guards everything below it, which the request loop and the poll
	// loop both reach.
	mu sync.Mutex

	bench   *bench.Bench
	library *verb.Library
	// scopeRoot is the directory a document has to lie under to be looked
	// at, which is the repository for a contained workbench and the
	// workbench itself otherwise.
	scopeRoot string
	cursor    string

	docs     map[string]*document
	memo     map[memoKey]memoed
	retained map[memoKey]annotation

	annotateProse bool
	// interval is how long the loop sleeps between walks when a walk was
	// quicker than it. This is the first of the two durations section 3.1
	// permits, and it is configured rather than measured.
	interval time.Duration
	// walked is how long the walk that has just finished took. This is the
	// second of the two, and it is an observation of something that already
	// happened, read for one purpose only: deciding how long to sleep
	// before the next walk starts.
	walked time.Duration
	// slow reports whether the last completed walk took longer than the
	// interval in force when it finished, which is the state whose two
	// edges each send one warning.
	slow bool

	refreshSupport  bool
	configuration   bool
	warnedNoRefresh bool

	shuttingDown bool
	stop         chan struct{}
	stopped      sync.Once
}

// New builds a server over a pair of streams.
func New(opts Options, in io.Reader, out io.Writer) *Server {
	if opts.Getenv == nil {
		opts.Getenv = func(string) string { return "" }
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Since == nil {
		opts.Since = time.Since
	}
	if opts.Messages == nil {
		opts.Messages = msg.For(msg.Base)
	}
	seconds := opts.PollSeconds
	if seconds < minimumPollSeconds {
		seconds = defaultPollSeconds
	}
	return &Server{
		opts:          opts,
		conn:          newConn(in, out),
		messages:      opts.Messages,
		docs:          map[string]*document{},
		memo:          map[memoKey]memoed{},
		retained:      map[memoKey]annotation{},
		annotateProse: opts.AnnotateProse,
		interval:      time.Duration(seconds) * time.Second,
		stop:          make(chan struct{}),
	}
}

// Serve reads frames until the client closes the stream or sends exit.
func (s *Server) Serve() error {
	defer s.halt()
	for {
		read, err := s.conn.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if read.Method == "" {
			// A response to a request this server originated. Nothing here
			// waits on one, so there is nothing to deliver it to.
			continue
		}
		if err := s.handle(read); err != nil {
			return err
		}
		if read.Method == methodExit {
			return nil
		}
	}
}

// halt stops the poll loop, once, however the session ended.
func (s *Server) halt() {
	s.stopped.Do(func() { close(s.stop) })
}

// handle dispatches one message.
func (s *Server) handle(read *message) error {
	switch read.Method {
	case methodInitialize:
		return s.initialize(read)
	case methodInitialized:
		return nil
	case methodShutdown:
		s.mu.Lock()
		s.shuttingDown = true
		s.mu.Unlock()
		return s.conn.respond(read.ID, nil)
	case methodExit:
		s.halt()
		return nil
	case methodDidOpen:
		return s.didOpen(read)
	case methodDidChange:
		return s.didChange(read)
	case methodDidClose:
		return s.didClose(read)
	case methodDidSave:
		return nil
	case methodDidChangeConfiguration:
		return s.didChangeConfiguration(read)
	case methodHover:
		return s.conn.respond(read.ID, s.hover(read.Params))
	case methodDocumentLink:
		return s.conn.respond(read.ID, s.documentLinks(read.Params))
	case methodDefinition:
		return s.conn.respond(read.ID, s.definition(read.Params))
	case methodCompletion:
		return s.conn.respond(read.ID, s.completion(read.Params))
	case methodInlayHint:
		return s.conn.respond(read.ID, s.inlayHints(read.Params))
	case methodAnnotations:
		return s.conn.respond(read.ID, s.annotations(read.Params))
	}
	if read.ID == nil {
		// An unknown notification is ignored, which is what the base
		// protocol asks of a server that does not implement one.
		return nil
	}
	return s.conn.refuse(read.ID, codeMethodNotFound, read.Method)
}

// initialize resolves the workbench, declares what this server offers, and
// starts the poll loop.
func (s *Server) initialize(read *message) error {
	var params initializeParams
	if len(read.Params) > 0 {
		if err := json.Unmarshal(read.Params, &params); err != nil {
			return s.conn.refuse(read.ID, codeParseError, err.Error())
		}
	}
	s.mu.Lock()
	s.refreshSupport = params.Capabilities.Workspace.InlayHint.RefreshSupport
	s.configuration = params.Capabilities.Workspace.Configuration
	root := s.discover(params)
	s.openAt(root)
	resolved := s.bench != nil
	s.mu.Unlock()

	if err := s.conn.respond(read.ID, initializeResult{
		Capabilities: declaredCapabilities(),
		ServerInfo:   serverInfo{Name: "dinah", Version: s.opts.Version},
	}); err != nil {
		return err
	}
	if resolved {
		if err := s.log(messageTypeInfo, keyLogWorkbench, "root", root); err != nil {
			return err
		}
	} else if err := s.show(messageTypeWarning, keyNoWorkbench); err != nil {
		return err
	}
	if !s.refreshSupport {
		s.mu.Lock()
		first := !s.warnedNoRefresh
		s.warnedNoRefresh = true
		s.mu.Unlock()
		if first {
			if err := s.log(messageTypeInfo, keyLogNoRefreshSupport); err != nil {
				return err
			}
		}
	}
	if s.configuration {
		if err := s.conn.request(methodConfiguration, configurationParams{Items: []configurationItem{{Section: "dinah.lsp"}}}); err != nil {
			return err
		}
	}
	go s.poll()
	return nil
}

// declaredCapabilities is the whole of what this server offers. No
// diagnostic capability appears here, and none ever will on this card:
// dinah-264 owns diagnostics.
func declaredCapabilities() capabilities {
	declared := capabilities{
		TextDocumentSync:     syncOptions{OpenClose: true, Change: 1},
		HoverProvider:        true,
		DocumentLinkProvider: resolveOption{},
		DefinitionProvider:   true,
		CompletionProvider:   completionOptions{TriggerCharacters: []string{"/", "-", " "}},
		InlayHintProvider:    resolveOption{},
	}
	declared.TextDocumentSync.Save.IncludeText = false
	return declared
}

// log sends one line to the client's own log, drawing its text from the
// catalogue. Nothing this server tells a person is a Go string literal.
func (s *Server) log(kind int, key string, pairs ...string) error {
	return s.conn.notify(methodLogMessage, logMessageParams{Type: kind, Message: s.messages.T(key, pairs...)})
}

// show sends one line to the client's own message area, from the catalogue
// for the same reason.
func (s *Server) show(kind int, key string, pairs ...string) error {
	return s.conn.notify(methodShowMessage, logMessageParams{Type: kind, Message: s.messages.T(key, pairs...)})
}

// openAt opens a workbench at a resolved root, or clears the held one when
// nothing resolved. The caller holds the lock.
//
// A root that will not open leaves the server holding nothing rather than
// holding a half-read workbench, and the next tick tries again.
func (s *Server) openAt(root string) {
	if root == "" {
		s.bench, s.library, s.scopeRoot = nil, nil, ""
		return
	}
	opened, err := bench.Open(root)
	if err != nil {
		return
	}
	s.bench = opened
	s.library = verb.New(opened, "")
	s.scopeRoot = scopeRootOf(opened)
	s.memo = map[memoKey]memoed{}
}

// scopeRootOf answers the directory a document has to lie under to be looked
// at. A workbench contained in a repository is served across that whole
// repository, which is what lets a design document carry live references; a
// workbench in the user base is served across itself alone.
func scopeRootOf(opened *bench.Bench) string {
	if bench.Contained(opened.Root) {
		return filepath.Dir(filepath.Dir(opened.Root))
	}
	return opened.Root
}

// discover resolves exactly one workbench, by the ladder of section 5.1,
// taking the first rung that yields one. The caller holds the lock.
//
// The four explicit rungs are what an operator or a harness said, and they
// win over anything the client reported, which is the precedence anybody who
// passes a flag expects. The three client-supplied rungs differ only in where
// the search starts.
func (s *Server) discover(params initializeParams) string {
	if root := s.named(s.opts.Workbench); root != "" {
		return root
	}
	if root := s.searched(s.opts.Root); root != "" {
		return root
	}
	if root := s.searched(s.opts.Getenv("DINAH_LSP_ROOT")); root != "" {
		return root
	}
	if root := s.named(s.opts.Getenv("DINAH_WORKBENCH")); root != "" {
		return root
	}
	if root := s.searched(uriPath(params.RootURI)); root != "" {
		return root
	}
	if len(params.WorkspaceFolders) > 0 {
		if root := s.searched(uriPath(params.WorkspaceFolders[0].URI)); root != "" {
			return root
		}
	}
	return s.searched(s.opts.Wd)
}

// named accepts a directory that is itself a workbench, which is what
// --workbench and DINAH_WORKBENCH each name outright.
func (s *Server) named(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil || !bench.Exists(filepath.Join(abs, bench.WorkbenchAnchor)) {
		return ""
	}
	return abs
}

// searched runs the discovery walk from a directory, which is what the last
// three rungs of the ladder share.
func (s *Server) searched(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	search := s.opts.Discover
	if search == nil {
		search = defaultDiscover
	}
	root, err := search(dir)
	if err != nil {
		return ""
	}
	return root
}

// defaultDiscover is the walk bench.Discover runs: from the given directory
// toward the drive root, then the user base.
func defaultDiscover(start string) (string, error) {
	root, _, err := bench.Discover(start, "", "", "")
	return root, err
}
