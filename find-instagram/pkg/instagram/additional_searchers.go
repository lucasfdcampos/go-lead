package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	// DuckDuckGo bloqueia site: operator em IPs de servidor; usa sufixo "instagram"
	searchQuery := query + " instagram"

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

	// Procura em links — descodifica redirects do DuckDuckGo (/l/?uddg=URL)
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if foundHandle != nil {
			return
		}

		href, exists := s.Attr("href")
		if exists {
			// Decode DuckDuckGo redirect: /l/?uddg=https%3A%2F%2Finstagram.com%2Fhandle
			if idx := strings.Index(href, "uddg="); idx != -1 {
				if decoded, err := url.QueryUnescape(href[idx+5:]); err == nil {
					href = decoded
				}
			}
			if strings.Contains(href, "instagram.com/") {
				handles := ExtractAllHandles(href)
				if len(handles) > 0 {
					foundHandle = handles[0]
					return
				}
			}
		}

		// Título frequentemente mostra "Business (@handle) • Instagram"
		text := s.Text()
		if strings.Contains(strings.ToLower(text), "instagram") || strings.Contains(text, "@") {
			handles := ExtractAllHandles(text)
			if len(handles) > 0 {
				foundHandle = handles[0]
				return
			}
		}
	})

	// URL exibida pelo DuckDuckGo (e.g. "instagram.com/academiaatom")
	if foundHandle == nil {
		doc.Find(".result__url, .result__extras__url, .result__extras").Each(func(_ int, s *goquery.Selection) {
			if foundHandle != nil {
				return
			}
			urlText := strings.TrimSpace(s.Text())
			if strings.Contains(urlText, "instagram.com/") {
				handles := ExtractAllHandles("https://" + strings.TrimPrefix(urlText, "https://"))
				if len(handles) > 0 {
					foundHandle = handles[0]
				}
			}
		})
	}

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

	// Busca em todo o corpo de resultados (scoped) se não encontrou ainda
	if foundHandle == nil {
		doc.Find(".result__body, .result, #links").Each(func(_ int, s *goquery.Selection) {
			if foundHandle != nil {
				return
			}
			handles := ExtractAllHandles(s.Text())
			if len(handles) > 0 {
				foundHandle = handles[0]
			}
		})
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
	// Usa site:instagram.com para forçar resultados apenas do Instagram
	searchQuery := instagramSiteQuery(query)

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

	// Busca em containers de resultado (scoped)
	if foundHandle == nil {
		doc.Find("#search, #rso, .g").Each(func(_ int, s *goquery.Selection) {
			if foundHandle != nil {
				return
			}
			handles := ExtractAllHandles(s.Text())
			if len(handles) > 0 {
				foundHandle = handles[0]
			}
		})
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
	possibleHandles, businessWords := i.generatePossibleHandles(query)

	type candidate struct {
		handle    string
		score     int
		followers int
	}

	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var matches []candidate

	for idx, handle := range possibleHandles {
		wg.Add(1)
		go func(h string, delay time.Duration) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Stagger startup to avoid simultaneous hits
			if delay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}

			if ctx.Err() != nil {
				return
			}

			ok, ogContent := i.checkProfileExists(ctx, h)
			if !ok {
				return
			}

			score := 0
			displayName := extractOGDisplayName(ogContent)
			for _, w := range businessWords {
				if strings.Contains(displayName, w) {
					score++
				}
			}
			followers := extractFollowerCount(ogContent)

			mu.Lock()
			matches = append(matches, candidate{h, score, followers})
			mu.Unlock()
		}(handle, time.Duration(idx)*150*time.Millisecond)
	}

	wg.Wait()

	if len(matches) == 0 {
		return nil, fmt.Errorf("nenhum handle válido encontrado")
	}

	// Retorna o candidato com maior pontuação (mais palavras do negócio no display name).
	// Em caso de empate de pontuação, prefere o com mais seguidores (conta mais ativa).
	best := matches[0]
	for _, c := range matches[1:] {
		if c.score > best.score || (c.score == best.score && c.followers > best.followers) {
			best = c
		}
	}

	return NewInstagram(best.handle), nil
}

