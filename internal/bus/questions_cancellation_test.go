package bus

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"modernc.org/sqlite"
)

// The wrapper places cancellation at transaction boundaries without sleeps.
type cancelCommitDriver struct{ hooks *questionHooks }

func (d cancelCommitDriver) Open(name string) (driver.Conn, error) {
	c, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &cancelCommitConn{Conn: c, hooks: d.hooks}, nil
}

type cancelCommitConn struct {
	driver.Conn
	hooks *questionHooks
}

func (c *cancelCommitConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if err != nil {
		return nil, err
	}
	return cancelCommitTx{Tx: tx, hooks: c.hooks}, nil
}
func (c *cancelCommitConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}
func (c *cancelCommitConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.HasPrefix(q, messageSelect) && c.hooks.beforeRead != nil {
		c.hooks.beforeRead()
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}

type cancelCommitTx struct {
	driver.Tx
	hooks *questionHooks
}

func (t cancelCommitTx) Commit() error {
	if t.hooks.commitErr != nil {
		_ = t.Tx.Rollback()
		return t.hooks.commitErr
	}
	err := t.Tx.Commit()
	if err == nil && t.hooks.afterCommit != nil {
		t.hooks.afterCommit()
	}
	return err
}

type questionHooks struct {
	beforeRead  func()
	afterCommit func()
	commitErr   error
}

var questionDriverID atomic.Uint64

func TestAskCancellationAtTransactionBoundaries(t *testing.T) {
	for _, phase := range []string{"before-read", "after-commit", "commit-error"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bus.db")
			b, err := OpenAt(path)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			ctx := context.Background()
			ws := t.TempDir()
			from, err := b.RegisterSession(ctx, "from", ws, "From")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = b.RegisterSession(ctx, "to", ws, "To"); err != nil {
				t.Fatal(err)
			}
			if err = b.db.Close(); err != nil {
				t.Fatal(err)
			}
			hooks := &questionHooks{}
			name := fmt.Sprintf("question-cancel-%d", questionDriverID.Add(1))
			sql.Register(name, cancelCommitDriver{hooks: hooks})
			b.db, err = sql.Open(name, "file:"+filepath.ToSlash(path)+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate")
			if err != nil {
				t.Fatal(err)
			}
			askCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			commitErr := errors.New("commit refused")
			switch phase {
			case "before-read":
				hooks.beforeRead = cancel
			case "after-commit":
				hooks.afterCommit = cancel
			case "commit-error":
				hooks.commitErr = commitErr
			}
			m, err := b.Ask(askCtx, from, "to", "question")
			hooks.beforeRead, hooks.afterCommit, hooks.commitErr = nil, nil, nil
			waiting, readErr := b.WaitingQuestions(ctx)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if phase != "after-commit" {
				want := error(context.Canceled)
				if phase == "commit-error" {
					want = commitErr
				}
				if !errors.Is(err, want) || !reflect.DeepEqual(m, Message{}) || len(waiting) != 0 {
					t.Fatalf("failed transaction: message=%+v err=%v waiting=%v", m, err, waiting)
				}
				return
			}
			if err != nil || askCtx.Err() != context.Canceled || m.ID == 0 || len(waiting) != 1 || waiting[0] != m.ID {
				t.Fatalf("committed question: message=%+v err=%v waiting=%v", m, err, waiting)
			}
			stored, err := b.Message(ctx, m.ID)
			if err != nil || !reflect.DeepEqual(m, stored) || m.Attachments != nil {
				t.Fatalf("returned representation differs: returned=%+v stored=%+v err=%v", m, stored, err)
			}
			if err := b.WithdrawQuestion(ctx, m.ID); err != nil {
				t.Fatal(err)
			}
			waiting, err = b.WaitingQuestions(ctx)
			if err != nil || len(waiting) != 0 {
				t.Fatalf("withdrawal: %v %v", waiting, err)
			}
		})
	}
}
