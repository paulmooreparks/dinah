package main

import (
	"regexp"
	"sort"
	"strings"

	"studio/internal/sessionhost"
)

// The assistant's actions on chats and teams (docs/mockups/window-assistant,
// Paul's rulings of 20 November 2026).
//
// Besides the windows, the assistant is told of the chats, each with its
// provider, its team and its state, of the teams by name, and of the
// providers a chat can be switched to. It can then say three more things to
// do: switch chats to another provider, stop the turns of chats, and send a
// team a message in its group chat. The page carries each out by the
// program's own operation for it, the one a person uses by hand.
//
// A request that says "team" must say a team that exists, by its name. With
// no such team nothing is done and the answer lists the teams, since a typed
// request is carried out at once.

// AssistantChat is one chat as the assistant is told of it.
type AssistantChat struct {
	ID       string `json:"id" doc:"the chat's identity, which an answer names it by"`
	Title    string `json:"title" doc:"the chat's title"`
	Provider string `json:"provider" doc:"the identity of the provider the chat uses"`
	Team     string `json:"team,omitempty" doc:"the name of the team the chat is a member of, when it is in one"`
	State    string `json:"state" doc:"working for a chat in a turn, waiting for a chat that waits for the person, idle for a chat whose turn has ended, closed for a chat that is not open"`
	// ProviderName is the provider's name, which the engine adds for the
	// page; the assistant is told the identity only.
	ProviderName string `json:"providerName,omitempty"`
}

// AssistantProvider is one provider a chat can be switched to.
type AssistantProvider struct {
	ID   string `json:"id" doc:"the provider's identity, which an answer names it by"`
	Name string `json:"name" doc:"the provider's name"`
}

const (
	assistantSwitch = "switch" // switch these chats to the provider in Provider
	assistantStop   = "stop"   // stop the turns of these chats
	assistantTell   = "tell"   // send Text to the team in Team, in its group chat

	assistantChatLimit = 80
	assistantTextLimit = 2000
)

func assistantChatState(c SavedChatInfo) string {
	st := c.State
	switch {
	case st != nil && st.Life == sessionhost.LifeClosed, st == nil && !c.Open:
		return "closed"
	case st != nil && st.Exec != nil:
		switch st.Exec.State {
		case "working", "background":
			return "working"
		case "waiting", "stuck":
			return "waiting"
		}
		return "idle"
	case c.Working:
		return "working"
	}
	return "idle"
}

// assistantChats are the chats the assistant is told of, those in a team
// first and then those that are open, and the group chat of each team by
// the team's name. A team is a group chat with a lead, and its name is the
// group's title.
func (a *App) assistantChats() ([]AssistantChat, map[string]string) {
	groups := map[string]string{}
	teamOf := map[string]string{}
	if over, err := a.BusOverview(); err == nil {
		for _, room := range over.Rooms {
			if room.Lead == "" || strings.TrimSpace(room.Title) == "" {
				continue
			}
			if _, taken := groups[room.Title]; taken {
				continue
			}
			groups[room.Title] = room.Name
			for _, m := range room.Members {
				if m.Chat != "" && teamOf[m.Chat] == "" {
					teamOf[m.Chat] = room.Title
				}
			}
		}
	}
	var chats []AssistantChat
	if inv, err := a.SavedChats(); err == nil {
		for _, c := range inv.Chats {
			if c.ID == "" || (c.State != nil && c.State.Life == sessionhost.LifeRetired) {
				continue
			}
			chat := AssistantChat{ID: c.ID, Title: c.Title, Provider: c.Provider, Team: teamOf[c.ID], State: assistantChatState(c)}
			if chat.Team == "" && chat.State == "closed" {
				continue
			}
			chats = append(chats, chat)
		}
	}
	sort.SliceStable(chats, func(i, j int) bool { return chats[i].Team != "" && chats[j].Team == "" })
	if len(chats) > assistantChatLimit {
		chats = chats[:assistantChatLimit]
	}
	return chats, groups
}

func assistantProviders() []AssistantProvider {
	out := make([]AssistantProvider, 0, len(product.Providers))
	for _, id := range product.Providers {
		out = append(out, AssistantProvider{ID: id, Name: providerName(id)})
	}
	return out
}

var (
	assistantTeamWord   = regexp.MustCompile(`\bteams?\b`)
	assistantSwitchAsk  = regexp.MustCompile(`\b(?:change|switch|move)\b.*?\b(?:from (\S+) )?to (\S+)`)
	assistantStopAsk    = regexp.MustCompile(`\bstop\b`)
	assistantTellAsk    = regexp.MustCompile(`(?i)^\s*(?:tell|ask)\s+(?:the\s+)?.+?\s+team\s+(?:to\s+|that\s+)?(.+?)\s*$`)
	assistantEveryChats = regexp.MustCompile(`\b(?:every|all)(?: the)? chats?\b`)
)

