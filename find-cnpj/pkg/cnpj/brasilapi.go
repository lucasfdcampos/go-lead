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
		CNPJ         string `json:"cnpj"`
		RazaoSocial  string `json:"razao_social"`
		NomeFantasia string `json:"nome_fantasia"`
		DDD          string `json:"ddd_telefone_1"`
		Telefone     string `json:"telefone_1"`
		QSA          []struct {
			Nome string `json:"nome_socio"`
		} `json:"qsa"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if result.CNPJ == "" {
		return nil, fmt.Errorf("CNPJ não encontrado")
	}

	cnpjObj := ExtractCNPJ(result.CNPJ)
	if cnpjObj == nil {
		return nil, fmt.Errorf("CNPJ inválido")
	}

	// Adiciona informações extras
	cnpjObj.RazaoSocial = result.RazaoSocial
	cnpjObj.NomeFantasia = result.NomeFantasia

	// Adiciona telefone se disponível
	if result.DDD != "" && result.Telefone != "" {
		telefone := fmt.Sprintf("(%s) %s", result.DDD, result.Telefone)
		cnpjObj.Telefones = append(cnpjObj.Telefones, telefone)
	}

	// Adiciona sócios
	for _, socio := range result.QSA {
		if socio.Nome != "" {
			cnpjObj.Socios = append(cnpjObj.Socios, socio.Nome)
		}
	}

	return cnpjObj, nil
}

// EnrichCNPJData busca dados adicionais de um CNPJ já encontrado
// Primeiro tenta BrasilAPI, se falhar usa cnpj.biz como fallback
func EnrichCNPJData(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	// Tenta primeiro BrasilAPI
	searcher := NewBrasilAPISearcher(cnpj.Number)
	enriched, err := searcher.Search(ctx, "")
	if err == nil {
		// Atualiza dados da BrasilAPI
		if enriched.RazaoSocial != "" {
			cnpj.RazaoSocial = enriched.RazaoSocial
		}
		if enriched.NomeFantasia != "" {
			cnpj.NomeFantasia = enriched.NomeFantasia
		}
		if len(enriched.Telefones) > 0 {
			cnpj.Telefones = enriched.Telefones
		}
		if len(enriched.Socios) > 0 {
			cnpj.Socios = enriched.Socios
		}
		return nil
	}

	// Se BrasilAPI falhar, tenta cnpj.biz como fallback
	fmt.Printf("⚠️  BrasilAPI falhou (%v), tentando cnpj.biz...\n", err)
	if err := EnrichCNPJFromCNPJBiz(ctx, cnpj); err != nil {
		return fmt.Errorf("falhou BrasilAPI e cnpj.biz: %w", err)
	}

	return nil
}

func ValidateCNPJ(ctx context.Context, cnpj string) (bool, error) {
	searcher := NewBrasilAPISearcher(cnpj)
	result, err := searcher.Search(ctx, "")
	if err != nil {
		return false, err
	}
	return result != nil, nil
}
