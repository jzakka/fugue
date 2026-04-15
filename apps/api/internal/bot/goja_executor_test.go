package bot

import (
	"context"
	"strings"
	"testing"
)

func TestGojaExecutor_NormalExecution(t *testing.T) {
	executor := NewGojaExecutor(0) // default timeout

	html := `<html><body>
		<div class="item">
			<h1>Test Title</h1>
			<img src="https://example.com/img.jpg"/>
		</div>
	</body></html>`

	script := `
	(function() {
		var items = document.querySelectorAll('.item');
		var results = [];
		for (var i = 0; i < items.length; i++) {
			var el = items[i];
			var title = el.querySelector('h1').textContent;
			var img = el.querySelector('img').getAttribute('src');
			results.push({
				title: title,
				mediaURL: img,
				mediaType: 'image',
				sourceURL: 'https://example.com/page1'
			});
		}
		return results;
	})()
	`

	items, err := executor.Execute(context.Background(), script, html, "https://example.com/page1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", items[0].Title)
	}
	if items[0].MediaURL != "https://example.com/img.jpg" {
		t.Errorf("expected mediaURL, got %q", items[0].MediaURL)
	}
	if items[0].MediaType != "image" {
		t.Errorf("expected mediaType 'image', got %q", items[0].MediaType)
	}
}

func TestGojaExecutor_SyntaxError(t *testing.T) {
	executor := NewGojaExecutor(0)

	_, err := executor.Execute(context.Background(), "function {{{", "<html></html>", "https://example.com")
	if err == nil {
		t.Fatal("expected error for syntax error script")
	}
	if !strings.Contains(err.Error(), "script error") {
		t.Errorf("expected 'script error', got %q", err.Error())
	}
}

func TestGojaExecutor_RuntimeError(t *testing.T) {
	executor := NewGojaExecutor(0)

	script := `
	(function() {
		var x = null;
		x.foo(); // runtime error
		return [];
	})()
	`

	_, err := executor.Execute(context.Background(), script, "<html></html>", "https://example.com")
	if err == nil {
		t.Fatal("expected error for runtime error script")
	}
}

func TestGojaExecutor_EmptyHTML(t *testing.T) {
	executor := NewGojaExecutor(0)

	script := `
	(function() {
		var items = document.querySelectorAll('.item');
		var results = [];
		for (var i = 0; i < items.length; i++) {
			results.push({title: 'x', mediaURL: 'y', mediaType: 'image'});
		}
		return results;
	})()
	`

	items, err := executor.Execute(context.Background(), script, "", "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error on empty HTML: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty HTML, got %d", len(items))
	}
}

func TestGojaExecutor_MissingRequiredFields(t *testing.T) {
	executor := NewGojaExecutor(0)

	// Return items with missing title, mediaURL, mediaType
	script := `
	[
		{title: 'Good', mediaURL: 'http://img.jpg', mediaType: 'image'},
		{title: '', mediaURL: 'http://img.jpg', mediaType: 'image'},
		{title: 'NoURL', mediaURL: '', mediaType: 'image'},
		{title: 'NoType', mediaURL: 'http://img.jpg', mediaType: ''}
	]
	`

	items, err := executor.Execute(context.Background(), script, "<html></html>", "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 valid item (3 skipped), got %d", len(items))
	}
}

func TestGojaExecutor_NonArrayReturn(t *testing.T) {
	executor := NewGojaExecutor(0)

	tests := []struct {
		name   string
		script string
	}{
		{"null", "null"},
		{"undefined", "undefined"},
		{"string", "'hello'"},
		{"number", "42"},
		{"single object", "({title: 'x', mediaURL: 'y', mediaType: 'z'})"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Execute(context.Background(), tt.script, "<html></html>", "https://example.com")
			if err == nil {
				t.Errorf("expected error for non-array return (%s)", tt.name)
			}
		})
	}
}

func TestGojaExecutor_Timeout(t *testing.T) {
	executor := NewGojaExecutor(100) // 100ms timeout

	script := `
	(function() {
		while(true) {} // infinite loop
		return [];
	})()
	`

	_, err := executor.Execute(context.Background(), script, "<html></html>", "https://example.com")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected 'timeout' in error, got %q", err.Error())
	}
}

func TestGojaExecutor_SourceURLDefault(t *testing.T) {
	executor := NewGojaExecutor(0)

	script := `
	[{title: 'Test', mediaURL: 'http://img.jpg', mediaType: 'image'}]
	`

	items, err := executor.Execute(context.Background(), script, "<html></html>", "https://example.com/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].SourceURL != "https://example.com/page" {
		t.Errorf("expected sourceURL default to node URL, got %q", items[0].SourceURL)
	}
}

func TestGojaExecutor_DOMHelpers(t *testing.T) {
	executor := NewGojaExecutor(0)

	html := `<html><body>
		<a href="https://example.com/link" class="link">Click here</a>
		<span class="info">Info text</span>
	</body></html>`

	script := `
	(function() {
		var link = document.querySelector('.link');
		var info = document.querySelector('.info');
		var missing = document.querySelector('.nonexistent');
		var results = [];
		if (link && info && missing === null) {
			results.push({
				title: link.textContent,
				mediaURL: link.getAttribute('href'),
				mediaType: 'image',
				description: info.textContent
			});
		}
		return results;
	})()
	`

	items, err := executor.Execute(context.Background(), script, html, "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Click here" {
		t.Errorf("textContent: expected 'Click here', got %q", items[0].Title)
	}
	if items[0].MediaURL != "https://example.com/link" {
		t.Errorf("getAttribute: expected URL, got %q", items[0].MediaURL)
	}
	if items[0].Description != "Info text" {
		t.Errorf("description: expected 'Info text', got %q", items[0].Description)
	}
}
