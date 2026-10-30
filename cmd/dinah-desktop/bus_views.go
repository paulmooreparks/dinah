package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"studio/internal/bus"
)

// The Group Chats pane, its group chats and its threads draw from these
// calls (frontend/bus-applets.js). Paul is the sender of everything the
// window posts, as on the bus's own page. The pane learns of changes from
// the bus:changed event, which the watcher sends when the bus's change
// counter moves (watchBus).

// BusMessage is a message as the window shows it: a chat by its title as it
// is now, a workspace by its label, and the person by the person's name.
// FromSession is the sending chat's session on the bus, empty for the
// person, and FromPerson is set for the person's own messages. A message in
// a group chat has the group's key as Room and its name as RoomTitle, and
// Mentions are the members it mentions. Line is the sentence a group shows
// for a member who joined or left (bus.MemberKind).
type BusMessage struct {
	ID            int64           `json:"id"`
	CreatedAt     string          `json:"createdAt"`
	Kind          string          `json:"kind"`
	From          string          `json:"from"`
	FromSession   string          `json:"fromSession,omitempty"`
	FromPerson    bool            `json:"fromPerson"`
	FromWorkspace string          `json:"fromWorkspace"`
	To            string          `json:"to"`
	Room          string          `json:"room"`
	RoomTitle     string          `json:"roomTitle,omitempty"`
	Mentions      []BusMention    `json:"mentions"`
	Line          string          `json:"line,omitempty"`
	ReplyTo       int64           `json:"replyTo"`
	Text          string          `json:"text"`
	Version       string          `json:"version,omitempty"`
	Verdict       string          `json:"verdict,omitempty"`
	Attachments   []BusAttachment `json:"attachments"`
	Unread        bool            `json:"unread"`
	// Relay is set on a team member's message to the person that was rerouted to its lead: the
	// group shows it for the person, who is not notified (teams.go).
	Relay *BusRelay `json:"relay,omitempty"`
}

// BusRelay says a message is for the person through a lead: the lead's session and title, the
// group's key and the message's number, which is what a link to it opens.
type BusRelay struct {
	Lead      string `json:"lead"`
	LeadTitle string `json:"leadTitle"`
	Group     string `json:"group"`
	Message   int64  `json:"message"`
}

// BusMention is a member a group's message mentions: a chat's session and
// its title as it is now, or the person, whose ID is bus.Human and whose
// title is the person's name.
type BusMention struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// BusAttachment is a file sent with a message, as it was when sent. Rel is
// an agent's file's path in its workspace, or the name of a file the person
// attached. Mime is its type, when the bus knows it. A file the person
// attached is kept by the bus (Stored): URI is its address for opening it as
// a single chat's files open, Picture says whether the window shows it as a
// picture, from Src, and Removed is set once the person has removed it.
type BusAttachment struct {
	Index   int    `json:"index"`
	Path    string `json:"path"`
	Rel     string `json:"rel"`
	Size    int64  `json:"size"`
	Mime    string `json:"mime,omitempty"`
	Stored  bool   `json:"stored,omitempty"`
	Removed bool   `json:"removed,omitempty"`
	URI     string `json:"uri,omitempty"`
	Picture bool   `json:"picture,omitempty"`
	Src     string `json:"src,omitempty"`
}

// busAttachment is an attachment as the window shows it.
func busAttachment(id int64, a bus.Attachment) BusAttachment {
	out := BusAttachment{Index: a.Index, Path: a.Path, Rel: a.Rel, Size: a.Size, Mime: a.Mime, Stored: a.Stored, Removed: a.Removed}
	if a.Stored && !a.Removed {
		out.URI = fileURI(a.Path)
		if busPictureTypes[a.Mime] {
			out.Picture, out.Src = true, busAttachmentSrc(id, a.Index)
		}
	}
	return out
}

