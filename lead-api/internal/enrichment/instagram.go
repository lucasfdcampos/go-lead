package enrichment

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasfdcampos/lead-api/internal/cache"
	"github.com/lucasfdcampos/lead-api/internal/store"

	igpkg "github.com/lucasfdcampos/find-instagram/pkg/instagram"
)

// InstagramResult holds Instagram enrichment data for a single lead.
type InstagramResult struct {
	Handle    string // e.g. "dimazzomenswear"
	Formatted string // e.g. "@dimazzomenswear"
	Followers string // e.g. "1.2K"
}

// EnrichInstagram looks up Instagram data for a given lead name + city.
// Cache strategy:
//  1. Redis (L1)
//  2. MongoDB (L2)
//  3. Live search via find-instagram with DuckDuckGo+Bing fallback
func EnrichInstagram(
	ctx context.Context,
	name, city string,
	rdb *cache.Client,
	mdb *store.Client,
) (*InstagramResult, error) {
	cacheKey := cache.EnrichmentKey("ig:"+name, city)

	// L1 – Redis
	if rdb != nil {
		if cached, err := rdb.GetEnrichment(ctx, cacheKey); err == nil && cached != nil && cached.Instagram != "" {
			return &InstagramResult{
				Handle:    cached.Instagram,
				Formatted: "@" + cached.Instagram,
				Followers: cached.Followers,
			}, nil
		}
	}

	// L2 – MongoDB
	if mdb != nil {
		if cached, err := mdb.GetEnrichment(ctx, cacheKey); err == nil && cached != nil && cached.Instagram != "" {
			// Warm Redis
			if rdb != nil {
				_ = rdb.SetEnrichment(ctx, cacheKey, &cache.EnrichedLead{
					Instagram: cached.Instagram,
					Followers: cached.Followers,
				})
			}
			return &InstagramResult{
				Handle:    cached.Instagram,
				Formatted: "@" + cached.Instagram,
				Followers: cached.Followers,
			}, nil
		}
	}

	// Live search
	searchQuery := fmt.Sprintf("%s %s instagram", name, city)
	tctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	searchers := []igpkg.Searcher{
		igpkg.NewDuckDuckGoSearcher(),
		igpkg.NewBingSearcher(),
		igpkg.NewSearXNGSearcher(),
		igpkg.NewMojeekSearcher(),
		igpkg.NewSwisscowsSearcher(),
	}

	result := igpkg.SearchWithFallbackQuiet(tctx, searchQuery, searchers...)
	if result.Error != nil || result.Instagram == nil {
		return nil, fmt.Errorf("instagram não encontrado: %w", result.Error)
	}

	ig := result.Instagram

	// Try to get follower count via multi-scraper cascade (12 sources)
	if ig.Followers == "" {
		fCtx, fCancel := context.WithTimeout(ctx, 30*time.Second)
		defer fCancel()
		_ = igpkg.EnrichInstagramFollowers(fCtx, ig)
	}

	out := &InstagramResult{
		Handle:    ig.Handle,
		Formatted: ig.Formatted,
		Followers: ig.Followers,
	}

	// Persist to caches
	enriched := &cache.EnrichedLead{
		Instagram: out.Handle,
		Followers: out.Followers,
	}
	if rdb != nil {
		_ = rdb.SetEnrichment(ctx, cacheKey, enriched)
	}
	if mdb != nil {
		_ = mdb.SaveEnrichment(ctx, &store.CachedEnrichment{
			Key:       cacheKey,
			Instagram: out.Handle,
			Followers: out.Followers,
		})
	}

	return out, nil
}
