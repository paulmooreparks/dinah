package main

import "studio/internal/sessionhost"

func (a *App) transcriptCapture() *sessionhost.TranscriptCapture {
	a.captureOnce.Do(func() {
		a.captureService = sessionhost.NewTranscriptCapture(&a.cmu, sessionhost.TranscriptCaptureHost{
			StateDir:    configDir,
			Session:     a.session,
			FlushReplay: a.flushReplay,
			ReplayBatch: replayBatch,
			VisibleText: visibleConversationText,
			RedactText:  redactAppletContexts,
			Log:         logf,
			Emit:        a.emit,
			TurnEnd:     func(id, reason string, err error) { a.chatLogs().TurnEnd(id, reason, err) },
		})
	})
	return a.captureService
}
