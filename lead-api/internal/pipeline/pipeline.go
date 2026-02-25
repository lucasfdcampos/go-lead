// Package pipeline orchestrates the complete lead discovery + enrichment flow.
//
// Phases:
//
//	0a. Redis cache (L1)        – return immediately on hit
//	0b. MongoDB cache (L2)      – return immediately on hit (hydrate leads from results collection)
//	0c. CNAE hint               – discover / load CNAE codes for the query
//	1.  Discovery               – run all find-leads scrapers via SearchAll
//	2.  Base leads              – build domain.Lead slice, name-relevance filter
//	3.  CNPJ enrichment         – concurrent pool (5 workers)
//	3b. Location + category     – post-CNPJ filters
//	4.  Instagram enrichment    – concurrent pool (4 workers)
//	5.  Build response
//	6.  Persist                 – save metadata → searches, leads → results
//	7.  Warm Redis
package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	leadsearch "github.com/lucasfdcampos/find-leads/pkg/leads"

	"github.com/lucasfdcampos/lead-api/internal/cache"
	"github.com/lucasfdcampos/lead-api/internal/cnae"
	"github.com/lucasfdcampos/lead-api/internal/domain"
	"github.com/lucasfdcampos/lead-api/internal/enrichment"
	"github.com/lucasfdcampos/lead-api/internal/filter"
	"github.com/lucasfdcampos/lead-api/internal/store"
)

const (
	cnpjWorkers      = 5
	instagramWorkers = 4
)

// Config holds injectable dependencies.
type Config struct {
	Redis *cache.Client
	Mongo *store.Client
}

