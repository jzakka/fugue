package bot

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// PlaywrightFetcher renders pages with a headless Chromium browser driven by
// Playwright. This lets Pioneer crawl JS-heavy sites (e.g. pixiv) that return
// a mostly-empty shell when fetched via plain HTTP.
//
// A single browser instance is reused across Fetch calls. Each Fetch opens a
// fresh BrowserContext so cookies and storage are isolated per request.
type PlaywrightFetcher struct {
	pw        *playwright.Playwright
	browser   playwright.Browser
	userAgent string
	timeoutMs int

	mu sync.Mutex
}

// PlaywrightFetcherConfig configures PlaywrightFetcher.
type PlaywrightFetcherConfig struct {
	// UserAgent sent on every request. Defaults to FugueBot/1.0 when empty.
	UserAgent string
	// TimeoutMs is the per-navigation timeout. Defaults to 30000.
	TimeoutMs int
	// WaitUntil controls when Goto resolves. Defaults to "domcontentloaded".
	// Accepted: "load", "domcontentloaded", "networkidle", "commit".
	WaitUntil string
}

// NewPlaywrightFetcher starts Playwright and launches a headless Chromium.
// Callers MUST call Close when done to release the browser process.
func NewPlaywrightFetcher(cfg PlaywrightFetcherConfig) (*PlaywrightFetcher, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w (hint: run `go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium`)", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("launch chromium: %w", err)
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = "FugueBot/1.0 (+https://fugue.app)"
	}
	timeoutMs := cfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}

	return &PlaywrightFetcher{
		pw:        pw,
		browser:   browser,
		userAgent: ua,
		timeoutMs: timeoutMs,
	}, nil
}

// Close shuts down the browser and stops Playwright.
func (f *PlaywrightFetcher) Close() error {
	var errs []error
	if f.browser != nil {
		if err := f.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close browser: %w", err))
		}
	}
	if f.pw != nil {
		if err := f.pw.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop playwright: %w", err))
		}
	}
	return errors.Join(errs...)
}

// Fetch navigates to rawURL and returns the rendered HTML plus the final URL
// after any redirects. The provided context is honored by closing the page
// early if it is canceled.
func (f *PlaywrightFetcher) Fetch(ctx context.Context, rawURL string) (string, string, error) {
	// Playwright's Go bindings are not goroutine-safe at the connection level,
	// so serialize Fetch calls against the shared browser.
	f.mu.Lock()
	defer f.mu.Unlock()

	pctx, err := f.browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(f.userAgent),
	})
	if err != nil {
		return "", "", fmt.Errorf("new browser context: %w", err)
	}
	defer func() {
		_ = pctx.Close()
	}()

	page, err := pctx.NewPage()
	if err != nil {
		return "", "", fmt.Errorf("new page: %w", err)
	}

	// Watch ctx cancellation and close the page early.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = page.Close()
		case <-done:
		}
	}()

	resp, err := page.Goto(rawURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(f.timeoutMs)),
	})
	if err != nil {
		return "", "", fmt.Errorf("goto %s: %w", rawURL, err)
	}
	if resp != nil {
		if status := resp.Status(); status < 200 || status >= 400 {
			return "", "", fmt.Errorf("HTTP error: status code %d", status)
		}
	}

	html, err := page.Content()
	if err != nil {
		return "", "", fmt.Errorf("get content: %w", err)
	}
	return html, page.URL(), nil
}
