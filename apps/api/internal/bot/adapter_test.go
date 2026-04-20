package bot

import (
	"context"
	"testing"
)

type stubAdapter struct {
	domain string
	name   string
}

func (s *stubAdapter) Domain() string { return s.domain }
func (s *stubAdapter) Name() string   { return s.name }
func (s *stubAdapter) Extract(_ context.Context, _ []byte, _ string) (PinDocument, error) {
	return PinDocument{}, nil
}

func TestRegistry_ExactMatch(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "pixiv.net", name: "pixiv"})

	a, ok := r.Resolve("pixiv.net")
	if !ok || a.Name() != "pixiv" {
		t.Fatalf("expected pixiv adapter, got ok=%v a=%v", ok, a)
	}
}

func TestRegistry_CaseInsensitive(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "Pixiv.NET", name: "pixiv"})
	if _, ok := r.Resolve("PIXIV.NET"); !ok {
		t.Fatal("expected case-insensitive match")
	}
}

func TestRegistry_WildcardSubdomain(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "*.example.com", name: "ex"})

	if a, ok := r.Resolve("foo.example.com"); !ok || a.Name() != "ex" {
		t.Fatalf("foo.example.com should match wildcard, got ok=%v", ok)
	}
	if a, ok := r.Resolve("a.b.example.com"); !ok || a.Name() != "ex" {
		t.Fatalf("a.b.example.com should match wildcard, got ok=%v", ok)
	}
}

func TestRegistry_WildcardDoesNotMatchBareDomain(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "*.example.com", name: "ex"})
	if _, ok := r.Resolve("example.com"); ok {
		t.Fatal("*.example.com must NOT match bare example.com")
	}
}

func TestRegistry_ExactBeatsWildcard(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "*.example.com", name: "wild"})
	r.Register(&stubAdapter{domain: "foo.example.com", name: "exact"})
	a, ok := r.Resolve("foo.example.com")
	if !ok || a.Name() != "exact" {
		t.Fatalf("expected exact match to win, got %v", a)
	}
}

func TestRegistry_UnregisteredDomainReturnsFalse(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(&stubAdapter{domain: "pixiv.net", name: "pixiv"})
	if a, ok := r.Resolve("artstation.com"); ok {
		t.Fatalf("expected miss, got %v", a)
	}
}

func TestRegistry_RegisterNilNoop(t *testing.T) {
	r := NewInMemoryAdapterRegistry()
	r.Register(nil)
	r.Register(&stubAdapter{domain: "", name: "x"})
	if _, ok := r.Resolve(""); ok {
		t.Fatal("empty domain must not resolve")
	}
}

func TestFillExtractorIdentity(t *testing.T) {
	doc := PinDocument{}
	FillExtractorIdentity(&doc, "generic")
	if doc.OGData.Extractor != "generic" {
		t.Fatalf("got %q", doc.OGData.Extractor)
	}

	// Already set: should not overwrite.
	doc2 := PinDocument{OGData: OGData{Extractor: "script:abc"}}
	FillExtractorIdentity(&doc2, "generic")
	if doc2.OGData.Extractor != "script:abc" {
		t.Fatalf("expected preserved value, got %q", doc2.OGData.Extractor)
	}
}
