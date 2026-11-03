package agents

import "os"

// FileIdentity is a metadata snapshot, independent of a pathname or a later
// network lookup. It is retained only in memory, never as a persisted grant.
type FileIdentity struct{ Volume, High, Low uint64 }
type identityFileInfo struct {
	os.FileInfo
	identity FileIdentity
}

const UnsupportedFileIdentityReason = "The file system does not support FileIdInfo; this deny rule cannot be enforced against other paths to the same folder, such as a mapped drive letter"

type unavailableIdentityFileInfo struct {
	os.FileInfo
	reason string
}

func (info *unavailableIdentityFileInfo) IdentityUnavailableReason() string { return info.reason }

func SnapshotIdentityReason(info os.FileInfo) string {
	if unavailable, ok := info.(interface{ IdentityUnavailableReason() string }); ok {
		return unavailable.IdentityUnavailableReason()
	}
	return ""
}

func SnapshotIdentity(info os.FileInfo) (FileIdentity, bool) {
	captured, ok := info.(*identityFileInfo)
	if !ok {
		return FileIdentity{}, false
	}
	return captured.identity, true
}
func originalFileInfo(info os.FileInfo) os.FileInfo {
	if unavailable, ok := info.(*unavailableIdentityFileInfo); ok {
		return unavailable.FileInfo
	}
	if captured, ok := info.(*identityFileInfo); ok {
		return captured.FileInfo
	}
	return info
}
func StatFileIdentity(path string) (os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return snapshotOpenedIdentity(file, info)
}
func rootIdentitySnapshot(root *os.Root, capture func(*os.File, os.FileInfo) (os.FileInfo, error)) (os.FileInfo, error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return capture(file, info)
}
