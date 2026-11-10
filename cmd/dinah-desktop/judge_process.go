package main

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/judge"
	"studio/internal/sessionhost"
)

// A provider's running process may be started with rules that let its agent act
// without asking, because the person chose that for their chats. A judgement call
// on that process would run under the same rules, and nothing the client refuses
// would stop it, since an agent that is allowed to act asks the client nothing.
// Such a provider judges in a process of its own, started with rules that refuse
// every tool (docs/plans/judgement-agents.md, A Process Only for Judging). The
// process starts with the first call, serves each call in a session of its own, and
// ends when no call has used it for judgeProcessIdle, so that it costs nothing while
// nothing is judged.

// judgeProcessIdle is how long the process is kept after its last call. A burst of
// judgements, such as the titles of several new chats, then starts it once.
var judgeProcessIdle = 5 * time.Minute

// judgeProcessInitWithin is how long the process may take to say it is ready.
const judgeProcessInitWithin = time.Minute

// judgeProcessStart runs a provider's agent for judging in dir, an empty folder
// made for it, with the handler given, under rules that refuse every tool.
type judgeProcessStart func(a *App, dir string, handler acp.Handler) (agentConnection, error)

// processJudgeModule is the judge module of a provider that judges in a process of
// its own. calibration is why its models are not calibrated, or "" when they are.
func processJudgeModule(start judgeProcessStart, note, calibration string) *judgeModule {
	open := func(a *App, provider string) (judgeLink, error) {
		return a.judgeProcessOf(provider, start).acquire(a)
	}
	return &judgeModule{
		Build: func(a *App, provider, model string) judge.Judge {
			return &acpJudge{app: a, provider: provider, model: model, link: open, unmeasured: true}
		},
		Installed: func(a *App, ctx context.Context, provider string) ([]judge.Model, error) {
			return acpJudgeModels(a, ctx, provider, open)
		},
		Available:     acpJudgeAvailable,
		Note:          note,
		NoCalibration: calibration,
	}
}

// judgeProcess is one provider's process for judging, when it runs.
type judgeProcess struct {
	start judgeProcessStart

	mu       sync.Mutex
	conn     agentConnection
	init     acp.InitializeResult
	sessions *sessionhost.JudgeSessions
	dir      string
	calls    int
	idle     *time.Timer
	closed   bool
}

// judgeProcessOf is the provider's process for judging, which may not be running.
func (a *App) judgeProcessOf(provider string, start judgeProcessStart) *judgeProcess {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.processes == nil {
		j.processes = map[string]*judgeProcess{}
	}
	if j.processes[provider] == nil {
		j.processes[provider] = &judgeProcess{start: start}
	}
	return j.processes[provider]
}

// stopJudgeProcesses ends every process for judging, for good: the window is
// closing.
func (a *App) stopJudgeProcesses() {
	j := &a.judging
	j.mu.Lock()
	processes := make([]*judgeProcess, 0, len(j.processes))
	for _, p := range j.processes {
		processes = append(processes, p)
	}
	j.mu.Unlock()
	for _, p := range processes {
		p.mu.Lock()
		p.closed = true
		p.endLocked()
		p.mu.Unlock()
	}
}

// acquire gives a call the process's connection, starting the process when it is
// not running. Calls that arrive while it starts wait for the one start.
func (p *judgeProcess) acquire(a *App) (judgeLink, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return judgeLink{}, errors.New("the window is closing")
	}
	if p.conn != nil {
		select {
		case <-p.conn.Done():
			p.endLocked()
		default:
		}
	}
	if p.conn == nil {
		if err := p.startLocked(a); err != nil {
			return judgeLink{}, err
		}
	}
	if p.idle != nil {
		p.idle.Stop()
		p.idle = nil
	}
	p.calls++
	conn := p.conn
	var once sync.Once
	return judgeLink{conn: conn, init: p.init, sessions: p.sessions, release: func() { once.Do(func() { p.release(conn) }) }}, nil
}

// release says a call is over. The last one starts the wait after which the
// process ends.
func (p *judgeProcess) release(conn agentConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != conn {
		return
	}
	p.calls--
	if p.calls > 0 {
		return
	}
	p.idle = time.AfterFunc(judgeProcessIdle, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.conn == conn && p.calls == 0 {
			p.endLocked()
		}
	})
}

// startLocked starts the process in an empty folder and waits until it is ready.
// The client offers the agent nothing: no file access and no terminal.
func (p *judgeProcess) startLocked(a *App) error {
	dir, err := os.MkdirTemp("", "dinah-judge-")
	if err != nil {
		return err
	}
	sessions := sessionhost.NewJudgeSessions()
	sessions.EndOnToolCall = true
	conn, err := p.start(a, dir, &sessionhost.JudgeHandler{Sessions: sessions})
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), judgeProcessInitWithin)
	defer cancel()
	var init acp.InitializeResult
	err = conn.Call(ctx, "initialize", acp.InitializeParams{
		ProtocolVersion:    1,
		ClientCapabilities: acp.ClientCapabilities{},
		ClientInfo:         &acp.Implementation{Name: "dinah", Title: "Dinah", Version: appVersion},
	}, &init)
	if err != nil {
		conn.Close()
		_ = os.RemoveAll(dir)
		return err
	}
	p.conn, p.init, p.sessions, p.dir, p.calls = conn, init, sessions, dir, 0
	return nil
}

// endLocked ends the process and removes its folder.
func (p *judgeProcess) endLocked() {
	if p.idle != nil {
		p.idle.Stop()
		p.idle = nil
	}
	if p.conn == nil {
		return
	}
	p.conn.Close()
	_ = os.RemoveAll(p.dir)
	p.conn, p.sessions, p.dir, p.calls = nil, nil, "", 0
}
