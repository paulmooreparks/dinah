package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

// Importing a workspace's chats (docs/design/workspace-export.md, "A Chat
// That Is Already Here" and "Continuing a Moved Chat"). Each chat of the
// archive is matched with the chats here by its logical chat ID and its
// stages, and its stages decide what the import does with it:
//
//   - New: no chat here has its identity. It is added, moved here.
//   - Already Up to Date: the archive's stages are this computer's, or older.
//   - Updated From the Other Computer: the archive's stages continue from
//     this computer's newest stage. The newer stages are added.
//   - Changed on Both Computers: each has stages the other lacks. This
//     computer's chat is left as it is, unless the person ticks Import as a
//     Separate Chat, which adds the archive's chat with new IDs.
//   - Close It to Update It: an update to a chat that is open here.
//
// A chat is added with the records master's logical chat model reads: its
// provider, its titles, its options, whether it is hidden, its stages joined
// by committed bindings, a bus identity made on this computer, its chat pins,
// and its newest stage's saved history, with the paths in it rewritten from
// the exporting computer's main folder to this one's. No native session
// record is written. The chat is recorded as moved here (moved_chats.go).

// The marks an imported chat can have.
const (
	importNew           = "new"
	importUpToDate      = "up-to-date"
	importUpdated       = "updated"
	importChangedOnBoth = "changed-on-both"
	importCloseToUpdate = "close-to-update"
)

// WorkspaceImportChat is one chat of an import. Mark says what importing it
// does; ProviderMissing that its provider is not on this computer, so it
// opens on its saved history and asks for another; Separate that the person
// chose to import a chat changed on both computers as a separate chat.
type WorkspaceImportChat struct {
	Key             string `json:"key"`
	ChatID          string `json:"chatId"`
	Title           string `json:"title"`
	Provider        string `json:"provider"`
	ProviderName    string `json:"providerName"`
	Model           string `json:"model,omitempty"`
	Hidden          bool   `json:"hidden,omitempty"`
	Mark            string `json:"mark"`
	ProviderMissing bool   `json:"providerMissing,omitempty"`
	Separate        bool   `json:"separate,omitempty"`
	// local is the chat here that the archive's chat matched.
	local *exportChat
}

// importedTitle is the title an imported chat is shown with.
func importedTitle(chat workspaces.ExportChat) string {
	for _, title := range []string{chat.Title, chat.JudgedTitle} {
		if title != "" {
			return title
		}
	}
	return "Untitled Chat"
}

// chatCatalog reads every chat this window lists, hidden ones included, with
// the sessions that are open.
func (a *App) chatCatalog() (*workspaceChats, map[string]bool, error) {
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
			if chat.Info().Open {
				open[chat.ID()] = true
			}
		}
	}
	a.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	return readWorkspaceChats(saved, headers, WorkspaceRecord{}), open, nil
}

// isPrefix reports whether every stage of a is the stage of b in the same
// place.
func isPrefix(a, b []string) bool {
	return len(a) <= len(b) && slices.Equal(a, b[:len(a)])
}

// sessionTaken reports whether a session ID is in use here by anything other
// than a listed chat, such as a chat in the Recycle Bin or a deleted one.
func sessionTaken(saved Settings, id string) bool {
	if _, ok := saved.RemovedChats[id]; ok {
		return true
	}
	if saved.SessionProviders[id] != "" || saved.BusSessions[id] != "" || saved.ChatBindings[id] != "" {
		return true
	}
	_, err := os.Stat(sessionhost.TranscriptPath(configDir(), id))
	return err == nil
}

