package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"time"

	"studio/internal/bus"
	"studio/internal/providers"
)

// Recovery notices are a projection of one incident per original recipient.
// Dismissal changes this projection, never its queue or transfer authority.
func (a *App) recoveryNotifications() ([]Notification, error) {
	waiting, err := a.waitingChatNotifications()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	transfers := maps.Clone(a.settings.ChatTransfers)
	titles := maps.Clone(a.settings.SessionTitles)
	a.mu.Unlock()
	records, readErr := a.busCoordinator().WaitingFailures()
	items := append(waiting, a.transferNotifications()...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Lead < items[j].Lead })
	grouped := map[string]Notification{}
	versions := map[string][]string{}
	for _, item := range items {
		if a.removedChat(item.Lead) {
			continue
		}
		if item.Kind != "delivery-failure" {
			data, _ := json.Marshal(item.Details)
			item.IncidentVersion = fmt.Sprintf("%x", sha256.Sum256(data))
			grouped[item.ID] = item
			continue
		}
		old, ok := grouped[item.ID]
		if ok {
			item.Details = append(old.Details, item.Details...)
			if old.Created.Before(item.Created) {
				item.Created = old.Created
			}
		}
		grouped[item.ID] = item
		if r, ok := records[item.Lead]; ok {
			versions[item.ID] = append(versions[item.ID], r.Attempt+":"+r.Stage)
		}
		if tr, ok := transfers[item.Lead]; ok {
			versions[item.ID] = append(versions[item.ID], tr.Revision+":"+tr.Stage+":"+tr.Target)
		}
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []Notification{}
	for _, key := range keys {
		item := grouped[key]
		if item.Kind != "delivery-failure" {
			out = append(out, item)
			continue
		}
		origin := item.Lead
		recipient := records[origin].Route.Recipient
		tr, hasTransfer := transfers[origin]
		if hasTransfer && tr.Recipient != "" {
			recipient = tr.Recipient
		}
		item.OriginTitle = titles[origin]
		item.OriginProvider, _ = a.savedSessionProvider(origin)
		count := -1
		queueErr := a.busHost().With(func(b *bus.Bus) error {
			var err error
			count, err = b.PendingSessionDeliveries(context.Background(), recipient)
			if err != nil {
				return err
			}
			if session, err := b.Session(context.Background(), recipient); err == nil {
				item.OriginStarted = session.StartedAt
				if item.OriginTitle == "" {
					item.OriginTitle = session.Title
				}
			}
			return nil
		})
		if item.OriginTitle == "" {
			item.OriginTitle = "Untitled Chat"
		}
		label := item.OriginTitle
		if item.OriginProvider != "" {
			label += " (" + providers.Name(item.OriginProvider)
			if started, err := time.Parse(time.RFC3339Nano, item.OriginStarted); err == nil {
				label += ", " + started.Local().Format("2 Jan 15:04")
			}
			label += ")"
		}
		item.Title = label + " needs attention"
		item.SessionTitle = label
		switch {
		case queueErr != nil || readErr != nil:
			item.Body = "The chat needs attention. Its waiting-message count could not be checked."
			item.Details = append(item.Details, fmt.Sprintf("Queue check: %v; recovery check: %v", queueErr, readErr))
		case hasTransfer && tr.Stage != "committed":
			item.Body = "Recovery did not finish. "
			if count == 0 {
				item.Body += "No messages are waiting."
			} else {
				item.Body += pendingWords(count)
			}
			item.RetryAllowed = false
		case count == 0:
			item.Body = "This earlier chat could not reconnect. No messages are waiting."
		default:
			item.Body = pendingWords(count) + " Recover this original chat or remove it."
		}
		if hasTransfer && tr.Stage == "committed" {
			item.Body = "Recovery is committed. Refresh Recovery Status to reconcile the remaining notice."
			item.RetryAllowed = false
		}
		sort.Strings(versions[key])
		data, _ := json.Marshal(versions[key])
		item.IncidentVersion = fmt.Sprintf("%x", sha256.Sum256(data))
		out = append(out, item)
	}
	return out, nil
}

func pendingWords(count int) string {
	if count == 1 {
		return "1 message is waiting."
	}
	return fmt.Sprintf("%d messages are waiting.", count)
}
