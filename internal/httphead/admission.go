package httphead

import (
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// admittedHost reports whether a Host header names the bound literal or
// localhost, with the bound port. A Host carrying no port means port 80.
// This is what refuses a DNS-rebinding page, which reaches the loopback
// address under a hostile name.
func (h *head) admittedHost(hostport string) bool {
	host, port, ok := splitHost(hostport)
	if !ok || port != strconv.Itoa(h.cfg.Port) {
		return false
	}
	return strings.EqualFold(host, "localhost") || host == h.cfg.Host
}

// admittedOrigin reports whether an Origin header names this server as a
// browser serialises it: http://, an admitted host, and its port.
func (h *head) admittedOrigin(origin string) bool {
	hostport, found := strings.CutPrefix(origin, "http://")
	if !found {
		return false
	}
	return h.admittedHost(hostport)
}

// splitHost splits a host and an optional port as a Host header or an
// origin writes them, with an IPv6 literal bracketed. A missing port is 80.
func splitHost(hostport string) (host, port string, ok bool) {
	if hostport == "" {
		return "", "", false
	}
	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		return hostport[1 : len(hostport)-1], "80", true
	}
	if !strings.Contains(hostport, ":") {
		return hostport, "80", true
	}
	host, port, err := net.SplitHostPort(hostport)
	if err != nil || port == "" {
		return "", "", false
	}
	return host, port, true
}

// originOf reads a request's Origin header, reporting whether one was sent
// at all. Several Origin lines are read as one value, which no admitted
// origin equals.
func originOf(r *http.Request) (string, bool) {
	values := r.Header.Values("Origin")
	if len(values) == 0 {
		return "", false
	}
	return strings.Join(values, ", "), true
}

// provesSameOrigin reports whether a request a page on another site could
// send without asking first proves it came from this server's own pages: an
// Origin naming this server, which step 3 has already admitted when one is
// present, or, with no Origin, Sec-Fetch-Site: same-origin.
func (h *head) provesSameOrigin(r *http.Request) bool {
	if _, present := originOf(r); present {
		return true
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

// parseMediaType reads the media type of a Content-Type value, lowercased
// and without its parameters. It reports false for a value
// mime.ParseMediaType rejects, the empty value included.
func parseMediaType(raw string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "", false
	}
	return strings.ToLower(mediaType), true
}

// listed reports whether a media type is one of a list.
func listed(types []string, mediaType string) bool {
	for _, t := range types {
		if t == mediaType {
			return true
		}
	}
	return false
}
