package bench

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// ViewsKey is the top-level frontmatter key a view is declared under, in the
// workbench's own workbench.md and in the user's config.md alike. It carries
// the layer prefix every name Dinah mints carries, so it can never collide
// with a key the profile declares, and one block shape serves both files so
// that a view copies from one to the other without an edit.
const ViewsKey = contract.ViewsKey

// The sources a view can come from, in the order a name resolves through them.
const (
	ViewSourceUser      = "user"
	ViewSourceWorkbench = "workbench"
	ViewSourceBuiltIn   = "built-in"
)

// The eight defects that make a view malformed, as machine tokens. A view
// carries at most one, the first of these that applies, tried in this order.
const (
	ViewInvalidName         = "invalid-name"
	ViewNotAMapping         = "not-a-mapping"
	ViewMalformedMember     = "malformed-member"
	ViewNoSections          = "no-sections"
	ViewSectionWithoutQuery = "section-without-query"
	ViewUnknownLayout       = "unknown-layout"
	ViewUnknownOrder        = "unknown-order"
	ViewUnknownScope        = "unknown-scope"
)

// ViewDefects is the closed set of defect tokens, in the order a view is
// tried against them.
var ViewDefects = []string{
	ViewInvalidName, ViewNotAMapping, ViewMalformedMember, ViewNoSections,
	ViewSectionWithoutQuery, ViewUnknownLayout, ViewUnknownOrder, ViewUnknownScope,
}

// The layout and order words this build draws. A later build adds a word to
// one of these sets rather than a member to the declaration, so the shape of
// a view does not change when a layout or an order arrives.
const (
	ViewLayoutList    = "list"
	ViewLayoutColumns = "columns"
	ViewOrderArrival  = "arrival"
	ViewOrderColumn   = "column"
	ViewOrderUrgency  = "urgency"
)

// ViewLayouts and ViewOrders are the values this build admits for a view's
// layout and order. A value outside them makes the view malformed.
var (
	ViewLayouts = []string{ViewLayoutList, ViewLayoutColumns}
	ViewOrders  = []string{ViewOrderArrival, ViewOrderColumn, ViewOrderUrgency}
)

// ViewScopeActionable is the one scope this build admits: the cards the
// caller can act on, which no query can express because it depends on the
// caller's tier, every card's route and every claim.
const ViewScopeActionable = "actionable"

// ViewScopes are the values this build admits for a section's scope. A value
// outside them makes the view malformed.
var ViewScopes = []string{ViewScopeActionable}

// The member names a view and a section may carry. Anything else is ignored
// and reported by check as a member this build does not read.
const (
	viewTitle     = "title"
	viewLayout    = "layout"
	viewOrder     = "order"
	viewCollapsed = "collapsed"
	viewSections  = "sections"
	sectionQuery  = "query"
	sectionScope  = "scope"
)

// View is one declared view, as read. Every member is kept as the file
// spells it, and the defaults of the declaration table are applied by the
// accessors, so a listing can report what the author wrote.
type View struct {
	Name         string
	Title        string // "" where absent or blank
	Layout       string // "" where absent
	Order        string // "" where absent
	Collapsed    []string
	HasCollapsed bool // true where the member is present, [] included
	Sections     []ViewSection
	Source       string   // ViewSourceUser, ViewSourceWorkbench, or ViewSourceBuiltIn
	Defect       string   // "" when the view can be drawn; one of ViewDefects otherwise
	Unknown      []string // the member paths check reports, in the order read
}

// ViewSection is one section of a view: a query, a scope, or both, and the
// heading it is drawn under. A section carrying both selects the cards the
// query matches that are also in the scope.
type ViewSection struct {
	Title string // "" where absent or blank
	Query string
	Scope string // "" where absent; one of ViewScopes on a view that can be drawn
}

// EffectiveTitle is the view's title, or its name where it declares none.
func (v View) EffectiveTitle() string {
	if v.Title != "" {
		return v.Title
	}
	return v.Name
}

// EffectiveLayout is the view's layout, or list where it declares none.
func (v View) EffectiveLayout() string {
	if v.Layout != "" {
		return v.Layout
	}
	return ViewLayoutList
}

// EffectiveOrder is the view's order, or arrival where it declares none.
func (v View) EffectiveOrder() string {
	if v.Order != "" {
		return v.Order
	}
	return ViewOrderArrival
}

// Heading is the section's title, or its query exactly as declared where it
// declares none, or its scope word where it declares neither.
func (s ViewSection) Heading() string {
	if s.Title != "" {
		return s.Title
	}
	if s.Query != "" {
		return s.Query
	}
	return s.Scope
}

