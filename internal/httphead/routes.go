package httphead

import (
	"net/http"
	"net/url"
	"strings"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/verb"
)

// The media types the head reads and writes.
const (
	// typeJSON is the plain JSON type, accepted for the creations and served
	// with the canonical bytes to a client that asks for it by name.
	typeJSON = "application/json"
	// typeForm is what an HTML form posts. It is accepted on POST alone.
	typeForm = "application/x-www-form-urlencoded"
	// typeMove, typeBlock and typeUnblock select the act a PATCH on a card
	// performs.
	typeMove    = "application/vnd.dinah.move+json"
	typeBlock   = "application/vnd.dinah.block+json"
	typeUnblock = "application/vnd.dinah.unblock+json"
	// typePull is the body of a POST to the claims collection.
	typePull = "application/vnd.dinah.pull+json"
	// typeHTML is offered by the window route alone, which has no renderer
	// yet.
	typeHTML = "text/html"
)

// typeVendor is the media type every body the library produces is served
// in, carrying the contract version as its profile parameter.
var typeVendor = `application/vnd.dinah+json; profile="` + profileVersion() + `"`

// jsonOffers is what every route but the window offers, in preference order.
// dinah-338 gives a route an HTML representation by adding text/html and a
// renderer to that route's own list.
var jsonOffers = []string{typeVendor, typeJSON}

// pathKind is the kind of thing a path's leading segment binds its first
// variable to.
type pathKind int

const (
	// pathNone is a route whose path binds no reference.
	pathNone pathKind = iota
	// pathWorkbench is /workbench.
	pathWorkbench
	// pathRoster is one of the roster words the library lists.
	pathRoster
	// pathCard is /cards/{card} and everything below it.
	pathCard
	// pathColumn is /columns/{column} and everything below it.
	pathColumn
	// pathWorkstream is /workstreams/{slug}.
	pathWorkstream
)

// rosterPaths are the collections the head serves at the root of its path
// space, each named by the roster word the library lists it under.
var rosterPaths = []string{verb.RosterCards, verb.RosterColumns, verb.RosterWorkstreams, verb.RosterRoutes, verb.RosterAttachments}

// refForPath maps a request path to the reference the library resolves and
// the kind its leading segment binds. It answers false for a path that names
// no reference. It and pathForRef are the one place the head composes either.
func refForPath(p string) (pathKind, string, bool) {
	segments := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return pathNone, "", false
	}
	for i, segment := range segments {
		unescaped, err := url.PathUnescape(segment)
		if err != nil {
			return pathNone, "", false
		}
		segments[i] = unescaped
	}
	head, rest := segments[0], segments[1:]
	switch {
	case head == "workbench" && len(rest) == 0:
		return pathWorkbench, bench.WorkbenchRef, true
	case head == "workstreams" && len(rest) == 1 && rest[0] != "":
		return pathWorkstream, workstreamPrefix + rest[0], true
	case head == "cards" && len(rest) > 0 && rest[0] != "":
		return pathCard, strings.Join(rest, "/"), true
	case head == "columns" && len(rest) > 0 && rest[0] != "":
		return pathColumn, strings.Join(rest, "/"), true
	case len(rest) == 0:
		for _, word := range rosterPaths {
			if head == word {
				return pathRoster, word, true
			}
		}
	}
	return pathNone, "", false
}

// pathForRef composes the path a reference of the given kind is served at.
// It is refForPath's inverse, and every Location header the head writes is
// composed here from the canonical reference the response carried.
func pathForRef(kind pathKind, ref string) string {
	switch kind {
	case pathWorkbench:
		return "/workbench"
	case pathRoster:
		return "/" + escapeSegments(ref)
	case pathCard:
		return "/cards/" + escapeSegments(ref)
	case pathColumn:
		return "/columns/" + escapeSegments(ref)
	case pathWorkstream:
		return "/workstreams/" + escapeSegments(strings.TrimPrefix(ref, workstreamPrefix))
	}
	return "/"
}

