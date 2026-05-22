package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
)

const defaultScriptTimeoutMs = 10000

// GojaExecutor executes JavaScript parsing scripts using the goja engine.
type GojaExecutor struct {
	timeoutMs int
}

// NewGojaExecutor creates a GojaExecutor. If timeoutMs <= 0, defaults to 10000ms.
func NewGojaExecutor(timeoutMs int) *GojaExecutor {
	if timeoutMs <= 0 {
		timeoutMs = defaultScriptTimeoutMs
	}
	return &GojaExecutor{timeoutMs: timeoutMs}
}

// Execute runs a JavaScript script against HTML and returns extracted items.
func (e *GojaExecutor) Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error) {
	vm := goja.New()

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	// Inject DOM helpers
	injectDOMHelpers(vm, doc)

	// Set up timeout via goroutine + Interrupt
	timeout := time.Duration(e.timeoutMs) * time.Millisecond
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go func() {
		<-timeoutCtx.Done()
		// Interrupt on both DeadlineExceeded (the executor's 10s upper bound
		// fired) and Canceled (parent ctx cancellation — typically cmd/bot's
		// signal.NotifyContext runCtx receiving SIGTERM, propagated through
		// script_adapter.Execute(ctx, ...)). Without the Canceled branch a
		// parent cancel arrives at timeoutCtx as Canceled (Go stdlib
		// context.WithTimeout child semantics) and the 10s upper bound is
		// silently bypassed on the SIGTERM path — a malicious or buggy site
		// script (e.g. `while(true){}`) keeps running until natural
		// completion, hanging the harvester worker past k8s
		// terminationGracePeriodSeconds and forcing SIGKILL, which skips the
		// in-flight URL's SetStatus/RecordHarvestError finalize and leaves
		// the harvester_frontier row claimed until lease expiry. The VM
		// instance is single-use per Execute call, so calling Interrupt on
		// an already-finished VM (RunString returned before Done fired) is
		// a no-op and safe to race against script completion.
		vm.Interrupt(timeoutCtx.Err())
	}()

	// Unwrap JSON-wrapped scripts (e.g. {"script":"..."})
	script = unwrapScript(script)

	// Inject location object for scripts that reference location.href
	_, _ = vm.RunString(fmt.Sprintf(`var location = {href: %q};`, url))

	// Polyfill Array.from for goja (ES5 engine)
	_, _ = vm.RunString(`if (!Array.from) {
		Array.from = function(arrayLike, mapFn) {
			var result = [];
			for (var i = 0; i < arrayLike.length; i++) {
				result.push(mapFn ? mapFn(arrayLike[i], i) : arrayLike[i]);
			}
			return result;
		};
	}`)

	// Polyfill URL constructor for relative URL resolution
	_, _ = vm.RunString(`if (typeof URL === 'undefined') {
		function URL(url, base) {
			if (!url) throw new TypeError('Invalid URL');
			url = String(url);
			if (/^https?:\/\//i.test(url)) {
				this.href = url;
			} else if (base) {
				base = String(base);
				var m = base.match(/^(https?:\/\/[^\/]+)/i);
				if (!m) throw new TypeError('Invalid base URL');
				var origin = m[1];
				if (url.startsWith('/')) {
					this.href = origin + url;
				} else {
					var basePath = base.replace(/[?#].*$/, '').replace(/\/[^\/]*$/, '/');
					this.href = basePath + url;
				}
			} else {
				this.href = url;
			}
		}
	}`)

	// Execute the script
	val, err := vm.RunString(script)
	if err != nil {
		if interrupted, ok := err.(*goja.InterruptedError); ok {
			return nil, fmt.Errorf("timeout: %s", interrupted.Value())
		}
		return nil, fmt.Errorf("script error: %w", err)
	}

	// Convert result to RawItem array
	items, err := convertResult(vm, val, url)
	if err != nil {
		return nil, err
	}

	return items, nil
}

