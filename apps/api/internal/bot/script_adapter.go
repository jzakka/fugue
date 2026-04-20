package bot

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// scriptCtxKey is an unexported context key type so callers can only set the
// node type via WithNodeType. (Avoids accidental key collisions.)
type scriptCtxKey struct{}

// WithNodeType returns a context that carries the harvester node type.
// ScriptAdapter reads this when looking up the (site_id, node_type) script.
func WithNodeType(ctx context.Context, nt string) context.Context {
	return context.WithValue(ctx, scriptCtxKey{}, nt)
}

// NodeTypeFromContext returns the node type previously set by WithNodeType,
// or "" if none.
func NodeTypeFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(scriptCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// ScriptAdapter wraps the existing ScriptExecutor (GojaExecutor) so that the
// per-site script flow becomes a PerSiteAdapter override on top of the
// generic extractor.
//
// One ScriptAdapter is registered per (siteID, domain) at bootstrap; the
// adapter discovers the actual script to run by looking up
// (siteID, node_type) from scriptRepo when Extract is called. The node type
// is propagated via context (see WithNodeType).
type ScriptAdapter struct {
	siteID     uuid.UUID
	domain     string
	scriptRepo ScriptRepository
	executor   ScriptExecutor
}

// NewScriptAdapter constructs a ScriptAdapter bound to a single site.
func NewScriptAdapter(siteID uuid.UUID, domain string, scriptRepo ScriptRepository, executor ScriptExecutor) *ScriptAdapter {
	return &ScriptAdapter{
		siteID:     siteID,
		domain:     domain,
		scriptRepo: scriptRepo,
		executor:   executor,
	}
}

// Domain returns the site's domain.
func (a *ScriptAdapter) Domain() string { return a.domain }

// Name returns the og_data.extractor identifier for this adapter.
func (a *ScriptAdapter) Name() string {
	return "script:" + a.siteID.String()
}

// Extract loads the (site_id, node_type) script, runs it via the embedded
// ScriptExecutor, and reduces the resulting RawItem array to a single
// PinDocument. The first RawItem becomes the canonical document; remaining
// RawItems are appended to og_data.media_candidates.
//
// Returns an error when:
//   - node type is missing from context
//   - script lookup fails (no script for site/node_type)
//   - script execution fails
//   - script returns zero RawItems
//
// On error the Harvester is expected to fall back to GenericExtractor.
func (a *ScriptAdapter) Extract(ctx context.Context, htmlBytes []byte, fetchURL string) (PinDocument, error) {
	nodeType := NodeTypeFromContext(ctx)
	if nodeType == "" {
		return PinDocument{}, fmt.Errorf("script adapter: node type missing from context")
	}

	script, err := a.scriptRepo.GetBySiteType(ctx, db.GetScriptBySiteTypeParams{
		SiteID:   a.siteID,
		NodeType: nodeType,
	})
	if err != nil {
		return PinDocument{}, fmt.Errorf("script adapter: load script: %w", err)
	}

	items, err := a.executor.Execute(ctx, script.ScriptCode, string(htmlBytes), fetchURL)
	if err != nil {
		return PinDocument{}, fmt.Errorf("script adapter: execute: %w", err)
	}
	if len(items) == 0 {
		return PinDocument{}, fmt.Errorf("script adapter: zero items")
	}

	doc := rawItemsToPinDocument(items, fetchURL)
	doc.OGData.Extractor = a.Name()
	return doc, nil
}

// rawItemsToPinDocument reduces N RawItems into a single canonical PinDocument:
// the first item provides the primary metadata; subsequent items become
// secondary MediaCandidates. The Harvester step has the final say on
// pins.url / pins.description truncation; this function only assembles the
// in-memory document.
func rawItemsToPinDocument(items []RawItem, fetchURL string) PinDocument {
	primary := items[0]

	doc := PinDocument{
		Title:        primary.Title,
		BodyText:     primary.Description,
		CanonicalURL: pickFirstNonEmpty(primary.SourceURL, fetchURL),
		ThumbnailURL: primary.MediaURL,
		OGData: OGData{
			Source: fetchURL,
		},
	}

	if primary.MediaURL != "" {
		doc.MediaCandidates = append(doc.MediaCandidates, MediaCandidate{
			Type: primary.MediaType,
			URL:  primary.MediaURL,
		})
	}

	for _, it := range items[1:] {
		if it.MediaURL == "" {
			continue
		}
		doc.MediaCandidates = append(doc.MediaCandidates, MediaCandidate{
			Type: it.MediaType,
			URL:  it.MediaURL,
		})
	}

	if len(doc.MediaCandidates) > MaxMediaCandidates {
		doc.MediaCandidates = doc.MediaCandidates[:MaxMediaCandidates]
	}
	doc.OGData.MediaCandidates = doc.MediaCandidates

	return doc
}

// RegisterScriptAdaptersForActiveSites scans active sites and registers a
// ScriptAdapter for any site whose domain has at least one script row.
// Bootstrap-only: callers must invoke this once at process start. Runtime
// DB changes are out of scope for this change and require a process
// restart.
func RegisterScriptAdaptersForActiveSites(
	ctx context.Context,
	registry AdapterRegistry,
	siteRepo SiteRepository,
	scriptRepo ScriptRepository,
	executor ScriptExecutor,
	hasAnyScript func(ctx context.Context, siteID uuid.UUID) (bool, error),
) error {
	sites, err := siteRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active sites: %w", err)
	}
	for _, site := range sites {
		ok, err := hasAnyScript(ctx, site.ID)
		if err != nil {
			return fmt.Errorf("check scripts for site %s: %w", site.ID, err)
		}
		if !ok {
			continue
		}
		adapter := NewScriptAdapter(site.ID, site.Domain, scriptRepo, executor)
		registry.Register(adapter)
	}
	return nil
}
