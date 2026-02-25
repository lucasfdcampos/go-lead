package leads

import (
	"context"
	"fmt"
	"strings"
	"time"

	cnpjpkg "github.com/lucasfdcampos/find-cnpj/pkg/cnpj"
)

// EnrichCNPJ busca dados de CNPJ para um lead pelo nome + cidade/estado
// e preenche os campos RazaoSocial, NomeFantasia, Situacao, CNAE, Partners, etc.
func EnrichCNPJ(ctx context.Context, lead *Lead) error {
	query := fmt.Sprintf("%s %s %s cnpj", lead.Name, lead.City, lead.State)

	tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	searchers := []cnpjpkg.Searcher{
		cnpjpkg.NewDuckDuckGoSearcher(),
		cnpjpkg.NewSearXNGSearcher(),
		cnpjpkg.NewMojeekSearcher(),
		cnpjpkg.NewSwisscowsSearcher(),
		cnpjpkg.NewCNPJSearcher(),
	}

	result := cnpjpkg.SearchWithFallbackQuiet(tctx, query, searchers...)
	if result.Error != nil || result.CNPJ == nil {
		return fmt.Errorf("cnpj não encontrado para %q: %w", lead.Name, result.Error)
	}

	// Enriquece via BrasilAPI → ReceitaWS → fallback chain
	eCtx, eCancel := context.WithTimeout(ctx, 30*time.Second)
	defer eCancel()
	_ = cnpjpkg.EnrichCNPJData(eCtx, result.CNPJ)

	c := result.CNPJ
	lead.CNPJ = c.Formatted
	lead.RazaoSocial = c.RazaoSocial
	lead.NomeFantasia = c.NomeFantasia
	lead.Situacao = c.Situacao
	lead.CNAECode = strings.TrimSpace(c.CNAE)
	lead.CNAEDesc = c.CNAEDesc
	lead.Municipio = c.Municipio
	lead.UF = c.UF
	lead.Partners = c.Socios

	return nil
}
