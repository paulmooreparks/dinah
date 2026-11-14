package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"studio/internal/appletcontext"
)

type DecisionBroadcastTarget struct {
	Session   string `json:"session"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
}

type DecisionBroadcastPreview struct {
	Token     string                    `json:"token"`
	Scope     string                    `json:"scope"`
	Text      string                    `json:"text"`
	CreatedAt string                    `json:"createdAt"`
	ExpiresAt string                    `json:"expiresAt"`
	Count     int                       `json:"count"`
	Targets   []DecisionBroadcastTarget `json:"targets"`
}

type ConfirmDecisionBroadcastInput struct {
	OperationID string `json:"operationId"`
	Token       string `json:"token"`
}

// PreviewDecisionBroadcast freezes a recipient snapshot without delivering it.
// working contains session identities observed working by the calling window.
func (b *Bus) PreviewDecisionBroadcast(ctx context.Context, scope, text string, working []string) (DecisionBroadcastPreview, error) {
	out := DecisionBroadcastPreview{Scope: scope, Text: appletcontext.Redact(text), Targets: []DecisionBroadcastTarget{}}
	if (scope != "active" && scope != "all") || !decisionText(text, 16000) {
		return out, errors.New("choose active or all and give broadcast text")
	}
	var err error
	out.Token, err = decisionToken()
	if err != nil {
		return out, err
	}
	now := b.now().UTC()
	out.CreatedAt = now.Format(time.RFC3339Nano)
	out.ExpiresAt = now.Add(5 * time.Minute).Format(time.RFC3339Nano)
	liveWorking := map[string]bool{}
	for _, id := range working {
		liveWorking[id] = true
	}
	err = b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT s.id,s.title,COALESCE(NULLIF(w.label,''),w.name),
   EXISTS (SELECT 1 FROM hook_turns h WHERE h.session=s.id AND h.active=1)
   FROM sessions s JOIN workspaces w ON w.key=s.workspace
   WHERE s.removed_at IS NULL AND (s.ended_at IS NULL OR substr(s.id,1,?)=?)
   AND NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.member=s.id AND tm.retired=1)
   ORDER BY s.id`, len(WindowSessionPrefix), WindowSessionPrefix)
		if err != nil {
			return err
		}
		for rows.Next() {
			var target DecisionBroadcastTarget
			var hook bool
			if err = rows.Scan(&target.Session, &target.Title, &target.Workspace, &hook); err != nil {
				rows.Close()
				return err
			}
			if scope == "all" || hook || liveWorking[target.Session] {
				target.Title = DisplayTitle(target.Title, target.Workspace)
				out.Targets = append(out.Targets, target)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		out.Count = len(out.Targets)
		encoded, err := json.Marshal(out)
		if err != nil {
			return err
		}
		// Expired unconsumed previews have no effect and need not accumulate.
		// Used tokens remain as durable replay protection.
		if _, err = t.Exec(`DELETE FROM decision_previews WHERE consumed=0 AND expires_at<=?`, now.UnixNano()); err != nil {
			return err
		}
		_, err = t.Exec(`INSERT INTO decision_previews (token,preview,expires_at) VALUES (?,?,?)`, out.Token, string(encoded), now.Add(5*time.Minute).UnixNano())
		return err
	})
	return out, err
}

func (b *Bus) ConfirmDecisionBroadcast(ctx context.Context, in ConfirmDecisionBroadcastInput) (DecisionReceipt, error) {
	var out DecisionReceipt
	if !decisionOperationID(in.OperationID) || in.Token == "" {
		return out, errors.New("give an operation identity and preview token")
	}
	fp := decisionFingerprint("broadcast", in)
	err := b.tx(ctx, func(t *sql.Tx) error {
		var found bool
		var err error
		out, found, err = decisionReceipt(t, in.OperationID, fp)
		if err != nil || found {
			return err
		}
		var encoded string
		var consumed bool
		if err = t.QueryRow(`SELECT preview,consumed FROM decision_previews WHERE token=?`, in.Token).Scan(&encoded, &consumed); errors.Is(err, sql.ErrNoRows) {
			out, err = rejectDecision(t, in.OperationID, fp, "", 0, true, "preview_unavailable", "This broadcast preview is no longer available. Preview it again.")
			return err
		}
		if err != nil {
			return err
		}
		var preview DecisionBroadcastPreview
		if err = json.Unmarshal([]byte(encoded), &preview); err != nil {
			return err
		}
		expiry, err := time.Parse(time.RFC3339Nano, preview.ExpiresAt)
		if err != nil {
			return err
		}
		if consumed {
			out, err = rejectDecision(t, in.OperationID, fp, "", 0, true, "preview_consumed", "This broadcast preview has already been sent. Preview again to send a new message.")
			return err
		}
		if !b.now().Before(expiry) {
			out, err = rejectDecision(t, in.OperationID, fp, "", 0, true, "preview_expired", "This broadcast preview expired. Preview it again before sending.")
			return err
		}
		for _, target := range preview.Targets {
			available, err := decisionSessionAvailable(t, target.Session)
			if err != nil {
				return err
			}
			if !available {
				out, err = rejectDecision(t, in.OperationID, fp, "", 0, true, "preview_unavailable", "A chat in this preview is no longer available. Nothing was sent. Preview again.")
				return err
			}
		}
		out = DecisionReceipt{OperationID: in.OperationID, State: "committed", Closed: true, MessageIDs: []int64{}}
		for _, target := range preview.Targets {
			id, err := b.insertMessage(t, Sender{}, "text", chatResolved(target.Session, target.Title), 0, preview.Text, "", "", nil)
			if err != nil {
				return err
			}
			out.MessageIDs = append(out.MessageIDs, id)
		}
		if _, err = t.Exec(`UPDATE decision_previews SET consumed=1 WHERE token=?`, in.Token); err != nil {
			return err
		}
		return storeDecisionReceipt(t, fp, out)
	})
	return out, err
}
