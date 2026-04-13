package crawler

import (
	"strings"
	"testing"
)

func TestExtractLinksWithSelectors_NavLinks(t *testing.T) {
	html := `<html><body><nav><a href="https://example.com/page">link</a></nav></body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	assertContainsTag(t, links[0].Selectors, "nav")
}

func TestExtractLinksWithSelectors_FooterLinks(t *testing.T) {
	html := `<html><body><footer><a href="https://example.com/about">link</a></footer></body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	assertContainsTag(t, links[0].Selectors, "footer")
}

func TestExtractLinksWithSelectors_MainLinks(t *testing.T) {
	html := `<html><body><main><a href="https://example.com/article">link</a></main></body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	assertContainsTag(t, links[0].Selectors, "main")
}

func TestExtractLinksWithSelectors_AsideLinks(t *testing.T) {
	html := `<html><body><aside><a href="https://example.com/sidebar">link</a></aside></body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	assertContainsTag(t, links[0].Selectors, "aside")
}

func TestExtractLinksWithSelectors_NestedStructure(t *testing.T) {
	html := `<body><main><div class="content"><a href="https://example.com/deep">link</a></div></main></body>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	expected := []Selector{
		{TagName: "body"},
		{TagName: "main"},
		{TagName: "div", Class: "content"},
	}
	sels := links[0].Selectors
	if len(sels) != len(expected) {
		t.Fatalf("expected %d selectors, got %d: %+v", len(expected), len(sels), sels)
	}
	for i, exp := range expected {
		if sels[i].TagName != exp.TagName || sels[i].Class != exp.Class {
			t.Errorf("selector[%d]: expected %+v, got %+v", i, exp, sels[i])
		}
	}
}

func TestExtractLinksWithSelectors_SkipsJavascriptMailto(t *testing.T) {
	html := `<html><body>
		<a href="javascript:void(0)">js</a>
		<a href="mailto:test@test.com">mail</a>
		<a href="https://example.com/real">real</a>
	</body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].URL != "https://example.com/real" {
		t.Errorf("expected real link, got %s", links[0].URL)
	}
}

func TestExtractLinksWithSelectors_SkipsEmptyHref(t *testing.T) {
	html := `<html><body>
		<a href="">empty</a>
		<a>no href</a>
		<a href="https://example.com/valid">valid</a>
	</body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].URL != "https://example.com/valid" {
		t.Errorf("expected valid link, got %s", links[0].URL)
	}
}

func TestExtractLinksWithSelectors_ClasslessDivIgnored(t *testing.T) {
	html := `<html><body><div><a href="https://example.com/page">link</a></div></body></html>`
	links, err := ExtractLinksWithSelectors(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	for _, sel := range links[0].Selectors {
		if sel.TagName == "div" {
			t.Error("classless div should not appear in selectors")
		}
	}
}

func assertContainsTag(t *testing.T, selectors []Selector, tagName string) {
	t.Helper()
	for _, s := range selectors {
		if s.TagName == tagName {
			return
		}
	}
	t.Errorf("selectors %+v do not contain tag %q", selectors, tagName)
}