// injectDOMHelpers registers querySelectorAll, querySelector, textContent, getAttribute
// on a document object in the goja runtime.
func injectDOMHelpers(vm *goja.Runtime, doc *goquery.Document) {
	// Declare wrapElement first to allow recursive reference
	var wrapElement func(sel *goquery.Selection) goja.Value
	wrapElement = func(sel *goquery.Selection) goja.Value {
		if sel == nil || sel.Length() == 0 {
			return goja.Null()
		}
		obj := vm.NewObject()

		_ = obj.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(strings.TrimSpace(sel.Text()))
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		_ = obj.Set("getAttribute", func(name string) goja.Value {
			val, exists := sel.Attr(name)
			if !exists {
				return goja.Null()
			}
			return vm.ToValue(val)
		})

		_ = obj.Set("querySelector", func(selector string) goja.Value {
			found := sel.Find(selector)
			if found.Length() == 0 {
				return goja.Null()
			}
			return wrapElement(found.First())
		})

		_ = obj.Set("querySelectorAll", func(selector string) goja.Value {
			found := sel.Find(selector)
			var results []interface{}
			found.Each(func(_ int, s *goquery.Selection) {
				results = append(results, wrapElement(s))
			})
			return vm.NewArray(results...)
		})

		_ = obj.DefineAccessorProperty("innerHTML", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			h, _ := sel.Html()
			return vm.ToValue(h)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		// closest: walk up the tree to find a matching ancestor
		_ = obj.Set("closest", func(selector string) goja.Value {
			cur := sel
			for cur.Length() > 0 {
				if cur.Is(selector) {
					return wrapElement(cur)
				}
				cur = cur.Parent()
			}
			return goja.Null()
		})

		// classList property with contains method
		classListObj := vm.NewObject()
		_ = classListObj.Set("contains", func(className string) bool {
			return sel.HasClass(className)
		})
		_ = obj.Set("classList", classListObj)

		// children property
		_ = obj.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			var results []interface{}
			sel.Children().Each(func(_ int, s *goquery.Selection) {
				results = append(results, wrapElement(s))
			})
			return vm.NewArray(results...)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		// parentElement property
		_ = obj.DefineAccessorProperty("parentElement", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			parent := sel.Parent()
			if parent.Length() == 0 {
				return goja.Null()
			}
			return wrapElement(parent)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		// tagName property (uppercase, like browser DOM)
		_ = obj.DefineAccessorProperty("tagName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if sel.Length() == 0 {
				return goja.Null()
			}
			return vm.ToValue(strings.ToUpper(goquery.NodeName(sel)))
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		return obj
	}

	// Create document object
	docObj := vm.NewObject()

	_ = docObj.Set("querySelectorAll", func(selector string) goja.Value {
		found := doc.Find(selector)
		var results []interface{}
		found.Each(func(_ int, s *goquery.Selection) {
			results = append(results, wrapElement(s))
		})
		return vm.NewArray(results...)
	})

	_ = docObj.Set("querySelector", func(selector string) goja.Value {
		found := doc.Find(selector)
		if found.Length() == 0 {
			return goja.Null()
		}
		return wrapElement(found.First())
	})

	_ = vm.Set("document", docObj)
}

// convertResult converts the script return value to a slice of RawItem.
func convertResult(vm *goja.Runtime, val goja.Value, nodeURL string) ([]RawItem, error) {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, fmt.Errorf("script returned %v, expected array", val)
	}

	// Check if it's an array using Array.isArray
	isArray, err := vm.RunString("Array.isArray")
	if err != nil {
		return nil, fmt.Errorf("failed to get Array.isArray: %w", err)
	}
	isArrayFn, ok := goja.AssertFunction(isArray)
	if !ok {
		return nil, fmt.Errorf("Array.isArray is not a function")
	}
	result, err := isArrayFn(goja.Undefined(), val)
	if err != nil {
		return nil, fmt.Errorf("Array.isArray call failed: %w", err)
	}
	if !result.ToBoolean() {
		return nil, fmt.Errorf("script returned non-array value, expected array")
	}

	obj := val.ToObject(vm)
	lengthVal := obj.Get("length")

	length := int(lengthVal.ToInteger())
	var items []RawItem

	for i := 0; i < length; i++ {
		elemVal := obj.Get(fmt.Sprintf("%d", i))
		if elemVal == nil || goja.IsUndefined(elemVal) || goja.IsNull(elemVal) {
			continue
		}

		elemObj := elemVal.ToObject(vm)
		if elemObj == nil {
			continue
		}

		title := getStringField(vm, elemObj, "title")
		mediaURL := getStringField(vm, elemObj, "mediaURL")
		mediaType := getStringField(vm, elemObj, "mediaType")

		// Skip items with missing required fields
		if title == "" || mediaURL == "" || mediaType == "" {
			continue
		}

		description := getStringField(vm, elemObj, "description")
		sourceURL := getStringField(vm, elemObj, "sourceURL")

		// Default sourceURL to node URL if empty
		if sourceURL == "" {
			sourceURL = nodeURL
		}

		items = append(items, RawItem{
			Title:       title,
			Description: description,
			MediaURL:    mediaURL,
			SourceURL:   sourceURL,
			MediaType:   mediaType,
		})
	}

	return items, nil
}

// unwrapScript handles JSON-wrapped scripts like {"script":"(function(){...})()"}.
func unwrapScript(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var wrapper map[string]string
		if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
			if code, ok := wrapper["script"]; ok {
				return code
			}
		}
	}
	return raw
}

// getStringField safely extracts a string field from a goja object.
func getStringField(vm *goja.Runtime, obj *goja.Object, field string) string {
	val := obj.Get(field)
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return ""
	}
	return val.String()
}
