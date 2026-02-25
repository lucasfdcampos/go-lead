package leads

import (
	"context"
	"fmt"
	"time"

	igpkg "github.com/lucasfdcampos/find-instagram/pkg/instagram"
)

// EnrichInstagram busca o perfil do Instagram para um lead pelo nome + cidade
// e preenche os campos Instagram (handle formatado) e Followers.
func EnrichInstagram(ctx context.Context, lead *Lead) error {
	query := lead.Name
	if lead.City != "" {
		query = fmt.Sprintf("%s %s", lead.Name, lead.City)
	}

	tctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// ProfileChecker primeiro: mais preciso (scruta og:title + follower count)
	// DuckDuckGo como fallback: útil quando o perfil não está no top handles
	searchers := []igpkg.Searcher{
		igpkg.NewInstagramProfileChecker(),
		igpkg.NewDuckDuckGoSearcher(),
	}

	result := igpkg.SearchWithFallbackQuiet(tctx, query, searchers...)
	if result.Error != nil || result.Instagram == nil {
		return fmt.Errorf("instagram não encontrado para %q: %w", lead.Name, result.Error)
	}

	lead.Instagram = result.Instagram.Formatted // @handle
	lead.Followers = result.Instagram.Followers

	return nil
}