// ReadViews reads the dinah.views key of whatever frontmatter it is handed,
// stamping every view it reads with source. blockDefect is true when the key
// is present and its value is not a mapping, in which case no view is read.
//
// The block goes through blockValue, the reader every structured frontmatter
// value is read by, and each view is decoded from the JSON value it returns.
// A malformed view is still returned, carrying its defect, because it still
// takes its place in name resolution. A read never rewrites the file, so a
// member this build does not know survives every read untouched.
func ReadViews(fm *Frontmatter, source string) (views []View, blockDefect bool) {
	if fm == nil || !fm.Has(ViewsKey) {
		return nil, false
	}
	members, mapping := firstMembers(blockValue(fm, ViewsKey))
	if !mapping {
		return nil, true
	}
	for _, member := range members {
		views = append(views, readView(member.name, member.value, source))
	}
	return views, false
}

// readView decodes one view and records its first defect.
func readView(name string, raw json.RawMessage, source string) View {
	view := View{Name: name, Source: source}
	var defects []string
	if !HarnessName(name) {
		defects = append(defects, ViewInvalidName)
	}
	members, mapping := firstMembers(raw)
	if !mapping {
		view.Defect = firstDefect(append(defects, ViewNotAMapping))
		return view
	}
	malformed := false
	sectionsSeen := false
	for _, member := range members {
		switch member.name {
		case viewTitle, viewLayout, viewOrder:
			text, ok := viewScalar(member.value)
			if !ok {
				malformed = true
				continue
			}
			switch member.name {
			case viewTitle:
				view.Title = blankAsAbsent(text)
			case viewLayout:
				view.Layout = text
			case viewOrder:
				view.Order = text
			}
		case viewCollapsed:
			view.HasCollapsed = true
			entries, ok := jsonEntries(member.value)
			if !ok {
				malformed = true
				continue
			}
			view.Collapsed = []string{}
			for _, entry := range entries {
				text, ok := viewScalar(entry)
				if !ok {
					malformed = true
					continue
				}
				view.Collapsed = append(view.Collapsed, text)
			}
		case viewSections:
			sectionsSeen = true
			entries, ok := jsonEntries(member.value)
			if !ok {
				malformed = true
				continue
			}
			for i, entry := range entries {
				section, unknown, ok := readViewSection(entry, i+1)
				if !ok {
					malformed = true
				}
				view.Unknown = append(view.Unknown, unknown...)
				view.Sections = append(view.Sections, section)
			}
		default:
			view.Unknown = append(view.Unknown, member.name)
		}
	}
	if malformed {
		defects = append(defects, ViewMalformedMember)
	}
	if !sectionsSeen || len(view.Sections) == 0 {
		defects = append(defects, ViewNoSections)
	}
	for _, section := range view.Sections {
		if strings.TrimSpace(section.Query) == "" && section.Scope == "" {
			defects = append(defects, ViewSectionWithoutQuery)
			break
		}
	}
	if view.Layout != "" && !containsWord(ViewLayouts, view.Layout) {
		defects = append(defects, ViewUnknownLayout)
	}
	if view.Order != "" && !containsWord(ViewOrders, view.Order) {
		defects = append(defects, ViewUnknownOrder)
	}
	for _, section := range view.Sections {
		if section.Scope != "" && !containsWord(ViewScopes, section.Scope) {
			defects = append(defects, ViewUnknownScope)
			break
		}
	}
	view.Defect = firstDefect(defects)
	return view
}

// readViewSection decodes one section, answering the member paths it did not
// recognise and whether its shape was one the declaration admits.
func readViewSection(raw json.RawMessage, position int) (ViewSection, []string, bool) {
	var section ViewSection
	members, mapping := firstMembers(raw)
	if !mapping {
		return section, nil, false
	}
	var unknown []string
	ok := true
	for _, member := range members {
		switch member.name {
		case viewTitle, sectionQuery, sectionScope:
			text, scalar := viewScalar(member.value)
			if !scalar {
				ok = false
				continue
			}
			switch member.name {
			case viewTitle:
				section.Title = blankAsAbsent(text)
			case sectionQuery:
				section.Query = text
			case sectionScope:
				section.Scope = text
			}
		default:
			unknown = append(unknown, viewSections+"/"+strconv.Itoa(position)+"/"+member.name)
		}
	}
	return section, unknown, ok
}

// firstDefect is the first defect of a list in the order ViewDefects fixes, or
// the empty string where the list is empty.
func firstDefect(found []string) string {
	for _, defect := range ViewDefects {
		if containsWord(found, defect) {
			return defect
		}
	}
	return ""
}

