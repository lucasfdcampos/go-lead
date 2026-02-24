package instagram

import (
	"context"
	"fmt"
	"time"
)

// SearchResult representa o resultado de uma busca
type SearchResult struct {
	Instagram *Instagram
	Source    string // Fonte da informação
	Query     string // Query utilizada
	Duration  time.Duration
	Error     error
}

// Searcher interface para diferentes estratégias de busca
type Searcher interface {
	Search(ctx context.Context, query string) (*Instagram, error)
	Name() string
}

// SearchWithFallback busca Instagram usando múltiplas estratégias com fallback
func SearchWithFallback(ctx context.Context, query string, searchers ...Searcher) *SearchResult {
	return searchWithFallback(ctx, query, true, searchers...)
}

// SearchWithFallbackQuiet busca sem imprimir mensagens (para listas)
func SearchWithFallbackQuiet(ctx context.Context, query string, searchers ...Searcher) *SearchResult {
	return searchWithFallback(ctx, query, false, searchers...)
}

func searchWithFallback(ctx context.Context, query string, verbose bool, searchers ...Searcher) *SearchResult {
	query = NormalizarQuery(query)
	startTime := time.Now()

	for _, searcher := range searchers {
		if verbose {
			fmt.Printf("🔍 Tentando estratégia: %s\n", searcher.Name())
		}

		// Criar contexto com timeout por estratégia
		searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		instagram, err := searcher.Search(searchCtx, query)
		cancel()

		if err != nil {
			if verbose {
				fmt.Printf("   ❌ Falhou: %v\n", err)
			}
			// Pequeno delay entre estratégias para evitar sobrecarga
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if instagram != nil {
			if verbose {
				fmt.Printf("   ✅ Sucesso!\n")
			}
			return &SearchResult{
				Instagram: instagram,
				Source:    searcher.Name(),
				Query:     query,
				Duration:  time.Since(startTime),
				Error:     nil,
			}
		}
	}

	return &SearchResult{
		Instagram: nil,
		Source:    "none",
		Query:     query,
		Duration:  time.Since(startTime),
		Error:     fmt.Errorf("nenhuma estratégia conseguiu encontrar o Instagram"),
	}
}
