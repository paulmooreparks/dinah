package main

import (
	"context"
	"errors"
	"strings"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

// Importing the groups and teams of a workspace's chats, and the groups'
// conversations (docs/design/workspace-export.md, "Groups and Teams"). A
// group is matched by its title: the import joins the group of that title
// here, or makes it. A member is added when its chat is in the archive or
// already here; a member whose chat is in neither waits for its workspace,
// and nothing is stored for it, so importing that workspace later adds it.
// A group that has a lead keeps it. A team pair is added when both chats are
// here and the member has no other lead, and a team's budget goes with a
// lead the import added. The messages get new IDs, and their origin is
// recorded on the bus, so a message that comes twice is added once. The bus
// references in the imported chats' histories are then rewritten to the new
// IDs. Trusted links are never imported; the window lists them.

// WorkspaceImportMember is a member of an imported group or team that waits
// for its workspace to be imported.
type WorkspaceImportMember struct {
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
}

// WorkspaceImportGroup is a group of an import. Existing says it joins the
// group of its title here. InFile counts the members whose chats are in the
// file, Here those already on this computer, and Waiting the others.
// Messages is how many messages its conversation has, and New how many of
// them are not here yet. KeptLead is the title of the lead the group here
// keeps, when the file gives it another.
type WorkspaceImportGroup struct {
	Title    string                  `json:"title"`
	Existing bool                    `json:"existing"`
	InFile   int                     `json:"inFile"`
	Here     int                     `json:"here"`
	Waiting  []WorkspaceImportMember `json:"waiting"`
	Messages int                     `json:"messages"`
	New      int                     `json:"new"`
	KeptLead string                  `json:"keptLead,omitempty"`
}

// WorkspaceImportTeam is a lead and its members as the file gives them.
// Existing says the lead already leads a team here. Waiting are the chats of
// its pairs that are neither in the file nor here.
type WorkspaceImportTeam struct {
	Lead     string                  `json:"lead"`
	Members  []string                `json:"members"`
	Existing bool                    `json:"existing"`
	Waiting  []WorkspaceImportMember `json:"waiting"`
}

// WorkspaceImportLink is a trusted link the exporting computer had, which
// the import does not make.
type WorkspaceImportLink struct {
	A string `json:"a"`
	B string `json:"b"`
}

// groupImport resolves the chats an archive's groups and teams refer to.
type groupImport struct {
	export  workspaces.Export
	catalog *workspaceChats
}

// recipientHere is the bus identity here of the chat a reference is to, from
// the chats an import brought or found here, or else from the chats already
// here; "" when the chat is not on this computer.
func (g groupImport) recipientHere(ref workspaces.ChatRef, imported map[string]string) string {
	if id := imported[ref.ChatID]; id != "" {
		return id
	}
	if chat := g.catalog.byID[ref.ChatID]; chat != nil {
		return g.catalog.saved.BusSessions[chat.head]
	}
	return ""
}

// present says whether a reference's chat is here or comes with the file.
func (g groupImport) present(ref workspaces.ChatRef) bool {
	return ref.Key != "" || g.catalog.byID[ref.ChatID] != nil
}

// planImportGroups says what importing an export's groups and teams does,
// and lists the trusted links it does not make.
func (a *App) planImportGroups(archive *workspaces.Archive, plan *WorkspaceImportPlan) error {
	export := archive.Export
	plan.Groups, plan.Teams, plan.Links = []WorkspaceImportGroup{}, []WorkspaceImportTeam{}, []WorkspaceImportLink{}
	for _, link := range export.Links {
		plan.Links = append(plan.Links, WorkspaceImportLink{A: link.A, B: link.B})
	}
	if len(export.Groups) == 0 && len(export.Teams) == 0 {
		return nil
	}
	catalog, _, err := a.chatCatalog()
	if err != nil {
		return err
	}
	g := groupImport{export: export, catalog: catalog}
	self, err := workspaces.ExporterIdentity(configDir())
	if err != nil {
		return err
	}
	return a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		for _, group := range export.Groups {
			item := WorkspaceImportGroup{Title: group.Title, Waiting: []WorkspaceImportMember{}, Messages: group.Messages}
			room, err := b.Room(ctx, group.Title)
			item.Existing = err == nil
			for _, member := range group.Members {
				switch {
				case member.Key != "":
					item.InFile++
				case catalog.byID[member.ChatID] != nil:
					item.Here++
				default:
					item.Waiting = append(item.Waiting, WorkspaceImportMember{Title: member.Title, Workspace: member.Workspace})
				}
			}
			if item.Existing && group.Lead != nil && room.Lead != "" {
				if lead := g.recipientHere(*group.Lead, nil); lead != room.Lead {
					if session, err := b.Session(ctx, room.Lead); err == nil {
						item.KeptLead = session.Display()
					}
				}
			}
			if group.Messages > 0 && export.Exporter != "" {
				messages, err := archive.GroupMessages(group.Key)
				if err != nil {
					return err
				}
				for _, m := range messages {
					if _, here, err := messageHere(ctx, b, self, group.Title, export.Exporter, m); err != nil {
						return err
					} else if !here {
						item.New++
					}
				}
			}
			plan.Groups = append(plan.Groups, item)
		}
		for _, team := range export.Teams {
			item := WorkspaceImportTeam{Lead: team.Lead.Title, Members: []string{}, Waiting: []WorkspaceImportMember{}}
			if lead := g.recipientHere(team.Lead, nil); lead != "" {
				item.Existing, _ = b.IsLead(ctx, lead)
			}
			if !g.present(team.Lead) {
				item.Waiting = append(item.Waiting, WorkspaceImportMember{Title: team.Lead.Title, Workspace: team.Lead.Workspace})
			}
			for _, member := range team.Members {
				item.Members = append(item.Members, member.Chat.Title)
				if !g.present(member.Chat) {
					item.Waiting = append(item.Waiting, WorkspaceImportMember{Title: member.Chat.Title, Workspace: member.Chat.Workspace})
				}
			}
			plan.Teams = append(plan.Teams, item)
		}
		return nil
	})
}

