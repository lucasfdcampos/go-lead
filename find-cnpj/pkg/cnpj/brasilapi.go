package cnpj

import (
"context"
"encoding/json"
"fmt"
"io"
"net/http"
"time"
)

type BrasilAPISearcher struct {
CNPJ string
}

func NewBrasilAPISearcher(cnpj string) *BrasilAPISearcher {
return &BrasilAPISearcher{CNPJ: cnpj}
}

func (b *BrasilAPISearcher) Name() string {
return "BrasilAPI"
}

func (b *BrasilAPISearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
if b.CNPJ == "" {
return nil, fmt.Errorf("CNPJ não fornecido para validação")
}

url := fmt.Sprintf("https://brasilapi.com.br/api/cnpj/v1/%s", b.CNPJ)
req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
if err != nil {
return nil, fmt.Errorf("erro ao criar requisição: %w", err)
}

client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Do(req)
if err != nil {
return nil, fmt.Errorf("erro ao fazer requisição: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
body, _ := io.ReadAll(resp.Body)
return nil, fmt.Errorf("API retornou status %d: %s", resp.StatusCode, string(body))
}

var result struct {
CNPJ string `json:"cnpj"`
}

if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
}

if result.CNPJ != "" {
return ExtractCNPJ(result.CNPJ), nil
}

return nil, fmt.Errorf("CNPJ não encontrado")
}

func ValidateCNPJ(ctx context.Context, cnpj string) (bool, error) {
searcher := NewBrasilAPISearcher(cnpj)
result, err := searcher.Search(ctx, "")
if err != nil {
return false, err
}
return result != nil, nil
}
