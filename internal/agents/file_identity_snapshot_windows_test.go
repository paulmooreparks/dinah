//go:build windows

package agents

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsSnapshotUses128BitFileID(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "identity"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, _ := file.Stat()
	data := fileIDInfo{VolumeSerialNumber: 1 << 48}
	binary.LittleEndian.PutUint64(data.FileID[:8], 7)
	binary.LittleEndian.PutUint64(data.FileID[8:], 1<<55)
	captured, err := snapshotOpenedIdentityWithQuery(file, info, func(_ windows.Handle, out *fileIDInfo) error { *out = data; return nil })
	key, ok := SnapshotIdentity(captured)
	if err != nil || !ok || key != (FileIdentity{1 << 48, 1 << 55, 7}) {
		t.Fatalf("truncated identity: %+v, %v", key, err)
	}
	data.FileID[15] ^= 1
	other, _ := snapshotOpenedIdentityWithQuery(file, info, func(_ windows.Handle, out *fileIDInfo) error { *out = data; return nil })
	otherKey, _ := SnapshotIdentity(other)
	if otherKey == key {
		t.Fatal("upper file ID bits were ignored")
	}
	actual, err := snapshotOpenedIdentity(file, info)
	if err != nil {
		t.Fatal(err)
	}
	var expected fileIDInfo
	raw, _ := file.SyscallConn()
	var queryErr error
	if err := raw.Control(func(handle uintptr) { queryErr = queryFileID(windows.Handle(handle), &expected) }); err != nil || queryErr != nil {
		t.Fatalf("FileIdInfo query: %v, %v", err, queryErr)
	}
	actualKey, ok := SnapshotIdentity(actual)
	if !ok || actualKey != (FileIdentity{expected.VolumeSerialNumber, binary.LittleEndian.Uint64(expected.FileID[8:]), binary.LittleEndian.Uint64(expected.FileID[:8])}) {
		t.Fatal("snapshot differs from FileIdInfo")
	}
}

func TestWindowsUnsupportedFileIDPreservesOrdinaryMetadata(t *testing.T) {
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "ordinary"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("ordinary content"); err != nil {
		t.Fatal(err)
	}
	info, _ := file.Stat()
	for _, unsupported := range []error{windows.ERROR_NOT_SUPPORTED, windows.ERROR_INVALID_PARAMETER, windows.ERROR_INVALID_FUNCTION} {
		captured, err := snapshotOpenedIdentityWithQuery(file, info, func(windows.Handle, *fileIDInfo) error { return unsupported })
		if err != nil || !os.SameFile(info, originalFileInfo(captured)) {
			t.Fatalf("ordinary metadata lost: %v", err)
		}
		if _, ok := SnapshotIdentity(captured); ok {
			t.Fatal("unsupported identity claimed verification")
		}
		if SnapshotIdentityReason(captured) != UnsupportedFileIdentityReason {
			t.Fatal("unsupported reason missing")
		}
		opened, err := openObjectSnapshotChecked(dir, "ordinary", false, func(chain []os.FileInfo) error {
			for _, ancestor := range chain {
				if SnapshotIdentityReason(ancestor) != UnsupportedFileIdentityReason {
					t.Fatal("read lost unsupported status")
				}
			}
			return nil
		}, func(file *os.File, info os.FileInfo) (os.FileInfo, error) {
			return snapshotOpenedIdentityWithQuery(file, info, func(windows.Handle, *fileIDInfo) error { return unsupported })
		})
		if err != nil {
			t.Fatal("unsupported identity blocked ordinary read", err)
		}
		data, err := io.ReadAll(opened)
		opened.Close()
		if err != nil || string(data) != "ordinary content" {
			t.Fatalf("ordinary read: %q %v", data, err)
		}
	}
	if _, err := snapshotOpenedIdentityWithQuery(file, info, func(windows.Handle, *fileIDInfo) error { return windows.ERROR_ACCESS_DENIED }); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatal("unexpected query error was hidden", err)
	}
}
