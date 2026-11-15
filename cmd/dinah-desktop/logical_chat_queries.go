package main

import (
	"fmt"
	"sort"
	"studio/internal/layout"
)

// These interim pure bodies can be replaced by the durable logical model.
func logicalChatReferencesFromSettings(s Settings) []string {
	known := map[string]bool{}
	for id, row := range s.ListedChats {
		if !providerRegistry[row.Provider].NativeSessionsRequireImport || row.LocalObservation {
			known[id] = true
		}
	}
	for id, provider := range s.SessionProviders {
		if !providerRegistry[provider].NativeSessionsRequireImport {
			known[id] = true
		}
	}
	for id := range s.SessionTitles {
		known[id] = true
	}
	for id := range s.BusSessions {
		known[id] = true
	}
	for id, header := range s.logicalEvidence {
		if !header.Deleted {
			known[id] = true
		}
	}
	for id := range s.RemovedChats {
		known[id] = true
	}
	for source, target := range s.ChatBindings {
		known[source], known[target] = true, true
	}
	for _, tr := range append(valuesOfChatTransfers(s.ChatTransfers), s.ChatTransferHistory...) {
		known[tr.Source] = true
		if tr.Target != "" {
			known[tr.Target] = true
		}
	}
	delete(known, "")
	for id, reservation := range s.RemovedChats {
		if reservation.Stage == "deleted" || reservation.Stage == "removed" {
			delete(known, id)
		}
	}
	out := make([]string, 0, len(known))
	for id := range known {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

type LogicalChatMetadata struct{ Title, Workspace, Lifecycle string }

func logicalChatMetadataFromSettings(s Settings, reference string) (LogicalChatMetadata, error) {
	members, err := logicalChatMembersFromSettings(s, reference)
	out := LogicalChatMetadata{Lifecycle: "active"}
	for _, member := range members.Members {
		if _, reserved := s.RemovedChats[member.SessionID]; reserved {
			out.Lifecycle = "reserved"
		}
	}
	if _, reserved := s.RemovedChats[reference]; reserved {
		out.Lifecycle = "reserved"
	}
	if err != nil {
		return out, err
	}
	providerTitle := s.ListedChats[members.Head].Title
	if providerTitle == "" {
		providerTitle = s.ListedChats[members.Head].LocalTitle
	}
	out.Title = inventoryTitle(s, members.Head, providerTitle, "")
	out.Workspace = s.logicalEvidence[members.Head].Workspace
	provider := s.SessionProviders[members.Head]
	if provider == "" {
		provider = s.ListedChats[members.Head].Provider
	}
	if provider == "" {
		provider = s.logicalEvidence[members.Head].Provider
	}
	if provider != "" {
		if _, recognized := providerRegistry[provider]; !recognized {
			return out, fmt.Errorf("recorded chat program is unrecognized")
		}
	}
	if cached, ok := s.ListedChats[members.Head]; ok {
		if cached.Repair != "" {
			return out, fmt.Errorf("%s", cached.Repair)
		}
		if cached.Provider == "" || cached.Workspace == "" {
			return out, fmt.Errorf("recorded chat metadata is incomplete")
		}
		if provider := s.SessionProviders[members.Head]; provider != "" && provider != cached.Provider {
			return out, fmt.Errorf("recorded chat program conflicts with saved ownership")
		}
		if header := s.logicalEvidence[members.Head]; (header.Provider != "" && header.Provider != cached.Provider) || (header.Workspace != "" && !layout.SamePath(header.Workspace, cached.Workspace)) {
			return out, fmt.Errorf("recorded chat metadata conflicts with local ownership")
		}
		if out.Workspace == "" {
			out.Workspace = cached.Workspace
		}
	}
	return out, nil
}

func inventoryTitle(s Settings, id, providerTitle, busTitle string) string {
	if title := s.SessionTitles[id]; title != "" {
		return title
	}
	if fromAppletContext(providerTitle) {
		providerTitle = ""
	}
	if judged, ok := s.JudgedTitles[id]; ok && (providerTitle == "" || isCutOf(providerTitle, judged.Line)) {
		return judged.Title
	}
	if providerTitle != "" {
		return providerTitle
	}
	if !fromAppletContext(busTitle) {
		return busTitle
	}
	return ""
}
