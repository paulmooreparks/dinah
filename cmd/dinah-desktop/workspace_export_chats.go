package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

// A workspace export takes the workspace's chats, their saved histories, the
// groups and teams they belong to and the groups' conversations
// (docs/design/workspace-export.md, "Chats, Their History and Their
// Groups"). Export Workspace… first plans the export, so that the window can
// list the chats for the person to untick, and then writes the archive with
// the chats the person kept (internal/workspaces, archive.go).
//
// The chats are found as the Chats list finds them (saved_chats.go), from
// master's logical chat records: every chat whose folder is in the
// workspace, open or closed, hidden ones included, and never one in the
// Recycle Bin. A chat that needs its ownership repaired, or has no saved
// history, cannot be exported and is left out.

// WorkspaceExportPlan is what an export would take, for the export window.
// Pins counts the pins that are not chat pins; a chat pin goes with its chat.
type WorkspaceExportPlan struct {
	WorkspaceID string                  `json:"workspaceId"`
	Name        string                  `json:"name"`
	Sources     []WorkspaceExportSource `json:"sources"`
	Pins        int                     `json:"pins"`
	Chats       []WorkspaceExportChat   `json:"chats"`
	Groups      []WorkspaceExportGroup  `json:"groups"`
	Teams       []WorkspaceExportTeam   `json:"teams"`
}

// WorkspaceExportSource is a source that goes with an export.
type WorkspaceExportSource struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Main bool   `json:"main"`
}

// WorkspaceExportChat is a chat the export can take. Size is the size of its
// saved history in bytes. Open says whether it is open in this window.
type WorkspaceExportChat struct {
	ChatID       string `json:"chatId"`
	Title        string `json:"title"`
	Provider     string `json:"provider"`
	ProviderName string `json:"providerName"`
	Model        string `json:"model,omitempty"`
	Size         int64  `json:"size"`
	Open         bool   `json:"open"`
	Hidden       bool   `json:"hidden"`
}

