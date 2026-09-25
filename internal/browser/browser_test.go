package browser

// refused are the values Open must hand to no platform mechanism: anything
// but an http URL naming a host and the root path.
var refused = []string{
	"file:///etc/passwd",
	"http://127.0.0.1:7340/x",
	"javascript:alert(1)",
	"http://127.0.0.1:7340/ x",
	"http://user:pw@127.0.0.1:7340/",
	"http://127.0.0.1:7340/?a=b",
	"http://127.0.0.1:7340/?",
	"http://127.0.0.1:7340/#x",
	"http://127.0.0.1:7340/#",
	"http://127.0.0.1:7340/%2F",
	`http://127.0.0.1:7340/"`,
	"https://127.0.0.1:7340/",
	"http:///",
	"",
}

// accepted are the values Open hands on, each unchanged.
var accepted = []string{
	"http://127.0.0.1:7340/",
	"http://[::1]:7340/",
}