// escapeSegments escapes each slash-separated segment of a reference for a
// path, leaving the slashes between them.
func escapeSegments(ref string) string {
	segments := strings.Split(ref, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

// workstreamPrefix is the prefix the reference grammar gives a workstream.
const workstreamPrefix = bench.WorkstreamRefPrefix

// act is one command a method of a route performs.
type act struct {
	// command is the library command the act runs.
	command string
	// selectedBy is the media type that selects this act among several a
	// PATCH performs, and empty for a method performing one act.
	selectedBy string
	// created is the success status 201 in place of 200.
	created bool
	// href is the URL template GET /affordances publishes for the act,
	// written in the document's own placeholders.
	href string
}

// method is one method a route takes, as received.
type method struct {
	// name is the HTTP method.
	name string
	// accepts are the Content-Types the method takes, compared on the media
	// type alone.
	accepts []string
	// empty admits an empty body with no Content-Type.
	empty bool
	// tunnel marks the POST a route takes only as a form carrying _method,
	// which Allow never lists.
	tunnel bool
	// acts are the commands the method performs; a PATCH performs several,
	// chosen by type, and every other method performs one.
	acts []act
}

// route is one row of the route table: a path pattern, the kind its first
// variable binds, the representations it offers, and the methods it takes.
type route struct {
	// pattern is the path-only pattern registered on the ServeMux.
	pattern string
	// binds is the kind the path's first variable names.
	binds pathKind
	// read answers GET and HEAD, and is nil on a route taking neither.
	read func(h *head, x *exchange)
	// reads names the commands read may run, for the roster guard and the
	// affordance document.
	reads []string
	// readHref is the URL template GET /affordances publishes for the
	// reads this route answers.
	readHref string
	// methods are the unsafe methods the route takes.
	methods []method
	// offered are the representations the route offers, in preference
	// order.
	offered []string
}

// command is the command a refusal the head raises on this route names
// before it knows which act the request asks for: the route's first read, or
// its first act where it has no read.
func (r *route) command() string {
	if len(r.reads) > 0 {
		return r.reads[0]
	}
	for _, m := range r.methods {
		for _, a := range m.acts {
			return a.command
		}
	}
	return "serve"
}

// allow lists the methods a route takes, in the order an Allow header
// carries them. A POST that only the form tunnel takes is left out.
func (r *route) allow() []string {
	var names []string
	if r.read != nil {
		names = append(names, http.MethodGet, http.MethodHead)
	}
	for _, m := range r.methods {
		if !m.tunnel {
			names = append(names, m.name)
		}
	}
	return names
}

// takes reports whether a route takes a method as received, which includes
// the POST the form tunnel arrives as.
func (r *route) takes(name string) bool {
	if name == http.MethodGet || name == http.MethodHead {
		return r.read != nil
	}
	return r.method(name) != nil
}

// method returns the route's entry for an unsafe method, or nil.
func (r *route) method(name string) *method {
	for i := range r.methods {
		if r.methods[i].name == name {
			return &r.methods[i]
		}
	}
	return nil
}

// acceptHeader is the header a route answers with to name the types a method
// takes: Accept-Patch for PATCH and Accept-Post for POST.
func acceptHeader(name string) string {
	switch name {
	case http.MethodPatch:
		return "Accept-Patch"
	case http.MethodPost:
		return "Accept-Post"
	}
	return ""
}

// The types each unsafe method takes, per section 6.2 of the specification.
var (
	createTypes  = []string{typeJSON, typeForm}
	patchTypes   = []string{typeMove, typeBlock, typeUnblock}
	pullTypes    = []string{typePull, typeForm}
	commentTypes = []string{typeJSON, typeForm}
)

// routes is the whole roster. A path no pattern here matches reaches the
// catch-all and answers 404. It is filled at start rather than in its
// declaration because the affordance document is generated from it, and a
// declaration that reached its own reader would be an initialisation cycle.
var routes []*route

func init() {
	routes = []*route{
		{pattern: "/{$}", read: runRead("status"), reads: []string{"status"}, readHref: "/", offered: jsonOffers},
		{pattern: "/workbench", binds: pathWorkbench, read: runShowAt, reads: []string{"show"}, offered: jsonOffers},
		{
			pattern: "/cards", binds: pathRoster, read: readCards, reads: []string{"list", "query"}, offered: jsonOffers,
			methods: []method{
				{name: http.MethodPost, accepts: createTypes, acts: []act{{command: "add", created: true}}},
			},
		},
		{
			pattern: "/cards/{card}", binds: pathCard, read: runShowAt, reads: []string{"show"}, offered: jsonOffers,
			methods: []method{
				{name: http.MethodPatch, accepts: patchTypes, acts: []act{
					{command: verb.Move, selectedBy: typeMove, href: "/cards/{card}"},
					{command: verb.Block, selectedBy: typeBlock, href: "/cards/{card}"},
					{command: verb.Unblock, selectedBy: typeUnblock, href: "/cards/{card}"},
				}},
				{name: http.MethodPost, accepts: []string{typeForm}, tunnel: true},
			},
		},
		{pattern: "/cards/{card}/instructions", binds: pathCard, read: readInstructions, reads: []string{"instructions"}, offered: jsonOffers},
		{
			pattern: "/cards/{card}/claim", binds: pathCard, read: readClaim, offered: jsonOffers,
			methods: []method{
				{name: http.MethodPost, accepts: createTypes, empty: true, acts: []act{{command: verb.Claim, created: true, href: "/cards/{card}/claim"}}},
				{name: http.MethodDelete, empty: true, acts: []act{{command: verb.Release, href: "/cards/{card}/claim"}}},
			},
		},
		{pattern: "/cards/{card}/window", binds: pathCard, read: readWindow, offered: []string{typeHTML}},
		{
			pattern: "/cards/{card}/{rest...}", binds: pathCard, read: readBelow, reads: []string{"list", "show"}, readHref: "{path}", offered: jsonOffers,
			methods: []method{
				{name: http.MethodPost, accepts: commentTypes, empty: true, acts: []act{{command: "comment", created: true, href: "{path}/comments"}}},
			},
		},
		{pattern: "/columns", binds: pathRoster, read: runListAt, reads: []string{"list"}, offered: jsonOffers},
		{pattern: "/columns/{column}", binds: pathColumn, read: runShowAt, reads: []string{"show"}, offered: jsonOffers},
		{pattern: "/columns/{column}/instructions", binds: pathColumn, read: readInstructions, reads: []string{"instructions"}, offered: jsonOffers},
		{
			pattern: "/columns/{column}/{rest...}", binds: pathColumn, read: readBelow, reads: []string{"list", "show"}, offered: jsonOffers,
			methods: []method{
				{name: http.MethodPost, accepts: commentTypes, empty: true, acts: []act{{command: "comment", created: true, href: "{path}/comments"}}},
			},
		},
		{pattern: "/workstreams", binds: pathRoster, read: runListAt, reads: []string{"list"}, offered: jsonOffers},
		{pattern: "/workstreams/{slug}", binds: pathWorkstream, read: runShowAt, reads: []string{"show"}, offered: jsonOffers},
		{pattern: "/routes", binds: pathRoster, read: runListAt, reads: []string{"list"}, offered: jsonOffers},
		{pattern: "/attachments", binds: pathRoster, read: runListAt, reads: []string{"list"}, offered: jsonOffers},
		{pattern: "/next", read: runRead("next"), reads: []string{"next"}, readHref: "/next", offered: jsonOffers},
		{pattern: "/changes", read: runRead("changes"), reads: []string{"changes"}, offered: jsonOffers},
		{pattern: "/tree", read: runRead("tree"), reads: []string{"tree"}, offered: jsonOffers},
		{pattern: "/search", read: runRead("search"), reads: []string{"search"}, offered: jsonOffers},
		{pattern: "/views", read: readViews, reads: []string{"view"}, offered: jsonOffers},
		{pattern: "/views/{view}", read: readView, reads: []string{"view"}, offered: jsonOffers},
		{pattern: "/whoami", read: runRead("whoami"), reads: []string{"whoami"}, offered: jsonOffers},
		{pattern: "/version", read: runRead("version"), reads: []string{"version"}, offered: jsonOffers},
		{pattern: "/affordances", read: readAffordances, offered: jsonOffers},
		{
			pattern: "/claims", offered: jsonOffers,
			methods: []method{
				{name: http.MethodPost, accepts: pullTypes, acts: []act{{command: verb.Pull, created: true, href: "/claims"}}},
			},
		},
	}
}

// routedCommands lists every library command some route runs, each once.
func routedCommands() []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, r := range routes {
		for _, name := range r.reads {
			add(name)
		}
		for _, m := range r.methods {
			for _, a := range m.acts {
				add(a.command)
			}
		}
	}
	return names
}

// exemption is one command the head does not route: the ground it is held
// out on and the reason a reader wants.
type exemption struct {
	ground string
	reason string
}

// The grounds a route exemption may stand on. The first four are the MCP
// head's own; the fifth holds a command for a later card, whose reference
// the reason names.
const (
	GroundShellOrFilesystem   = "shell-or-filesystem"
	GroundMachineNotWorkbench = "machine-not-workbench"
	GroundTheHeadItself       = "the-head-itself"
	GroundProtocolServesIt    = "protocol-serves-it"
	GroundLaterCard           = "later-card"
)

// routeGrounds is the closed set, in the order the constants declare it.
var routeGrounds = []string{
	GroundShellOrFilesystem, GroundMachineNotWorkbench,
	GroundTheHeadItself, GroundProtocolServesIt, GroundLaterCard,
}

// laterCard is the reason every command held for the next cut carries.
const laterCard = "dinah-612 routes the workbench's remaining acts and reads; this cut routes the reads a page needs and the acts a card's affordances name"

// routeExemptions names every library command the head deliberately does not
// route, with its ground and its reason. The roster guard holds the routed
// commands and these together to verb.Commands(), so a command nobody argued
// for cannot reach a green build.
var routeExemptions = map[string]exemption{
	"path":              {GroundShellOrFilesystem, "resolves a filesystem path for a shell to consume, so it means nothing over HTTP"},
	"edit":              {GroundShellOrFilesystem, "opens a file in the reader's own editor, which needs a terminal this head does not have"},
	"init":              {GroundShellOrFilesystem, "creates a workbench in a directory, which is a filesystem act rather than a workbench act"},
	"extract":           {GroundShellOrFilesystem, "copies a workbench definition out to a directory, which is the same filesystem act"},
	"reshape":           {GroundShellOrFilesystem, "reads its new column layout from a definition file or another workbench's directory, which is the same filesystem act"},
	"completion":        {GroundShellOrFilesystem, "prints a script for the caller's own interactive shell to load, which means nothing over HTTP"},
	"config":            {GroundMachineNotWorkbench, "writes the user's own machine settings, which travel with the person rather than the workbench"},
	"setup":             {GroundMachineNotWorkbench, "writes a harness's configuration files on the caller's machine, which belong to the machine rather than to any workbench"},
	"mcp":               {GroundTheHeadItself, "starts the MCP head, so a route for it would be one server offering to start another"},
	"lsp":               {GroundTheHeadItself, "starts the editor head, so a route for it would be one server offering to start another"},
	"serve":             {GroundTheHeadItself, "starts this head, so a route for it would be the server offering to start itself"},
	"help":              {GroundProtocolServesIt, "GET /affordances names what every route takes, which is what help prints at a terminal"},
	"raise":             {GroundLaterCard, laterCard},
	"join":              {GroundLaterCard, laterCard},
	"leave":             {GroundLaterCard, laterCard},
	"attach":            {GroundLaterCard, laterCard},
	"file":              {GroundLaterCard, laterCard},
	"cite":              {GroundLaterCard, laterCard},
	"resolve":           {GroundLaterCard, laterCard},
	"verify":            {GroundLaterCard, laterCard},
	"fail":              {GroundLaterCard, laterCard},
	"waive":             {GroundLaterCard, laterCard},
	"withdraw":          {GroundLaterCard, laterCard},
	"grant":             {GroundLaterCard, laterCard},
	"revoke":            {GroundLaterCard, laterCard},
	"reopen":            {GroundLaterCard, laterCard},
	"settle":            {GroundLaterCard, laterCard},
	"link":              {GroundLaterCard, laterCard},
	"unlink":            {GroundLaterCard, laterCard},
	"archive":           {GroundLaterCard, laterCard},
	"restore":           {GroundLaterCard, laterCard},
	"delete":            {GroundLaterCard, laterCard},
	"accept-divergence": {GroundLaterCard, laterCard},
	"rename":            {GroundLaterCard, laterCard},
	"guide":             {GroundLaterCard, laterCard},
	"export":            {GroundLaterCard, laterCard},
	"workbench":         {GroundLaterCard, laterCard},
	"column":            {GroundLaterCard, laterCard},
	"workstream":        {GroundLaterCard, laterCard},
	"get":               {GroundLaterCard, laterCard},
	"set":               {GroundLaterCard, laterCard},
	"check":             {GroundLaterCard, laterCard},
	"prime":             {GroundLaterCard, laterCard},
}

// rootScoped is the reason root and max-depth are held back wherever a
// command declares them.
const rootScoped = "the head serves one workbench, and a root-scoped read answers about every workbench under a directory"

// paramExemptions names, per command, the declared parameters the head does
// not publish, each with the reason it is held back.
var paramExemptions = map[string]map[string]string{
	"status": {"root": rootScoped, "max-depth": rootScoped},
	"list":   {"root": rootScoped, "max-depth": rootScoped},
	"next":   {"root": rootScoped, "max-depth": rootScoped},
	"tree":   {"root": rootScoped, "max-depth": rootScoped},
	"search": {"root": rootScoped, "max-depth": rootScoped},
	"changes": {
		"root":      rootScoped,
		"max-depth": rootScoped,
		"wait":      "waitForChange sleeps until a change or its deadline and takes no cancellation, so a client that disconnected would leave a goroutine polling the workbench; the pages poll with the cursor instead",
		"timeout":   "bounds a wait this head does not offer",
	},
	"view": {
		"all":   "lifts the per-column cap of a drawing this head never makes",
		"plain": "chooses the marks of a drawing this head never makes",
		"watch": "redraws the view until interrupted, which holds the call open",
	},
}

// published lists the members a command takes over HTTP on a route binding
// the given parameters from its path: every declared parameter carrying a
// Field, minus the bound ones and the held-back ones.
func published(command string, bound ...string) map[string]verb.Param {
	members := map[string]verb.Param{}
	for _, param := range verb.Params(command) {
		if param.Field == "" {
			continue
		}
		if _, held := paramExemptions[command][param.Name]; held {
			continue
		}
		skip := false
		for _, name := range bound {
			if name == param.Name {
				skip = true
			}
		}
		if !skip {
			members[param.Name] = param
		}
	}
	return members
}

// surfaceName is the name the machine surfaces publish a command's
// affordance under.
func surfaceName(command string) string {
	return answer.Affordances([]string{command})[0]
}