// viewScalar reads a scalar member as text. A JSON string is its own text, and
// a number or a boolean is the literal the JSON carries, so `title: 2026`
// reads as the text 2026. A null, an array or an object has no scalar reading.
func viewScalar(raw json.RawMessage) (string, bool) {
	literal := strings.TrimSpace(string(raw))
	if strings.HasPrefix(literal, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", false
		}
		return text, true
	}
	if literal == "true" || literal == "false" || jsonNumber(literal) {
		return literal, true
	}
	return "", false
}

// blankAsAbsent reads a title of whitespace alone as no title at all.
func blankAsAbsent(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}

// containsWord reports whether a list holds a word.
func containsWord(words []string, want string) bool {
	for _, word := range words {
		if word == want {
			return true
		}
	}
	return false
}

// firstMembers decodes a JSON object into its members in the order the text
// carries them, keeping the first occurrence of a duplicated name, which is
// what reading a file's block already does. It reports false for any value
// that is not an object. jsonMembers refuses a duplicate outright because its
// callers write what they read; this one only reads.
func firstMembers(raw json.RawMessage) ([]jsonMember, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Token(json.Delim('{')) {
		return nil, false
	}
	var members []jsonMember
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		name, isName := key.(string)
		if !isName {
			return nil, false
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		members = append(members, jsonMember{name: name, value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false
	}
	return members, true
}

// UserViewsState is what reading the user's views found, which is more than
// LoadConfig reports: that one answers an empty configuration for any read
// error, which would make an unreadable file look like an absent one, and a
// user view that should shadow a workbench view would then vanish and the
// workbench's view be drawn in its place.
type UserViewsState int

const (
	// UserViewsAbsent is a config.md that does not exist, which is an empty
	// user layer.
	UserViewsAbsent UserViewsState = iota
	// UserViewsRead is a config.md that was read, carrying no dinah.views key
	// or one whose value is a mapping.
	UserViewsRead
	// UserViewsUnreadable is a config.md that exists and cannot be read,
	// which includes a directory standing at its path.
	UserViewsUnreadable
	// UserViewsNotAMapping is a config.md that was read and whose dinah.views
	// value is not a mapping.
	UserViewsNotAMapping
)

// UserViewsPath is the file the user's views are read from, which is the file
// LoadConfig reads.
func UserViewsPath(home string) string {
	return filepath.Join(UserBase(home), ConfigName)
}

// LoadUserViews reads the views the user's config.md declares, and reports
// which of the four states the file was in.
func LoadUserViews(home string) ([]View, UserViewsState) {
	path := UserViewsPath(home)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, UserViewsAbsent
	}
	text, err := ReadText(path)
	if err != nil {
		return nil, UserViewsUnreadable
	}
	fm, _ := ParseAnchor(text)
	views, blockDefect := ReadViews(fm, ViewSourceUser)
	if blockDefect {
		return nil, UserViewsNotAMapping
	}
	return views, UserViewsRead
}

// ViewNames lists the names of the views the user's settings declare, in the
// order the file declares them. dinah config reports them as the value of
// the dinah.views row, so a reader who opens the file and sees the block also
// sees that the tool reads it.
func (c *Config) ViewNames() []string {
	views, _ := ReadViews(c.fm, ViewSourceUser)
	names := make([]string, 0, len(views))
	for _, view := range views {
		names = append(names, view.Name)
	}
	return names
}

// Views returns the views the workbench's own definition declares, in
// declaration order, and whether its dinah.views value was present and not a
// mapping, in which case no view was read from it.
func (b *Bench) Views() ([]View, bool) {
	views := make([]View, len(b.views))
	copy(views, b.views)
	return views, b.viewsBlockDefect
}

// writeViewsMember writes the dinah.views member of an imported definition as
// a block, with the renderer's multi-member entry form switched on, so that a
// section of two members comes back as a dashed entry a person can still edit
// by hand. It falls back to the one raw JSON line as writeMember does.
//
// The form is switched on for this member alone because the renderer's
// read-back guard holds the lines against blockValue and not against the
// reader that will consume them. ReadViews reads through blockValue, so the
// guard is the right arbiter here; a member read by a reader of its own, as
// levels is, would inherit a form that reader cannot read.
func writeViewsMember(fm *Frontmatter, raw json.RawMessage) {
	if lines, renderable := renderBlock(ViewsKey, 0, raw, true); renderable {
		fm.SetRaw(ViewsKey, lines)
		return
	}
	setRawJSON(fm, ViewsKey, raw)
}