// assistantChatRule reads a request about chats and teams without a judge,
// and says false when the request is not one.
func assistantChatRule(in assistantIn) (AssistantPlan, bool) {
	say := func(text string) (AssistantPlan, bool) {
		return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: text}, true
	}
	ask := strings.ToLower(in.Ask)
	// The team is the one whose name the request has, the longest such name.
	team := ""
	for _, name := range in.Teams {
		if strings.Contains(ask, strings.ToLower(name)) && len(name) > len(team) {
			team = name
		}
	}
	provider := func(word string) string {
		word = strings.Trim(word, ".,;:!?\"'")
		for _, p := range in.Providers {
			if word != "" && (word == strings.ToLower(p.ID) || word == strings.ToLower(p.Name)) {
				return p.ID
			}
		}
		return ""
	}
	sw := assistantSwitchAsk.FindStringSubmatch(ask)
	to := ""
	if sw != nil {
		to = provider(sw[2])
	}
	tell := assistantTellAsk.FindStringSubmatch(in.Ask)
	stop := assistantStopAsk.MatchString(ask)
	if to == "" && tell == nil && !stop {
		return AssistantPlan{}, false
	}
	if assistantTeamWord.MatchString(ask) && team == "" {
		if len(in.Teams) == 0 {
			return say("There are no teams, so nothing is done.")
		}
		return say("No team is called that, so nothing is done. The teams are: " + strings.Join(in.Teams, ", ") + ".")
	}
	var mine []AssistantChat
	for _, c := range in.Chats {
		if team == "" || c.Team == team {
			mine = append(mine, c)
		}
	}
	ids := func(list []AssistantChat) []string {
		out := make([]string, 0, len(list))
		for _, c := range list {
			out = append(out, c.ID)
		}
		return out
	}
	of := ""
	if team != "" {
		of = " of the " + team + " team"
	}
	switch {
	case to != "":
		from := provider(sw[1])
		if team == "" && from == "" {
			return say("Say which chats to switch: a team, or the provider they use now.")
		}
		var list []AssistantChat
		for _, c := range mine {
			if c.Provider != to && (from == "" || c.Provider == from) {
				list = append(list, c)
			}
		}
		if len(list) == 0 {
			return say("No chat" + of + " is one to switch, so nothing is done.")
		}
		return AssistantPlan{Do: assistantSwitch, Windows: []string{}, Chats: ids(list), Provider: to, Team: team, Say: "Switching the chats" + of + "."}, true
	case tell != nil && team != "":
		return AssistantPlan{Do: assistantTell, Windows: []string{}, Team: team, Text: tell[1], Say: "Sending that to the " + team + " team."}, true
	case stop && (team != "" || assistantEveryChats.MatchString(ask)):
		var list []AssistantChat
		for _, c := range mine {
			if c.State == "working" {
				list = append(list, c)
			}
		}
		if len(list) == 0 {
			return say("No chat" + of + " is working, so there is nothing to stop.")
		}
		return AssistantPlan{Do: assistantStop, Windows: []string{}, Chats: ids(list), Team: team, Say: "Stopping the chats" + of + "."}, true
	}
	return AssistantPlan{}, false
}

// assistantChatCheck refuses an answer about chats that names a chat, a
// provider or a team the assistant was not told of.
func assistantChatCheck(in assistantIn, out AssistantPlan) string {
	chat := map[string]bool{}
	for _, c := range in.Chats {
		chat[c.ID] = true
	}
	for _, id := range out.Chats {
		if !chat[id] {
			return "the answer names a chat it was not told of: " + id
		}
	}
	isTeam := func(name string) bool {
		for _, t := range in.Teams {
			if t == name {
				return true
			}
		}
		return false
	}
	switch out.Do {
	case assistantSwitch:
		known := false
		for _, p := range in.Providers {
			known = known || p.ID == out.Provider
		}
		if !known || len(out.Chats) == 0 {
			return "the answer switches no chat, or to a provider it was not told of"
		}
	case assistantStop:
		if len(out.Chats) == 0 {
			return "the answer stops no chat"
		}
	case assistantTell:
		text := strings.TrimSpace(out.Text)
		if !isTeam(out.Team) || text == "" || len([]rune(text)) > assistantTextLimit {
			return "the answer sends nothing, too much, or to a team it was not told of"
		}
	default:
		if len(out.Chats) > 0 || out.Provider != "" || out.Text != "" {
			return "the answer names chats for something that is not done to chats"
		}
	}
	if out.Team != "" && !isTeam(out.Team) {
		return "the answer names a team it was not told of: " + out.Team
	}
	return ""
}

// assistantChatFacts adds to a plan about chats what the page needs to say
// and do it: each chat as it is, the provider's name and the team's group
// chat.
func assistantChatFacts(plan *AssistantPlan, in assistantIn, groups map[string]string) {
	by := map[string]AssistantChat{}
	for _, c := range in.Chats {
		by[c.ID] = c
	}
	plan.ChatFacts = []AssistantChat{}
	list := plan.Chats
	if plan.Do == assistantTell {
		// A message goes to the whole team, which the answer says by name.
		list = nil
		for _, c := range in.Chats {
			if c.Team == plan.Team {
				list = append(list, c.ID)
			}
		}
	}
	for _, id := range list {
		if c, ok := by[id]; ok {
			c.ProviderName = providerName(c.Provider)
			plan.ChatFacts = append(plan.ChatFacts, c)
		}
	}
	if plan.Provider != "" {
		plan.ProviderName = providerName(plan.Provider)
	}
	plan.Group = groups[plan.Team]
}
