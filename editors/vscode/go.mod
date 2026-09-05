// This module exists only to end the dinah module's boundary at this
// directory. editors/vscode is an npm-managed TypeScript project; one of
// its (transitive, dev-only) dependencies vendors third-party Go source
// as reference material (see dinah-274). Go's own package-matching walk
// skips any subdirectory that carries its own go.mod, so nothing under
// this directory is ever part of the dinah module's build, vet, test or
// package graph, regardless of which npm dependency ships stray Go
// source here in the future.
//
// This module declares no packages, needs no dependencies, and is never
// built.
module dinah/editors/vscode/boundary

go 1.25.0
