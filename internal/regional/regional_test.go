package regional

import "testing"

func TestLocaleTags(t *testing.T) {
	for _, c := range []struct {
		name string
		h24  int
		want string
	}{
		{"en-SG", 1, "en-SG-u-hc-h23"},
		{"en-US", 0, "en-US-u-hc-h12"},
		{"de_DE", -1, "de-DE"},
		{"", 1, ""},
		{"x-custom", 1, ""},
	} {
		if got := localeTag(c.name, c.h24); got != c.want {
			t.Errorf("localeTag(%q, %d) = %q, want %q", c.name, c.h24, got, c.want)
		}
	}
	for in, want := range map[string]string{"en_SG.UTF-8": "en_SG", "de_DE@euro": "de_DE", "C": "", "POSIX.UTF-8": "", "fr_FR": "fr_FR"} {
		if got := posixLocale(in); got != want {
			t.Errorf("posixLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTheHostLocaleIsATagOrNothing(t *testing.T) {
	got := HostLocale()
	if got != "" && localeTag(got, -1) == "" {
		t.Fatalf("HostLocale() = %q is not a usable tag", got)
	}
	t.Logf("host locale: %q", got)
}
