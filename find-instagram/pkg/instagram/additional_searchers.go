package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DuckDuckGoSearcher busca usando DuckDuckGo HTML
type DuckDuckGoSearcher struct{}

func NewDuckDuckGoSearcher() *DuckDuckGoSearcher {
	return &DuckDuckGoSearcher{}
}

func (d *DuckDuckGoSearcher) Name() string {
	return "DuckDuckGo Search"
}

func (d *DuckDuckGoSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	// Adiciona "instagram" à query se não estiver presente
	searchQuery := query
	if !strings.Contains(strings.ToLower(query), "instagram") {
		searchQuery = query + " instagram"
	}

	// Monta URL de busca
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(searchQuery))

	// Cria request com headers
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	// Delay para respeitar rate limit
	time.Sleep(1 * time.Second)

	// Faz request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Busca por handles no conteúdo
	var foundHandle *Instagram

	// Procura em links e textos
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if foundHandle != nil {
			return
		}

		href, exists := s.Attr("href")
		if exists {
			// Verifica se é link do Instagram
			if strings.Contains(href, "instagram.com/") {
				handles := ExtractAllHandles(href)
				if len(handles) > 0 {
					foundHandle = handles[0]
					return
				}
			}
		}

		// Verifica texto do link
		text := s.Text()
		if strings.HasPrefix(text, "@") {
			handles := ExtractAllHandles(text)
			if len(handles) > 0 {
				foundHandle = handles[0]
				return
			}
		}
	})

	// Busca no snippet de resultados
	if foundHandle == nil {
		doc.Find(".result__snippet").Each(func(i int, s *goquery.Selection) {
			if foundHandle != nil {
				return
			}

			text := s.Text()
			handles := ExtractAllHandles(text)
			if len(handles) > 0 {
				foundHandle = handles[0]
				return
			}
		})
	}

	// Busca em todo o corpo se não encontrou ainda
	if foundHandle == nil {
		bodyText := doc.Find("body").Text()
		handles := ExtractAllHandles(bodyText)
		if len(handles) > 0 {
			foundHandle = handles[0]
		}
	}

	if foundHandle == nil {
		return nil, fmt.Errorf("nenhum handle encontrado")
	}

	return foundHandle, nil
}

// GoogleSearcher busca usando Google HTML (sem API)
type GoogleSearcher struct{}

func NewGoogleSearcher() *GoogleSearcher {
	return &GoogleSearcher{}
}

func (g *GoogleSearcher) Name() string {
	return "Google Search"
}

func (g *GoogleSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	// Adiciona "instagram" à query
	searchQuery := query
	if !strings.Contains(strings.ToLower(query), "instagram") {
		searchQuery = query + " instagram"
	}

	// Monta URL de busca
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(searchQuery))

	// Cria request com headers
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	// Delay para respeitar rate limit
	time.Sleep(2 * time.Second)

	// Faz request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Busca por handles no conteúdo
	var foundHandle *Instagram

	// Procura em links
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if foundHandle != nil {
			return
		}

		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "instagram.com/") {
			handles := ExtractAllHandles(href)
			if len(handles) > 0 {
				foundHandle = handles[0]
				return
			}
		}
	})

	// Busca em todo o texto
	if foundHandle == nil {
		bodyText := doc.Find("body").Text()
		handles := ExtractAllHandles(bodyText)
		if len(handles) > 0 {
			foundHandle = handles[0]
		}
	}

	if foundHandle == nil {
		return nil, fmt.Errorf("nenhum handle encontrado")
	}

	return foundHandle, nil
}

// InstagramProfileChecker tenta adivinhar handles baseado no nome
type InstagramProfileChecker struct{}

func NewInstagramProfileChecker() *InstagramProfileChecker {
	return &InstagramProfileChecker{}
}

func (i *InstagramProfileChecker) Name() string {
	return "Instagram Profile Checker"
}

func (i *InstagramProfileChecker) Search(ctx context.Context, query string) (*Instagram, error) {
	// Gera possíveis handles baseado na query
	possibleHandles := i.generatePossibleHandles(query)

	// Tenta cada handle
	for _, handle := range possibleHandles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if i.checkProfileExists(ctx, handle) {
			return NewInstagram(handle), nil
		}

		// Delay entre verificações
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("nenhum handle válido encontrado")
}

func (i *InstagramProfileChecker) generatePossibleHandles(query string) []string {
	var handles []string
	seen := make(map[string]bool)

	// Normaliza query
	query = strings.ToLower(query)
	query = strings.TrimSpace(query)

	// Remove palavras comuns
	words := strings.Fields(query)
	var filtered []string
	stopWords := map[string]bool{
		"instagram": true, "ig": true, "perfil": true, "profile": true,
		"oficial": true, "official": true, "de": true, "da": true, "do": true,
	}

	for _, word := range words {
		if !stopWords[word] {
			filtered = append(filtered, word)
		}
	}

	if len(filtered) == 0 {
		return handles
	}

	// Estratégia 1: Tudo junto sem espaços
	handle1 := strings.Join(filtered, "")
	if IsValidHandle(handle1) && !seen[handle1] {
		handles = append(handles, handle1)
		seen[handle1] = true
	}

	// Estratégia 2: Com underscores
	handle2 := strings.Join(filtered, "_")
	if IsValidHandle(handle2) && !seen[handle2] {
		handles = append(handles, handle2)
		seen[handle2] = true
	}

	// Estratégia 3: Com pontos
	handle3 := strings.Join(filtered, ".")
	if IsValidHandle(handle3) && !seen[handle3] {
		handles = append(handles, handle3)
		seen[handle3] = true
	}

	// Estratégia 4: Apenas primeira palavra
	if len(filtered) > 0 {
		handle4 := filtered[0]
		if IsValidHandle(handle4) && !seen[handle4] {
			handles = append(handles, handle4)
			seen[handle4] = true
		}
	}

	// Estratégia 5: Primeiras duas palavras
	if len(filtered) >= 2 {
		handle5 := strings.Join(filtered[:2], "")
		if IsValidHandle(handle5) && !seen[handle5] {
			handles = append(handles, handle5)
			seen[handle5] = true
		}

		handle6 := strings.Join(filtered[:2], "_")
		if IsValidHandle(handle6) && !seen[handle6] {
			handles = append(handles, handle6)
			seen[handle6] = true
		}
	}

	return handles
}

func (i *InstagramProfileChecker) checkProfileExists(ctx context.Context, handle string) bool {
	// Usa a URL pública do Instagram para verificar se o perfil existe
	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "HEAD", profileURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Não segue redirects
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Se retornar 200 OK, o perfil existe
	return resp.StatusCode == 200
}

// BingSearcher busca usando Bing HTML
type BingSearcher struct{}

func NewBingSearcher() *BingSearcher {
	return &BingSearcher{}
}

func (b *BingSearcher) Name() string {
	return "Bing Search"
}

func (b *BingSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	searchQuery := query
	if !strings.Contains(strings.ToLower(query), "instagram") {
		searchQuery = query + " instagram"
	}

	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s", url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyText := string(body)
	handles := ExtractAllHandles(bodyText)
	if len(handles) > 0 {
		return handles[0], nil
	}

	return nil, fmt.Errorf("nenhum handle encontrado")
}
