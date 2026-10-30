package main

// PreviewSessionRecovery reads only local records, even when the original
// provider cannot load its thread. The normal handoff budget still applies.
func (a *App) PreviewSessionRecovery(id, provider string) (SessionContinuationPreview, error) {
	return a.continuations().PreviewRecovery(id, provider)
}

func (a *App) RecoverSession(id, provider, revision string) (SessionContinuationResult, error) {
	return a.continuations().Recover(id, provider, revision)
}
