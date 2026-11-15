package broker

import "studio/internal/layout"

// CancelOwner withdraws pending work after its host capability was revoked.
// The host serializes admission with revocation before calling this method.
// Completed records and resources referenced by invocations remain untouched.
func (b *Broker) CancelOwner(workspace, kind, owner string) {
	b.mu.Lock()
	var pending []*execution
	for _, ex := range b.active {
		if ex.ownerKind == kind && ex.owner == owner && layout.SamePath(ex.workspace, workspace) {
			pending = append(pending, ex)
		}
	}
	b.mu.Unlock()
	for _, ex := range pending {
		ex.cancel()
	}
}
