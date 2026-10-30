package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strings"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

var (
	ErrAssetNotFound  = errors.New("extension asset not found")
	ErrAssetRegistry  = errors.New("extension registry unavailable")
	ErrAssetIntegrity = errors.New("extension integrity check failed")
	ErrAssetModified  = errors.New("extension asset was modified")
)

// Asset is verified content with the permission the shell needs for its CSP.
type Asset struct {
	Data      []byte
	MediaType string
	Network   bool
}

// ReadAsset checks installation, path, type and integrity before returning bytes.
func (a *Service) ReadAsset(id, version, name string) (Asset, error) {
	if !extension.SafePath(name) {
		return Asset{}, ErrAssetNotFound
	}
	manager := a.Manager
	list, err := manager.Store.List()
	if err != nil {
		return Asset{}, ErrAssetRegistry
	}
	var found *extension.Installation
	for _, v := range list {
		if v.Enabled && v.Manifest.ID == id && v.Manifest.Version == version {
			found = &v
			break
		}
	}
	if found == nil {
		return Asset{}, ErrAssetNotFound
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".html", ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".woff2", ".woff":
	default:
		return Asset{}, ErrAssetNotFound
	}
	expected, ok := found.Manifest.Files[name]
	if !ok {
		return Asset{}, ErrAssetNotFound
	}
	if ext == ".html" {
		if _, err = manager.Installed(found.Manifest.ID); err != nil {
			return Asset{}, ErrAssetIntegrity
		}
	}
	path := filepath.Join(manager.Store.Path(found.Manifest), filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		return Asset{}, ErrAssetNotFound
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return Asset{}, ErrAssetModified
	}
	return Asset{Data: data, MediaType: mime.TypeByExtension(ext), Network: extension.Has(found.Manifest.Permissions, "network")}, nil
}