// Run executes the full pipeline for a search request.
func Run(ctx context.Context, req domain.SearchRequest, cfg Config) (*domain.SearchResponse, error) {
	start := time.Now()

	// ── Phase 0a: Redis search cache (L1) ────────────────────────────────────
	var cacheKey string
	if cfg.Redis != nil {
		cacheKey = cache.SearchKey(req.Query, req.Location, req.EnrichCNPJ, req.EnrichInstagram)
		if raw, err := cfg.Redis.GetSearch(ctx, cacheKey); err == nil && len(raw) > 0 {
			var resp domain.SearchResponse
			if err := json.Unmarshal(raw, &resp); err == nil {
				resp.Cached = true
				return &resp, nil
			}
		}
	}

	// ── Phase 0b: MongoDB cache (L2) ─────────────────────────────────────────
	if cfg.Mongo != nil {
		stored, err := cfg.Mongo.FindSearch(ctx, req.Query, req.Location, req.EnrichCNPJ, req.EnrichInstagram)
		if err == nil && stored != nil {
			// Hydrate leads from results collection
			leads, _ := cfg.Mongo.FindResultsBySearchID(ctx, stored.ID)
			resp := &domain.SearchResponse{
				Query:         req.Query,
				Location:      req.Location,
				Total:         stored.Total,
				Discarded:     stored.Discarded,
				Cached:        true,
				SearchID:      stored.ID,
				CNAEHintCodes: stored.CNAEHintCodes,
				StartedAt:     stored.CreatedAt,
				DurationMs:    stored.DurationMs,
				Leads:         leads,
			}
			// Warm Redis L1
			if cfg.Redis != nil && cacheKey != "" {
				_ = cfg.Redis.SetSearch(ctx, cacheKey, resp)
			}
			return resp, nil
		}
	}

	// ── Phase 0c: CNAE hint discovery ────────────────────────────────────────
	var cnaeHintCodes []string
	if cfg.Mongo != nil {
		// Check MongoDB cache for CNAE hint
		hint, _ := cfg.Mongo.GetCNAEHint(ctx, req.Query)
		if hint != nil && len(hint.Codes) > 0 {
			cnaeHintCodes = hint.Codes
		} else {
			// Discover from DuckDuckGo + Mojeek
			discovered, snippet, _ := cnae.DiscoverFromSearch(ctx, req.Query)
			if len(discovered) > 0 {
				cnaeHintCodes = discovered
				_ = cfg.Mongo.SaveCNAEHint(ctx, &domain.CNAEHintDoc{
					Query:   req.Query,
					Codes:   discovered,
					Snippet: snippet,
				})
			}
		}
	}

	// ── Phase 1: Discovery ───────────────────────────────────────────────────
	city, state := leadsearch.ParseLocation(req.Location)
	rawLeads, _ := leadsearch.SearchAll(ctx, req.Query, req.Location, buildSearchers()...)

	// ── Phase 2: Build base domain leads ─────────────────────────────────────
	leads := make([]domain.Lead, 0, len(rawLeads))
	for _, rl := range rawLeads {
		if rl.Name == "" {
			continue
		}
		leads = append(leads, domain.Lead{
			Name:     rl.Name,
			Phone:    rl.Phone,
			Phone2:   rl.Phone2,
			Address:  rl.Address,
			City:     rl.City,
			State:    rl.State,
			Category: rl.Category,
			Website:  rl.Website,
			Email:    rl.Email,
			Source:   rl.Source,
		})
	}

	// ── Phase 2b: Name-relevance pre-filter (always-on) ──────────────────────
	var totalDiscarded int
	leads, disc0 := filter.ByNameRelevance(leads, req.Query)
	totalDiscarded += disc0

	// ── Phase 3: CNPJ enrichment ─────────────────────────────────────────────
	if req.EnrichCNPJ && len(leads) > 0 {
		leads = enrichCNPJConcurrent(ctx, leads, req.Query, city, state, cfg)

		// ── Phase 3b: Location + category post-filters ───────────────────────
		var compatibleCodes []string

		// Priority 1: discovered CNAE hints
		compatibleCodes = mergeUnique(compatibleCodes, cnaeHintCodes)

		// Priority 2: MongoDB CNAE reference
		if cfg.Mongo != nil {
			dbCodes, _ := cfg.Mongo.QueryCNAEs(ctx, extractKeywords(req.Query))
			compatibleCodes = mergeUnique(compatibleCodes, dbCodes)
		}

		// Priority 3: static map fallback
		if len(compatibleCodes) == 0 {
			compatibleCodes = cnae.StaticCompatibleCodes(req.Query)
		}

		var d1, d2 int
		leads, d1 = filter.ByLocation(leads, city, state)
		leads, d2 = filter.ByCategory(leads, compatibleCodes)
		totalDiscarded += d1 + d2
	}

	// ── Phase 4: Instagram enrichment ────────────────────────────────────────
	if req.EnrichInstagram && len(leads) > 0 {
		leads = enrichInstagramConcurrent(ctx, leads, city, cfg)
	}

	// ── Phase 5: Build response ───────────────────────────────────────────────
	resp := &domain.SearchResponse{
		Query:         req.Query,
		Location:      req.Location,
		Total:         len(leads),
		Discarded:     totalDiscarded,
		Cached:        false,
		CNAEHintCodes: cnaeHintCodes,
		StartedAt:     start,
		DurationMs:    time.Since(start).Milliseconds(),
		Leads:         leads,
	}

	// ── Phase 6: Persist to MongoDB ───────────────────────────────────────────
	if cfg.Mongo != nil {
		doc := &domain.StoredSearch{
			Query:           req.Query,
			Location:        req.Location,
			EnrichCNPJ:      req.EnrichCNPJ,
			EnrichInstagram: req.EnrichInstagram,
			Total:           resp.Total,
			Discarded:       resp.Discarded,
			DurationMs:      resp.DurationMs,
			CNAEHintCodes:   cnaeHintCodes,
		}
		if id, err := cfg.Mongo.SaveSearch(ctx, doc); err == nil {
			resp.SearchID = id
			// Save individual results linked to search_id
			_ = cfg.Mongo.SaveResults(ctx, id, leads)
		}
	}

	// ── Phase 7: Cache in Redis ────────────────────────────────────────────────
	if cfg.Redis != nil && cacheKey != "" {
		_ = cfg.Redis.SetSearch(ctx, cacheKey, resp)
	}

	return resp, nil
}

