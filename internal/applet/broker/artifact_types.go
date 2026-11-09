package broker

// ArtifactMIME are the media types an action's artifact may have. Text and
// the drawings ValidateSVG accepts can be shown; the documents the content
// applets export (Word, PowerPoint, PDF) and PNG pictures are kept as
// bytes, which Dinah never renders itself, for a person or an agent to
// save.
var ArtifactMIME = map[string]bool{
	"image/svg+xml":    true,
	"text/plain":       true,
	"text/markdown":    true,
	"application/json": true,
	"image/png":        true,
	"application/pdf":  true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
}
