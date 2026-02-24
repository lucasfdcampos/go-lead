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

// InstaStoriesViewerScraper busca seguidores no insta-stories-viewer.com
type InstaStoriesViewerScraper struct{}

func NewInstaStoriesViewerScraper() *InstaStoriesViewerScraper {
	return &InstaStoriesViewerScraper{}
}

func (i *InstaStoriesViewerScraper) Name() string {
	return "InstaStoriesViewer"
}

func (i *InstaStoriesViewerScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	// Extrai handle do Instagram
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://insta-stories-viewer.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	// Delay para respeitar rate limit
	time.Sleep(1 * time.Second)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Permite até 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("muitos redirects")
			}
			return nil
		},
	}

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

	// Busca por seguidores em diferentes padrões
	// Padrão 1: meta tags
	if meta, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		followers = extractFollowersFromText(meta)
	}

	// Padrão 2: texto visível com "followers"
	if followers == "" {
		doc.Find("*").Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(s.Text())
			if strings.Contains(text, "follower") || strings.Contains(text, "seguidores") {
				followers = extractFollowersFromText(s.Text())
				if followers != "" {
					return
				}
			}
		})
	}

	// Padrão 3: classes comuns de estatísticas
	if followers == "" {
		selectors := []string{
			".followers", ".follower-count", ".user-followers",
			"[class*='follower']", "[class*='stats']",
			".statistics", ".profile-stats",
		}
		for _, selector := range selectors {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := s.Text()
				if found := extractFollowersFromText(text); found != "" {
					followers = found
					return
				}
			})
			if followers != "" {
				break
			}
		}
	}

	// Padrão 4: JSON-LD
	if followers == "" {
		doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if found := extractFollowersFromText(text); found != "" {
				followers = found
				return
			}
		})
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// StoryNavigationScraper busca seguidores no storynavigation.com (fallback)
type StoryNavigationScraper struct{}

func NewStoryNavigationScraper() *StoryNavigationScraper {
	return &StoryNavigationScraper{}
}

func (s *StoryNavigationScraper) Name() string {
	return "StoryNavigation"
}

func (s *StoryNavigationScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://storynavigation.com/user/%s", handle)

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

	// Busca similar ao InstaStoriesViewer
	// Meta tags
	if meta, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		followers = extractFollowersFromText(meta)
	}

	// Texto visível
	if followers == "" {
		doc.Find("*").Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(s.Text())
			if strings.Contains(text, "follower") || strings.Contains(text, "seguidores") {
				followers = extractFollowersFromText(s.Text())
				if followers != "" {
					return
				}
			}
		})
	}

	// Classes de estatísticas
	if followers == "" {
		selectors := []string{
			".followers", ".follower-count", ".user-followers",
			"[class*='follower']", "[class*='stats']",
			".user-info", ".profile-info",
		}
		for _, selector := range selectors {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := s.Text()
				if found := extractFollowersFromText(text); found != "" {
					followers = found
					return
				}
			})
			if followers != "" {
				break
			}
		}
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// extractFollowersFromText extrai número de seguidores de um texto
// Suporta formatos: "1.2K", "15.3M", "523", "1,234", etc.
func extractFollowersFromText(text string) string {
	// Padrão 1: número seguido de K/M/B (ex: 1.2K, 15.3M)
	re1 := regexp.MustCompile(`([\d,\.]+)\s*([KMBkmb])\s*[Ff]ollowers?`)
	if matches := re1.FindStringSubmatch(text); len(matches) >= 3 {
		return matches[1] + strings.ToUpper(matches[2])
	}

	// Padrão 2: número puro próximo a "followers" (ex: 1234 followers)
	re2 := regexp.MustCompile(`([\d,\.]+)\s*[Ff]ollowers?`)
	if matches := re2.FindStringSubmatch(text); len(matches) >= 2 {
		return strings.ReplaceAll(matches[1], ",", "")
	}

	// Padrão 3: formato JSON com followers
	re3 := regexp.MustCompile(`"followers?":\s*"?([\d,\.]+[KMB]?)"?`)
	if matches := re3.FindStringSubmatch(text); len(matches) >= 2 {
		return matches[1]
	}

	// Padrão 4: números entre tags/elementos que mencionam followers
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "follower") {
			// Verifica linhas adjacentes por números
			for j := max(0, i-2); j < min(len(lines), i+3); j++ {
				re4 := regexp.MustCompile(`^\s*([\d,\.]+[KMB]?)\s*$`)
				if matches := re4.FindStringSubmatch(lines[j]); len(matches) >= 2 {
					return strings.TrimSpace(matches[1])
				}
			}
		}
	}

	return ""
}

// EnrichInstagramFollowers busca dados de seguidores para um Instagram já encontrado
// Sistema de fallback em cascata com 12 fontes:
// 1. InstaStoriesViewer (primária)
// 2. StoryNavigation (fallback 1)
// 3. Imginn (fallback 2)
// 4. StoriesDown (fallback 3)
// 5. Picuki (fallback 4)
// 6. Greatfon (fallback 5)
// 7. Instalk (fallback 6)
// 8. DuckDuckGo Search (fallback 7)
// 9. Bing Search (fallback 8)
// 10. Brave Search (fallback 9)
// 11. Yandex Search (fallback 10)
// 12. Instagram Direct (fallback 11)
func EnrichInstagramFollowers(ctx context.Context, instagram *Instagram) error {
	if instagram == nil || instagram.Handle == "" {
		return fmt.Errorf("instagram inválido")
	}

	// Já tem seguidores?
	if instagram.Followers != "" {
		return nil
	}

	// Lista de scrapers em ordem de prioridade
	scrapers := []struct {
		name    string
		scraper interface {
			Search(context.Context, string) (*Instagram, error)
			Name() string
		}
	}{
		{"InstaStoriesViewer", NewInstaStoriesViewerScraper()},
		{"StoryNavigation", NewStoryNavigationScraper()},
		{"Imginn", NewImginnScraper()},
		{"StoriesDown", NewStoriesDownScraper()},
		{"Picuki", NewPicukiScraper()},
		{"Greatfon", NewGreatfonScraper()},
		{"Instalk", NewInstalkScraper()},
		{"DuckDuckGo", NewDuckDuckGoFollowersScraper()},
		{"Bing", NewBingSearchScraper()},
		{"Brave", NewBraveSearchFollowersScraper()},
		{"Yandex", NewYandexSearchFollowersScraper()},
		{"InstagramDirect", NewInstagramDirectScraper()},
	}

	var lastErr error
	for i, s := range scrapers {
		if i > 0 {
			fmt.Printf("⚠️  %s falhou (%v), tentando %s...\n", scrapers[i-1].name, lastErr, s.name)
		}

		result, err := s.scraper.Search(ctx, instagram.Handle)
		if err == nil && result.Followers != "" && result.Followers != "0" {
			instagram.Followers = result.Followers
			if i > 0 {
				fmt.Printf("✅ Sucesso com fallback %s\n", s.name)
			}
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("todas as 12 fontes falharam para %s", instagram.Handle)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