// BusRoom is a group chat: its key, which its window's address uses, its
// name, its members, when it was last spoken in and how many of its
// messages the person has not seen. FileLimit is the limit on the files in
// the person's message to it, the smallest among its member chats that have
// one (file_limits.go), or nil when none has. Budget is the number of turns
// agents may start in it between the person's posts, or NoTurnBudget in the
// overview for none, and Used how many they have started since the person
// last posted; the header shows them only when a budget is set.
type BusRoom struct {
	Name    string           `json:"name"`
	Title   string           `json:"title"`
	Members []BusGroupMember `json:"members"`
	LastAt  string           `json:"lastAt"`
	Unread  int              `json:"unread"`
	// Latest is the newest unread message, and absent when nothing is unread.
	Latest    *BusLatest      `json:"latest,omitempty"`
	FileLimit *GroupFileLimit `json:"fileLimit"`
	Budget    int             `json:"budget"`
	Used      int             `json:"used"`
	// Lead is the session on the bus of the chat whose team's channel the group is, or
	// empty for an ordinary group.
	Lead string `json:"lead,omitempty"`
	// Counts are how many of the group's chats are in each state, and the state the group shows as a whole.
	Counts *StateCounts `json:"counts,omitempty"`
}

// BusLatest is the newest unread message of a group as its unread marker shows
// it: its number, who sent it, and the one line that stands for it, which the
// page does not cut again (bus.NoticeLine).
type BusLatest struct {
	ID   int64  `json:"id"`
	From string `json:"from"`
	Line string `json:"line"`
}

