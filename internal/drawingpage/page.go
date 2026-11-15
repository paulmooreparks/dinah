// Package drawingpage owns a disposable native diagram page.
package drawingpage

type Request struct {
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Scale int    `json:"scale"`
}
type Picture struct {
	PNG           []byte
	Width, Height int
}
type Options struct {
	// DataFolder must be a fresh, private folder supplied by the host.
	DataFolder string
	// Event records native lifecycle transitions, never source text.
	Event func(string)
}
