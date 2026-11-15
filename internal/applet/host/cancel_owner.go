package host

// CancelOwner cancels work in every retained broker generation without
// creating a registry, connection or broker merely to revoke an owner.
func (s *Service) CancelOwner(workspace, kind, owner string) {
	for _, b := range s.snapshot() {
		b.CancelOwner(workspace, kind, owner)
	}
}