// extractOGDisplayName extrai o nome de exibição do og:title do Instagram.
// Formato: 'og:title" content="DisplayName (@handle) • Instagram profile"'
// Retorna o display name em minúsculas, ou string vazia se não encontrado.
func extractOGDisplayName(ogContent string) string {
	const marker = `og:title" content="`
	idx := strings.Index(ogContent, marker)
	if idx == -1 {
		return ""
	}
	rest := ogContent[idx+len(marker):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	title := rest[:end] // "DisplayName (@handle) • Instagram profile"
	// Remove tudo a partir do "@" — queremos só o display name
	if atIdx := strings.Index(title, " ("); atIdx != -1 {
		title = title[:atIdx]
	}
	return strings.ToLower(title)
}

// extractFollowerCount extrai a contagem de seguidores do og:description.
// Formato: "4,575 Followers, 134 Following, 193 Posts - See Instagram..."
// Retorna 0 se não encontrado ou em caso de erro.
func extractFollowerCount(ogContent string) int {
	const marker = `og:description" content="`
	idx := strings.Index(ogContent, marker)
	if idx == -1 {
		return 0
	}
	rest := ogContent[idx+len(marker):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return 0
	}
	desc := rest[:end] // "4,575 Followers, 134 Following, 193 Posts - ..."
	// Extrai o número antes de " Followers"
	followerIdx := strings.Index(strings.ToLower(desc), " followers")
	if followerIdx == -1 {
		return 0
	}
	numStr := desc[:followerIdx]
	// Remove vírgulas/pontos de milhares e converte
	numStr = strings.ReplaceAll(numStr, ",", "")
	numStr = strings.ReplaceAll(numStr, ".", "")
	numStr = strings.TrimSpace(numStr)
	// Pega apenas os dígitos finais (pode ter texto antes do número)
	i := len(numStr) - 1
	for i >= 0 && numStr[i] >= '0' && numStr[i] <= '9' {
		i--
	}
	numStr = numStr[i+1:]
	if numStr == "" {
		return 0
	}
	n := 0
	for _, c := range numStr {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func (i *InstagramProfileChecker) generatePossibleHandles(query string) (handles []string, businessWords []string) {
	seen := make(map[string]bool)

	query = strings.ToLower(strings.TrimSpace(query))

	words := strings.Fields(query)
	var filtered []string
	stopWords := map[string]bool{
		"instagram": true, "ig": true, "perfil": true, "profile": true,
		"oficial": true, "official": true,
		"de": true, "da": true, "do": true, "das": true, "dos": true,
		"em": true, "na": true, "no": true, "nas": true, "nos": true,
		"e": true, "a": true, "o": true, "as": true, "os": true,
		"ltda": true, "eireli": true, "mei": true, "sa": true, "s.a": true,
		"ac": true, "al": true, "ap": true, "am": true, "ba": true,
		"ce": true, "df": true, "es": true, "go": true, "ma": true,
		"mt": true, "ms": true, "mg": true, "pa": true, "pb": true,
		"pr": true, "pe": true, "pi": true, "rj": true, "rn": true,
		"rs": true, "ro": true, "rr": true, "sc": true, "sp": true,
		"se": true, "to": true,
	}

	// Palavras que indicam tipo de negócio — não são cidade, fazem parte do nome
	businessTypeWords := map[string]bool{
		"academia": true, "restaurante": true, "loja": true, "salao": true,
		"salon": true, "clinica": true, "studio": true, "estudio": true,
		"mercado": true, "farmacia": true, "escola": true, "colegio": true,
		"hospital": true, "oficina": true, "barbearia": true, "pet": true,
		"shop": true, "store": true, "fitness": true, "gym": true,
		"estetica": true, "moda": true, "boutique": true, "sorveteria": true,
		"padaria": true, "acougue": true, "auto": true, "motos": true,
		"veiculos": true, "imoveis": true, "construcao": true,
	}

	for _, word := range words {
		if len(word) <= 2 {
			continue
		}
		if !stopWords[word] {
			filtered = append(filtered, word)
		}
		if len(filtered) >= 3 {
			break
		}
	}

	if len(filtered) == 0 {
		return
	}

	// Palavras do negócio usadas para pontuação de candidatos (no máximo 2).
	// A terceira palavra filtrada costuma ser a cidade e NÃO entra nos candidatos.
	// EXCEÇÃO: se for uma palavra de tipo de negócio (ex: "academia"), ela faz
	// parte do nome e também gera combinações de 3 palavras.
	thirdIsBusinessType := len(filtered) == 3 && businessTypeWords[filtered[2]]
	businessWords = filtered[:min(2, len(filtered))]
	if len(filtered) == 3 && !thirdIsBusinessType {
		// 3ª palavra é cidade — usa apenas as 2 primeiras para gerar candidatos
		filtered = filtered[:2]
	}

	addHandle := func(h string) {
		h = strings.ToLower(h)
		if IsValidHandle(h) && !seen[h] {
			handles = append(handles, h)
			seen[h] = true
		}
	}

	w0, w1 := filtered[0], ""
	if len(filtered) >= 2 {
		w1 = filtered[1]
	}
	w2 := ""
	if len(filtered) >= 3 {
		w2 = filtered[2]
	}

	if w1 != "" {
		// Ordem direta: palavraA + palavraB
		addHandle(w0 + w1)       // "actionacademia"
		addHandle(w0 + "_" + w1) // "action_academia"
		addHandle(w0 + "." + w1) // "action.academia"
		// Ordem inversa: palavraB + palavraA (frequente em nomes de Instagram)
		addHandle(w1 + w0)       // "academiaaction"
		addHandle(w1 + "_" + w0) // "academia_action"
		addHandle(w1 + "." + w0) // "academia.action"  ← ex: academia.action
	}

	// Combinações de 3 palavras quando a 3ª palavra é tipo de negócio
	// Ex: "MVB FIT ACADEMIA" → gera "mvbfitacademia"
	if w1 != "" && w2 != "" {
		addHandle(w0 + w1 + w2)             // "mvbfitacademia" ← soluciona MVB FIT
		addHandle(w0 + "." + w1 + "." + w2) // "mvb.fit.academia"
		addHandle(w0 + w2 + w1)             // "mvbacademiafit"
		addHandle(w2 + w0 + w1)             // "academiamvbfit"
	}

	// Apenas a primeira palavra — só para nomes de uma única palavra significativa
	if w1 == "" {
		addHandle(w0)
	}

	return
}

func (i *InstagramProfileChecker) checkProfileExists(ctx context.Context, handle string) (bool, string) {
	// Usa User-Agent de bot de redes sociais (Facebot) para obter Open Graph tags.
	// Perfis válidos retornam: <meta property="og:type" content="profile" />
	// O conteúdo OG (title + description) é retornado para validação por cidade.
	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	if err != nil {
		return false, ""
	}

	req.Header.Set("User-Agent", "Facebot Twitterbot/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: 6 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, ""
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	content := string(body)

	if !strings.Contains(content, `og:type" content="profile"`) {
		return false, ""
	}
	return true, content
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
	searchQuery := instagramSiteQuery(query)

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

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var found *Instagram
	// Scope to Bing result containers only
	doc.Find("#b_results .b_algo").Each(func(_ int, s *goquery.Selection) {
		if found != nil {
			return
		}
		s.Find("a").Each(func(_ int, a *goquery.Selection) {
			if found != nil {
				return
			}
			if href, ok := a.Attr("href"); ok && strings.Contains(href, "instagram.com/") {
				handles := ExtractAllHandles(href)
				if len(handles) > 0 {
					found = handles[0]
				}
			}
		})
		if found == nil {
			text := s.Text()
			handles := ExtractAllHandles(text)
			if len(handles) > 0 {
				found = handles[0]
			}
		}
	})
	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("nenhum handle encontrado")
}

// ─── SearXNG Instagram Searcher ───────────────────────────────────────────────

var searxngIGInstances = []string{
	"https://searx.be",
	"https://search.bus-hit.me",
	"https://paulgo.io",
}

// SearXNGSearcher busca usando SearXNG (instâncias públicas)
type SearXNGSearcher struct{}

func NewSearXNGSearcher() *SearXNGSearcher { return &SearXNGSearcher{} }
func (s *SearXNGSearcher) Name() string    { return "SearXNG" }

func (s *SearXNGSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	q := instagramSiteQuery(query)

	client := &http.Client{Timeout: 15 * time.Second}

	for _, instance := range searxngIGInstances {
		searchURL := fmt.Sprintf("%s/search?q=%s&language=pt-BR&format=html", instance, url.QueryEscape(q))

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

		time.Sleep(800 * time.Millisecond)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		// Only scan result content areas, skip nav/footer/header
		var found *Instagram
		doc.Find(".result-content, .result-title, .result-url, article, [class*='result']").Each(func(_ int, sel *goquery.Selection) {
			if found != nil {
				return
			}
			sel.Find("a").Each(func(_ int, a *goquery.Selection) {
				if found != nil {
					return
				}
				if href, ok := a.Attr("href"); ok && strings.Contains(href, "instagram.com/") {
					handles := ExtractAllHandles(href)
					if len(handles) > 0 {
						found = handles[0]
					}
				}
			})
			if found == nil {
				handles := ExtractAllHandles(sel.Text())
				if len(handles) > 0 {
					found = handles[0]
				}
			}
		})

		if found != nil {
			return found, nil
		}
	}

	return nil, fmt.Errorf("nenhum handle encontrado no SearXNG")
}

// ─── Mojeek Instagram Searcher ────────────────────────────────────────────────

// MojeekSearcher busca usando Mojeek
type MojeekSearcher struct{}

func NewMojeekSearcher() *MojeekSearcher { return &MojeekSearcher{} }
func (m *MojeekSearcher) Name() string   { return "Mojeek" }

func (m *MojeekSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	q := instagramSiteQuery(query)

	searchURL := "https://www.mojeek.com/search?q=" + url.QueryEscape(q)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("mojeek: request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(800 * time.Millisecond)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mojeek: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mojeek: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mojeek: parse: %w", err)
	}

	// Only scan result content areas, skip nav/footer/header
	var found *Instagram
	doc.Find(".result, .result-wrap, li.result, [class*='result']").Each(func(_ int, sel *goquery.Selection) {
		if found != nil {
			return
		}
		sel.Find("a").Each(func(_ int, a *goquery.Selection) {
			if found != nil {
				return
			}
			if href, ok := a.Attr("href"); ok && strings.Contains(href, "instagram.com/") {
				handles := ExtractAllHandles(href)
				if len(handles) > 0 {
					found = handles[0]
				}
			}
		})
		if found == nil {
			handles := ExtractAllHandles(sel.Text())
			if len(handles) > 0 {
				found = handles[0]
			}
		}
	})

	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("nenhum handle encontrado no Mojeek")
}

// ─── Swisscows Instagram Searcher ─────────────────────────────────────────────

// SwisscowsSearcher busca usando Swisscows
type SwisscowsSearcher struct{}

func NewSwisscowsSearcher() *SwisscowsSearcher { return &SwisscowsSearcher{} }
func (s *SwisscowsSearcher) Name() string      { return "Swisscows" }

func (s *SwisscowsSearcher) Search(ctx context.Context, query string) (*Instagram, error) {
	q := instagramSiteQuery(query)

	searchURL := "https://swisscows.com/web?query=" + url.QueryEscape(q) + "&region=pt-BR"

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("swisscows: request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("swisscows: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("swisscows: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("swisscows: parse: %w", err)
	}

	// Swisscows renders via React; results appear in .web-results > .item or [class*='item']
	// Explicitly skip footer/nav by only looking at main content
	var found *Instagram
	doc.Find(".web-results .item, .result-item, [class*='web-results'] a, main a").Each(func(_ int, sel *goquery.Selection) {
		if found != nil {
			return
		}
		if href, ok := sel.Attr("href"); ok && strings.Contains(href, "instagram.com/") {
			handles := ExtractAllHandles(href)
			if len(handles) > 0 {
				found = handles[0]
			}
		}
	})

	if found != nil {
		return found, nil
	}

	// Swisscows is JS-heavy; often nothing useful in static HTML
	return nil, fmt.Errorf("nenhum handle encontrado no Swisscows")
}
