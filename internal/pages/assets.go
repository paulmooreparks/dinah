package pages

import "embed"

// assetFiles are the files the pages load: PUDL's runtime files, copied byte
// for byte out of a tagged release, and Dinah's own stylesheet, script and
// mark. No font file is among them.
//
//go:embed assets
var assetFiles embed.FS

// assetTypes is the fixed table the assets route answers from: every path it
// serves, below /assets/, and the type each is served in. A path it does not
// list, PUDL's licence and provenance among them, is not served.
var assetTypes = map[string]string{
	"pudl/pudl.css":         "text/css; charset=utf-8",
	"pudl/pudl-windows.css": "text/css; charset=utf-8",
	"dinah.css":             "text/css; charset=utf-8",
	"pudl/pudl-theme.js":    "text/javascript; charset=utf-8",
	"pudl/pudl-windows.js":  "text/javascript; charset=utf-8",
	"dinah.js":              "text/javascript; charset=utf-8",
	"dinah-lantern.svg":     "image/svg+xml",
}

// Asset answers the embedded file a path below /assets/ names and the type
// it is served in, and false for any path the fixed table does not list.
func Asset(path string) ([]byte, string, bool) {
	contentType, listed := assetTypes[path]
	if !listed {
		return nil, "", false
	}
	data, err := assetFiles.ReadFile("assets/" + path)
	if err != nil {
		return nil, "", false
	}
	return data, contentType, true
}
