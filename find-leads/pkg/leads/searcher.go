package leads

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
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
	location = strings.TrimSpace(location)

	// Normalise separators: "Arapongas - PR", "Arapongas-PR", "Arapongas, PR", "Arapongas,PR"
	// Replace any combination of spaces/commas/dashes around a 2-letter state code.
	// Strategy: split on comma or dash (allowing surrounding spaces), then check last token.
	re := regexp.MustCompile(`[\s,\-]+`)
	tokens := re.Split(location, -1)

	if len(tokens) >= 2 {
		last := strings.ToUpper(tokens[len(tokens)-1])
		if len(last) == 2 && last >= "AA" && last <= "ZZ" {
			state = last
			city = strings.Join(tokens[:len(tokens)-1], " ")
			return
		}
	}

	// Fallback: entire string is the city
	city = location
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

// SearchAll executa todas as fontes concorrentemente (máx 5 simultâneas) e retorna leads deduplicados
func SearchAll(ctx context.Context, query, location string, searchers ...Searcher) ([]*Lead, []SearchResult) {
	const maxConcurrent = 5

	results := make([]SearchResult, len(searchers))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex // protege fmt.Printf (evita linhas entrelaçadas)

	for i, s := range searchers {
		wg.Add(1)
		go func(idx int, src Searcher) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				results[idx] = SearchResult{Source: src.Name(), Err: ctx.Err()}
				return
			}

			mu.Lock()
			fmt.Printf("  🔍 [%-30s] buscando...\n", src.Name())
			mu.Unlock()

			start := time.Now()
			leads, err := src.Search(ctx, query, location)
			took := time.Since(start)

			results[idx] = SearchResult{
				Source: src.Name(),
				Leads:  leads,
				Err:    err,
				Took:   took,
			}

			mu.Lock()
			if err != nil {
				fmt.Printf("  ❌ [%-30s] erro: %v\n", src.Name(), err)
			} else {
				fmt.Printf("  ✅ [%-30s] %d leads (%v)\n", src.Name(), len(leads), took.Round(time.Millisecond))
			}
			mu.Unlock()
		}(i, s)
	}

	wg.Wait()

	var allLeads []*Lead
	for _, r := range results {
		if r.Err == nil {
			allLeads = append(allLeads, r.Leads...)
		}
	}

	deduplicated := Deduplicate(allLeads)
	return deduplicated, results
}
