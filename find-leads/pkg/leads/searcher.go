package leads

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Searcher interface que todas as fontes devem implementar
type Searcher interface {
	Name() string
	Search(ctx context.Context, query, location string) ([]*Lead, error)
}

// SearchResult resultado de uma fonte
type SearchResult struct {
	Source string
	Leads  []*Lead
	Err    error
	Took   time.Duration
}

// ParseLocation divide "Arapongas-PR" em cidade e estado
func ParseLocation(location string) (city, state string) {
	parts := strings.SplitN(location, "-", 2)
	city = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		state = strings.ToUpper(strings.TrimSpace(parts[1]))
	}
	return
}

// CitySlug converte nome de cidade para URL slug
func CitySlug(city string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s := normalizeString(city)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// QuerySlug converte query para URL slug
func QuerySlug(q string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s := normalizeString(q)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// SearchAll executa todas as fontes e retorna leads deduplicados
func SearchAll(ctx context.Context, query, location string, searchers ...Searcher) ([]*Lead, []SearchResult) {
	var allLeads []*Lead
	var results []SearchResult

	for _, s := range searchers {
		if ctx.Err() != nil {
			break
		}

		start := time.Now()
		fmt.Printf("  🔍 [%-30s] buscando...\r", s.Name())

		leadsFound, err := s.Search(ctx, query, location)
		took := time.Since(start)

		r := SearchResult{
			Source: s.Name(),
			Leads:  leadsFound,
			Err:    err,
			Took:   took,
		}
		results = append(results, r)

		if err != nil {
			fmt.Printf("  ❌ [%-30s] erro: %v\n", s.Name(), err)
		} else {
			fmt.Printf("  ✅ [%-30s] %d leads (%v)\n", s.Name(), len(leadsFound), took.Round(time.Millisecond))
			allLeads = append(allLeads, leadsFound...)
		}
	}

	deduplicated := Deduplicate(allLeads)
	return deduplicated, results
}
