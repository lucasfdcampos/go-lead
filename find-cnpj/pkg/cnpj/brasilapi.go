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
		Situacao     string `json:"descricao_situacao_cadastral"`
		DDD          string `json:"ddd_telefone_1"`
		Telefone     string `json:"telefone_1"`
		CNAEFiscal   int    `json:"cnae_fiscal"`
		CNAEDesc     string `json:"cnae_fiscal_descricao"`
		Municipio    string `json:"municipio"`
		UF           string `json:"uf"`
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
	cnpjObj.Situacao = result.Situacao

	// Adiciona CNAE se disponível
	if result.CNAEFiscal > 0 {
		cnpjObj.CNAE = fmt.Sprintf("%07d", result.CNAEFiscal) // Formata com 7 dígitos
		cnpjObj.CNAEDesc = result.CNAEDesc
	}

	// Adiciona município e UF
	cnpjObj.Municipio = result.Municipio
	cnpjObj.UF = result.UF

	// Adiciona sócios
	for _, socio := range result.QSA {
		if socio.Nome != "" {
			cnpjObj.Socios = append(cnpjObj.Socios, socio.Nome)
		}
	}

	return cnpjObj, nil
}

// EnrichCNPJData busca dados adicionais de um CNPJ já encontrado
// Sistema de fallback em cascata: BrasilAPI → ReceitaWS → cnpj.biz → Serasa Experian → DuckDuckGo → Bing → Brave → Yandex
func EnrichCNPJData(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	// Helper para verificar se dados estão completos
	isComplete := func() bool {
		return cnpj.RazaoSocial != "" && len(cnpj.Socios) > 0
	}

	// 1. Tenta BrasilAPI primeiro (oficial e rápida)
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
		if enriched.Situacao != "" {
			cnpj.Situacao = enriched.Situacao
		}
		if len(enriched.Telefones) > 0 {
			cnpj.Telefones = enriched.Telefones
		}
		if len(enriched.Socios) > 0 {
			cnpj.Socios = enriched.Socios
		}
		if enriched.CNAE != "" {
			cnpj.CNAE = enriched.CNAE
			cnpj.CNAEDesc = enriched.CNAEDesc
		}

		// Se já temos dados completos, retorna
		if isComplete() {
			return nil
		}
	}

	// 2. Se BrasilAPI falhou ou dados incompletos, tenta ReceitaWS
	if err != nil || !isComplete() {
		if err != nil {
			fmt.Printf("⚠️  BrasilAPI falhou (%v), tentando ReceitaWS...\n", err)
		} else {
			fmt.Printf("⚠️  BrasilAPI com dados incompletos, tentando ReceitaWS...\n")
		}

		errReceitaWS := EnrichFromReceitaWS(ctx, cnpj)
		if errReceitaWS == nil && isComplete() {
			return nil
		}

		if errReceitaWS != nil {
			fmt.Printf("⚠️  ReceitaWS falhou (%v), tentando cnpj.biz...\n", errReceitaWS)
		}
	}

	// 3. Tenta cnpj.biz (scraping)
	if !isComplete() {
		errCnpjBiz := EnrichCNPJFromCNPJBiz(ctx, cnpj)
		if errCnpjBiz == nil && isComplete() {
			return nil
		}

		if errCnpjBiz != nil {
			fmt.Printf("⚠️  cnpj.biz falhou (%v), tentando Serasa Experian...\n", errCnpjBiz)
		}
	}

	// 4. Última tentativa tradicional: Serasa Experian (scraping complexo)
	if !isComplete() {
		errSerasa := EnrichFromSerasaExperian(ctx, cnpj)
		if errSerasa == nil && isComplete() {
			return nil
		}

		if errSerasa != nil {
			fmt.Printf("⚠️  Serasa Experian falhou (%v), tentando DuckDuckGo Search...\n", errSerasa)
		}
	}

	// 5. Fallback para DuckDuckGo (busca por snippets)
	if !isComplete() {
		errDDG := EnrichFromDuckDuckGo(ctx, cnpj)
		if errDDG == nil && isComplete() {
			fmt.Printf("✅ Sucesso com fallback DuckDuckGo\n")
			return nil
		}

		if errDDG != nil {
			fmt.Printf("⚠️  DuckDuckGo falhou (%v), tentando Bing Search...\n", errDDG)
		}
	}

	// 6. Fallback para Bing Search
	if !isComplete() {
		errBing := EnrichFromBing(ctx, cnpj)
		if errBing == nil && isComplete() {
			fmt.Printf("✅ Sucesso com fallback Bing\n")
			return nil
		}

		if errBing != nil {
			fmt.Printf("⚠️  Bing falhou (%v), tentando Brave Search...\n", errBing)
		}
	}

	// 7. Fallback para Brave Search
	if !isComplete() {
		errBrave := EnrichFromBrave(ctx, cnpj)
		if errBrave == nil && isComplete() {
			fmt.Printf("✅ Sucesso com fallback Brave\n")
			return nil
		}

		if errBrave != nil {
			fmt.Printf("⚠️  Brave falhou (%v), tentando Yandex Search...\n", errBrave)
		}
	}

	// 8. Fallback final: Yandex Search
	if !isComplete() {
		errYandex := EnrichFromYandex(ctx, cnpj)
		if errYandex == nil && isComplete() {
			fmt.Printf("✅ Sucesso com fallback Yandex\n")
			return nil
		}
	}

	// Se nenhum funcionou completamente mas tem algo
	if cnpj.RazaoSocial != "" || len(cnpj.Socios) > 0 || len(cnpj.Telefones) > 0 {
		return nil // Retorna sucesso parcial
	}

	return fmt.Errorf("todas as 8 fontes falharam")
}

func ValidateCNPJ(ctx context.Context, cnpj string) (bool, error) {
	searcher := NewBrasilAPISearcher(cnpj)
	result, err := searcher.Search(ctx, "")
	if err != nil {
		return false, err
	}
	return result != nil, nil
}
