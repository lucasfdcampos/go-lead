package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ImginnScraper busca seguidores no Imginn (viewer de Instagram)
type ImginnScraper struct{}

func NewImginnScraper() *ImginnScraper {
	return &ImginnScraper{}
}

func (i *ImginnScraper) Name() string {
	return "Imginn"
}

func (i *ImginnScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://imginn.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")

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

	// Imginn mostra followers em divs específicas
	doc.Find(".sum, .followers-count, .user-stats").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	// Busca mais genérica em todo o corpo do documento
	if followers == "" {
		bodyText := doc.Find("body").Text()
		// Procura por padrões de seguidores próximos ao handle
		if strings.Contains(bodyText, handle) {
			followers = extractFollowersFromText(bodyText)
		}
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Imginn")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// StoriesDownScraper busca no StoriesDown
type StoriesDownScraper struct{}

func NewStoriesDownScraper() *StoriesDownScraper {
	return &StoriesDownScraper{}
}

func (s *StoriesDownScraper) Name() string {
	return "StoriesDown"
}

func (s *StoriesDownScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://storiesdown.com/users/%s", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

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

	// Busca em elementos específicos do StoriesDown
	doc.Find(".user-info, .stats, .followers").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no StoriesDown")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// GreatfonScraper busca no Greatfon (API pública)
type GreatfonScraper struct{}

func NewGreatfonScraper() *GreatfonScraper {
	return &GreatfonScraper{}
}

func (g *GreatfonScraper) Name() string {
	return "Greatfon"
}

func (g *GreatfonScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://greatfon.com/v/%s", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json,text/html")

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

	// Tenta parsear como JSON primeiro
	var jsonData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonData); err == nil {
		// Se tem dados JSON, busca followers
		if user, ok := jsonData["user"].(map[string]interface{}); ok {
			if followerCount, ok := user["follower_count"].(float64); ok {
				instagram := NewInstagram(handle)
				instagram.Followers = formatFollowerCount(fmt.Sprintf("%.0f", followerCount))
				return instagram, nil
			}
		}
	}

	// Senão, parseia como HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear: %w", err)
	}

	followers := ""
	doc.Find(".followers, .stats, .user-stats").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Greatfon")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// InstalkScraper busca no Instalk.net
type InstalkScraper struct{}

func NewInstalkScraper() *InstalkScraper {
	return &InstalkScraper{}
}

func (i *InstalkScraper) Name() string {
	return "Instalk"
}

func (i *InstalkScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	url := fmt.Sprintf("https://instalk.net/%s", handle)

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

	// Instalk tem classes específicas
	doc.Find(".user_followers, .profile-stats, .followers-count").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	// Busca em meta tags também
	if followers == "" {
		if meta, exists := doc.Find("meta[name='description']").Attr("content"); exists {
			followers = extractFollowersFromText(meta)
		}
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Instalk")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// BingSearchScraper busca via Bing Search (alternativa ao DuckDuckGo)
type BingSearchScraper struct{}

func NewBingSearchScraper() *BingSearchScraper {
	return &BingSearchScraper{}
}

func (b *BingSearchScraper) Name() string {
	return "Bing Search"
}

func (b *BingSearchScraper) Search(ctx context.Context, query string) (*Instagram, error) {
	handle := NormalizeHandle(query)
	if handle == "" {
		return nil, fmt.Errorf("handle inválido")
	}

	searchQuery := fmt.Sprintf("instagram @%s followers", handle)
	url := fmt.Sprintf("https://www.bing.com/search?q=%s", searchQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

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

	// Busca nos snippets de resultados do Bing
	doc.Find(".b_caption, .b_attribution, .b_snippet").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if found := extractFollowersFromText(text); found != "" {
			followers = found
			return
		}
	})

	// Busca em knowledge panels
	if followers == "" {
		doc.Find(".b_entityTitle, .b_factrow").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(strings.ToLower(text), "follower") {
				if found := extractFollowersFromText(text); found != "" {
					followers = found
					return
				}
			}
		})
	}

	if followers == "" {
		return nil, fmt.Errorf("seguidores não encontrados no Bing")
	}

	instagram := NewInstagram(handle)
	instagram.Followers = followers
	return instagram, nil
}

// extractNumberFromJSON extrai números de strings JSON
func extractNumberFromJSON(text, key string) string {
	pattern := fmt.Sprintf(`"%s"\s*:\s*(\d+)`, regexp.QuoteMeta(key))
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(text); len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
