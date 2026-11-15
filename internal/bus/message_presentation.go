package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"unicode/utf8"
)

type MessageReference struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Workspace string `json:"workspace,omitempty"`
}

type MessagePresentation struct {
	ShownText  string
	References []MessageReference
}

func (b *Bus) SendFilesPresented(ctx context.Context, to, text string, files []PersonFile, replyTo int64, mentions []string, presentation MessagePresentation) (Message, error) {
	if !utf8.ValidString(presentation.ShownText) || len(presentation.ShownText) > 1<<20 || len(presentation.References) > 128 {
		return Message{}, errors.New("message references exceed their limits")
	}
	for _, r := range presentation.References {
		if !slices.Contains([]string{"draft", "chat", "window", "applet", "resource"}, r.Kind) || !utf8.ValidString(r.Name+r.Token+r.Workspace) || len(r.Name) > 4096 || len(r.Token) > 4096 || len(r.Workspace) > 4096 {
			return Message{}, errors.New("invalid message reference")
		}
	}
	atts, err := b.storeFiles(files)
	if err != nil {
		return Message{}, err
	}
	m, err := b.sendPresented(ctx, Sender{}, to, text, atts, replyTo, mentions, false, &presentation)
	if err != nil {
		for _, at := range atts {
			_ = os.Remove(at.Path)
		}
	}
	return m, err
}

func storeMessagePresentation(t *sql.Tx, id int64, p MessagePresentation) error {
	refs := slices.Clone(p.References)
	if refs == nil {
		refs = []MessageReference{}
	}
	encoded, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	_, err = t.Exec(`INSERT INTO message_presentation VALUES(?,?,?)`, id, p.ShownText, string(encoded))
	return err
}

func (b *Bus) messagePresentation(ctx context.Context, m *Message) error {
	var shown, encoded string
	err := b.db.QueryRowContext(ctx, `SELECT shown_text,references_json FROM message_presentation WHERE message_id=?`, m.ID).Scan(&shown, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	m.ShownText = &shown
	return json.Unmarshal([]byte(encoded), &m.References)
}
