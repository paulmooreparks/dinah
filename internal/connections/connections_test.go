package connections

import (
	"context"
	"testing"
)

func TestRegisterListsByLabelAndRefusesBadIDs(t *testing.T) {
	saved := Settings{}
	c := Connection{Label: "Zed", Load: func() (Settings, error) { return saved, nil }, Save: func(s Settings) error { saved = s; return nil }}
	Register("zed-test", c)
	Register("alpha-test", Connection{Label: "Alpha", Load: c.Load, Save: c.Save, Test: func(context.Context) (string, error) { return "ok", nil }})
	all := All()
	if len(all) < 2 || all[0].Label != "Alpha" {
		t.Fatalf("connections are not listed by label: %+v", all)
	}
	if _, ok := Lookup("zed-test"); !ok {
		t.Fatal("a registered connection is not found")
	}
	for _, bad := range []struct {
		id string
		c  Connection
	}{{"Bad", c}, {"zed-test", c}, {"nolabel", Connection{Load: c.Load, Save: c.Save}}, {"noload", Connection{Label: "x", Save: c.Save}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registering %q did not panic", bad.id)
				}
			}()
			Register(bad.id, bad.c)
		}()
	}
}
