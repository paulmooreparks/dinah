package main

import (
	"encoding/json"
	"maps"
	"slices"

	"studio/internal/workspaces"
)

// chatCleanupSettings cannot serialize unrelated application or workspace
// state. Of a workspace it has only the chat's own pins.
type chatCleanupSettings struct {
	ListedChats            map[string]listedChat                 `json:"listedChats,omitempty"`
	SessionProviders       map[string]string                     `json:"sessionProviders,omitempty"`
	SessionTitles          map[string]string                     `json:"sessionTitles,omitempty"`
	SessionOptions         map[string]map[string]json.RawMessage `json:"sessionOptions,omitempty"`
	JudgedTitles           map[string]JudgedTitle                `json:"judgedTitles,omitempty"`
	NativeSessions         map[string]nativeSessionRecord        `json:"nativeSessions,omitempty"`
	NativeRecoveryAttempts map[string]string                     `json:"nativeRecoveryAttempts,omitempty"`
	BusSessions            map[string]string                     `json:"busSessions,omitempty"`
	ChatBindings           map[string]string                     `json:"chatBindings,omitempty"`
	ChatTransfers          map[string]chatTransfer               `json:"chatTransfers,omitempty"`
	ChatTransferHistory    []chatTransfer                        `json:"chatTransferHistory,omitempty"`
	Hidden                 []string                              `json:"hidden,omitempty"`
	// Pins are the chat's pins, each with its workspace and its place among
	// that workspace's pins. Removing the chat takes them off, and restoring it
	// puts them back.
	Pins []chatCleanupPin `json:"pins,omitempty"`
}

type chatCleanupPin struct {
	WorkspaceID string       `json:"workspaceId"`
	Index       int          `json:"index"`
	Pin         WorkspacePin `json:"pin"`
}

// cleanupChatPins are the pins of the sessions given, in the order of the
// workspaces and of each workspace's pins.
func cleanupChatPins(s Settings, ids []string) []chatCleanupPin {
	out := []chatCleanupPin{}
	for _, ws := range s.WorkspaceRecords {
		for index, pin := range ws.Pins {
			if pin.Kind == "chat" && slices.Contains(ids, pin.Session) {
				out = append(out, chatCleanupPin{WorkspaceID: ws.ID, Index: index, Pin: pin})
			}
		}
	}
	return out
}

func cleanupRemovePins(s *Settings, ids []string) {
	for i := range s.WorkspaceRecords {
		s.WorkspaceRecords[i].Pins = slices.DeleteFunc(s.WorkspaceRecords[i].Pins, func(pin WorkspacePin) bool {
			return pin.Kind == "chat" && slices.Contains(ids, pin.Session)
		})
	}
}

// cleanupRestorePins puts each pin back at its place, or at the end when the
// workspace has fewer pins now. A workspace that is gone, and a pin the
// person made again meanwhile, are passed over.
func cleanupRestorePins(s *Settings, pins []chatCleanupPin) {
	for _, saved := range pins {
		index := workspaces.Index(s.WorkspaceRecords, saved.WorkspaceID)
		if index < 0 {
			continue
		}
		current := s.WorkspaceRecords[index].Pins
		if slices.ContainsFunc(current, func(pin WorkspacePin) bool { return workspaces.PinKey(pin) == workspaces.PinKey(saved.Pin) }) {
			continue
		}
		s.WorkspaceRecords[index].Pins = slices.Insert(current, min(max(saved.Index, 0), len(current)), saved.Pin)
	}
}

// cleanupSavedSettings retains only fields owned by the selected sessions.
// Referenced documents, workspace data and application credentials never enter
// the recycle payload or the restoration write set.
func cleanupSavedSettings(s Settings, ids []string) chatCleanupSettings {
	out := chatCleanupSettings{
		SessionProviders: map[string]string{}, SessionTitles: map[string]string{},
		ListedChats:    map[string]listedChat{},
		SessionOptions: map[string]map[string]json.RawMessage{},
		JudgedTitles:   map[string]JudgedTitle{}, NativeSessions: map[string]nativeSessionRecord{},
		NativeRecoveryAttempts: map[string]string{}, BusSessions: map[string]string{},
		ChatBindings: map[string]string{}, ChatTransfers: map[string]chatTransfer{},
	}
	members := map[string]bool{}
	for _, id := range ids {
		members[id] = true
	}
	for _, id := range ids {
		if v, ok := s.ListedChats[id]; ok {
			out.ListedChats[id] = v
		}
		if v, ok := s.SessionProviders[id]; ok {
			out.SessionProviders[id] = v
		}
		if v, ok := s.SessionTitles[id]; ok {
			out.SessionTitles[id] = v
		}
		if v, ok := s.SessionOptions[id]; ok {
			out.SessionOptions[id] = maps.Clone(v)
		}
		if v, ok := s.JudgedTitles[id]; ok {
			out.JudgedTitles[id] = v
		}
		if v, ok := s.NativeSessions[id]; ok {
			out.NativeSessions[id] = v
		}
		if v, ok := s.NativeRecoveryAttempts[id]; ok {
			out.NativeRecoveryAttempts[id] = v
		}
		if v, ok := s.BusSessions[id]; ok {
			out.BusSessions[id] = v
		}
		if v, ok := s.ChatBindings[id]; ok && members[v] {
			out.ChatBindings[id] = v
		}
		if v, ok := s.ChatTransfers[id]; ok {
			out.ChatTransfers[id] = v
		}
	}
	for _, tr := range s.ChatTransferHistory {
		if members[tr.Source] {
			out.ChatTransferHistory = append(out.ChatTransferHistory, tr)
		}
	}
	for _, id := range s.Hidden {
		if members[id] {
			out.Hidden = append(out.Hidden, id)
		}
	}
	out.Pins = cleanupChatPins(s, ids)
	return out
}

func cleanupMergeMap[K comparable, V any](dst *map[K]V, source map[K]V) {
	if *dst == nil {
		*dst = map[K]V{}
	}
	maps.Copy(*dst, source)
}

func cleanupRestoreSettings(s *Settings, saved chatCleanupSettings) {
	cleanupMergeMap(&s.ListedChats, saved.ListedChats)
	cleanupMergeMap(&s.SessionProviders, saved.SessionProviders)
	cleanupMergeMap(&s.SessionTitles, saved.SessionTitles)
	cleanupMergeMap(&s.SessionOptions, saved.SessionOptions)
	cleanupMergeMap(&s.JudgedTitles, saved.JudgedTitles)
	cleanupMergeMap(&s.NativeSessions, saved.NativeSessions)
	cleanupMergeMap(&s.NativeRecoveryAttempts, saved.NativeRecoveryAttempts)
	cleanupMergeMap(&s.BusSessions, saved.BusSessions)
	cleanupMergeMap(&s.ChatBindings, saved.ChatBindings)
	cleanupMergeMap(&s.ChatTransfers, saved.ChatTransfers)
	for _, tr := range saved.ChatTransferHistory {
		if !slices.Contains(s.ChatTransferHistory, tr) {
			s.ChatTransferHistory = append(s.ChatTransferHistory, tr)
		}
	}
	cleanupRestorePins(s, saved.Pins)
}
