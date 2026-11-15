// Package diagrampicture implements the workspace-bound diagram helper exchange.
package diagrampicture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/png"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

const Route = "/diagram-pictures"
const MaxRequest = 64 << 10
const MaxResponse = 24 << 20
const maxSource = 2 << 20
const maxImage = 1 << 20

type Snapshot struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Request struct {
	RequestID   string     `json:"requestId"`
	WorkspaceID string     `json:"workspaceId"`
	Source      Snapshot   `json:"source"`
	Pictures    []Snapshot `json:"pictures"`
	Format      string     `json:"format"`
	Scale       int        `json:"scale"`
}
type Picture struct {
	RequestID   string     `json:"requestId"`
	WorkspaceID string     `json:"workspaceId"`
	Source      Snapshot   `json:"source"`
	Pictures    []Snapshot `json:"pictures"`
	MIME        string     `json:"mime"`
	Data        []byte     `json:"data"`
	SHA256      string     `json:"sha256"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	Problems    []string   `json:"problems"`
}
type Refusal struct {
	RequestID  string   `json:"requestId"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Problems   []string `json:"problems"`
	HTTPStatus int      `json:"-"`
}

func (r *Refusal) Error() string { return r.Message }
func refused(status int, code, message string) *Refusal {
	return &Refusal{HTTPStatus: status, Code: code, Message: message, Problems: []string{}}
}

const managedMessage = "A new Canvas or Mermaid picture needs a managed chat in the relevant workspace inside the product window. Publish through that chat. Nothing was uploaded."

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func validPath(p string) bool {
	if p == "." || !fs.ValidPath(p) || strings.ContainsAny(p, `\:`) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}
func validateRequest(r Request) error {
	if !uuid.MatchString(r.RequestID) || r.WorkspaceID == "" || r.Format != "png" || r.Scale < 1 || r.Scale > 3 {
		return refused(400, "invalid_request", "The diagram request is invalid.")
	}
	if len(r.Pictures) > 32 {
		return refused(413, "request_limit", "At most 32 picture dependencies are allowed.")
	}
	seen := map[string]bool{}
	for _, s := range append([]Snapshot{r.Source}, r.Pictures...) {
		if !validPath(s.Path) || !digest.MatchString(s.SHA256) || seen[s.Path] {
			return refused(400, "invalid_snapshot", "The snapshot path or hash is invalid.")
		}
		seen[s.Path] = true
	}
	return nil
}
func validatePicture(r Request, p Picture) error {
	if p.RequestID != r.RequestID || p.WorkspaceID != r.WorkspaceID || p.Source != r.Source || !slices.Equal(p.Pictures, r.Pictures) || p.MIME != "image/png" || len(p.Problems) > 0 || len(p.Data) > 16<<20 || Hash(p.Data) != p.SHA256 {
		return errors.New("the drawing response does not match the requested snapshot")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(p.Data))
	if err != nil {
		return err
	}
	if cfg.Width != p.Width || cfg.Height != p.Height || p.Width < 1 || p.Height < 1 || p.Width > 8192 || p.Height > 8192 || int64(p.Width)*int64(p.Height) > 16777216 {
		return errors.New("the drawing geometry is invalid")
	}
	_, err = png.Decode(bytes.NewReader(p.Data))
	return err
}
