package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

const (
	unsplashAPIBase = "https://api.unsplash.com"
	unsplashPerPage = 30
)

// Unsplash implements the bot.Source interface for the Unsplash REST API.
type Unsplash struct {
	accessKey string
	client    *http.Client
}

// NewUnsplash creates a new Unsplash source.
func NewUnsplash(accessKey string) *Unsplash {
	return &Unsplash{
		accessKey: accessKey,
		client:    &http.Client{},
	}
}

func (u *Unsplash) Name() string { return "unsplash" }

// unsplashPhoto represents the relevant fields from the Unsplash API response.
type unsplashPhoto struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	AltDesc     string `json:"alt_description"`
	URLs        struct {
		Regular string `json:"regular"`
		Full    string `json:"full"`
	} `json:"urls"`
	Links struct {
		HTML string `json:"html"`
	} `json:"links"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
}

// Crawl fetches recent photos from the Unsplash API.
func (u *Unsplash) Crawl(ctx context.Context) ([]bot.RawItem, error) {
	url := fmt.Sprintf("%s/photos?per_page=%d&order_by=latest", unsplashAPIBase, unsplashPerPage)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unsplash: create request: %w", err)
	}
	req.Header.Set("Authorization", "Client-ID "+u.accessKey)
	req.Header.Set("Accept-Version", "v1")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unsplash: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsplash: API returned HTTP %d", resp.StatusCode)
	}

	var photos []unsplashPhoto
	if err := json.NewDecoder(resp.Body).Decode(&photos); err != nil {
		return nil, fmt.Errorf("unsplash: decode: %w", err)
	}

	items := make([]bot.RawItem, 0, len(photos))
	for _, p := range photos {
		title := buildTitle(p)
		if title == "" {
			continue
		}

		desc := p.Description
		if desc == "" {
			desc = p.AltDesc
		}

		// Use regular size (1080px wide) for reasonable file sizes
		mediaURL := p.URLs.Regular
		if mediaURL == "" {
			continue
		}

		items = append(items, bot.RawItem{
			Title:       title,
			Description: desc,
			MediaURL:    mediaURL,
			SourceURL:   p.Links.HTML,
			MediaType:   "image",
		})
	}

	return items, nil
}

// buildTitle creates a title from available photo data.
func buildTitle(p unsplashPhoto) string {
	if p.AltDesc != "" {
		// Capitalize first letter
		return strings.ToUpper(p.AltDesc[:1]) + p.AltDesc[1:]
	}
	if p.Description != "" {
		desc := p.Description
		if len(desc) > 100 {
			desc = desc[:100]
		}
		return desc
	}
	if p.User.Name != "" {
		return fmt.Sprintf("Photo by %s", p.User.Name)
	}
	return ""
}
