package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// What the page reads of teams (docs/plans/teams.md, teams-ui.md): the strip in a lead's window, the lead of a
// member, and the methods Paul changes a team with. The strip is numbered like a chat's information and sent as an
// event when anything it shows changes, so it follows a member that works.

// TeamChat is a chat in a team: its session on the bus, its chat ID in the window (empty for a session outside it)
// and its title as it is now.
type TeamChat struct {
	Bus   string `json:"bus"`
	Chat  string `json:"chat"`
	Title string `json:"title"`
}

// TeamMemberView is a member as the strip shows it. State is working, waiting (for its lead to relay), stuck, idle,
// closed or retired, and StateWhy says why in words, so the glyph is never the only fact. Task is the first line of
// the last brief or assignment the lead sent it, TaskMessage that message's number and Group the team group's key,
// so a task opens its full brief. StuckBelow counts the stuck members under it, nested ones included.
type TeamMemberView struct {
	TeamChat
	Workspace string `json:"workspace"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	// State, StateWhy and Open are the old vocabulary, kept until the page reads ChatState.
	State    string `json:"state"`
	StateWhy string `json:"stateWhy"`
	Task     string `json:"task"`
	// ChatState is the member's life, window and execution state (chat_state.go), and Counts those of the members
	// under it, nested ones included, when it leads a team of its own.
	ChatState
	Counts  *StateCounts `json:"counts,omitempty"`
	TaskMsg int64        `json:"taskMessage"`
	// TaskGroup is the key of the group the assignment was posted in, which can differ from the team's group
	// for a nested assignment.
	TaskGroup string `json:"taskGroup"`
	// Usage is what the member's own turns used in the period, per provider, with the turns that could not
	// be counted, so each figure says whether it is complete.
	Usage      []TeamUsageRow   `json:"usage"`
	Open       bool             `json:"open"`
	Retired    bool             `json:"retired"`
	StuckBelow int              `json:"stuckBelow"`
	Members    []TeamMemberView `json:"members"`
}

// TeamView is a lead's team: its channel's stable key, its members, and how many members under it are stuck.
type TeamView struct {
	Lead    TeamChat         `json:"lead"`
	Group   string           `json:"group"`
	Members []TeamMemberView `json:"members"`
	Stuck   int              `json:"stuck"`
	// Counts are how many of the team's members, nested ones included, are in each state, and the state the team
	// shows as a whole.
	Counts StateCounts `json:"counts"`
	// Budget is the team's allowances and use, per provider, over the Judgements budget's period.
	Budget   TeamBudget `json:"budget"`
	StateGen string     `json:"stateGen"`
	StateVer uint64     `json:"stateVer"`
}

// LeadView is a chat's lead as its information gives it: the lead and the team's group key, which the member's
// window opens with the member mentioned.
type LeadView struct {
	TeamChat
	Group   string `json:"group"`
	Retired bool   `json:"retired"`
}

const taskLineLimit = 140

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = text[:i]
	}
	if r := []rune(text); len(r) > taskLineLimit {
		text = string(r[:taskLineLimit]) + "…"
	}
	return text
}

func (a *App) teamChat(b *bus.Bus, ctx context.Context, busID string) TeamChat {
	c := TeamChat{Bus: busID, Chat: a.chatOfBus(busID)}
	if s, err := b.Session(ctx, busID); err == nil {
		c.Title = bus.DisplayTitle(s.Title, s.WorkspaceLabel)
	}
	return c
}

// lastTurnFailures are the errors the last turn of a chat ended in, so that a chat that failed is shown stuck until
// its next turn starts.
var lastTurnFailures = struct {
	sync.Mutex
	byChat map[string]string
}{byChat: map[string]string{}}

func noteTurnResult(chat string, err error) {
	lastTurnFailures.Lock()
	defer lastTurnFailures.Unlock()
	if err == nil {
		delete(lastTurnFailures.byChat, chat)
		return
	}
	lastTurnFailures.byChat[chat] = err.Error()
}

func turnFailure(chat string) string {
	lastTurnFailures.Lock()
	defer lastTurnFailures.Unlock()
	return lastTurnFailures.byChat[chat]
}

// memberState is a member's state in the old vocabulary, which the team page reads until it reads the chat state.
func (a *App) memberState(b *bus.Bus, ctx context.Context, m bus.TeamMember, st ChatState) (state, why string, open bool) {
	switch {
	case st.Life == lifeRetired:
		return "retired", "it has been retired and its session ended", false
	case st.Life == lifeClosed:
		return "closed", "its chat is not open", false
	}
	return st.Exec.State, st.Exec.Why, true
}

// teamMembers builds the members under a lead, nested ones under theirs, and counts the stuck ones.
func (a *App) teamMembers(b *bus.Bus, ctx context.Context, lead bus.Session, group string, seen map[string]bool) ([]TeamMemberView, StateCounts) {
	members, err := b.TeamMembers(ctx, lead.ID)
	if err != nil {
		return nil, StateCounts{}
	}
	var out []TeamMemberView
	var counts StateCounts
	for _, m := range members {
		if seen[m.Session] {
			continue
		}
		seen[m.Session] = true
		v := TeamMemberView{TeamChat: a.teamChat(b, ctx, m.Session), Retired: m.Retired, Members: []TeamMemberView{}, Usage: []TeamUsageRow{}}
		v.ChatState = a.chatState(ctx, b, m.Session, v.Chat, m.Retired, false)
		v.State, v.StateWhy, v.Open = a.memberState(b, ctx, m, v.ChatState)
		counts.Add(v.ChatState)
		if s, err := b.Session(ctx, m.Session); err == nil {
			v.Workspace = s.WorkspaceLabel
			if v.Workspace == "" {
				v.Workspace = s.WorkspaceName
			}
		}
		if conv := a.session(v.Chat); conv != nil {
			v.Provider = conv.Info().Provider
			for _, o := range conv.Options() {
				if o.Category == "model" {
					v.Model = judgeOptionString(o.CurrentValue)
				}
			}
		}
		if asg, ok, err := b.LastAssignment(ctx, lead.ID, m.Session); err == nil && ok {
			v.Task, v.TaskMsg, v.TaskGroup = firstLine(asg.Text), asg.Message, asg.Room
		}
		v.Usage = a.memberUsage(b, ctx, m.Session)
		if led, _ := b.IsLead(ctx, m.Session); led {
			if child, err := b.Session(ctx, m.Session); err == nil {
				sub, subCounts := a.teamMembers(b, ctx, child, "", seen)
				v.Members, v.StuckBelow = sub, subCounts.Stuck
				subCounts.Finish()
				v.Counts = &subCounts
				counts.Merge(subCounts)
			}
		}
		out = append(out, v)
	}
	return out, counts
}

// teamSnapshot builds a lead's team without a number.
func (a *App) teamSnapshot(b *bus.Bus, ctx context.Context, leadBus string) (TeamView, error) {
	lead, err := b.Session(ctx, leadBus)
	if err != nil {
		return TeamView{}, err
	}
	group, err := b.TeamGroup(ctx, leadBus)
	if err != nil {
		return TeamView{}, err
	}
	v := TeamView{Lead: a.teamChat(b, ctx, leadBus), Group: group}
	var counts StateCounts
	v.Members, counts = a.teamMembers(b, ctx, lead, group, map[string]bool{leadBus: true})
	counts.Finish()
	v.Counts, v.Stuck = counts, counts.Stuck
	if v.Budget, err = a.teamBudget(b, ctx, leadBus); err != nil {
		return TeamView{}, err
	}
	if v.Members == nil {
		v.Members = []TeamMemberView{}
	}
	return v, nil
}

// Team is a lead's team as the strip shows it, numbered. Its members are the chats the lead leads, nested teams
// under their leads.
func (a *App) Team(leadChat string) (TeamView, error) {
	a.teamsActive.Store(true)
	leadBus := a.knownBusID(leadChat)
	if leadBus == "" {
		// A chat that is not on the bus leads no one.
		v := TeamView{Lead: TeamChat{Chat: leadChat}, Members: []TeamMemberView{}}
		v.Budget.Rows = []TeamBudgetRow{}
		v.StateGen, v.StateVer = a.teamClock().Next()
		return v, nil
	}
	var v TeamView
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		v, err = a.teamSnapshot(b, context.Background(), leadBus)
		return err
	})
	if err != nil {
		return TeamView{}, err
	}
	v.StateGen, v.StateVer = a.teamClock().Next()
	return v, nil
}

// ChatLead is a chat's lead, or nil for a chat that has none, with the team group's key.
func (a *App) ChatLead(chat string) (*LeadView, error) {
	memberBus := a.knownBusID(chat)
	if memberBus == "" {
		return nil, nil
	}
	var out *LeadView
	err := a.busHost().With(func(b *bus.Bus) error {
		lead, retired, err := b.LeadOf(context.Background(), memberBus)
		if err != nil || lead == "" {
			return err
		}
		group, err := b.TeamGroup(context.Background(), lead)
		if err != nil {
			return err
		}
		out = &LeadView{TeamChat: a.teamChat(b, context.Background(), lead), Group: group, Retired: retired}
		return nil
	})
	return out, err
}

// SetTeamLead makes a chat a member of another's team, or moves it there; it is Paul's, since no agent moves a chat to
// another lead. It returns the team group's key.
func (a *App) SetTeamLead(memberChat, leadChat string) (string, error) {
	a.teamsActive.Store(true)
	if memberChat == "" || leadChat == "" {
		return "", errors.New("choose a chat and its lead")
	}
	memberBus, leadBus := a.knownBusID(memberChat), a.knownBusID(leadChat)
	if memberBus == "" || leadBus == "" {
		return "", errors.New("both chats must be open chats that " + product.Name + " knows on the bus")
	}
	var group string
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		group, err = b.SetLead(context.Background(), memberBus, leadBus)
		return err
	})
	if err == nil {
		a.teamsChanged()
	}
	return group, err
}

// ClearTeamLead turns a team member back into an ordinary chat that reaches the person directly.
func (a *App) ClearTeamLead(memberChat string) error {
	memberBus := a.knownBusID(memberChat)
	if memberBus == "" {
		return nil
	}
	err := a.busHost().With(func(b *bus.Bus) error { return b.ClearLead(context.Background(), memberBus) })
	if err == nil {
		a.teamsChanged()
	}
	return err
}

// MadeTeam is what making a group a team did: the members it made and the chats it left out, each with why.
type MadeTeam struct {
	Group   string     `json:"group"`
	Lead    string     `json:"lead"`
	Made    []TeamChat `json:"made"`
	LeftOut []LeftOut  `json:"leftOut"`
}

// LeftOut is a chat that making a group a team did not make a member.
type LeftOut struct {
	TeamChat
	Why string `json:"why"`
}

// MakeGroupATeam makes a group chat a team led by one of its chats: its other chats become members, except one that has
// another lead already, since a chat has only one.
func (a *App) MakeGroupATeam(group, leadChat string) (MadeTeam, error) {
	a.teamsActive.Store(true)
	group, err := groupName(group)
	if err != nil {
		return MadeTeam{}, err
	}
	var out MadeTeam
	leadBus := a.knownBusID(leadChat)
	if leadBus == "" {
		return MadeTeam{}, errors.New("the lead must be a chat that " + product.Name + " knows on the bus")
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		made, left, err := b.MakeGroupTeam(ctx, group, leadBus)
		if err != nil {
			return err
		}
		out = MadeTeam{Group: group, Lead: leadBus, Made: []TeamChat{}, LeftOut: []LeftOut{}}
		for _, m := range made {
			out.Made = append(out.Made, a.teamChat(b, ctx, m))
		}
		for _, l := range left {
			out.LeftOut = append(out.LeftOut, LeftOut{TeamChat: a.teamChat(b, ctx, l.Session), Why: l.Why})
		}
		return nil
	})
	if err == nil {
		a.teamsChanged()
	}
	return out, err
}

func (a *App) teamClock() *sessionhost.StateClock {
	a.teamMu.Lock()
	defer a.teamMu.Unlock()
	if a.teamClockValue == nil {
		a.teamClockValue = sessionhost.NewStateClock()
	}
	return a.teamClockValue
}

// teamsChanged says that something a strip shows may have changed. The strips are rebuilt a moment later, all
// together, and each whose content differs from the last sent is sent as a team:state event with the next number, so
// a working member updates its row and usage changes are not sent more than once a second.
func (a *App) teamsChanged() {
	if !a.teamsActive.Load() {
		return
	}
	a.teamMu.Lock()
	defer a.teamMu.Unlock()
	if a.teamTimer != nil || a.teamClosed {
		return
	}
	a.teamTimer = time.AfterFunc(teamEventDelay, a.sendTeamStates)
}

const teamEventDelay = time.Second

// closeTeams stops the strips being sent, as the app ends: a timer still pending then reads the bus no more.
func (a *App) closeTeams() {
	a.teamMu.Lock()
	a.teamClosed = true
	if a.teamTimer != nil {
		a.teamTimer.Stop()
		a.teamTimer = nil
	}
	a.teamMu.Unlock()
	// A pass that is running finishes before the bus is closed.
	a.teamRun.Lock()
	a.teamRun.Unlock()
}

// teamsOpen says the teams may still use the bus: not once the app has closed them.
func (a *App) teamsOpen() bool {
	a.teamMu.Lock()
	defer a.teamMu.Unlock()
	return !a.teamClosed
}

// refreshChatLeads sets each open chat's lead in its information, and sends the information again for a chat whose lead
// changed, so a member's window knows its team without asking.
func (a *App) refreshChatLeads() {
	a.mu.Lock()
	chats := a.runtime.ConversationsLocked()
	a.mu.Unlock()
	for _, s := range chats {
		lead, err := a.ChatLead(s.ID())
		if err != nil {
			continue
		}
		var ref *sessionhost.LeadRef
		if lead != nil {
			ref = &sessionhost.LeadRef{Bus: lead.Bus, Chat: lead.Chat, Title: lead.Title, Group: lead.Group, Retired: lead.Retired}
		}
		if s.SetLead(ref) {
			sessionhost.EmitSession(a.emit, s)
		}
	}
}

func (a *App) sendTeamStates() {
	a.teamRun.Lock()
	defer a.teamRun.Unlock()
	a.teamMu.Lock()
	a.teamTimer = nil
	closed := a.teamClosed
	a.teamMu.Unlock()
	if closed {
		return
	}
	var leads []string
	_ = a.busHost().With(func(b *bus.Bus) error {
		var err error
		leads, err = b.Leads(context.Background())
		return err
	})
	a.refreshChatLeads()
	for _, leadBus := range leads {
		var v TeamView
		err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			v, err = a.teamSnapshot(b, context.Background(), leadBus)
			return err
		})
		if err != nil {
			continue
		}
		data, _ := json.Marshal(v)
		a.teamMu.Lock()
		if a.teamSent == nil {
			a.teamSent = map[string]string{}
		}
		same := a.teamSent[leadBus] == string(data)
		a.teamSent[leadBus] = string(data)
		a.teamMu.Unlock()
		if same {
			continue
		}
		v.StateGen, v.StateVer = a.teamClock().Next()
		a.emit("team:state", v)
		a.warnOverAllowances(v, v.Lead.Title)
	}
}

// detectTeams finds out, once as the app starts, whether any team exists, so that the strips are kept
// up to date from then on.
func (a *App) detectTeams() {
	var leads []string
	_ = a.withTeamBus(func(b *bus.Bus) error {
		var err error
		leads, err = b.Leads(context.Background())
		return err
	})
	if len(leads) > 0 {
		a.teamsActive.Store(true)
		a.teamsChanged()
	}
}

// withTeamBus runs something on the bus for the teams, unless the app has closed them; a pass in progress finishes before the
// bus is closed, so a goroutine that outlives the app never opens it again.
func (a *App) withTeamBus(fn func(b *bus.Bus) error) error {
	a.teamRun.Lock()
	defer a.teamRun.Unlock()
	if !a.teamsOpen() {
		return nil
	}
	return a.busHost().With(fn)
}

// knownBusID is the session a chat has on the bus, or empty when it has none, which is not the same as asking for a new one.
func (a *App) knownBusID(chat string) string {
	if chat == "" {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.BusSessions[chat]
}
