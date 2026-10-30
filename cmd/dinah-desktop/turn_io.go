package main

import (
	"context"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func (a *App) turnIO() *sessionhost.TurnIO {
	a.turnOnce.Do(func() {
		a.turnService = sessionhost.NewTurnIO(sessionhost.TurnHost{
			LookupConversation: a.session, SessionConnection: a.sessionConn, HoldingTurns: a.holdingTurns, ContextGate: a.contextGate,
			ResolveContinuation: a.ResolveSessionContinuation, MakePrompt: a.makePrompt, PromptFrom: a.promptFrom,
			DraftDocRef: a.draftDocRef, WithDoc: withDoc, Remember: a.remember, Unpark: func(s *Session) { a.busCoordinator().Unpark(s) }, Journal: a.journal,
			RecordPrompt: func(s *Session, p *prompt) {
				a.noteFirstPrompt(s, p)
				a.noteRelayTurnStart(s, p)
				a.transcriptCapture().RecordPrompt(s, p)
			}, LogPrompt: func(id, label, text string) { a.chatLogs().Prompt(id, label, text) },
			RecordDelivered: func(s *Session, promptText string, entries []ConversationMessage) {
				a.transcriptCapture().RecordDelivered(s, promptText, entries)
			}, FinishTranscript: func(s *Session, err error, reason string) {
				a.relayTurnEnded(s, err)
				a.transcriptCapture().RecordTurnEnd(s, err, reason)
			}, CommitTurn: func(s *Session, turn map[string]any) *sessionhost.LoggedEvent {
				return a.transcriptCapture().CommitTurn(s, turn)
			}, DisconnectSubagents: func(root *Session) { a.subagentService().Disconnect(root) },
			CancelQuestions: func(session string) { a.permissions().CancelSession(session) }, Emit: a.emit, PrepareProviderApplets: prepareProviderApplets,
			AppletPromptContext: a.appletPromptContext, PendingContinuationContext: func(s *Session) (string, error) { return a.continuations().PendingContext(s) },
			ConsumeContinuationContext: func(s *Session) { a.transcriptCapture().ConsumeHandoff(s) }, Call: a.call, BusWaiting: func(s *Session) (string, []ConversationMessage) { return a.busCoordinator().Waiting(s) }, BusAfterTurn: func(s *Session) *prompt { return a.busCoordinator().AfterTurn(s) },
			HooksReach: func(id string) (bool, error) {
				var value bool
				err := a.busHost().With(func(b *bus.Bus) error { var err error; value, err = b.HooksReach(context.Background(), id); return err })
				return value, err
			},
			QueueHookSteer: func(id, text string) (int64, error) {
				var value int64
				err := a.busHost().With(func(b *bus.Bus) error {
					var err error
					value, err = b.QueueSteer(context.Background(), id, text)
					return err
				})
				return value, err
			},
			WithdrawHookSteer: func(id int64) (bool, error) {
				var value bool
				err := a.busHost().With(func(b *bus.Bus) error {
					var err error
					value, err = b.WithdrawSteer(context.Background(), id)
					return err
				})
				return value, err
			},
			BeginHookTurn: func(id, token string) error {
				return a.busHost().With(func(b *bus.Bus) error { return b.BeginHookTurn(context.Background(), id, token) })
			},
			EndHookTurn: func(id, token string) error {
				return a.busHost().With(func(b *bus.Bus) error { return b.EndHookTurn(context.Background(), id, token) })
			},
			SaveSteering: func(s *Session, text string, doc *DocRef, atts []Attachment, p prompt) (int64, error) {
				return a.steeringLedger().Save(s, text, doc, atts, p)
			}, SteeringState: func(session string, id int64, state string) { a.steeringLedger().State(session, id, state) }, LogError: func(area, format string, args ...any) { logf(area, "error", format, args...) },
			VisibleConversationText: visibleConversationText,
		})
	})
	return a.turnService
}
