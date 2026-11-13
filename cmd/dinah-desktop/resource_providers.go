package main

import (
	"errors"
	"fmt"
	"path"
	"strings"

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

// resourceMissing is resourceErrorText for a read of something by its path,
// where what is missing is said by its name. The read may be of a file that
// an open document shows, such as a diagram, and "the document no longer
// exists" then sent a person looking for a deleted document when one file
// was missing (Paul, 12 November 2026). The error is still
// resource.ErrNotFound to errors.Is, so a caller that treats a missing file
// as an answer and not as a failure still can (ReadWorkspaceMountText); as
// plain text it was not, and a document was given an error where it should
// have been told that its reference is missing.
func resourceMissing(err error, at string) error {
	if errors.Is(err, resource.ErrNotFound) {
		return missingResource(fmt.Sprintf("%s was not found; it may have been moved or renamed, or not put there yet", path.Base(strings.TrimRight(strings.ReplaceAll(at, `\`, "/"), "/"))))
	}
	return resourceErrorText(err)
}

// missingResource is what a person reads about something that is not there.
type missingResource string

func (m missingResource) Error() string        { return string(m) }
func (m missingResource) Is(target error) bool { return target == resource.ErrNotFound }
