package host

import "errors"

// CancelKnown reaches retained broker generations without creating a broker.
func (s *Service) CancelKnown(workspace, kind, owner, id string) error {
	var errs []error
	for _, b := range s.snapshot() {
		errs = append(errs, b.Cancel(workspace, kind, owner, id))
	}
	return errors.Join(errs...)
}

// CancelOwner cancels work in every retained broker generation without
// creating a registry, connection or broker merely to revoke an owner.
func (s *Service) CancelOwner(workspace, kind, owner string) {
	for _, b := range s.snapshot() {
		b.CancelOwner(workspace, kind, owner)
	}
}