// ─── CNPJ concurrent enrichment ───────────────────────────────────────────────

func enrichCNPJConcurrent(
	ctx context.Context,
	leads []domain.Lead,
	query, city, state string,
	cfg Config,
) []domain.Lead {
	sem := make(chan struct{}, cnpjWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup

	enriched := make([]domain.Lead, len(leads))
	copy(enriched, leads)

	t, f := true, false

	for i := range enriched {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := enrichment.EnrichCNPJ(ctx, enriched[idx].Name, city, state, query, cfg.Redis, cfg.Mongo)
			if err != nil {
				return
			}

			mu.Lock()
			enriched[idx].CNPJ = res.CNPJ
			enriched[idx].RazaoSocial = res.RazaoSocial
			enriched[idx].NomeFantasia = res.NomeFantasia
			enriched[idx].Situacao = res.Situacao
			enriched[idx].Partners = res.Partners
			enriched[idx].CNAEDesc = res.CNAEDesc
			enriched[idx].Municipio = res.Municipio
			enriched[idx].UF = res.UF
			if res.CNAEMatch {
				enriched[idx].CNAEMatch = &t
			} else {
				enriched[idx].CNAEMatch = &f
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return enriched
}

// ─── Instagram concurrent enrichment ──────────────────────────────────────────

func enrichInstagramConcurrent(
	ctx context.Context,
	leads []domain.Lead,
	city string,
	cfg Config,
) []domain.Lead {
	sem := make(chan struct{}, instagramWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup

	enriched := make([]domain.Lead, len(leads))
	copy(enriched, leads)

	for i := range enriched {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := enrichment.EnrichInstagram(ctx, enriched[idx].Name, city, cfg.Redis, cfg.Mongo)
			if err != nil {
				return
			}

			mu.Lock()
			enriched[idx].Instagram = res.Formatted
			enriched[idx].Followers = res.Followers
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return enriched
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mergeUnique appends elements from src to dst, skipping duplicates.
func mergeUnique(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range src {
		if !seen[v] {
			seen[v] = true
			dst = append(dst, v)
		}
	}
	return dst
}

// extractKeywords splits a query into significant keywords for CNAE lookup.
func extractKeywords(query string) []string {
	stopwords := map[string]bool{
		"de": true, "do": true, "da": true, "e": true, "em": true,
		"a": true, "o": true, "as": true, "os": true,
	}
	var kws []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if len(w) >= 3 && !stopwords[w] {
			kws = append(kws, w)
		}
	}
	return kws
}

// ─── Scraper wiring ───────────────────────────────────────────────────────────

func buildSearchers() []leadsearch.Searcher {
	s := []leadsearch.Searcher{
		leadsearch.NewOverpassScraper(),
		leadsearch.NewSolutudoScraper(),
		leadsearch.NewGuiaMaisScraper(),
		leadsearch.NewAppLocalScraper(),
		leadsearch.NewApontadorScraper(),
		leadsearch.NewTeleListasScraper(),
		leadsearch.NewDDGLeadScraper(),
		leadsearch.NewBingLeadScraper(),
		leadsearch.NewBraveLeadScraper(),
		leadsearch.NewYandexLeadScraper(),
		// Novos: SearXNG, Mojeek, Swisscows como fallback
		leadsearch.NewSearXNGLeadScraper(),
		leadsearch.NewMojeekLeadScraper(),
		leadsearch.NewSwisscowsLeadScraper(),
	}
	if key := os.Getenv("GEOAPIFY_API_KEY"); key != "" {
		s = append(s, leadsearch.NewGeoapifyScraper(key))
	}
	if key := os.Getenv("TOMTOM_API_KEY"); key != "" {
		s = append(s, leadsearch.NewTomTomScraper(key))
	}
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		s = append(s, leadsearch.NewGroqScraper(key))
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		s = append(s, leadsearch.NewGeminiScraper(key))
	}
	return s
}
