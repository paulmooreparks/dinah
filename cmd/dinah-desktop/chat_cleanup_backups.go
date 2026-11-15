package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// A final deletion also removes chat-owned records from the application's
// migration rollback copies. Unknown fields and unrelated records survive.
func cleanupRedactBackups(ids []string) error {
	for _, name := range []string{"settings.json.before-split", "settings.json" + beforeDecisions} {
		if err := cleanupRewriteBackup(name, func(root map[string]json.RawMessage) error {
			for _, key := range []string{"sessionProviders", "sessionTitles", "sessionOptions", "judgedTitles", "nativeSessions", "nativeRecoveryAttempts", "busSessions", "chatBindings", "chatTransfers"} {
				if raw, ok := root[key]; ok {
					var values map[string]json.RawMessage
					if err := json.Unmarshal(raw, &values); err != nil {
						return err
					}
					for _, id := range ids {
						delete(values, id)
					}
					root[key], _ = json.Marshal(values)
				}
			}
			if raw, ok := root["chatTransferHistory"]; ok {
				var history []json.RawMessage
				if err := json.Unmarshal(raw, &history); err != nil {
					return err
				}
				kept := history[:0]
				for _, entry := range history {
					var transfer struct {
						Source string `json:"source"`
					}
					if err := json.Unmarshal(entry, &transfer); err != nil {
						return err
					}
					if !slices.Contains(ids, transfer.Source) {
						kept = append(kept, entry)
					}
				}
				root["chatTransferHistory"], _ = json.Marshal(kept)
			}
			if raw, ok := root["hidden"]; ok {
				var hidden []string
				if err := json.Unmarshal(raw, &hidden); err != nil {
					return err
				}
				hidden = slices.DeleteFunc(hidden, func(id string) bool { return slices.Contains(ids, id) })
				root["hidden"], _ = json.Marshal(hidden)
			}
			if raw, ok := root["workspaceRecords"]; ok {
				var workspaces []map[string]json.RawMessage
				if err := json.Unmarshal(raw, &workspaces); err != nil {
					return err
				}
				for _, workspace := range workspaces {
					if rawPins, ok := workspace["pins"]; ok {
						var pins []json.RawMessage
						if err := json.Unmarshal(rawPins, &pins); err != nil {
							return err
						}
						kept := pins[:0]
						for _, rawPin := range pins {
							var pin WorkspacePin
							if err := json.Unmarshal(rawPin, &pin); err != nil {
								return err
							}
							if pin.Kind != "chat" || !slices.Contains(ids, pin.Session) {
								kept = append(kept, rawPin)
							}
						}
						workspace["pins"], _ = json.Marshal(kept)
					}
				}
				root["workspaceRecords"], _ = json.Marshal(workspaces)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return cleanupRewriteBackup("notifications.json"+beforeDecisions, func(root map[string]json.RawMessage) error {
		var items []json.RawMessage
		if raw, ok := root["items"]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return err
			}
		}
		kept := items[:0]
		for _, raw := range items {
			var item Notification
			if err := json.Unmarshal(raw, &item); err != nil {
				return err
			}
			if slices.Contains(ids, item.SessionID) || slices.Contains(ids, item.Lead) {
				continue
			}
			if len(item.Chats) != 0 {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(raw, &fields); err != nil {
					return err
				}
				item.Chats = slices.DeleteFunc(item.Chats, func(chat BusChatRef) bool { return slices.Contains(ids, chat.ID) })
				if item.Kind == "restart-left" && len(item.Chats) == 0 {
					continue
				}
				fields["chats"], _ = json.Marshal(item.Chats)
				raw, _ = json.Marshal(fields)
			}
			kept = append(kept, raw)
		}
		root["items"], _ = json.Marshal(kept)
		return nil
	})
}

func cleanupRewriteBackup(name string, change func(map[string]json.RawMessage) error) error {
	path := filepath.Join(configDir(), name)
	data, err := readStateFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if root == nil {
		return errors.New("a rollback copy must contain an object")
	}
	if err := change(root); err != nil {
		return err
	}
	data, err = json.Marshal(root)
	if err != nil {
		return err
	}
	return writeStateFile(path, data)
}
