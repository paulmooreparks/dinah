package workspaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The chats of a workspace export (docs/design/workspace-export.md, "Chats,
// Their History and Their Groups"). The document lists each chat with a key
// that groups, teams and chat pins refer to; the chat's saved history is the
// archive entry chats/<key>.json, and a group's conversation is the entry
// groups/<key>.jsonl (archive.go). A chat in another workspace, or one the
// person left out, is referred to by its logical chat ID, its title and its
// workspace's name, without a key. Each record keeps the fields this build
// does not know (extra.go), as a source and a pin do.

// The most chats, groups, stages and members an export may have.
const (
	MaxExportChats     = 10000
	MaxExportGroups    = 2000
	MaxExportTeams     = 2000
	MaxChatStages      = 1000
	MaxGroupMembers    = 1000
	MaxChatFolderDepth = 64
)

// ExportChat is a chat in an export. ChatID is its logical chat ID and
// Stages the IDs of its stages, oldest first, the last of which is the one
// whose saved history the archive holds. Options are the choices made in it,
// by option ID, such as its model, and Model the model's name as the
// exporting computer showed it. Title is the name the person gave it and
// JudgedTitle the one Dinah judged. Folder is where it works, relative to
// the workspace's main folder and separated by "/", or empty for the main
// folder itself.
type ExportChat struct {
	Key         string                     `json:"key"`
	ChatID      string                     `json:"chatId"`
	Stages      []string                   `json:"stages"`
	Provider    string                     `json:"provider"`
	Options     map[string]json.RawMessage `json:"options,omitempty"`
	Model       string                     `json:"model,omitempty"`
	Title       string                     `json:"title,omitempty"`
	JudgedTitle string                     `json:"judgedTitle,omitempty"`
	Hidden      bool                       `json:"hidden,omitempty"`
	Folder      string                     `json:"folder,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

// ChatRef refers to a chat from a group, a team or a group message: by its
// logical chat ID, with its title and its workspace's name, and with Key
// when the chat is in the export.
type ChatRef struct {
	ChatID    string                     `json:"chatId"`
	Key       string                     `json:"key,omitempty"`
	Title     string                     `json:"title,omitempty"`
	Workspace string                     `json:"workspace,omitempty"`
	Extra     map[string]json.RawMessage `json:"-"`
}

// ExportGroup is a group chat with at least one member among the export's
// chats. Lead is the chat whose team's channel the group is, if any.
// Budget is the group's turn budget, -1 for no limit. Person says whether
// the person is a member. Messages is how many messages its conversation
// entry holds, 0 when it has no entry.
type ExportGroup struct {
	Key      string                     `json:"key"`
	Title    string                     `json:"title"`
	Lead     *ChatRef                   `json:"lead,omitempty"`
	Budget   int                        `json:"budget"`
	Person   bool                       `json:"person,omitempty"`
	Members  []ChatRef                  `json:"members"`
	Messages int                        `json:"messages,omitempty"`
	Extra    map[string]json.RawMessage `json:"-"`
}

// ExportTeam is a lead with the members it leads, each pair having at least
// one of its two chats in the export. Budget is the team's budget, given
// only when the lead is in the export.
type ExportTeam struct {
	Lead    ChatRef                    `json:"lead"`
	Members []ExportTeamMember         `json:"members"`
	Budget  []ExportAllowance          `json:"budget,omitempty"`
	Extra   map[string]json.RawMessage `json:"-"`
}

// ExportTeamMember is one member of a team: the chat, since when it has had
// its lead, and whether it is retired.
type ExportTeamMember struct {
	Chat    ChatRef                    `json:"chat"`
	Since   string                     `json:"since,omitempty"`
	Retired bool                       `json:"retired,omitempty"`
	Extra   map[string]json.RawMessage `json:"-"`
}

// ExportAllowance is a team's allowance on one provider, in that provider's
// own unit.
type ExportAllowance struct {
	Provider  string                     `json:"provider"`
	Unit      string                     `json:"unit"`
	Allowance int                        `json:"allowance"`
	Extra     map[string]json.RawMessage `json:"-"`
}

// GroupMessage is one line of a group's conversation entry. ID is the
// message's ID on the exporting computer, which replies and the chats'
// histories refer to. OriginExporter and OriginID are where the message was
// first made: the identity of that Dinah and the message's ID there, which
// an import records so that bringing the same message twice, by any route,
// adds it once. An export written before they were added has neither, and
// the message was then made by the export's Exporter, with ID as its ID. A
// message the person sent has Person set; a chat's has From.
type GroupMessage struct {
	ID             int64                      `json:"id"`
	OriginExporter string                     `json:"originExporter,omitempty"`
	OriginID       int64                      `json:"originId,omitempty"`
	CreatedAt      string                     `json:"createdAt"`
	Kind           string                     `json:"kind"`
	Person         bool                       `json:"person,omitempty"`
	From           *ChatRef                   `json:"from,omitempty"`
	FromName       string                     `json:"fromName,omitempty"`
	Text           string                     `json:"text"`
	ReplyTo        int64                      `json:"replyTo,omitempty"`
	Mentions       []ChatRef                  `json:"mentions,omitempty"`
	MentionsPerson bool                       `json:"mentionsPerson,omitempty"`
	Version        string                     `json:"version,omitempty"`
	Verdict        string                     `json:"verdict,omitempty"`
	Attachments    []GroupAttachment          `json:"attachments,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

// GroupAttachment is a file attached to a group message. File is its entry
// in the archive, files/<sha256>, or empty when the file did not travel
// because it was removed, changed or too large.
type GroupAttachment struct {
	Name   string                     `json:"name"`
	Size   int64                      `json:"size"`
	SHA256 string                     `json:"sha256"`
	Mime   string                     `json:"mime,omitempty"`
	File   string                     `json:"file,omitempty"`
	Extra  map[string]json.RawMessage `json:"-"`
}

// ExportLink is a trusted link between two workspaces, by the names the
// exporting computer's bus knew them by. An import lists it as not
// imported, since a file must not be able to grant agents that trust.
type ExportLink struct {
	A     string                     `json:"a"`
	B     string                     `json:"b"`
	Extra map[string]json.RawMessage `json:"-"`
}

func (l ExportLink) MarshalJSON() ([]byte, error) {
	type plain ExportLink
	return marshalWithExtra(plain(l), l.Extra)
}

func (l *ExportLink) UnmarshalJSON(data []byte) error {
	type plain ExportLink
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*l = ExportLink(p)
	l.Extra = extra
	return nil
}

func (c ExportChat) MarshalJSON() ([]byte, error) {
	type plain ExportChat
	return marshalWithExtra(plain(c), c.Extra)
}

func (c *ExportChat) UnmarshalJSON(data []byte) error {
	type plain ExportChat
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*c = ExportChat(p)
	c.Extra = extra
	return nil
}

func (r ChatRef) MarshalJSON() ([]byte, error) {
	type plain ChatRef
	return marshalWithExtra(plain(r), r.Extra)
}

func (r *ChatRef) UnmarshalJSON(data []byte) error {
	type plain ChatRef
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*r = ChatRef(p)
	r.Extra = extra
	return nil
}

func (g ExportGroup) MarshalJSON() ([]byte, error) {
	type plain ExportGroup
	return marshalWithExtra(plain(g), g.Extra)
}

func (g *ExportGroup) UnmarshalJSON(data []byte) error {
	type plain ExportGroup
	// A group whose document leaves the budget out has no limit.
	p := plain{Budget: -1}
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*g = ExportGroup(p)
	g.Extra = extra
	return nil
}

func (t ExportTeam) MarshalJSON() ([]byte, error) {
	type plain ExportTeam
	return marshalWithExtra(plain(t), t.Extra)
}

func (t *ExportTeam) UnmarshalJSON(data []byte) error {
	type plain ExportTeam
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*t = ExportTeam(p)
	t.Extra = extra
	return nil
}

func (m ExportTeamMember) MarshalJSON() ([]byte, error) {
	type plain ExportTeamMember
	return marshalWithExtra(plain(m), m.Extra)
}

func (m *ExportTeamMember) UnmarshalJSON(data []byte) error {
	type plain ExportTeamMember
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*m = ExportTeamMember(p)
	m.Extra = extra
	return nil
}

func (a ExportAllowance) MarshalJSON() ([]byte, error) {
	type plain ExportAllowance
	return marshalWithExtra(plain(a), a.Extra)
}

func (a *ExportAllowance) UnmarshalJSON(data []byte) error {
	type plain ExportAllowance
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*a = ExportAllowance(p)
	a.Extra = extra
	return nil
}

// Origin is where a message of an export was first made: its recorded
// origin, or else the export's exporter and the message's ID.
func (m GroupMessage) Origin(exporter string) (string, int64) {
	if m.OriginExporter != "" {
		return m.OriginExporter, m.OriginID
	}
	return exporter, m.ID
}

func (m GroupMessage) MarshalJSON() ([]byte, error) {
	type plain GroupMessage
	return marshalWithExtra(plain(m), m.Extra)
}

func (m *GroupMessage) UnmarshalJSON(data []byte) error {
	type plain GroupMessage
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*m = GroupMessage(p)
	m.Extra = extra
	return nil
}

func (a GroupAttachment) MarshalJSON() ([]byte, error) {
	type plain GroupAttachment
	return marshalWithExtra(plain(a), a.Extra)
}

func (a *GroupAttachment) UnmarshalJSON(data []byte) error {
	type plain GroupAttachment
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*a = GroupAttachment(p)
	a.Extra = extra
	return nil
}

// recordKey is the form of a chat's or a group's key, which is also part
// of an archive entry's name, so it can hold nothing a path could use.
var recordKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidKey reports whether a chat's or a group's key has a form this build
// reads.
func ValidKey(key string) bool { return recordKey.MatchString(key) }

// validIdentity is the form of a logical chat ID, a stage ID or an
// exporter's identity: not empty, not too long, and without control
// characters.
func validIdentity(id string) bool {
	if id == "" || len(id) > 512 || strings.TrimSpace(id) != id {
		return false
	}
	for _, r := range id {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

// checkChats checks an export's chats, groups and teams: keys of a form
// ValidKey accepts that are unique, logical chat IDs and stage IDs that are
// unique, a folder that stays inside the main folder, references whose key,
// when they have one, belongs to a chat with the same logical chat ID, turn
// budgets of -1 or more, and allowances of 0 or more. It cleans the titles
// and returns the chats' keys.
func checkChats(out *Export) (map[string]bool, error) {
	if out.Exporter != "" && !validIdentity(out.Exporter) {
		return nil, errors.New("the workspace export has an invalid exporter identity")
	}
	if len(out.Chats) > MaxExportChats || len(out.Groups) > MaxExportGroups || len(out.Teams) > MaxExportTeams {
		return nil, errors.New("the workspace export has more chats, groups or teams than can be imported")
	}
	byKey := map[string]string{}
	ids, stages := map[string]bool{}, map[string]bool{}
	for i := range out.Chats {
		chat := &out.Chats[i]
		if !ValidKey(chat.Key) || byKey[chat.Key] != "" {
			return nil, errors.New("the workspace export has a chat without a key of its own")
		}
		if !validIdentity(chat.ChatID) || ids[chat.ChatID] {
			return nil, fmt.Errorf("the chat %s in the workspace export has no identity of its own", chat.Key)
		}
		ids[chat.ChatID] = true
		byKey[chat.Key] = chat.ChatID
		if len(chat.Stages) == 0 || len(chat.Stages) > MaxChatStages {
			return nil, fmt.Errorf("the chat %s in the workspace export has no stages that can be read", chat.Key)
		}
		for _, stage := range chat.Stages {
			if !validIdentity(stage) || stages[stage] {
				return nil, fmt.Errorf("the chat %s in the workspace export has a stage that is invalid or belongs to another chat", chat.Key)
			}
			stages[stage] = true
		}
		chat.Provider = strings.TrimSpace(chat.Provider)
		if chat.Provider == "" {
			return nil, fmt.Errorf("the chat %s in the workspace export has no program", chat.Key)
		}
		if chat.Folder != "" && !cleanRelative(chat.Folder, MaxChatFolderDepth) {
			return nil, fmt.Errorf("the chat %s in the workspace export has an invalid folder", chat.Key)
		}
		chat.Title, chat.JudgedTitle, chat.Model = strings.TrimSpace(chat.Title), strings.TrimSpace(chat.JudgedTitle), strings.TrimSpace(chat.Model)
	}
	checkRef := func(ref *ChatRef) error {
		if !validIdentity(ref.ChatID) {
			return errors.New("the workspace export refers to a chat without an identity")
		}
		if ref.Key != "" && byKey[ref.Key] != ref.ChatID {
			return errors.New("the workspace export refers to a chat by a key that is not that chat's")
		}
		ref.Title, ref.Workspace = strings.TrimSpace(ref.Title), strings.TrimSpace(ref.Workspace)
		return nil
	}
	groups := map[string]bool{}
	for i := range out.Groups {
		group := &out.Groups[i]
		if !ValidKey(group.Key) || groups[group.Key] {
			return nil, errors.New("the workspace export has a group without a key of its own")
		}
		groups[group.Key] = true
		group.Title = strings.TrimSpace(group.Title)
		if group.Title == "" {
			return nil, fmt.Errorf("the group %s in the workspace export has no name", group.Key)
		}
		if group.Budget < -1 || group.Messages < 0 {
			return nil, fmt.Errorf("the group %q in the workspace export has an invalid turn budget or message count", group.Title)
		}
		if group.Lead != nil {
			if err := checkRef(group.Lead); err != nil {
				return nil, err
			}
		}
		if len(group.Members) > MaxGroupMembers {
			return nil, fmt.Errorf("the group %q in the workspace export has too many members", group.Title)
		}
		members := map[string]bool{}
		for j := range group.Members {
			if err := checkRef(&group.Members[j]); err != nil {
				return nil, err
			}
			if members[group.Members[j].ChatID] {
				return nil, fmt.Errorf("the group %q in the workspace export lists a member twice", group.Title)
			}
			members[group.Members[j].ChatID] = true
		}
	}
	leads := map[string]bool{}
	for i := range out.Teams {
		team := &out.Teams[i]
		if err := checkRef(&team.Lead); err != nil {
			return nil, err
		}
		if leads[team.Lead.ChatID] {
			return nil, errors.New("the workspace export lists one lead's team twice")
		}
		leads[team.Lead.ChatID] = true
		if len(team.Members) == 0 || len(team.Members) > MaxGroupMembers {
			return nil, errors.New("the workspace export has a team with no members or too many")
		}
		members := map[string]bool{}
		for j := range team.Members {
			member := &team.Members[j]
			if err := checkRef(&member.Chat); err != nil {
				return nil, err
			}
			if member.Chat.ChatID == team.Lead.ChatID || members[member.Chat.ChatID] {
				return nil, errors.New("the workspace export has a team that lists a member twice or leads itself")
			}
			members[member.Chat.ChatID] = true
		}
		for _, allowance := range team.Budget {
			if strings.TrimSpace(allowance.Provider) == "" || strings.TrimSpace(allowance.Unit) == "" || allowance.Allowance < 0 {
				return nil, errors.New("the workspace export has a team budget that cannot be read")
			}
		}
	}
	if len(out.Links) > MaxExportTeams {
		return nil, errors.New("the workspace export lists more trusted links than can be read")
	}
	for i := range out.Links {
		link := &out.Links[i]
		link.A, link.B = strings.TrimSpace(link.A), strings.TrimSpace(link.B)
		if link.A == "" || link.B == "" {
			return nil, errors.New("the workspace export lists a trusted link without its workspaces")
		}
	}
	keys := make(map[string]bool, len(byKey))
	for key := range byKey {
		keys[key] = true
	}
	return keys, nil
}
