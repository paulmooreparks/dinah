package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"

	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/statefile"
)

var applicationStateDir string

const migrationPlanName = ".legacy-migration-plan.json"
const migrationDoneName = ".legacy-migration-complete.json"

type stateMigrationPlan struct {
	Version     int               `json:"version"`
	Source      string            `json:"source"`
	Directories []string          `json:"directories"`
	Files       map[string]string `json:"files"`
}

func initializeStateDirectory() error {
	if configOverride != "" {
		return os.MkdirAll(configOverride, 0o700)
	}
	// Dinah keeps its state in its own home folder, beside the bus, and
	// never in Dinah's.
	target, err := bus.Home()
	if err != nil {
		return fmt.Errorf("find %s's home folder: %w", product.Name, err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	applicationStateDir = target
	return nil
}

func stateDirectory(root string) error { return statefile.Directory(root) }

// stateLayout is where things are kept under the app's state folder.
func stateLayout() layout.Layout { return layout.Layout{State: configDir()} }

func stateFileHash(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func scanStateMigration(source string) (stateMigrationPlan, error) {
	plan := stateMigrationPlan{Version: 1, Source: source, Directories: []string{}, Files: map[string]string{}}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == source && errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot migrate the symbolic link %s; resolve it before retrying", path)
		}
		if rel == "." {
			if !entry.IsDir() {
				return fmt.Errorf("%s is not a directory", source)
			}
			return nil
		}
		if rel == migrationPlanName || rel == migrationDoneName {
			return fmt.Errorf("legacy state contains the reserved migration filename %s", path)
		}
		if entry.IsDir() {
			plan.Directories = append(plan.Directories, rel)
			return nil
		}
		hash, err := stateFileHash(path)
		if err != nil {
			return err
		}
		plan.Files[rel] = hash
		return nil
	})
	return plan, err
}

// publishStateFile creates a new name for a fully written, closed file, so
// migration never replaces another file.
func publishStateFile(path string, data []byte) error { return statefile.Publish(path, data) }

func copyMigrationFile(source, target, expected string) error {
	if actual, err := stateFileHash(target); err == nil {
		if actual == expected {
			return nil
		}
		return fmt.Errorf("migration conflict at %s; keep both versions and resolve the difference before retrying", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(target), ".state-copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmp, hash), in)
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("%s changed during migration; close the earlier application before retrying", source)
	}
	if err := os.Link(tmp.Name(), target); err != nil {
		if actual, readErr := stateFileHash(target); readErr == nil && actual == expected {
			return nil
		}
		return fmt.Errorf("publish migrated file %s without replacing existing data: %w", target, err)
	}
	return nil
}

func migrateStateDirectory(source, target string) error {
	source, target = filepath.Clean(source), filepath.Clean(target)
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || layout.SamePath(source, target) {
		return errors.New("migration requires separate absolute directories")
	}
	for _, pair := range [][2]string{{source, target}, {target, source}} {
		rel, _ := filepath.Rel(pair[0], pair[1])
		if filepath.IsLocal(rel) {
			return errors.New("migration directories must not contain one another")
		}
	}
	if err := stateDirectory(target); err != nil {
		return err
	}
	if data, err := readRegularStateFile(filepath.Join(target, migrationDoneName)); err == nil {
		var completed stateMigrationPlan
		if json.Unmarshal(data, &completed) != nil || completed.Version != 1 || !layout.SamePath(completed.Source, source) {
			return errors.New("the migration completion record is invalid or from an unsupported version")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	plan, err := scanStateMigration(source)
	if err != nil {
		return err
	}
	planPath := filepath.Join(target, migrationPlanName)
	if data, err := readRegularStateFile(planPath); err == nil {
		var previous stateMigrationPlan
		if json.Unmarshal(data, &previous) != nil || !reflect.DeepEqual(previous, plan) {
			return errors.New("the previous settings changed after migration began; close earlier " + product.Name + " instances and reconcile the migration plan before retrying")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, rel := range plan.Directories {
		if err := stateDirectory(filepath.Join(target, rel)); err != nil {
			return err
		}
	}
	// Check every destination before copying any source data.
	for rel, expected := range plan.Files {
		actual, err := stateFileHash(filepath.Join(target, rel))
		if err == nil && actual != expected {
			return fmt.Errorf("migration conflict at %s; existing files have not been replaced", filepath.Join(target, rel))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := publishStateFile(planPath, data); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		current, readErr := readRegularStateFile(planPath)
		var other stateMigrationPlan
		if readErr != nil || json.Unmarshal(current, &other) != nil || !reflect.DeepEqual(other, plan) {
			return errors.New("the migration plan changed; no existing file has been replaced")
		}
	}
	for rel, expected := range plan.Files {
		if err := copyMigrationFile(filepath.Join(source, rel), filepath.Join(target, rel), expected); err != nil {
			return err
		}
	}
	verified, err := scanStateMigration(source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, verified) {
		return errors.New("the previous settings changed during migration; close earlier " + product.Name + " instances before retrying")
	}
	if err := publishStateFile(filepath.Join(target, migrationDoneName), data); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		current, readErr := readRegularStateFile(filepath.Join(target, migrationDoneName))
		var other stateMigrationPlan
		if readErr != nil || json.Unmarshal(current, &other) != nil || !reflect.DeepEqual(other, plan) {
			return errors.New("the migration completion record changed; no existing file has been replaced")
		}
	}
	return nil
}
