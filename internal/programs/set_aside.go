package programs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SetAsideProgram is a retained executable and the actions legal for it now.
type SetAsideProgram struct {
	ID          string   `json:"id"`
	Program     string   `json:"program"`
	Name        string   `json:"name"`
	SizeBytes   int64    `json:"sizeBytes"`
	Date        string   `json:"date"`
	RunningPids []uint32 `json:"runningPids"`
	Fallback    bool     `json:"fallback"`
	CanRemove   bool     `json:"canRemove"`
	ReasonCode  string   `json:"reasonCode"`
	Reason      string   `json:"reason"`
}

type SetAsideList struct {
	Items      []SetAsideProgram `json:"items"`
	TotalBytes int64             `json:"totalBytes"`
	Warnings   []string          `json:"warnings"`
}

type RemoveSetAsideInput struct {
	IDs []string `json:"ids"`
}

type RemovalResult struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	ReasonCode string `json:"reasonCode"`
	Reason     string `json:"reason"`
}

type RemoveSetAsideResult struct {
	Items        []RemovalResult `json:"items"`
	RemovedBytes int64           `json:"removedBytes"`
}

// SetAsideService inspects one product bin directory. Names have no extension.
type SetAsideService struct {
	bin      string
	programs []string
}

// cleanupMu serializes removals, including through separate service instances.
var cleanupMu sync.Mutex

