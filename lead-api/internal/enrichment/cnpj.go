// Package enrichment provides per-lead CNPJ and Instagram enrichment logic.
package enrichment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lucasfdcampos/lead-api/internal/cache"
	"github.com/lucasfdcampos/lead-api/internal/cnae"
	"github.com/lucasfdcampos/lead-api/internal/store"

	cnpjpkg "github.com/lucasfdcampos/find-cnpj/pkg/cnpj"
)

// CNPJResult holds CNPJ enrichment data for a single lead.
type CNPJResult struct {
	CNPJ      string
	Partners  []string
	CNAECode  string
	CNAEDesc  string
	CNAEMatch bool
	Municipio string
	UF        string
}

// EnrichCNPJ looks up and enriches CNPJ data for a given lead name + city.
// Cache strategy:
//  1. Redis (L1)
//  2. MongoDB (L2)
//  3. Live search via find-cnpj with fallback chain
func EnrichCNPJ(
	ctx context.Context,
	name, city, state, query string,
	rdb *cache.Client,
	mdb *store.Client,
) (*CNPJResult, error) {
	cacheKey := cache.EnrichmentKey(name, city)

	// L1 – Redis
	if rdb != nil {
		if cached, err := rdb.GetEnrichment(ctx, cacheKey); err == nil && cached != nil && cached.CNPJ != "" {
			match := cnae.IsCompatible(query, cached.CNAECode)
			return &CNPJResult{
				CNPJ:      cached.CNPJ,
				Partners:  cached.Partners,
				CNAECode:  cached.CNAECode,
				CNAEDesc:  cached.CNAEDesc,
				CNAEMatch: match,
				Municipio: cached.Municipio,
				UF:        cached.UF,
			}, nil
		}
	}

	// L2 – MongoDB
	if mdb != nil {
		if cached, err := mdb.GetEnrichment(ctx, cacheKey); err == nil && cached != nil && cached.CNPJ != "" {
			match := cnae.IsCompatible(query, cached.CNAECode)
			// Warm Redis
			if rdb != nil {
				_ = rdb.SetEnrichment(ctx, cacheKey, &cache.EnrichedLead{
					CNPJ:      cached.CNPJ,
					Partners:  cached.Partners,
					CNAECode:  cached.CNAECode,
					CNAEDesc:  cached.CNAEDesc,
					Municipio: cached.Municipio,
					UF:        cached.UF,
				})
			}
			return &CNPJResult{
				CNPJ:      cached.CNPJ,
				Partners:  cached.Partners,
				CNAECode:  cached.CNAECode,
				CNAEDesc:  cached.CNAEDesc,
				CNAEMatch: match,
				Municipio: cached.Municipio,
				UF:        cached.UF,
			}, nil
		}
	}

	// Live search
	searchQuery := fmt.Sprintf("%s %s %s cnpj", name, city, state)
	tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	searchers := []cnpjpkg.Searcher{
		cnpjpkg.NewDuckDuckGoSearcher(),
		cnpjpkg.NewSearXNGSearcher(),
		cnpjpkg.NewMojeekSearcher(),
		cnpjpkg.NewSwisscowsSearcher(),
		cnpjpkg.NewCNPJSearcher(),
	}

	result := cnpjpkg.SearchWithFallbackQuiet(tctx, searchQuery, searchers...)
	if result.Error != nil || result.CNPJ == nil {
		return nil, fmt.Errorf("cnpj não encontrado: %w", result.Error)
	}

	// Enrich full data (BrasilAPI → ReceitaWS → ...)
	eCtx, eCancel := context.WithTimeout(ctx, 30*time.Second)
	defer eCancel()
	_ = cnpjpkg.EnrichCNPJData(eCtx, result.CNPJ)

	cnaeCode := strings.TrimSpace(result.CNPJ.CNAE)

	out := &CNPJResult{
		CNPJ:      result.CNPJ.Formatted,
		Partners:  result.CNPJ.Socios,
		CNAECode:  cnaeCode,
		CNAEDesc:  result.CNPJ.CNAEDesc,
		CNAEMatch: cnae.IsCompatible(query, cnaeCode),
		Municipio: result.CNPJ.Municipio,
		UF:        result.CNPJ.UF,
	}

	// Persist to caches
	enriched := &cache.EnrichedLead{
		CNPJ:      out.CNPJ,
		Partners:  out.Partners,
		CNAECode:  out.CNAECode,
		CNAEDesc:  out.CNAEDesc,
		Municipio: out.Municipio,
		UF:        out.UF,
	}
	if rdb != nil {
		_ = rdb.SetEnrichment(ctx, cacheKey, enriched)
	}
	if mdb != nil {
		_ = mdb.SaveEnrichment(ctx, &store.CachedEnrichment{
			Key:       cacheKey,
			CNPJ:      enriched.CNPJ,
			Partners:  enriched.Partners,
			CNAECode:  enriched.CNAECode,
			CNAEDesc:  enriched.CNAEDesc,
			Municipio: enriched.Municipio,
			UF:        enriched.UF,
		})
	}

	return out, nil
}