// planImportChats marks each chat of an export with what importing it does,
// applying the person's choices of separate chats.
func (a *App) planImportChats(export workspaces.Export, choices []WorkspaceImportChoice) ([]WorkspaceImportChat, error) {
	out := []WorkspaceImportChat{}
	if len(export.Chats) == 0 {
		return out, nil
	}
	catalog, open, err := a.chatCatalog()
	if err != nil {
		return nil, err
	}
	separate := map[string]bool{}
	for _, c := range choices {
		if c.Chat != "" {
			separate[c.Chat] = c.Separate
		}
	}
	byStage := map[string]*exportChat{}
	for _, chat := range catalog.byID {
		for _, stage := range chat.sessions {
			byStage[stage] = chat
		}
	}
	here := map[string]bool{}
	for _, chat := range export.Chats {
		item := WorkspaceImportChat{Key: chat.Key, ChatID: chat.ChatID, Title: importedTitle(chat), Provider: chat.Provider, ProviderName: providerName(chat.Provider), Model: chat.Model, Hidden: chat.Hidden}
		if _, known := here[chat.Provider]; !known {
			here[chat.Provider] = providerHere(a, chat.Provider)
		}
		item.ProviderMissing = !here[chat.Provider]
		local := catalog.byID[chat.ChatID]
		for _, stage := range chat.Stages {
			if local == nil {
				local = byStage[stage]
			}
		}
		switch {
		case local != nil && isPrefix(chat.Stages, local.stages):
			item.Mark = importUpToDate
		case local != nil && isPrefix(local.stages, chat.Stages) && open[local.head]:
			item.Mark = importCloseToUpdate
		case local != nil && isPrefix(local.stages, chat.Stages):
			item.Mark = importUpdated
		case local != nil:
			item.Mark = importChangedOnBoth
		default:
			item.Mark = importNew
			for _, stage := range append([]string{chat.ChatID}, chat.Stages...) {
				if sessionTaken(catalog.saved, stage) {
					// A chat here that no list shows, such as one in the Recycle
					// Bin, has its identity, so the two are told apart as a chat
					// changed on both computers is.
					item.Mark = importChangedOnBoth
				}
			}
		}
		item.local = local
		item.Separate = item.Mark == importChangedOnBoth && separate[chat.Key]
		out = append(out, item)
	}
	return out, nil
}

// A moved chat's history is imported as it was. Until 18 November 2026 it
// ended with a line saying the chat had moved here, which Paul asked to be
// removed; the chat window leaves that line out of histories imported
// before then (chat-applet.js, prependContinuation).