// BusGroupMember is a member of a group chat as its window shows it. ID is
// a chat's session on the bus, or bus.Human for the person; Chat is the
// chat's ID in the window, empty for the person and for a session outside
// the window. State is working, open, background or closed, or person
// (busChatState).
type BusGroupMember struct {
	ID        string `json:"id"`
	Chat      string `json:"chat"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Address   string `json:"address"`
	// State is the old vocabulary, working, open, background or closed, kept until the page reads ChatState.
	State string `json:"state"`
	// ChatState is the member's life, window and execution state (chat_state.go).
	ChatState
}

// BusChat is a chat open on the bus. ID is its session on the bus, which
// nobody reads, and Chat its ID in the window; Title is its title as it is
// now, and Address how a message is sent to it, its title with its
// workspace.
type BusChat struct {
	ID      string `json:"id"`
	Chat    string `json:"chat"`
	Title   string `json:"title"`
	Address string `json:"address"`
}

// BusWorkspace is a workspace on the bus and the chats open in it. Name is
// its address on the bus and Label how a person reads it.
type BusWorkspace struct {
	Name  string    `json:"name"`
	Label string    `json:"label"`
	Path  string    `json:"path"`
	Chats []BusChat `json:"chats"`
}

// BusLink is a trusted link between two workspaces, by their names on the
// bus, with their labels to show. HopLimit is bus.NoHopLimit when the link
// has no limit on automatic handoffs.
type BusLink struct {
	A        string `json:"a"`
	B        string `json:"b"`
	ALabel   string `json:"aLabel"`
	BLabel   string `json:"bLabel"`
	HopLimit int    `json:"hopLimit"`
}

// BusOverview is what the Group Chats pane takes from the bus: Paul's inbox,
// newest first, the group chats, the workspaces and the chats open in them,
// and the links Paul trusts. The pane lists every chat, closed ones too,
// from the window's own list of chats. DefaultHops is the limit a new link
// is offered with, and NoHopLimit the value that turns a link's limit off.
type BusOverview struct {
	Inbox       []BusMessage   `json:"inbox"`
	Unread      int            `json:"unread"`
	Rooms       []BusRoom      `json:"rooms"`
	Workspaces  []BusWorkspace `json:"workspaces"`
	Links       []BusLink      `json:"links"`
	DefaultHops int            `json:"defaultHops"`
	DefaultMax  int            `json:"defaultMax"`
	NoHopLimit  int            `json:"noHopLimit"`
	// NoTurnBudget is the budget value of a group with no limit.
	NoTurnBudget int `json:"noTurnBudget"`
}

func busMessage(m bus.Message) BusMessage {
	person := m.FromSession == "" && m.FromName == bus.Human
	from := m.From
	if person {
		from = speakerName(bus.Human)
	}
	out := BusMessage{ID: m.ID, CreatedAt: m.CreatedAt, Kind: m.Kind, From: from, FromSession: m.FromSession, FromPerson: person,
		FromWorkspace: m.FromWorkspaceLabel, To: m.ToPerson, Room: m.Room, RoomTitle: m.RoomTitle, Mentions: []BusMention{},
		ReplyTo: m.ReplyTo, Text: m.Text, Version: m.Version, Verdict: m.Verdict, Attachments: []BusAttachment{}, Unread: m.Unread}
	if m.Kind == bus.MemberKind {
		out.Line = m.MemberLine(speakerName(bus.Human))
	}
	for _, x := range m.Mentioned {
		title := x.Title
		if x.Key == bus.Human {
			title = speakerName(bus.Human)
		}
		out.Mentions = append(out.Mentions, BusMention{ID: x.Key, Title: title})
	}
	for _, a := range m.Attachments {
		out.Attachments = append(out.Attachments, busAttachment(m.ID, a))
	}
	if m.ForPerson {
		out.Relay = &BusRelay{Lead: m.RelayLead, LeadTitle: m.RelayLeadTitle, Group: m.Room, Message: m.ID}
	}
	return out
}

func busMessages(ms []bus.Message) []BusMessage {
	out := make([]BusMessage, 0, len(ms))
	for _, m := range ms {
		out = append(out, busMessage(m))
	}
	return out
}

func labelOr(labels map[string]string, name string) string {
	if l := labels[name]; l != "" {
		return l
	}
	return name
}

// busInboxLimit is how many of Paul's messages the pane lists.
const busInboxLimit = 200

// BusOverview returns the pane's contents without marking anything read.
func (a *App) BusOverview() (BusOverview, error) {
	var out BusOverview
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		inbox, err := b.HumanInbox(ctx, busInboxLimit, false)
		if err != nil {
			return err
		}
		out.Inbox = busMessages(inbox)
		for _, m := range inbox {
			if m.Unread {
				out.Unread++
			}
		}
		rooms, err := b.Rooms(ctx)
		if err != nil {
			return err
		}
		out.Rooms = make([]BusRoom, 0, len(rooms))
		for _, r := range rooms {
			out.Rooms = append(out.Rooms, a.busRoom(b, r))
		}
		ws, err := b.Workspaces(ctx)
		if err != nil {
			return err
		}
		ss, err := b.Sessions(ctx)
		if err != nil {
			return err
		}
		live := map[string][]BusChat{}
		for _, s := range ss {
			live[s.Workspace] = append(live[s.Workspace], BusChat{ID: s.ID, Chat: a.chatOfBus(s.ID), Title: s.Display(), Address: s.Address()})
		}
		labels := map[string]string{}
		out.Workspaces = make([]BusWorkspace, 0, len(ws))
		for _, w := range ws {
			labels[w.Name] = w.Display()
			chats := append([]BusChat{}, live[w.Key]...)
			out.Workspaces = append(out.Workspaces, BusWorkspace{Name: w.Name, Label: w.Display(), Path: w.Path, Chats: chats})
		}
		sort.SliceStable(out.Workspaces, func(i, j int) bool {
			return strings.ToLower(out.Workspaces[i].Label) < strings.ToLower(out.Workspaces[j].Label)
		})
		links, err := b.Links(ctx)
		if err != nil {
			return err
		}
		out.Links = make([]BusLink, 0, len(links))
		for _, l := range links {
			out.Links = append(out.Links, BusLink{A: l.A, B: l.B, ALabel: labelOr(labels, l.A), BLabel: labelOr(labels, l.B), HopLimit: l.HopLimit})
		}
		return nil
	})
	out.DefaultHops, out.DefaultMax, out.NoHopLimit = bus.DefaultLinkHops, bus.MaxHops, bus.NoHopLimit
	out.NoTurnBudget = bus.NoTurnBudget
	return out, err
}

// BusGroupBudget sets how many turns agents may start in a group chat
// between the person's posts, or bus.NoTurnBudget for no limit, and refills
// it. Only the person sets it, from the group's settings; agents have no tool
// for it.
func (a *App) BusGroupBudget(group string, budget int) error {
	group, err := groupName(group)
	if err != nil {
		return err
	}
	return a.busHost().With(func(b *bus.Bus) error { return b.SetGroupBudget(context.Background(), group, budget) })
}

// BusTrust makes or changes the trusted link between two workspaces, with
// its limit on automatic handoffs, or bus.NoHopLimit for none. The page
// warns the person before it asks for no limit.
func (a *App) BusTrust(first, second string, hopLimit int) error {
	return a.busHost().With(func(b *bus.Bus) error { return b.Trust(context.Background(), first, second, hopLimit) })
}

// BusUntrust removes the trusted link between two workspaces.
func (a *App) BusUntrust(first, second string) error {
	return a.busHost().With(func(b *bus.Bus) error { return b.Untrust(context.Background(), first, second) })
}

// MarkBusInboxRead marks every message to Paul read.
func (a *App) MarkBusInboxRead() error {
	return a.busHost().With(func(b *bus.Bus) error {
		_, err := b.HumanInbox(context.Background(), 1, true)
		return err
	})
}

// busRoomLimit is how much of a group's conversation its window shows.
const busRoomLimit = 300

// busRoom is a group chat as the window shows it, each member chat with its
// ID in the window and its state.
func (a *App) busRoom(b *bus.Bus, r bus.Room) BusRoom {
	out := BusRoom{Name: r.Name, Title: r.Title, Members: []BusGroupMember{}, LastAt: r.LastAt, Unread: r.Unread, Budget: r.Budget, Used: r.Used, Lead: r.Lead}
	ctx := context.Background()
	var states []ChatState
	for _, m := range r.Members {
		if m.Kind == "human" {
			out.Members = append(out.Members, BusGroupMember{ID: bus.Human, Title: speakerName(bus.Human), State: "person"})
			continue
		}
		chat := a.chatOfBus(m.Key)
		_, retired, _ := b.LeadOf(ctx, m.Key)
		st := a.chatState(ctx, b, m.Key, chat, retired, m.Open)
		states = append(states, st)
		out.Members = append(out.Members, BusGroupMember{ID: m.Key, Chat: chat, Title: m.Title, Workspace: m.Workspace,
			Address: m.Address, State: a.busChatState(chat, m.Open), ChatState: st})
	}
	counts := countStates(states)
	out.Counts = &counts
	if r.NewestID != 0 {
		out.Latest = &BusLatest{ID: r.NewestID, From: speakerName(r.NewestFrom), Line: r.NewestLine}
	}
	out.FileLimit = a.groupFileLimit(out.Members)
	return out
}

func groupName(group string) (string, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return "", errors.New("say which group chat")
	}
	return group, nil
}

// BusGroup returns a group chat, by its key or its name.
func (a *App) BusGroup(group string) (BusRoom, error) {
	group, err := groupName(group)
	if err != nil {
		return BusRoom{}, err
	}
	var out BusRoom
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.Room(context.Background(), group)
		out = a.busRoom(b, r)
		return err
	})
	return out, err
}

// BusGroupMessages returns a group chat's conversation, oldest first, with
// the lines saying who joined and left.
func (a *App) BusGroupMessages(group string) ([]BusMessage, error) {
	group, err := groupName(group)
	if err != nil {
		return nil, err
	}
	var out []BusMessage
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.Room(context.Background(), group)
		if err != nil {
			return err
		}
		ms, err := b.RoomMessages(context.Background(), r.Name, busRoomLimit)
		out = busMessages(ms)
		return err
	})
	return out, err
}

// BusMarkGroupRead records that the person has seen a group chat's
// messages, as its window does while it is open.
func (a *App) BusMarkGroupRead(group string) error {
	return a.busHost().With(func(b *bus.Bus) error { return b.MarkRoomRead(context.Background(), group) })
}

// BusStartGroup makes a group chat of the person and the chats chosen in
// the picker, open or closed, and returns it. A group given no name takes
// one from its chats' titles.
func (a *App) BusStartGroup(name string, chats []BusChatRef) (BusRoom, error) {
	ids, err := a.busChats(chats)
	if err != nil {
		return BusRoom{}, err
	}
	var out BusRoom
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.CreateGroup(context.Background(), bus.Sender{}, name, ids)
		out = a.busRoom(b, r)
		return err
	})
	return out, err
}

// BusAddToGroup puts the chats chosen in the picker in a group chat.
func (a *App) BusAddToGroup(group string, chats []BusChatRef) error {
	ids, err := a.busChats(chats)
	if err != nil {
		return err
	}
	return a.busHost().With(func(b *bus.Bus) error { return b.AddToGroup(context.Background(), bus.Sender{}, group, ids) })
}

// BusRemoveFromGroup takes a chat, by its session on the bus, out of a
// group chat.
func (a *App) BusRemoveFromGroup(group, member string) error {
	return a.busHost().With(func(b *bus.Bus) error { return b.RemoveFromGroup(context.Background(), bus.Sender{}, group, member) })
}

// BusRenameGroup gives a group chat a new title, which only the person may do. The
// group keeps its members, messages, budget and unread marks, messages delivered
// afterwards use the new title, and a chat that addresses the group by its old title
// is refused with the new one (bus.RenameGroup).
func (a *App) BusRenameGroup(group, title string) (BusRoom, error) {
	group, err := groupName(group)
	if err != nil {
		return BusRoom{}, err
	}
	var out BusRoom
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.RenameGroup(context.Background(), bus.Sender{}, group, title)
		out = a.busRoom(b, r)
		return err
	})
	return out, err
}

// BusDeleteGroup removes a group chat; what was said in it stays.
func (a *App) BusDeleteGroup(group string) error {
	return a.busHost().With(func(b *bus.Bus) error { return b.DeleteRoom(context.Background(), group) })
}

// BusSendToGroup posts the person's message in a group chat, waking the
// members it mentions, each by its session on the bus (bus.SendMentioning),
// with the files the person attached, which the bus keeps a copy of
// (bus.SendFiles). The files come as a single chat's pasted and dropped
// files do (Attachment of kind file). A message with more files than the
// group's limit (groupFileLimit) is refused with the member that set it, as
// the group's box refuses the file past the limit.
func (a *App) BusSendToGroup(group, text string, mentions []string, files []Attachment) (BusMessage, error) {
	kept, err := personFiles(files)
	if err != nil {
		return BusMessage{}, err
	}
	var out BusMessage
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.Room(context.Background(), group)
		if err != nil {
			return err
		}
		if limit := a.busRoom(b, r).FileLimit; limit != nil && len(kept) > limit.Limit {
			return fmt.Errorf("the message has %d files, and %s", len(kept), limit.Reason)
		}
		m, err := b.SendFiles(context.Background(), "#"+r.Name, text, kept, 0, mentions)
		out = busMessage(m)
		return err
	})
	return out, err
}

// personFiles decodes the files the person attached in the window. Only
// files go with a group's message, and one over the limit is refused with
// the reason, as a single chat refuses it.
func personFiles(files []Attachment) ([]bus.PersonFile, error) {
	var out []bus.PersonFile
	for _, f := range files {
		if f.Kind != "file" {
			return nil, fmt.Errorf("%s cannot go with a group's message; attach files from this computer", f.Name)
		}
		data, err := base64.StdEncoding.DecodeString(f.Data)
		if err != nil {
			return nil, fmt.Errorf("%s could not be read: %v", f.Name, err)
		}
		if len(data) > maxAttachmentFile {
			return nil, bus.TooLarge(f.Name)
		}
		out = append(out, bus.PersonFile{Name: f.Name, Mime: f.MimeType, Data: data})
	}
	return out, nil
}

// busAttachmentSrc is the address the window shows a picture the person
// attached from (busAttachmentFiles).
func busAttachmentSrc(id int64, index int) string {
	return "/bus-attachment?message=" + strconv.FormatInt(id, 10) + "&index=" + strconv.Itoa(index)
}

// busAttachmentFiles serves the pictures the person attached to bus
// messages, at /bus-attachment?message=<id>&index=<n>, from the bus's own
// copies. Only a kept copy that has not been removed and is a picture the
// window shows is served, so nothing else on the computer can be read here.
func (a *App) busAttachmentFiles(w http.ResponseWriter, r *http.Request) {
	id, err1 := strconv.ParseInt(r.URL.Query().Get("message"), 10, 64)
	index, err2 := strconv.Atoi(r.URL.Query().Get("index"))
	if err1 != nil || err2 != nil {
		http.NotFound(w, r)
		return
	}
	var at bus.Attachment
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		at, err = b.PersonFile(r.Context(), id, index)
		return err
	})
	if err != nil || !busPictureTypes[at.Mime] {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(at.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", at.Mime)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// BusFiles is a count of the person's attached files and their size, for
// the command in Settings that removes the old ones.
type BusFiles struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// attachmentCutoff is the moment before which a file counts as older than
// the age chosen, in days; an age of 0 counts every file.
func attachmentCutoff(days int) (time.Time, error) {
	if days < 0 {
		return time.Time{}, errors.New("choose an age of no days or more")
	}
	return time.Now().AddDate(0, 0, -days), nil
}

// OldAttachments counts the files the person attached to bus messages
// sent more than days ago, and their size, which Settings shows before it
// asks to remove them.
func (a *App) OldAttachments(days int) (BusFiles, error) {
	cutoff, err := attachmentCutoff(days)
	if err != nil {
		return BusFiles{}, err
	}
	var n bus.StoredFiles
	err = a.busHost().With(func(b *bus.Bus) error {
		n, err = b.PersonFilesBefore(context.Background(), cutoff)
		return err
	})
	return BusFiles{Files: n.Files, Bytes: n.Bytes}, err
}

// RemoveOldAttachments removes the files the person attached to bus
// messages sent more than days ago and returns how many were removed and
// their size. The messages stay, and each shows its file as removed.
func (a *App) RemoveOldAttachments(days int) (BusFiles, error) {
	cutoff, err := attachmentCutoff(days)
	if err != nil {
		return BusFiles{}, err
	}
	var n bus.StoredFiles
	err = a.busHost().With(func(b *bus.Bus) error {
		n, err = b.RemovePersonFiles(context.Background(), cutoff)
		return err
	})
	return BusFiles{Files: n.Files, Bytes: n.Bytes}, err
}

// BusThread returns the thread a message belongs to, oldest first: the
// message it answers, if any, and every reply to that one.
func (a *App) BusThread(id int64) ([]BusMessage, error) {
	var out []BusMessage
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		m, err := b.Message(ctx, id)
		if err != nil {
			return err
		}
		if m.ReplyTo != 0 {
			id = m.ReplyTo
		}
		ms, err := b.Thread(ctx, id)
		out = busMessages(ms)
		return err
	})
	return out, err
}

// BusReplyInThread answers a thread as Paul (bus.ReplyInThread).
func (a *App) BusReplyInThread(id int64, text string) (BusMessage, error) {
	var out BusMessage
	err := a.busHost().With(func(b *bus.Bus) error {
		m, err := b.ReplyInThread(context.Background(), bus.Sender{}, id, text)
		out = busMessage(m)
		return err
	})
	return out, err
}

// BusSend sends a message from Paul to an address: a chat's title, with its
// workspace when two chats share it, a workspace, # and a group chat's
// name, or a reply when to is empty and replyTo is set.
func (a *App) BusSend(to, text string, replyTo int64) (BusMessage, error) {
	var out BusMessage
	err := a.busHost().With(func(b *bus.Bus) error {
		m, err := b.Send(context.Background(), bus.Sender{}, strings.TrimSpace(to), text, nil, replyTo)
		out = busMessage(m)
		return err
	})
	return out, err
}
