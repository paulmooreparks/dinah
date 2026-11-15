package acp

// SessionAllocationError carries a received identity through subsequent setup
// failure. It does not imply that the session is configured or usable.
type SessionAllocationError struct {
	SessionID string
	Err       error
}

func (e *SessionAllocationError) Error() string { return e.Err.Error() }
func (e *SessionAllocationError) Unwrap() error { return e.Err }