// rebase rewrites a path recorded on the exporting computer, from inside its
// main folder there, oldMain, to the same place inside newMain. A path
// outside oldMain is kept as it was. Both computers' paths are compared with
// "/" separators and, since either may be Windows, without regard to case.
func rebase(path, oldMain, newMain string) string {
	if path == "" || oldMain == "" {
		return path
	}
	p, root := strings.TrimRight(filepath.ToSlash(strings.ReplaceAll(path, `\`, "/")), "/"), strings.TrimRight(strings.ReplaceAll(oldMain, `\`, "/"), "/")
	if strings.EqualFold(p, root) {
		return newMain
	}
	if len(p) > len(root) && strings.EqualFold(p[:len(root)+1], root+"/") {
		return filepath.Join(newMain, filepath.FromSlash(p[len(root)+1:]))
	}
	return path
}

// exportingMain is the exporting computer's main folder as a chat's history
// recorded it: the history's folder without the chat's folder relative to
// the main folder.
func exportingMain(workspace, folder string) string {
	p := strings.TrimRight(strings.ReplaceAll(workspace, `\`, "/"), "/")
	if folder == "" {
		return p
	}
	suffix := "/" + folder
	if len(p) > len(suffix) && strings.EqualFold(p[len(p)-len(suffix):], suffix) {
		return p[:len(p)-len(suffix)]
	}
	return ""
}

// separateID is a new session ID for a stage of a chat imported as a
// separate chat. It keeps the prefix that says which provider the session
// is of (providerForSession).
func separateID(id string) string {
	prefix := ""
	for _, row := range providerRegistry {
		if row.SessionPrefix != "" && strings.HasPrefix(id, row.SessionPrefix) && len(row.SessionPrefix) > len(prefix) {
			prefix = row.SessionPrefix
		}
	}
	return prefix + "imported-" + strings.ToLower(rand.Text())
}

// importedChat is one chat the import adds or updates: its stages as they
// are here, the stages that are new here, its saved history, and its bus
// identity.
type importedChat struct {
	plan      WorkspaceImportChat
	from      workspaces.ExportChat
	stages    []string
	added     []string
	title     string
	folder    string
	recipient string
	history   *conversationTranscript
}

func (c *importedChat) head() string { return c.stages[len(c.stages)-1] }

// importArchiveChats imports the chats of the export file into the workspace
// an import just made or added to, and counts them in result.
func (a *App) importArchiveChats(file, workspaceID string, choices []WorkspaceImportChoice, result *WorkspaceImportResult) error {
	archive, err := workspaces.OpenArchive(file)
	if err != nil {
		return err
	}
	defer archive.Close()
	var groups groupsImported
	chats, err := a.importChats(archive, file, workspaceID, choices, func(recipients map[string]string, added map[string]bool) (func(*conversationTranscript), error) {
		return a.importGroups(archive, recipients, added, &groups)
	})
	result.Chats, result.Updated, result.Separate = chats.Added, chats.Updated, chats.Separate
	result.Groups, result.Messages = groups.Groups, groups.Messages
	return err
}

// chatImport is what importing an archive's chats did: how many chats were
// added, updated and added as separate chats, and the bus identity here of
// each of the archive's chats that is here after the import, by its logical
// chat ID in the archive.
type chatImport struct {
	Added, Updated, Separate int
	Recipients               map[string]string
}

// importChats imports the chats of an archive into the workspace with the
// ID given, whose main folder is now settled, as the plan says, with the
// person's choices. Once the bus identities are made, between, when given,
// runs with them, and with the logical chat IDs of the chats the import
// adds, and returns how to rewrite the bus references in a history
// (workspace_import_groups.go); the histories are written after it. It runs
// when no chat is added too, since a group may gain members already here.
func (a *App) importChats(archive *workspaces.Archive, file, workspaceID string, choices []WorkspaceImportChoice, between func(recipients map[string]string, added map[string]bool) (func(*conversationTranscript), error)) (chatImport, error) {
	result := chatImport{Recipients: map[string]string{}}
	export := archive.Export
	if len(export.Chats) == 0 {
		return result, nil
	}
	plan, err := a.planImportChats(export, choices)
	if err != nil {
		return result, err
	}
	record, err := a.workspaceSourceRecord(workspaceID)
	if err != nil {
		return result, err
	}
	main := workspaces.ExecutionPath(record)
	now := time.Now()
	var chats []*importedChat
	for i, item := range plan {
		from := export.Chats[i]
		if item.Mark == importUpToDate || item.Mark == importCloseToUpdate || (item.Mark == importChangedOnBoth && !item.Separate) {
			if item.local != nil {
				a.mu.Lock()
				recipient := a.settings.BusSessions[item.local.head]
				a.mu.Unlock()
				if recipient != "" {
					result.Recipients[from.ChatID] = recipient
				}
			}
			continue
		}
		chat := &importedChat{plan: item, from: from, title: from.Title, folder: main}
		if from.Folder != "" {
			chat.folder = filepath.Join(main, filepath.FromSlash(from.Folder))
		}
		switch item.Mark {
		case importUpdated:
			chat.stages = append([]string{}, from.Stages...)
			chat.added = chat.stages[len(item.local.stages):]
			a.mu.Lock()
			chat.recipient = a.settings.BusSessions[item.local.head]
			a.mu.Unlock()
		case importChangedOnBoth:
			for _, stage := range from.Stages {
				chat.stages = append(chat.stages, separateID(stage))
			}
			chat.added = chat.stages
			chat.title = importedTitle(from) + " (Imported)"
		default:
			chat.stages = append([]string{}, from.Stages...)
			chat.added = chat.stages
		}
		if chat.recipient == "" {
			chat.recipient = bus.WindowSessionPrefix + rand.Text()
		}
		data, err := archive.History(from.Key)
		if err != nil {
			return result, err
		}
		var tr conversationTranscript
		if err := json.Unmarshal(data, &tr); err != nil {
			return result, fmt.Errorf("the saved history of %s could not be read: %w", importedTitle(from), err)
		}
		oldMain := exportingMain(tr.Workspace, from.Folder)
		renamed := map[string]string{}
		for j, stage := range from.Stages {
			renamed[stage] = chat.stages[j]
		}
		tr.SessionID = chat.head()
		if next := renamed[tr.ParentSessionID]; next != "" {
			tr.ParentSessionID = next
		}
		tr.Workspace = chat.folder
		if tr.ParentWorkspace != "" {
			tr.ParentWorkspace = rebase(tr.ParentWorkspace, oldMain, main)
		}
		tr.StartedQueueIDs = nil
		// The chat's earlier stages stay on the other computer, and the copy
		// of them this history keeps stands in for them here.
		tr.StagesElsewhere = tr.ParentSessionID != "" && tr.ParentHistory != nil
		chat.history = &tr
		chats = append(chats, chat)
		result.Recipients[from.ChatID] = chat.recipient
	}
	// The bus identities are made first, so that the groups and teams can
	// refer to them; a chat already here keeps its own.
	if err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		labels := a.busWorkspaceLabels()
		for _, chat := range chats {
			for p, label := range labels {
				if layout.SamePath(p, chat.folder) {
					if _, err := b.LabelWorkspace(ctx, p, label); err != nil {
						return err
					}
				}
			}
			title := chat.title
			if title == "" {
				title = importedTitle(chat.from)
			}
			if _, err := b.KnowSession(ctx, chat.recipient, chat.folder, title); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return result, err
	}
	var rewrite func(*conversationTranscript)
	if between != nil {
		added := map[string]bool{}
		for _, chat := range chats {
			if chat.plan.Mark != importUpdated {
				added[chat.from.ChatID] = true
			}
		}
		if rewrite, err = between(result.Recipients, added); err != nil {
			return result, err
		}
	}
	if len(chats) == 0 {
		return result, nil
	}
	return a.finishImportChats(archive, file, workspaceID, chats, now, rewrite, result)
}

// finishImportChats writes the histories of the chats the import adds or
// updates, with their bus references rewritten, and then their records, in
// one change of the settings. A history is written before the records that
// list it, so a failure in between leaves at most a history that lists as a
// chat on its own.
func (a *App) finishImportChats(archive *workspaces.Archive, file, workspaceID string, chats []*importedChat, now time.Time, rewrite func(*conversationTranscript), result chatImport) (chatImport, error) {
	for _, chat := range chats {
		if rewrite != nil {
			rewrite(chat.history)
		}
		if err := writeTranscript(chat.history); err != nil {
			return result, err
		}
	}
	pinsByKey := map[string][]workspaces.ExportPin{}
	for _, pin := range archive.Export.Pins {
		if pin.Kind == "chat" {
			pinsByKey[pin.Chat] = append(pinsByKey[pin.Chat], pin)
		}
	}
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, workspaceID)
		if index < 0 {
			return errors.New("the imported workspace is no longer registered")
		}
		for _, chat := range chats {
			head, from := chat.head(), chat.from
			if s.SessionProviders == nil {
				s.SessionProviders = map[string]string{}
			}
			s.SessionProviders[head] = from.Provider
			if chat.title != "" {
				if s.SessionTitles == nil {
					s.SessionTitles = map[string]string{}
				}
				s.SessionTitles[head] = chat.title
			}
			if from.JudgedTitle != "" && chat.plan.Mark != importChangedOnBoth {
				if s.JudgedTitles == nil {
					s.JudgedTitles = map[string]JudgedTitle{}
				}
				s.JudgedTitles[head] = JudgedTitle{Title: from.JudgedTitle}
			}
			if len(from.Options) > 0 {
				if s.SessionOptions == nil {
					s.SessionOptions = map[string]map[string]json.RawMessage{}
				}
				s.SessionOptions[head] = from.Options
			}
			if from.Hidden && !slices.Contains(s.Hidden, head) {
				s.Hidden = append(s.Hidden, head)
			}
			if s.ChatBindings == nil {
				s.ChatBindings = map[string]string{}
			}
			previous := ""
			if chat.plan.Mark == importUpdated {
				previous = chat.plan.local.head
			}
			for _, stage := range chat.added {
				if previous != "" {
					s.ChatBindings[previous] = stage
				}
				previous = stage
			}
			if s.BusSessions == nil {
				s.BusSessions = map[string]string{}
			}
			if chat.plan.Mark == importUpdated {
				delete(s.BusSessions, chat.plan.local.head)
				repinChat(s, chat.plan.local.head, head)
			}
			s.BusSessions[head] = chat.recipient
			if s.MovedChats == nil {
				s.MovedChats = map[string]movedChat{}
			}
			s.MovedChats[head] = movedChat{Exporter: archive.Export.Exporter, File: filepath.Base(file), At: now.UTC()}
			record := &s.WorkspaceRecords[index]
			for _, pin := range pinsByKey[from.Key] {
				next := workspaces.Pin{Kind: "chat", Title: pin.Title, Session: head}
				if !slices.ContainsFunc(record.Pins, func(p workspaces.Pin) bool { return workspaces.PinKey(p) == workspaces.PinKey(next) }) {
					record.Pins = append(record.Pins, next)
				}
			}
			switch chat.plan.Mark {
			case importUpdated:
				result.Updated++
			case importChangedOnBoth:
				result.Separate++
			default:
				result.Added++
			}
		}
		return nil
	})
	if err != nil {
		return chatImport{Recipients: result.Recipients}, err
	}
	a.emitSettingsChanged()
	return result, nil
}
