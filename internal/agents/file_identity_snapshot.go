package agents

import "os"

// FileIdentity is a metadata snapshot, independent of a pathname or a later
// network lookup. It is retained only in memory, never as a persisted grant.
type FileIdentity struct{ Volume, High, Low uint64 }
type identityFileInfo struct {
	os.FileInfo
	identity FileIdentity
}

func SnapshotIdentity(info os.FileInfo) (FileIdentity, bool) {
	captured, ok := info.(*identityFileInfo)
	if !ok {
		return FileIdentity{}, false
	}
	return captured.identity, true
}
func originalFileInfo(info os.FileInfo) os.FileInfo {
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
func rootIdentitySnapshot(root *os.Root) (os.FileInfo, error) {
	file, err := root.Open(".")
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