// WorkspaceExportMember is a chat a group or a team has as a member. Here
// says it is one of the plan's chats; otherwise Workspace is the name of
// the workspace it is in.
type WorkspaceExportMember struct {
	ChatID    string `json:"chatId"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Here      bool   `json:"here"`
}

// WorkspaceExportGroup is a group with at least one member among the plan's
// chats. It goes when one of those is ticked.
type WorkspaceExportGroup struct {
	Title   string                  `json:"title"`
	Members []WorkspaceExportMember `json:"members"`
}

// WorkspaceExportTeam is a lead and the members it leads, with at least one
// of each pair among the plan's chats. A pair goes when either chat is
// ticked, and the team's budget goes when the lead is.
type WorkspaceExportTeam struct {
	Lead    WorkspaceExportMember   `json:"lead"`
	Members []WorkspaceExportMember `json:"members"`
}

// WorkspaceExportChoice is the person's choice in the export window: the
// logical chat IDs of the chats to take. A chat not listed stays behind.
type WorkspaceExportChoice struct {
	Chats []string `json:"chats"`
}

// exportChat is a chat as an export sees it. Stages are the stages that
// make up the chat, oldest first, ending with its head; sessions are every
// session recorded for it, which chat pins and bus identities may refer to.
type exportChat struct {
	chatID      string
	head        string
	stages      []string
	sessions    []string
	provider    string
	title       string
	personTitle string
	judgedTitle string
	folder      string
	ownerID     string
	ownerName   string
	hidden      bool
	updatedAt   string
}

// workspaceChats is what an export reads of the window's state: the chats
// of one workspace, every chat a group or team could refer to, and which
// chat each bus identity belongs to.
type workspaceChats struct {
	record      WorkspaceRecord
	main        string
	saved       Settings
	chats       []*exportChat
	byID        map[string]*exportChat
	byRecipient map[string]string
}

// ownerOfFolder is the workspace a chat's folder belongs to: the one whose
// main folder holds it most closely.
func ownerOfFolder(records []WorkspaceRecord, folder string) (WorkspaceRecord, bool) {
	best, found := WorkspaceRecord{}, false
	for _, record := range records {
		main := workspaces.ExecutionPath(record)
		if main == "" || !(layout.SamePath(main, folder) || layout.Inside(main, folder)) {
			continue
		}
		if !found || len(main) > len(workspaces.ExecutionPath(best)) {
			best, found = record, true
		}
	}
	return best, found
}

// readWorkspaceChats finds the chats as the Chats list does, with hidden
// ones included, and keeps those of the workspace that have a saved
// history. saved is a snapshot the caller owns.
func readWorkspaceChats(saved Settings, headers map[string]*sessionhost.TranscriptHead, record WorkspaceRecord) *workspaceChats {
	saved.logicalEvidence = map[string]sessionhost.TranscriptHead{}
	for id, header := range headers {
		saved.logicalEvidence[id] = *header
	}
	out := &workspaceChats{record: record, main: workspaces.ExecutionPath(record), saved: saved, byID: map[string]*exportChat{}, byRecipient: map[string]string{}}
	continued := continuedConversations(headers, saved)
	unfinished := unfinishedTargets(headers, saved)
	notListed := hiddenChats(headers, saved)
	aliases := map[string]string{}
	for _, reference := range logicalChatReferencesFromSettings(saved) {
		members, err := logicalChatMembersFromSettings(saved, reference)
		if err != nil {
			continue
		}
		metadata, err := logicalChatMetadataFromSettings(saved, reference)
		if err != nil || metadata.Lifecycle != "active" || metadata.Workspace == "" {
			continue
		}
		head := members.Head
		if sessionhost.NotChat(reference) || sessionhost.NotChat(head) {
			continue
		}
		removed := false
		if _, held := saved.RemovedChats[reference]; held {
			removed = true
		}
		for _, member := range members.Members {
			if _, held := saved.RemovedChats[member.SessionID]; held {
				removed = true
			}
		}
		if header := headers[head]; removed || (header != nil && header.Deleted) || continued[head] || unfinished[head] {
			continue
		}
		id := members.ChatID
		if id == "" {
			id = reference
		}
		if out.byID[id] != nil {
			continue
		}
		chat := &exportChat{chatID: id, head: head, folder: metadata.Workspace, title: metadata.Title, personTitle: saved.SessionTitles[head],
			judgedTitle: saved.JudgedTitles[head].Title, hidden: notListed(reference) || notListed(head), updatedAt: saved.ListedChats[head].UpdatedAt}
		chat.provider = saved.SessionProviders[head]
		if chat.provider == "" {
			chat.provider = saved.ListedChats[head].Provider
		}
		if chat.provider == "" {
			chat.provider = saved.logicalEvidence[head].Provider
		}
		for _, member := range members.Members {
			chat.sessions = append(chat.sessions, member.SessionID)
			aliases[member.SessionID] = id
			if member.State == "active_head" || member.State == "committed_predecessor" {
				chat.stages = append(chat.stages, member.SessionID)
			}
		}
		if len(chat.stages) == 0 || chat.stages[len(chat.stages)-1] != head {
			chat.stages = append(chat.stages, head)
		}
		if owner, ok := ownerOfFolder(saved.WorkspaceRecords, chat.folder); ok {
			chat.ownerID, chat.ownerName = owner.ID, owner.Name
		}
		out.byID[id] = chat
		if chat.ownerID != record.ID || chat.provider == "" {
			continue
		}
		if info, err := os.Stat(sessionhost.TranscriptPath(configDir(), head)); err != nil || !info.Mode().IsRegular() {
			continue
		}
		out.chats = append(out.chats, chat)
	}
	sort.Slice(out.chats, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, out.chats[i].updatedAt)
		right, _ := time.Parse(time.RFC3339Nano, out.chats[j].updatedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return out.chats[i].chatID < out.chats[j].chatID
	})
	// A bus identity shared by two chats says nothing about either, as in the
	// org chart (org_chart.go).
	conflicts := map[string]bool{}
	for session, recipient := range saved.BusSessions {
		chatID := aliases[session]
		if recipient == "" || chatID == "" {
			continue
		}
		if previous := out.byRecipient[recipient]; previous != "" && previous != chatID {
			conflicts[recipient] = true
		}
		out.byRecipient[recipient] = chatID
	}
	for recipient := range conflicts {
		delete(out.byRecipient, recipient)
	}
	return out
}

// shownTitle is the title the export window and the references show.
func (c *exportChat) shownTitle() string {
	if c.title != "" {
		return c.title
	}
	return "Untitled Chat"
}

// folderOf is a chat's folder relative to the main folder, separated by "/",
// or "" for the main folder itself.
func (x *workspaceChats) folderOf(chat *exportChat) string {
	if layout.SamePath(chat.folder, x.main) {
		return ""
	}
	rel, err := filepath.Rel(x.main, chat.folder)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// ref is the reference to a chat from a group, a team or a message, with
// its key when it is one of keys.
func (x *workspaceChats) ref(chatID string, keys map[string]string) workspaces.ChatRef {
	chat := x.byID[chatID]
	out := workspaces.ChatRef{ChatID: chatID, Key: keys[chatID]}
	if chat != nil {
		out.Title, out.Workspace = chat.shownTitle(), chat.ownerName
	}
	return out
}

// relatedGroup is a group with a member among the workspace's chats, and
// relatedTeam a lead with its members, by their logical chat IDs.
type relatedGroup struct {
	room    string
	title   string
	lead    string
	budget  int
	person  bool
	members []string
}

type relatedTeam struct {
	lead       string
	recipients []string
	members    []bus.TeamMember
	chats      []string
}

// related reads the groups and teams that have a member among chats, which
// are logical chat IDs. A bus identity that belongs to no chat this window
// lists, such as a chat that runs outside the window, cannot be referred to
// and is passed over.
func (x *workspaceChats) related(ctx context.Context, b *bus.Bus, chats map[string]bool) ([]relatedGroup, []relatedTeam, error) {
	relations, err := b.TeamRelations(ctx)
	if err != nil {
		return nil, nil, err
	}
	groups := []relatedGroup{}
	for _, room := range relations.Rooms {
		group := relatedGroup{room: room.Name, title: room.Title, lead: x.byRecipient[room.Lead]}
		seen, wanted := map[string]bool{}, chats[group.lead]
		for _, member := range room.Members {
			if member.Kind == "human" {
				group.person = true
				continue
			}
			id := x.byRecipient[member.Key]
			if member.Kind != "session" || id == "" || seen[id] {
				continue
			}
			seen[id] = true
			group.members = append(group.members, id)
			wanted = wanted || chats[id]
		}
		if !wanted {
			continue
		}
		full, err := b.Room(ctx, room.Name)
		if err != nil {
			return nil, nil, err
		}
		group.budget = full.Budget
		groups = append(groups, group)
	}
	byLead := map[string]*relatedTeam{}
	order := []string{}
	for _, member := range relations.Members {
		lead, chat := x.byRecipient[member.Lead], x.byRecipient[member.Session]
		if lead == "" || chat == "" || lead == chat || !(chats[lead] || chats[chat]) {
			continue
		}
		team := byLead[lead]
		if team == nil {
			team = &relatedTeam{lead: lead}
			byLead[lead] = team
			order = append(order, lead)
		}
		if !containsString(team.recipients, member.Lead) {
			team.recipients = append(team.recipients, member.Lead)
		}
		if containsString(team.chats, chat) {
			continue
		}
		team.chats = append(team.chats, chat)
		team.members = append(team.members, member)
	}
	teams := make([]relatedTeam, 0, len(order))
	for _, lead := range order {
		teams = append(teams, *byLead[lead])
	}
	return groups, teams, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// plan is the export window's plan for the workspace's chats. open says
// which sessions are open in this window.
func (x *workspaceChats) plan(ctx context.Context, b *bus.Bus, open map[string]bool) (WorkspaceExportPlan, error) {
	plan := WorkspaceExportPlan{WorkspaceID: x.record.ID, Name: x.record.Name, Sources: []WorkspaceExportSource{}, Chats: []WorkspaceExportChat{}, Groups: []WorkspaceExportGroup{}, Teams: []WorkspaceExportTeam{}}
	for _, source := range x.record.Sources {
		if workspaces.ValidKind(source.Kind) {
			plan.Sources = append(plan.Sources, WorkspaceExportSource{Name: source.Name, Kind: source.Kind, Main: source.Kind == workspaces.LocalFolder && layout.SamePath(source.Path, mainFolderOf(x.record))})
		}
	}
	for _, pin := range workspaces.BuildExport(x.record, "", "").Pins {
		if pin.Kind != "chat" {
			plan.Pins++
		}
	}
	here := map[string]bool{}
	for _, chat := range x.chats {
		here[chat.chatID] = true
		item := WorkspaceExportChat{ChatID: chat.chatID, Title: chat.shownTitle(), Provider: chat.provider, ProviderName: providerName(chat.provider), Model: savedModelName(x.saved, chat.provider, chat.head), Hidden: chat.hidden, Open: open[chat.head]}
		if info, err := os.Stat(sessionhost.TranscriptPath(configDir(), chat.head)); err == nil {
			item.Size = info.Size()
		}
		plan.Chats = append(plan.Chats, item)
	}
	member := func(id string) WorkspaceExportMember {
		ref := x.ref(id, nil)
		return WorkspaceExportMember{ChatID: id, Title: ref.Title, Workspace: ref.Workspace, Here: here[id]}
	}
	if len(here) == 0 {
		return plan, nil
	}
	groups, teams, err := x.related(ctx, b, here)
	if err != nil {
		return plan, err
	}
	for _, group := range groups {
		item := WorkspaceExportGroup{Title: group.title, Members: []WorkspaceExportMember{}}
		for _, id := range group.members {
			item.Members = append(item.Members, member(id))
		}
		plan.Groups = append(plan.Groups, item)
	}
	for _, team := range teams {
		item := WorkspaceExportTeam{Lead: member(team.lead), Members: []WorkspaceExportMember{}}
		for _, id := range team.chats {
			item.Members = append(item.Members, member(id))
		}
		plan.Teams = append(plan.Teams, item)
	}
	return plan, nil
}

// writeArchive writes the export of the workspace with the chats chosen,
// by logical chat ID, to w. file is where the archive is going, and home
// the person's home folder, which the local folders' locations are recorded
// from.
func (x *workspaceChats) writeArchive(ctx context.Context, w io.Writer, b *bus.Bus, chosen map[string]bool, file, home, exporter string) error {
	keys, sessionKeys := map[string]string{}, map[string]string{}
	var chats []*exportChat
	for _, chat := range x.chats {
		if !chosen[chat.chatID] {
			continue
		}
		chats = append(chats, chat)
		keys[chat.chatID] = fmt.Sprintf("c%d", len(chats))
		for _, session := range chat.sessions {
			sessionKeys[session] = keys[chat.chatID]
		}
	}
	export := workspaces.BuildExportWithChats(x.record, file, home, sessionKeys)
	archive := workspaces.NewArchiveWriter(w)
	for _, chat := range chats {
		tr, err := sessionhost.ReadTranscript(configDir(), chat.head)
		if err != nil {
			return fmt.Errorf("the saved history of %s could not be read: %w", chat.shownTitle(), err)
		}
		// The queue does not travel, so neither does its record of which
		// queued messages started.
		tr.StartedQueueIDs = nil
		data, err := json.Marshal(tr)
		if err != nil {
			return err
		}
		key := keys[chat.chatID]
		if err := archive.WriteHistory(key, data); err != nil {
			return err
		}
		export.Chats = append(export.Chats, workspaces.ExportChat{Key: key, ChatID: chat.chatID, Stages: append([]string{}, chat.stages...), Provider: chat.provider,
			Options: x.saved.SessionOptions[chat.head], Model: savedModelName(x.saved, chat.provider, chat.head), Title: chat.personTitle, JudgedTitle: chat.judgedTitle,
			Hidden: chat.hidden, Folder: x.folderOf(chat)})
	}
	if len(chats) > 0 {
		export.Exporter = exporter
		wanted := map[string]bool{}
		for id := range keys {
			wanted[id] = true
		}
		groups, teams, err := x.related(ctx, b, wanted)
		if err != nil {
			return err
		}
		for i, group := range groups {
			item := workspaces.ExportGroup{Key: fmt.Sprintf("g%d", i+1), Title: group.title, Budget: group.budget, Person: group.person, Members: []workspaces.ChatRef{}}
			if group.lead != "" {
				lead := x.ref(group.lead, keys)
				item.Lead = &lead
			}
			for _, id := range group.members {
				item.Members = append(item.Members, x.ref(id, keys))
			}
			messages, err := x.groupMessages(ctx, b, archive, group.room, keys)
			if err != nil {
				return err
			}
			if item.Messages = len(messages); item.Messages > 0 {
				if err := archive.WriteGroup(item.Key, messages); err != nil {
					return err
				}
			}
			export.Groups = append(export.Groups, item)
		}
		for _, team := range teams {
			item := workspaces.ExportTeam{Lead: x.ref(team.lead, keys), Members: []workspaces.ExportTeamMember{}}
			for i, member := range team.members {
				item.Members = append(item.Members, workspaces.ExportTeamMember{Chat: x.ref(team.chats[i], keys), Since: member.Since, Retired: member.Retired})
			}
			if keys[team.lead] != "" {
				for _, recipient := range team.recipients {
					allowances, err := b.TeamAllowances(ctx, recipient)
					if err != nil {
						return err
					}
					for _, a := range allowances {
						item.Budget = append(item.Budget, workspaces.ExportAllowance{Provider: a.Provider, Unit: a.Unit, Allowance: a.Allowance})
					}
				}
			}
			export.Teams = append(export.Teams, item)
		}
		if export.Links, err = x.links(ctx, b, chats); err != nil {
			return err
		}
	}
	return archive.Finish(export)
}

// links are the trusted links of the workspaces the bus knows the main
// folder and the exported chats' folders by, which an import lists as not
// imported.
func (x *workspaceChats) links(ctx context.Context, b *bus.Bus, chats []*exportChat) ([]workspaces.ExportLink, error) {
	names, folders := map[string]bool{}, []string{x.main}
	for _, chat := range chats {
		folders = append(folders, chat.folder)
	}
	for _, folder := range folders {
		name, err := b.WorkspaceNameAt(ctx, folder)
		if err != nil {
			return nil, err
		}
		if name != "" {
			names[name] = true
		}
	}
	all, err := b.Links(ctx)
	if err != nil {
		return nil, err
	}
	var out []workspaces.ExportLink
	for _, link := range all {
		if names[link.A] || names[link.B] {
			out = append(out, workspaces.ExportLink{A: link.A, B: link.B})
		}
	}
	return out, nil
}

// groupMessages reads a group's conversation for the archive, with the
// files attached to it that are still on the computer as they were sent.
func (x *workspaceChats) groupMessages(ctx context.Context, b *bus.Bus, archive *workspaces.ArchiveWriter, room string, keys map[string]string) ([]workspaces.GroupMessage, error) {
	messages, err := b.RoomMessages(ctx, room, workspaces.MaxGroupMessages)
	if err != nil {
		return nil, err
	}
	out := make([]workspaces.GroupMessage, 0, len(messages))
	for _, m := range messages {
		item := workspaces.GroupMessage{ID: m.ID, CreatedAt: m.CreatedAt, Kind: m.Kind, FromName: m.FromName, Text: m.Text, ReplyTo: m.ReplyTo, Version: m.Version, Verdict: m.Verdict}
		if m.FromSession == "" {
			item.Person = m.FromName == bus.Human
		} else if id := x.byRecipient[m.FromSession]; id != "" {
			from := x.ref(id, keys)
			item.From = &from
		}
		for _, mention := range m.Mentioned {
			if mention.Key == bus.Human {
				item.MentionsPerson = true
			} else if id := x.byRecipient[mention.Key]; id != "" {
				item.Mentions = append(item.Mentions, x.ref(id, keys))
			}
		}
		for _, att := range m.Attachments {
			entry := workspaces.GroupAttachment{Name: att.Rel, Size: att.Size, SHA256: att.SHA256, Mime: att.Mime}
			if !att.Removed {
				if data, ok := readAttachment(att); ok {
					if entry.File, err = archive.WriteFile(data); err != nil {
						return nil, err
					}
				}
			}
			item.Attachments = append(item.Attachments, entry)
		}
		out = append(out, item)
	}
	return out, nil
}

// readAttachment reads a file attached to a group message, when it is still
// a regular file of at most an export's limit with the content it was sent
// with.
func readAttachment(att bus.Attachment) ([]byte, bool) {
	info, err := os.Lstat(att.Path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > workspaces.MaxFileEntry {
		return nil, false
	}
	data, err := os.ReadFile(att.Path)
	if err != nil || len(data) > workspaces.MaxFileEntry {
		return nil, false
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), att.SHA256) {
		return nil, false
	}
	return data, true
}

// workspaceChatsSnapshot reads the workspace's chats from the window's
// settings, and says which sessions are open.
func (a *App) workspaceChatsSnapshot(id string) (*workspaceChats, map[string]bool, error) {
	record, err := a.workspaceSourceRecord(id)
	if err != nil {
		return nil, nil, err
	}
	a.mu.Lock()
	if a.startupErr != nil {
		err := a.startupErr
		a.mu.Unlock()
		return nil, nil, err
	}
	saved, err := a.settings.clone()
	open := map[string]bool{}
	if err == nil {
		for _, chat := range a.runtime.ConversationsLocked() {
			open[chat.ID()] = true
		}
	}
	a.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	return readWorkspaceChats(saved, headers, record), open, nil
}

// PlanWorkspaceExport says what exporting a workspace would take: its
// sources, its pins, its chats with their sizes, and the groups and teams
// they belong to.
func (a *App) PlanWorkspaceExport(id string) (WorkspaceExportPlan, error) {
	chats, open, err := a.workspaceChatsSnapshot(id)
	if err != nil {
		return WorkspaceExportPlan{}, err
	}
	var plan WorkspaceExportPlan
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		plan, err = chats.plan(context.Background(), b, open)
		return err
	})
	return plan, err
}

// ExportWorkspace asks where to save a workspace's export, suggesting its
// main folder, and saves it there with the chats the person chose. It
// returns the file, or nothing when the person cancelled.
func (a *App) ExportWorkspace(id string, choice WorkspaceExportChoice) (string, error) {
	record, err := a.workspaceSourceRecord(id)
	if err != nil {
		return "", err
	}
	dir := mainFolderOf(record)
	if dir == "" || !folderExists(dir) {
		dir, _ = userHome()
	}
	file, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export Workspace", DefaultDirectory: dir, DefaultFilename: exportFileName(record.Name), Filters: workspaceFileFilter()})
	if err != nil || file == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(file), workspaces.ExportExtension) {
		file += workspaces.ExportExtension
	}
	return file, a.exportWorkspaceTo(id, file, choice)
}

// exportWorkspaceTo writes a workspace's export, with the chats chosen, to
// file.
func (a *App) exportWorkspaceTo(id, file string, choice WorkspaceExportChoice) error {
	chats, _, err := a.workspaceChatsSnapshot(id)
	if err != nil {
		return err
	}
	chosen := map[string]bool{}
	for _, chatID := range choice.Chats {
		chosen[chatID] = true
	}
	exporter := ""
	if len(chosen) > 0 {
		if exporter, err = workspaces.ExporterIdentity(configDir()); err != nil {
			return err
		}
	}
	home, _ := userHome()
	return a.busHost().With(func(b *bus.Bus) error {
		return workspaces.WriteArchiveFile(file, func(w io.Writer) error {
			return chats.writeArchive(context.Background(), w, b, chosen, file, home, exporter)
		})
	})
}
