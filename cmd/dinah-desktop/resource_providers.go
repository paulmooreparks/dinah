package main

import (
	"errors"

	"studio/internal/resource"
)

// provider returns the provider of the source a reference belongs to, from
// package workspaces' providers, where each kind of source plugs in once.
func (s workspaceResourceService) provider(ref ResourceRef) (any, WorkspaceSource, error) {
	return s.resolver().Provider(s.ctx, ref)
}

// resourceErrorText turns the shared resource errors into what a person reads.
func resourceErrorText(err error) error {
	switch {
	case errors.Is(err, resource.ErrConflict):
		return errors.New("the document was changed elsewhere since it was opened; compare or reload before saving")
	case errors.Is(err, resource.ErrUnsupported):
		return errors.New("this source cannot be written from " + product.Name)
	case errors.Is(err, resource.ErrNotFound):
		return errors.New("the document no longer exists")
	}
	return err
}
