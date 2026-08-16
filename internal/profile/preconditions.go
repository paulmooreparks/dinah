package profile

import (
	"regexp"
	"strconv"
	"strings"
)

// Precondition is one row of a verb's ordered precondition list, as section 6
// of the profile prints it.
type Precondition struct {
	// Order is the one-based position of the check in its verb's list, which
	// is the order CORE-OUT-6 makes observable and DOC-ORDER-1 protects.
	Order int
	// Check is the condition in the profile's own words.
	Check string
	// Refusal is the refusal name reported when the check is unsatisfied.
	Refusal string
}

// WorkbenchChecks is the key the two checks preceding every verb are filed
// under. They belong to the workbench rather than to any verb, and section
// 6.1 states them ahead of every list in 6.3 to 6.7.
const WorkbenchChecks = "workbench"

// preconditionRow matches one row of a precondition block: a number, then the
// check and the refusal name it reports. The two columns are aligned with
// spaces on most rows and separated by a single space on the one row whose
// check runs the full width, so the refusal is taken as the last word rather
// than by the gap ahead of it.
var preconditionRow = regexp.MustCompile(`^(\d+)\s\s+(\S.*\S)$`)

// sectionHeading matches any subsection heading of section 6.
var sectionHeading = regexp.MustCompile(`^### 6\.\d+ (.+)$`)

// listKey maps a subsection heading onto the key its precondition block is
// filed under. A heading absent from this map introduces a subsection that
// states no list, so its rows, if it ever grows any, are not a verb's.
var listKey = map[string]string{
	"outcomes and refusals": WorkbenchChecks,
	"claim":                 "claim",
	"move":                  "move",
	"release":               "release",
	"block":                 "block",
	"unblock":               "unblock",
}

// refusalWord matches the refusal name a row ends with.
var refusalWord = regexp.MustCompile(`^[a-z][a-z-]*$`)

// splitRefusal separates a row's check from the refusal name that closes it.
// A row whose last word is not spelled like a refusal name yields no refusal,
// which is how a stray fenced line that looks numbered is discarded.
func splitRefusal(row string) (string, string) {
	cut := strings.LastIndex(row, " ")
	if cut < 0 {
		return "", ""
	}
	name := row[cut+1:]
	if !refusalWord.MatchString(name) {
		return "", ""
	}
	return strings.TrimSpace(row[:cut]), name
}

// Preconditions reads the ordered precondition lists out of a profile
// document, keyed by the lowercase name of the verb each belongs to, with the
// two workbench-level checks under WorkbenchChecks.
//
// It reads the document line by line the way the statement extractor does, so
// a document growing new prose cannot break it and nothing here parses
// Markdown.
func Preconditions(text string) map[string][]Precondition {
	lists := map[string][]Precondition{}
	heading := ""
	inFence := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		if fence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if m := sectionHeading.FindStringSubmatch(line); m != nil {
			heading = listKey[strings.ToLower(strings.TrimSpace(m[1]))]
			continue
		}
		if !inFence || heading == "" {
			continue
		}
		m := preconditionRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		order, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		check, refusal := splitRefusal(m[2])
		if refusal == "" {
			continue
		}
		lists[heading] = append(lists[heading], Precondition{Order: order, Check: check, Refusal: refusal})
	}
	return lists
}
