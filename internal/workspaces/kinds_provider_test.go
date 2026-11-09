package workspaces

import (
	"context"
	"errors"
	"testing"

	"studio/internal/resource"
)

type fakeProvider struct{ source Source }

func TestRegisteredKindSuppliesProviderAndAttach(t *testing.T) {
	RegisterSourceKind("test-site-page", SourceKind{
		Label: "Test Site Page",
		Attach: &Attach{Fields: []Field{{ID: "page", Label: "Page", Kind: "text", Required: true}}, Build: func(_ context.Context, v map[string]string) (Source, error) {
			return Source{Kind: "test-site-page", Name: "Page " + v["page"], Extra: WithExtra(nil, "pageId", v["page"])}, nil
		}},
		Provider: func(_ Resolver, _ context.Context, s Source) (any, error) { return fakeProvider{source: s}, nil },
	})
	RegisterSourceKind("test-import-only", SourceKind{})

	kinds := AttachableSourceKinds()
	found := false
	for _, k := range kinds {
		if k.Kind == "test-import-only" {
			t.Fatal("a kind without a form is offered for adding")
		}
		found = found || k.Kind == "test-site-page"
	}
	if !found {
		t.Fatal("the registered kind is not offered for adding")
	}
	if !HasProvider("test-site-page") || HasProvider("test-import-only") || !HasProvider(LocalFolder) || HasProvider("nobody") {
		t.Fatal("HasProvider does not follow the registrations")
	}

	source, err := kinds[len(kinds)-1].Attach.Build(context.Background(), map[string]string{"page": "42"})
	if err != nil || ExtraString(source.Extra, "pageId") != "42" || ExtraBool(source.Extra, "pageId") {
		t.Fatalf("the built source does not keep its field: %+v %v", source, err)
	}
	source.ID = "src_1"
	record := Record{ID: "ws", Sources: []Source{source}}
	r := Resolver{Records: func() []Record { return []Record{record} }}
	p, got, err := r.Provider(context.Background(), Ref{WorkspaceID: "ws", SourceID: "src_1"})
	if err != nil || got.ID != "src_1" {
		t.Fatalf("Provider: %v %+v", err, got)
	}
	if fp, ok := p.(fakeProvider); !ok || fp.source.ID != "src_1" {
		t.Fatalf("the kind's provider was not built: %#v", p)
	}
	_, err = r.ProviderOf(context.Background(), Source{Kind: "test-import-only"})
	if !errors.Is(err, resource.ErrUnsupported) {
		t.Fatalf("a kind without a provider: %v", err)
	}
}
