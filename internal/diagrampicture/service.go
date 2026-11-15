package diagrampicture

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"studio/internal/helpercompat"
	"sync"
	"time"
	"unicode/utf8"
)

// ReadSnapshot reads through the workspace's file authority from one opened
// handle. It must enforce confinement and the supplied byte bound before return.
type ReadSnapshot func(context.Context, string, int) ([]byte, error)

// Drawing contains only the verified, retained bytes. A renderer must never
// reopen paths and must dispose its page before returning, including on errors.
type Drawing struct {
	Request Request
	Kind    string
	Source  []byte
	Images  map[string][]byte
}
type Renderer func(context.Context, Drawing) (Picture, error)
type active struct {
	cancel context.CancelFunc
	done   chan struct{}
}
type lease struct {
	workspace, program string
	read               ReadSnapshot
	expires            time.Time
	timer              *time.Timer
	active             map[string]*active
	terminal           map[string]time.Time
}
type Service struct {
	mu               sync.Mutex
	host             string
	render           Renderer
	leases           map[string]*lease
	count            int
	work             map[string]int
	closed, disabled bool
	unresolved       int
}

func NewService(host string, render Renderer) *Service {
	return &Service{host: host, render: render, leases: map[string]*lease{}, work: map[string]int{}}
}

// Issue is host-only. The launcher must first verify the profile's explicit
// permission and workspace registration. It never accepts agent-supplied tokens.
func (s *Service) Issue(workspace, program string, read ReadSnapshot) (string, error) {
	if workspace == "" || program == "" || read == nil {
		return "", errors.New("missing drawing lease scope")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("drawing service is closed")
	}
	token := rand.Text()
	l := &lease{workspace: workspace, program: program, read: read, expires: time.Now().Add(15 * time.Minute), active: map[string]*active{}, terminal: map[string]time.Time{}}
	s.leases[Hash([]byte(token))] = l
	l.timer = time.AfterFunc(15*time.Minute, func() { s.revoke(func(candidate *lease) bool { return candidate == l }) })
	return token, nil
}
func (s *Service) revoke(match func(*lease) bool) {
	var waits []chan struct{}
	s.mu.Lock()
	for token, l := range s.leases {
		if match(l) {
			delete(s.leases, token)
			l.timer.Stop()
			for _, a := range l.active {
				a.cancel()
				waits = append(waits, a.done)
			}
		}
	}
	s.mu.Unlock()
	for _, done := range waits {
		<-done
	}
}
func (s *Service) RevokeWorkspace(workspace string) {
	s.revoke(func(l *lease) bool { return l.workspace == workspace })
}
func (s *Service) Revoke(token string) {
	s.revoke(func(l *lease) bool { return s.leases[Hash([]byte(token))] == l })
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.revoke(func(*lease) bool { return true })
}
func writeRefusal(w http.ResponseWriter, id string, err error) {
	var r *Refusal
	if !errors.As(err, &r) {
		r = refused(422, "drawing_problem", err.Error())
		r.Problems = []string{err.Error()}
	}
	copy := *r
	copy.RequestID = id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(copy.HTTPStatus)
	_ = json.NewEncoder(w).Encode(copy)
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Host != s.host || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" || r.URL.RawQuery != "" {
		writeRefusal(w, "", refused(403, "forbidden", "This channel accepts only local tools requests."))
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	l := s.leases[Hash([]byte(token))]
	valid := !s.closed && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && l != nil && time.Now().Before(l.expires)
	s.mu.Unlock()
	if !valid {
		writeRefusal(w, "", refused(401, "lease_refused", "The drawing lease is absent or expired."))
		return
	}
	if r.Header.Get("X-Helper-Channel") != "diagram-picture" {
		writeRefusal(w, "", refused(400, "invalid_channel", "The helper channel is invalid."))
		return
	}
	w.Header().Set("X-Helper-Protocols", policy.Offer())
	if err := helpercompat.ValidateOffer(r.Header.Get("X-Helper-Protocols")); err != nil {
		writeRefusal(w, "", refused(400, "invalid_offer", err.Error()))
		return
	}
	n, err := policy.SelectOffer(r.Header.Get("X-Helper-Protocols"), true)
	if err != nil {
		writeRefusal(w, "", refused(412, "incompatible", err.Error()))
		return
	}
	w.Header().Set("X-Helper-Protocol", strconv.Itoa(n))
	if r.URL.Path == Route && r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if strings.HasPrefix(r.URL.Path, Route+"/") && r.Method == http.MethodDelete {
		id := strings.TrimPrefix(r.URL.Path, Route+"/")
		if !uuid.MatchString(id) {
			writeRefusal(w, "", refused(400, "invalid_request", "Invalid request identity."))
			return
		}
		s.mu.Lock()
		a := l.active[id]
		if a != nil {
			a.cancel()
		}
		s.mu.Unlock()
		if a != nil {
			<-a.done
		}
		w.WriteHeader(204)
		return
	}
	if r.URL.Path != Route || r.Method != http.MethodPost {
		writeRefusal(w, "", refused(405, "method_refused", "This drawing method is unavailable."))
		return
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, MaxRequest+1))
	if err != nil {
		writeRefusal(w, "", refused(400, "invalid_request", "The request could not be read."))
		return
	}
	if len(b) > MaxRequest {
		writeRefusal(w, "", refused(413, "request_limit", "The request exceeds 64 KiB."))
		return
	}
	var request Request
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		writeRefusal(w, "", refused(400, "invalid_request", "The request is not valid JSON."))
		return
	}
	if err = validateRequest(request); err != nil {
		writeRefusal(w, request.RequestID, err)
		return
	}
	if request.WorkspaceID != l.workspace {
		writeRefusal(w, request.RequestID, refused(403, "workspace_refused", "The lease belongs to another workspace."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	a, err := s.admit(l, request.RequestID, cancel)
	if err != nil {
		writeRefusal(w, request.RequestID, err)
		return
	}
	defer func() {
		s.mu.Lock()
		delete(l.active, request.RequestID)
		l.terminal[request.RequestID] = time.Now()
		s.count--
		s.work[l.workspace]--
		close(a.done)
		s.mu.Unlock()
	}()
	drawing, err := snapshot(ctx, l, request)
	var picture Picture
	if err == nil {
		picture, err = s.render(ctx, drawing)
	}
	var pending *startupPending
	if errors.As(err, &pending) {
		s.mu.Lock()
		s.unresolved++
		s.mu.Unlock()
		go func() {
			failure := <-pending.settled
			s.mu.Lock()
			s.unresolved--
			if failure != nil {
				s.disabled = true
			}
			s.mu.Unlock()
		}()
		err = refused(503, "renderer_unavailable", startupMessage)
	}
	var disposal *Refusal
	fatal := errors.As(err, &disposal) && disposal.Code == "renderer_unavailable"
	if fatal && pending == nil {
		s.mu.Lock()
		s.disabled = true
		s.mu.Unlock()
	}
	if ctx.Err() != nil && !fatal {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = refused(504, "deadline", "The drawing exceeded its time limit.")
		} else {
			err = refused(422, "cancelled", "The drawing was cancelled.")
		}
	}
	if err == nil {
		picture.RequestID = request.RequestID
		picture.WorkspaceID = request.WorkspaceID
		picture.Source = request.Source
		picture.Pictures = request.Pictures
		picture.MIME = "image/png"
		picture.SHA256 = Hash(picture.Data)
		if picture.Problems == nil {
			picture.Problems = []string{}
		}
		err = validatePicture(request, picture)
	}
	if err != nil {
		writeRefusal(w, request.RequestID, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(picture)
}
func (s *Service) admit(l *lease, id string, cancel context.CancelFunc) (*active, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.closed || !now.Before(l.expires) {
		return nil, refused(403, "lease_refused", "The lease expired.")
	}
	found := false
	for _, candidate := range s.leases {
		if candidate == l {
			found = true
			break
		}
	}
	if !found {
		return nil, refused(403, "lease_refused", "The lease was revoked.")
	}
	for key, at := range l.terminal {
		if now.Sub(at) > 2*time.Minute {
			delete(l.terminal, key)
		}
	}
	if l.active[id] != nil || !l.terminal[id].IsZero() {
		return nil, refused(409, "request_replay", "That request identity was already used.")
	}
	if len(l.terminal) >= 256 {
		return nil, refused(403, "lease_exhausted", "Renew the drawing lease before another request.")
	}
	if s.disabled || s.render == nil {
		return nil, refused(503, "renderer_unavailable", "Native drawing is unavailable.")
	}
	if s.unresolved > 0 {
		return nil, refused(503, "renderer_unavailable", startupMessage)
	}
	if s.count >= 2 || s.work[l.workspace] >= 1 {
		return nil, refused(429, "capacity", "The drawing capacity is in use.")
	}
	a := &active{cancel: cancel, done: make(chan struct{})}
	l.active[id] = a
	s.count++
	s.work[l.workspace]++
	return a, nil
}
func snapshot(ctx context.Context, l *lease, r Request) (Drawing, error) {
	d := Drawing{Request: r, Images: map[string][]byte{}}
	switch strings.ToLower(path.Ext(r.Source.Path)) {
	case ".canvas":
		d.Kind = "canvas"
	case ".mmd", ".mermaid":
		d.Kind = "mermaid"
	default:
		return d, refused(400, "invalid_source", "The diagram type is unsupported.")
	}
	read := func(s Snapshot, limit int) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		b, err := l.read(ctx, s.Path, limit)
		if err != nil {
			var refusal *Refusal
			if errors.As(err, &refusal) {
				return nil, err
			}
			return nil, refused(403, "snapshot_refused", "The workspace file authority refused a snapshot read.")
		}
		if len(b) > limit {
			return nil, refused(413, "snapshot_limit", "A snapshot exceeds its byte limit.")
		}
		if Hash(b) != s.SHA256 {
			return nil, refused(409, "snapshot_changed", "A file no longer matches the requested snapshot.")
		}
		return b, nil
	}
	var err error
	d.Source, err = read(r.Source, maxSource)
	if err != nil {
		return d, err
	}
	if !utf8.Valid(d.Source) {
		return d, refused(400, "invalid_source", "The diagram is not UTF-8.")
	}
	total := 0
	for _, image := range r.Pictures {
		switch strings.ToLower(path.Ext(image.Path)) {
		case ".png", ".jpg", ".jpeg", ".webp":
		default:
			return d, refused(400, "invalid_dependency", "An image type is unsupported.")
		}
		b, err := read(image, maxImage)
		if err != nil {
			return d, err
		}
		total += len(b)
		if total > 8<<20 {
			return d, refused(413, "snapshot_limit", "Image dependencies exceed 8 MiB.")
		}
		d.Images[image.Path] = b
	}
	for name, data := range d.Images {
		cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
		ext := strings.ToLower(path.Ext(name))
		want := "png"
		if ext == ".jpg" || ext == ".jpeg" {
			want = "jpeg"
		} else if ext == ".webp" {
			want = "webp"
		}
		if err != nil || kind != want || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16777216 {
			return d, refused(422, "invalid_dependency", "An image is invalid or exceeds the dimension limit.")
		}
	}
	if d.Kind == "canvas" {
		var doc struct {
			Nodes []struct {
				Type string `json:"type"`
				File string `json:"file"`
				Yavc struct {
					Picture string `json:"picture"`
				} `json:"yavc"`
			} `json:"nodes"`
		}
		if json.Unmarshal(d.Source, &doc) != nil {
			return d, refused(422, "invalid_source", "The Canvas source is not valid JSON.")
		}
		for _, node := range doc.Nodes {
			if node.Type != "file" {
				continue
			}
			ext := strings.ToLower(path.Ext(node.File))
			if ext == ".svg" || ext == ".gif" {
				return d, refused(422, "invalid_dependency", "Only PNG, JPEG and WebP images are supported.")
			}
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
				continue
			}
			if node.Yavc.Picture != "" {
				if !strings.HasPrefix(node.Yavc.Picture, "data:image/png;base64,") && !strings.HasPrefix(node.Yavc.Picture, "data:image/jpeg;base64,") && !strings.HasPrefix(node.Yavc.Picture, "data:image/webp;base64,") {
					return d, refused(422, "invalid_dependency", "An embedded image type is unsupported.")
				}
				continue
			}
			joined := path.Join(path.Dir(r.Source.Path), node.File)
			if strings.HasPrefix(node.File, "/") || strings.ContainsAny(node.File, `\:`) || !validPath(joined) || d.Images[joined] == nil {
				return d, refused(422, "undeclared_dependency", "A Canvas image was not declared in the snapshot.")
			}
		}
	}
	return d, nil
}
