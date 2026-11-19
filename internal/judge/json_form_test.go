package judge

import (
	"strings"
	"testing"
)

// A JSON answer may stand alone or be the whole of one Markdown code block,
// with or without the word json on the fence. Anything before or after the
// document or its block is refused, as are two blocks and an unknown field.
func TestAJSONAnswerMayBeTheWholeOfOneCodeBlock(t *testing.T) {
	fence := strings.Repeat("`", 3)
	form := JSONForm[pick]("an object with chosen, a list of item numbers")
	for _, good := range []string{
		`{"chosen":[1,2]}`,
		"  {\"chosen\":[1,2]}\n",
		fence + "json\n{\n  \"chosen\": [1, 2]\n}\n" + fence,
		fence + "\n{\"chosen\":[1,2]}\n" + fence + "\n",
		fence + "JSON\r\n{\"chosen\":[1,2]}\r\n" + fence,
	} {
		out, err := form.Parse(good)
		if err != nil || len(out.Chosen) != 2 || out.Chosen[0] != 1 || out.Chosen[1] != 2 {
			t.Fatalf("%q was read as %+v, %v", good, out, err)
		}
	}
	for _, bad := range []string{
		"Here you are:\n" + fence + "json\n{\"chosen\":[1]}\n" + fence,
		fence + "json\n{\"chosen\":[1]}\n" + fence + "\nI hope that helps.",
		fence + "json\n{\"chosen\":[1]}\n" + fence + "\n" + fence + "json\n{\"chosen\":[2]}\n" + fence,
		fence + "json\n{\"chosen\":[1],\"why\":\"because\"}\n" + fence,
		fence + "json\n" + fence,
		fence + fence,
		`{"chosen":[1]} and more`,
		"",
	} {
		if out, err := form.Parse(bad); err == nil {
			t.Fatalf("%q was accepted as %+v", bad, out)
		}
	}
}
