package diagrampicture

const startupMessage = "Drawing is unavailable until the earlier page has finished starting, or until the window restarts. Nothing was uploaded."

type startupPending struct{ settled <-chan error }

func (e *startupPending) Error() string { return startupMessage }

// RecoverAfterStartup holds admission closed until the native completion
// handler confirms disposal. A failed Close keeps drawing disabled.
func RecoverAfterStartup(settled <-chan error) error { return &startupPending{settled: settled} }
