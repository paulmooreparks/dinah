package docfiles

import (
	"strings"
	"testing"
)

func TestShown(t *testing.T) {
	text := "![a](one.png) and ![b](<two words.png> \"title\") ![c](https://example.com/x.png) ![d](data:image/png;base64,AAAA)\n" +
		"<IMG alt=\"x\" src='sub/three.png'> ![e](./four%20five.png#frag) ![f](/abs/six.png) ![g](C:\\pics\\seven.png)\n" +
		"{{figure:src=\"eight nine.png\" alt=\"x\"}} {{figure:alt=\"Ten\" src=ten.png}} `![h](code.png)` {{figure:one.png}}\n" +
		"~~~\n{{figure:fenced.png}}\n~~~\n{{figure:~/Pictures/home.png}} ![i](..\\up\\eleven.png)\n"
	got := strings.Join(Shown(text), "|")
	if want := "one.png|two words.png|four five.png|sub/three.png|eight nine.png|ten.png|../up/eleven.png"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestLinked(t *testing.T) {
	text := "[plan](files/plan.pdf) ![pic](pic.png) [doc](other.md) [web](https://example.com/a.pdf) [here](#top) [folder](notes/)" +
		" [mail](mailto:a@example.com) [two](<my files/two words.xlsx>) [again](files/plan.pdf) [abs](/srv/x.pdf) `[code](code.zip)` [q](data.csv?x=1#y)" + "\n"
	got := strings.Join(Linked(text), "|")
	if want := "files/plan.pdf|my files/two words.xlsx|data.csv"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestShownListsDiagramsAndListings(t *testing.T) {
	text := "{{diagram:mockups/plan.canvas alt=" + `"The plan"` + " width=80%}}" + "\n" + "{{code:scripts/run.sh lang=sh}} {{diagram:~/shared/flow.d2}} `{{diagram:code.d2}}`" + "\n"
	if got, want := strings.Join(Shown(text), "|"), "mockups/plan.canvas|scripts/run.sh"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
