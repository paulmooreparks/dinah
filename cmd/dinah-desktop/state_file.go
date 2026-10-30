package main

import "studio/internal/statefile"

// The app's state files are written through package statefile, which the
// command line shares for the files both programs write.

func readStateFile(path string) ([]byte, error) { return statefile.Read(path) }

func readRegularStateFile(path string) ([]byte, error) { return statefile.ReadRegular(path) }

func writeStateFile(path string, data []byte) error { return statefile.Write(path, data) }

func writeValidatedStateFile(path string, data []byte, validatePrevious func([]byte) error) error {
	return statefile.WriteValidated(path, data, validatePrevious)
}