func NewSetAsideService(bin string, programNames []string) *SetAsideService {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	names := []string{}
	seen := map[string]bool{}
	for _, name := range programNames {
		if name != "" && filepath.Base(name) == name && !strings.ContainsAny(name, `/\:`) && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return &SetAsideService{bin: abs, programs: names}
}

type asideFile struct {
	SetAsideProgram
	path     string
	identity string
	stamp    time.Time
	old      bool
	valid    bool
}

// asideDate recognizes only installer-owned prefixes. Bad dates stay visible.
func asideDate(name, program string) (stamp time.Time, old, valid, match bool) {
	if suffix, ok := strings.CutPrefix(name, FileName(program)+".old-"); ok {
		part, _, _ := strings.Cut(suffix, "-")
		if len(part) == 17 {
			stamp, err := time.Parse("20060102150405.000", part[:14]+"."+part[14:])
			return stamp, true, err == nil, true
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err == nil && n > 0 {
			return time.Unix(0, n).UTC(), true, true, true
		}
		return time.Time{}, true, false, true
	}
	if suffix, ok := strings.CutPrefix(name, FileName(program)+".before-"); ok {
		// Labels can contain hyphens, so only the final date and time count.
		if len(suffix) < 17 || suffix[len(suffix)-16] != '-' || suffix[len(suffix)-7] != '-' {
			return time.Time{}, false, false, true
		}
		stamp, err := time.Parse("20060102-150405", suffix[len(suffix)-15:])
		return stamp, false, err == nil, true
	}
	return time.Time{}, false, false, false
}

func (s *SetAsideService) scan() ([]asideFile, error) {
	entries, err := os.ReadDir(s.bin)
	if errors.Is(err, os.ErrNotExist) {
		return []asideFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []asideFile{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		for _, program := range s.programs {
			stamp, old, valid, match := asideDate(entry.Name(), program)
			if !match {
				continue
			}
			path := filepath.Join(s.bin, entry.Name())
			info, identity, err := inspectAside(path)
			if err != nil {
				return nil, fmt.Errorf("inspect %s: %w", entry.Name(), err)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			date := stamp
			if !valid {
				date = info.ModTime().UTC()
			}
			key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d", s.bin, entry.Name(), identity, info.Size(), info.ModTime().UnixNano())
			sum := sha256.Sum256([]byte(key))
			files = append(files, asideFile{
				SetAsideProgram: SetAsideProgram{ID: hex.EncodeToString(sum[:]), Program: program, Name: entry.Name(), SizeBytes: info.Size(), Date: date.Format(time.RFC3339Nano), RunningPids: []uint32{}},
				path:            path, identity: identity, stamp: date, old: old, valid: valid,
			})
			break
		}
	}
	for i := range files {
		file := &files[i]
		file.CanRemove = true
		for _, other := range files {
			if other.Program != file.Program {
				continue
			}
			if !other.valid {
				file.CanRemove = false
				file.ReasonCode = "unrecognized_date"
				file.Reason = "The date of a copy could not be read. No copies can be removed until the newest copy kept for going back can be identified."
			}
		}
		if !file.valid {
			continue
		}
		newest := true
		for _, other := range files {
			if other.Program == file.Program && other.valid && ((other.old && !file.old) || (other.old == file.old && other.stamp.After(file.stamp))) {
				newest = false
			}
		}
		file.Fallback = newest
		if newest {
			file.CanRemove = false
			file.ReasonCode = "fallback"
			file.Reason = "This is the newest set-aside copy kept for going back."
		}
	}
	sort.Slice(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.Program != b.Program {
			return a.Program < b.Program
		}
		if a.Date != b.Date {
			return a.stamp.After(b.stamp)
		}
		return a.Name < b.Name
	})
	return files, nil
}

func (s *SetAsideService) listFiles() ([]asideFile, []string, error) {
	files, err := s.scan()
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return files, []string{}, nil
	}
	images, warnings, err := runningAsideImages()
	if err != nil {
		return nil, nil, err
	}
	for i := range files {
		file := &files[i]
		if pids := images[file.identity]; len(pids) > 0 {
			file.RunningPids = pids
			file.CanRemove = false
			file.ReasonCode = "in_use"
			file.Reason = "This copy is in use by a running program."
		}
		if !asideRemovalSupported {
			file.CanRemove = false
			file.ReasonCode = "unsupported"
			file.Reason = "Removing set-aside programs is supported only on Windows."
		}
	}
	return files, warnings, nil
}

func (s *SetAsideService) List() (SetAsideList, error) {
	out := SetAsideList{Items: []SetAsideProgram{}, Warnings: []string{}}
	files, warnings, err := s.listFiles()
	if err != nil {
		return out, err
	}
	out.Warnings = append(out.Warnings, warnings...)
	for _, file := range files {
		out.Items = append(out.Items, file.SetAsideProgram)
		out.TotalBytes += file.SizeBytes
	}
	return out, nil
}

func (s *SetAsideService) Remove(in RemoveSetAsideInput) (RemoveSetAsideResult, error) {
	out := RemoveSetAsideResult{Items: []RemovalResult{}}
	if len(in.IDs) == 0 || len(in.IDs) > 100 {
		return out, errors.New("Choose between 1 and 100 set-aside copies.")
	}
	seen := map[string]bool{}
	for _, id := range in.IDs {
		if len(id) != 64 || seen[id] {
			return out, errors.New("The selected copies could not be identified. Refresh the list and select them again.")
		}
		if _, err := hex.DecodeString(id); err != nil {
			return out, errors.New("The selected copies could not be identified. Refresh the list and select them again.")
		}
		seen[id] = true
	}
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	files, _, err := s.listFiles()
	if err != nil {
		return out, err
	}
	for _, id := range in.IDs {
		result := RemovalResult{ID: id, State: "refused", ReasonCode: "changed_or_missing", Reason: "This copy changed or is no longer present; refresh the list."}
		for _, file := range files {
			if file.ID != id {
				continue
			}
			if !file.CanRemove {
				result.ReasonCode, result.Reason = file.ReasonCode, file.Reason
				break
			}
			code, err := removeAside(file, func() error {
				// The deletion handle keeps this file from being replaced while
				// fallback selection is refreshed. Ordinary disposition checks
				// mapped-image use in the kernel at the final mutation.
				current, err := s.scan()
				if err != nil {
					return err
				}
				for _, item := range current {
					if item.ID == id && item.CanRemove {
						return nil
					}
				}
				return errors.New("This copy changed or is now protected. Refresh the list.")
			})
			if err != nil {
				result.ReasonCode, result.Reason = code, err.Error()
			} else {
				result.State, result.ReasonCode, result.Reason = "removed", "", ""
				out.RemovedBytes += file.SizeBytes
			}
			break
		}
		out.Items = append(out.Items, result)
	}
	return out, nil
}

// SweepResult says what a sweep removed and what it left for the next one.
type SweepResult struct {
	Removed      int
	RemovedBytes int64
	// InUse is how many of the installer's copies a running program still
	// uses. They are removed by a later sweep.
	InUse int
	// Refused is how many the removal itself refused, as when a program
	// started from a copy after the list was made.
	Refused int
}

// Sweep removes every copy an installation set aside that Remove would
// remove for the person: all but the newest of each program, which is kept
// for going back, and all but those a running program still uses.
//
// Every installation sets the programs it replaces aside (Place), and for a
// time nothing removed them but the person, in Settings. Paul found some 750
// of them in his two program folders on 20 November 2026, 62 GB, and called
// that unacceptable. An installation's last step and the window's start now
// sweep. The rules of what may go are Remove's, unchanged.
//
// A copy somebody made by hand, named .before- and a label, is listed for
// the person and is not swept: nothing that was made on purpose is removed
// without being asked for.
func (s *SetAsideService) Sweep() (SweepResult, error) {
	var out SweepResult
	files, _, err := s.listFiles()
	if err != nil {
		return out, err
	}
	var ids []string
	for _, file := range files {
		switch {
		case !file.old:
		case file.CanRemove:
			ids = append(ids, file.ID)
		case file.ReasonCode == "in_use":
			out.InUse++
		}
	}
	for len(ids) > 0 {
		n := min(len(ids), 100)
		result, err := s.Remove(RemoveSetAsideInput{IDs: ids[:n]})
		if err != nil {
			return out, err
		}
		for _, item := range result.Items {
			if item.State == "removed" {
				out.Removed++
			} else {
				out.Refused++
			}
		}
		out.RemovedBytes += result.RemovedBytes
		ids = ids[n:]
	}
	return out, nil
}
