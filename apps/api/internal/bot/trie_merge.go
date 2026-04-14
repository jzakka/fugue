package bot

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// MergeResult holds statistics from a merge operation.
type MergeResult struct {
	MergedPrefixes int
	RemovedNodes   int
}

// RunDrainMerge loads all nodes for a site, builds a DrainTree, identifies
// merge targets (clusters > threshold), and merges them in the DB.
func RunDrainMerge(ctx context.Context, q *db.Queries, siteID uuid.UUID, countThreshold int) (*MergeResult, error) {
	nodes, err := q.ListNodeURLsBySite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	if len(nodes) == 0 {
		return &MergeResult{}, nil
	}

	tree := NewDrainTree(DefaultSimThreshold)
	for _, n := range nodes {
		tree.Insert(n.Url)
	}

	targets := tree.MergeTargets(countThreshold)
	if len(targets) == 0 {
		return &MergeResult{}, nil
	}

	result := &MergeResult{}
	for _, target := range targets {
		removed, err := mergeOnePrefix(ctx, q, target, nodes)
		if err != nil {
			return nil, fmt.Errorf("merge prefix %s: %w", target.Prefix, err)
		}
		result.MergedPrefixes++
		result.RemovedNodes += removed
	}

	return result, nil
}

func mergeOnePrefix(
	ctx context.Context,
	q *db.Queries,
	target MergeTarget,
	allNodes []db.ListNodeURLsBySiteRow,
) (int, error) {
	// Build path → node lookup (extract path from full URLs)
	pathToNode := make(map[string]db.ListNodeURLsBySiteRow)
	for _, n := range allNodes {
		p := "/" + strings.Join(extractPathSegments(n.Url), "/")
		pathToNode[p] = n
	}

	// Collect candidates matching this merge target
	type candidate struct {
		id        uuid.UUID
		createdAt int64
	}
	var candidates []candidate
	for _, paramVal := range target.ParamValues {
		pathKey := target.Prefix + "/" + paramVal + target.Suffix
		if n, ok := pathToNode[pathKey]; ok {
			candidates = append(candidates, candidate{id: n.ID, createdAt: n.CreatedAt.UnixNano()})
		}
	}
	if len(candidates) < 2 {
		return 0, nil
	}

	// Oldest first → representative
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].createdAt < candidates[j].createdAt
	})
	representativeID := candidates[0].id
	var victimIDs []uuid.UUID
	for _, c := range candidates[1:] {
		victimIDs = append(victimIDs, c.id)
	}

	victimSet := make(map[uuid.UUID]bool, len(victimIDs))
	for _, v := range victimIDs {
		victimSet[v] = true
	}

	// Load all edges touching victims OR representative
	allRefIDs := append(append([]uuid.UUID{}, victimIDs...), representativeID)
	edges, err := q.ListEdgesReferencingNodes(ctx, allRefIDs)
	if err != nil {
		return 0, fmt.Errorf("list edges: %w", err)
	}

	// Compute edge state after relink
	type edgePair struct{ from, to uuid.UUID }
	seen := make(map[edgePair]bool)
	var toDelete []uuid.UUID
	var toInsert []edgePair

	for _, e := range edges {
		newFrom := e.FromNodeID
		newTo := e.ToNodeID
		if victimSet[newFrom] {
			newFrom = representativeID
		}
		if victimSet[newTo] {
			newTo = representativeID
		}

		// Always delete the original edge if it involves a victim
		if victimSet[e.FromNodeID] || victimSet[e.ToNodeID] {
			toDelete = append(toDelete, e.ID)

			// Re-insert if not self-loop and not duplicate
			if newFrom != newTo && !seen[edgePair{newFrom, newTo}] {
				toInsert = append(toInsert, edgePair{newFrom, newTo})
				seen[edgePair{newFrom, newTo}] = true
			}
		} else {
			// Edge only touches representative, keep it but track for dedup
			seen[edgePair{e.FromNodeID, e.ToNodeID}] = true
		}
	}

	// Delete victim edges
	if len(toDelete) > 0 {
		if err := q.DeleteEdgesByIDs(ctx, toDelete); err != nil {
			return 0, fmt.Errorf("delete edges: %w", err)
		}
	}

	// Insert relinked edges (ON CONFLICT DO NOTHING handles races)
	for _, ep := range toInsert {
		if err := q.CreateEdge(ctx, db.CreateEdgeParams{
			FromNodeID: ep.from,
			ToNodeID:   ep.to,
		}); err != nil {
			return 0, fmt.Errorf("create relinked edge: %w", err)
		}
	}

	// Delete victim nodes
	if err := q.DeleteNodesByIDs(ctx, victimIDs); err != nil {
		return 0, fmt.Errorf("delete victim nodes: %w", err)
	}

	// Extract domain from a matching node for full URL template
	var domainPrefix string
	for _, c := range candidates {
		for _, n := range allNodes {
			if n.ID == c.id {
				if u, err := url.Parse(n.Url); err == nil && u.Host != "" {
					domainPrefix = u.Scheme + "://" + u.Host
				}
				break
			}
		}
		if domainPrefix != "" {
			break
		}
	}

	// Update representative to template pattern
	templateURL := domainPrefix + target.Prefix + "/{param}" + target.Suffix
	h := md5.Sum([]byte(templateURL))
	templateHash := fmt.Sprintf("%x", h)
	if err := q.UpdateNodeURLAndHash(ctx, db.UpdateNodeURLAndHashParams{
		ID:      representativeID,
		Url:     templateURL,
		UrlHash: templateHash,
	}); err != nil {
		return 0, fmt.Errorf("update representative: %w", err)
	}

	return len(victimIDs), nil
}
