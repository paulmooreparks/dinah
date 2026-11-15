package main

import "sort"

// These interim pure bodies can be replaced by the durable logical model.
// Native history and provider registrations alone supply no local reference.
func logicalChatReferencesFromSettings(s Settings) []string {
	known := map[string]bool{}
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
	out.Title = s.SessionTitles[members.Head]
	out.Workspace = s.logicalEvidence[members.Head].Workspace
	return out, nil
}
