package instagram

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// PicukiScraper busca seguidores no Picuki
type PicukiScraper struct{}

func NewPicukiScraper() *PicukiScraper {
	return &PicukiScraper{}
}

func (p *PicukiScraper) Name() string {
	return "Picuki"
}

func (p *PicukiScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://www.picuki.com/profile/%s", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	followers := ""

	// Busca seguidores em classes específicas do Picuki
	doc.Find(".profile-stats, .followers, .stats").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	// Busca em meta tags
	if followers == "" {
		if meta, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
			followers = extractFollowersFromText(meta)
		}
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Picuki")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// DuckDuckGoFollowersScraper busca seguidores via DuckDuckGo
type DuckDuckGoFollowersScraper struct{}

func NewDuckDuckGoFollowersScraper() *DuckDuckGoFollowersScraper {
	return &DuckDuckGoFollowersScraper{}
}

func (d *DuckDuckGoFollowersScraper) Name() string {
	return "DuckDuckGo Search"
}

func (d *DuckDuckGoFollowersScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	// Busca específica por seguidores
	searchQuery := fmt.Sprintf("instagram @%s followers", handle)
	url := fmt.Sprintf("https://duckduckgo.com/html/?q=%s", searchQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	followers := ""

	// Busca nos snippets de resultados
	doc.Find(".result__snippet, .result-snippet").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	// Busca em qualquer texto que mencione o handle e seguidores
	if followers == "" {
		doc.Find("*").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			// Verifica se menciona o handle e tem padrão de seguidores
			if strings.Contains(strings.ToLower(text), handle) {
				if found := extractFollowersFromText(text); found != "" {
					followers = found
					return
				}
			}
		})
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no DuckDuckGo")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// InstagramDirectScraper tenta obter dados diretamente do Instagram (endpoint público)
type InstagramDirectScraper struct{}

func NewInstagramDirectScraper() *InstagramDirectScraper {
	return &InstagramDirectScraper{}
}

func (i *InstagramDirectScraper) Name() string {
	return "Instagram Direct"
}

func (i *InstagramDirectScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	// Tenta o endpoint público do Instagram
	url := fmt.Sprintf("https://www.instagram.com/%s/?__a=1&__d=dis", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-IG-App-ID", "936619743392459")

	time.Sleep(2 * time.Second)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Instagram pode bloquear, tenta via HTML
		return i.searchViaHTML(ctx, handle)
	}

	// Tenta extrair JSON (pode não funcionar mais)
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear: %w", err)
	}

	followers := ""

	// Busca por padrões no HTML/JSON
	text := doc.Text()
	if found := extractFollowersFromText(text); found != "" {
		followers = found
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Instagram")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

func (i *InstagramDirectScraper) searchViaHTML(ctx context.Context, handle string) (*Instagram, error) {
	url := fmt.Sprintf("https://www.instagram.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição HTML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code HTML: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	followers := ""

	// Busca em meta tags
	if meta, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		followers = extractFollowersFromText(meta)
	}

	// Busca no JavaScript embutido
	if followers == "" {
		doc.Find("script").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "edge_followed_by") {
				// Regex para extrair: "edge_followed_by":{"count":12345}
				re := regexp.MustCompile(`"edge_followed_by":\s*\{\s*"count":\s*(\d+)`)
				if matches := re.FindStringSubmatch(text); len(matches) >= 2 {
					followers = formatFollowerCount(matches[1])
					return
				}
			}
		})
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no HTML do Instagram")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// formatFollowerCount formata número de seguidores (1234 → 1.2K)
func formatFollowerCount(count string) string {
	// Remove não-dígitos
	re := regexp.MustCompile(`\D`)
	cleanCount := re.ReplaceAllString(count, "")

	if len(cleanCount) == 0 {
		return count
	}

	// Se já está formatado (tem K, M, B), retorna como está
	if strings.ContainsAny(count, "KMBkmb") {
		return count
	}

	// Converte para número e formata
	if len(cleanCount) >= 7 {
		// Milhões
		millions := cleanCount[:len(cleanCount)-6]
		rest := cleanCount[len(cleanCount)-6 : len(cleanCount)-5]
		return fmt.Sprintf("%s.%sM", millions, rest)
	} else if len(cleanCount) >= 4 {
		// Milhares
		thousands := cleanCount[:len(cleanCount)-3]
		rest := cleanCount[len(cleanCount)-3 : len(cleanCount)-2]
		if rest != "0" {
			return fmt.Sprintf("%s.%sK", thousands, rest)
		}
		return fmt.Sprintf("%sK", thousands)
	}

	return count
}