// groupsImported is what importing the groups did.
type groupsImported struct {
	Groups, Messages int
}

// importGroups imports an archive's groups, teams and conversations, once
// the chats' bus identities are made. imported gives the bus identity here
// of each of the archive's chats that is here, by its logical chat ID in the
// archive, and leads the identities of the chats this import added. It
// returns how to rewrite the bus references in an imported chat's history.
func (a *App) importGroups(archive *workspaces.Archive, imported map[string]string, added map[string]bool, counts *groupsImported) (func(*conversationTranscript), error) {
	export := archive.Export
	if len(export.Groups) == 0 && len(export.Teams) == 0 {
		return nil, nil
	}
	catalog, _, err := a.chatCatalog()
	if err != nil {
		return nil, err
	}
	g := groupImport{export: export, catalog: catalog}
	self, err := workspaces.ExporterIdentity(configDir())
	if err != nil {
		return nil, err
	}
	newIDs := map[int64]int64{}
	rooms := map[string]string{}
	err = a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		for _, group := range export.Groups {
			in := bus.ImportedGroup{Title: group.Title, Budget: group.Budget}
			if group.Lead != nil {
				in.Lead = g.recipientHere(*group.Lead, imported)
			}
			for _, member := range group.Members {
				if id := g.recipientHere(member, imported); id != "" {
					in.Members = append(in.Members, id)
				}
			}
			result, err := b.ImportGroup(ctx, in)
			if err != nil {
				return err
			}
			rooms[strings.ToLower(group.Title)] = result.Key
			counts.Groups++
		}
		for _, team := range export.Teams {
			lead := g.recipientHere(team.Lead, imported)
			if lead == "" {
				continue
			}
			for _, member := range team.Members {
				id := g.recipientHere(member.Chat, imported)
				if id == "" {
					continue
				}
				current, _, err := b.LeadOf(ctx, id)
				if err != nil {
					return err
				}
				if current != "" {
					// A member keeps the lead it has here.
					continue
				}
				if b.CheckTeamLead(id, lead) != nil {
					continue
				}
				if _, err := b.SetLead(ctx, id, lead); err != nil {
					return err
				}
			}
			if added[team.Lead.ChatID] && len(team.Budget) > 0 {
				existing, err := b.TeamAllowances(ctx, lead)
				if err != nil {
					return err
				}
				if len(existing) == 0 {
					allowances := []bus.TeamAllowance{}
					for _, allowance := range team.Budget {
						allowances = append(allowances, bus.TeamAllowance{Provider: allowance.Provider, Unit: allowance.Unit, Allowance: allowance.Allowance})
					}
					if err := b.SetTeamAllowances(ctx, lead, allowances); err != nil {
						return err
					}
				}
			}
		}
		if export.Exporter == "" {
			return nil
		}
		for _, group := range export.Groups {
			if group.Messages == 0 {
				continue
			}
			messages, err := archive.GroupMessages(group.Key)
			if err != nil {
				return err
			}
			for _, m := range messages {
				room := rooms[strings.ToLower(group.Title)]
				// A message first made here, which went to another computer and
				// came back, is the message here with its own ID, if the group
				// still has it; nothing is added for it.
				if id, here, err := messageHere(ctx, b, self, room, export.Exporter, m); err != nil {
					return err
				} else if here {
					newIDs[m.ID] = id
					continue
				}
				origin, originID := m.Origin(export.Exporter)
				in := bus.ImportedMessage{Exporter: origin, Origin: originID, Group: room, CreatedAt: m.CreatedAt, Kind: m.Kind,
					FromName: m.FromName, Text: m.Text, Version: m.Version, Verdict: m.Verdict}
				if m.From != nil {
					in.FromSession = g.recipientHere(*m.From, imported)
				}
				if m.Person {
					in.FromName = bus.Human
				}
				if m.ReplyTo > 0 {
					in.ReplyTo = newIDs[m.ReplyTo]
				}
				if m.MentionsPerson {
					in.Mentions = append(in.Mentions, bus.Human)
				}
				for _, mention := range m.Mentions {
					if id := g.recipientHere(mention, imported); id != "" {
						in.Mentions = append(in.Mentions, id)
					}
				}
				for _, att := range m.Attachments {
					file := bus.ImportedFile{Name: att.Name, Mime: att.Mime, SHA256: att.SHA256, Size: att.Size}
					if att.File != "" {
						if file.Data, err = archive.File(att.File); err != nil {
							return err
						}
					}
					in.Files = append(in.Files, file)
				}
				id, wasAdded, err := b.ImportMessage(ctx, in)
				if err != nil {
					return err
				}
				newIDs[m.ID] = id
				if wasAdded {
					counts.Messages++
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return func(tr *conversationTranscript) { a.rewriteBusReferences(tr, export.Exporter, newIDs, rooms) }, nil
}

// messageHere finds a message of an archive's group conversation on this
// bus: by its own ID when it was first made here, self being this Dinah's
// identity, and the group still has it, or else by its recorded origin.
func messageHere(ctx context.Context, b *bus.Bus, self, group, exporter string, m workspaces.GroupMessage) (int64, bool, error) {
	origin, originID := m.Origin(exporter)
	if origin == self {
		if ok, err := b.GroupHasMessage(ctx, group, originID); err != nil && !errors.Is(err, bus.ErrNotFound) {
			return 0, false, err
		} else if ok {
			return originID, true, nil
		}
	}
	return b.ImportedMessageID(ctx, origin, originID)
}

// rewriteBusReferences points the bus messages a chat's history received
// and sent at their IDs here, through the same mapping as the messages
// themselves. A message the archive's conversations do not hold, such as one
// an earlier import brought, is looked up by the exporter's own origin for
// it; one that is not here keeps its text and loses its link.
func (a *App) rewriteBusReferences(tr *conversationTranscript, exporter string, newIDs map[int64]int64, rooms map[string]string) {
	lookup := func(origin int64) (int64, bool) {
		if id, ok := newIDs[origin]; ok {
			return id, true
		}
		var id int64
		var ok bool
		if exporter != "" {
			_ = a.busHost().With(func(b *bus.Bus) error {
				var err error
				id, ok, err = b.ImportedMessageID(context.Background(), exporter, origin)
				return err
			})
		}
		return id, ok
	}
	fix := func(messages []ConversationMessage) {
		for i := range messages {
			d := messages[i].Delivered
			if d == nil {
				continue
			}
			copied := *d
			copied.FromSession = ""
			if id, ok := lookup(d.ID); ok && d.ID > 0 {
				copied.ID = id
				copied.Room = rooms[strings.ToLower(d.RoomTitle)]
			} else {
				copied.ID, copied.Room = 0, ""
			}
			messages[i].Delivered = &copied
		}
	}
	fix(tr.ParentHistory)
	fix(tr.Messages)
}
