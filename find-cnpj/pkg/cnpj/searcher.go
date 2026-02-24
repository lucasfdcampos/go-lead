package cnpj

import (
"context"
"fmt"
"time"
)

// SearchResult representa o resultado de uma busca
type SearchResult struct {
CNPJ     *CNPJ
Source   string // Fonte da informação
Query    string // Query utilizada
Duration time.Duration
Error    error
}

// Searcher interface para diferentes estratégias de busca
type Searcher interface {
Search(ctx context.Context, query string) (*CNPJ, error)
Name() string
}

// SearchWithFallback busca CNPJ usando múltiplas estratégias com fallback
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
cnpj, err := searcher.Search(searchCtx, query)
cancel()

if err != nil {
if verbose {
fmt.Printf("   ❌ Falhou: %v\n", err)
}
// Pequeno delay entre estratégias para evitar sobrecarga
time.Sleep(500 * time.Millisecond)
continue
}

if cnpj != nil {
if verbose {
fmt.Printf("   ✅ Sucesso!\n")
}
return &SearchResult{
CNPJ:     cnpj,
Source:   searcher.Name(),
Query:    query,
Duration: time.Since(startTime),
Error:    nil,
}
}
}

return &SearchResult{
CNPJ:     nil,
Source:   "none",
Query:    query,
Duration: time.Since(startTime),
Error:    fmt.Errorf("nenhuma estratégia conseguiu encontrar o CNPJ"),
}
}
