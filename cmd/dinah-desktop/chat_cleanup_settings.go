package main

import (
	"encoding/json"
	"maps"
	"slices"
)

// chatCleanupSettings cannot serialize unrelated application or workspace state.
type chatCleanupSettings struct {
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
}

// cleanupSavedSettings retains only fields owned by the selected sessions.
// Referenced documents, workspace data and application credentials never enter
// the recycle payload or the restoration write set.
func cleanupSavedSettings(s Settings, ids []string) chatCleanupSettings {
	out := chatCleanupSettings{
		SessionProviders: map[string]string{}, SessionTitles: map[string]string{},
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
	return out
}

func cleanupMergeMap[K comparable, V any](dst *map[K]V, source map[K]V) {
	if *dst == nil {
		*dst = map[K]V{}
	}
	maps.Copy(*dst, source)
}

func cleanupRestoreSettings(s *Settings, saved chatCleanupSettings) {
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
}
