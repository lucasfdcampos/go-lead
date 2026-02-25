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

		// Delay curto entre verificações (respeita rate limit sem ser lento demais)
		time.Sleep(300 * time.Millisecond)
	}

	return nil, fmt.Errorf("nenhum handle válido encontrado")
}

func (i *InstagramProfileChecker) generatePossibleHandles(query string) []string {
	var handles []string
	seen := make(map[string]bool)

	// Normaliza query
	query = strings.ToLower(query)
	query = strings.TrimSpace(query)

	// Remove palavras comuns, siglas de estados brasileiros e outros ruídos.
	// Mantém no máximo 3 palavras significativas para gerar handles mais precisos.
	words := strings.Fields(query)
	var filtered []string
	stopWords := map[string]bool{
		// Redes sociais / termos genéricos
		"instagram": true, "ig": true, "perfil": true, "profile": true,
		"oficial": true, "official": true,
		// Artigos / preposições portuguesas
		"de": true, "da": true, "do": true, "das": true, "dos": true,
		"em": true, "na": true, "no": true, "nas": true, "nos": true,
		"e": true, "a": true, "o": true, "as": true, "os": true,
		// Termos jurídicos de empresas
		"ltda": true, "eireli": true, "mei": true, "sa": true, "s.a": true,
		// Estados brasileiros (siglas de 2 letras ficam fora via len ≤ 2)
		"ac": true, "al": true, "ap": true, "am": true, "ba": true,
		"ce": true, "df": true, "es": true, "go": true, "ma": true,
		"mt": true, "ms": true, "mg": true, "pa": true, "pb": true,
		"pr": true, "pe": true, "pi": true, "rj": true, "rn": true,
		"rs": true, "ro": true, "rr": true, "sc": true, "sp": true,
		"se": true, "to": true,
	}

	for _, word := range words {
		// Ignora palavras com 2 ou menos caracteres (siglas, preposições)
		if len(word) <= 2 {
			continue
		}
		if !stopWords[word] {
			filtered = append(filtered, word)
		}
		// Limita a 3 palavras significativas para evitar nomes de cidades
		// como parte do handle (ex: "academiaatom", não "academiaatomarapongas")
		if len(filtered) >= 3 {
			break
		}
	}

	if len(filtered) == 0 {
		return handles
	}

	// A ordem importa: candidatos mais prováveis primeiro para respeitar o timeout.
	// Para "Academia Atom Arapongas" → filtered = ["academia","atom","arapongas"]
	// O handle mais provável é "academiaatom" (as duas primeiras palavras).
	addHandle := func(h string) {
		h = strings.ToLower(h)
		if IsValidHandle(h) && !seen[h] {
			handles = append(handles, h)
			seen[h] = true
		}
	}

	if len(filtered) >= 2 {
		// Prioridade 1: Primeiras duas palavras juntas (ex: "academiaatom")
		addHandle(strings.Join(filtered[:2], ""))
		// Prioridade 2: Primeiras duas com underscore (ex: "academia_atom")
		addHandle(strings.Join(filtered[:2], "_"))
		// Prioridade 3: Primeiras duas com ponto (ex: "academia.atom")
		addHandle(strings.Join(filtered[:2], "."))
	}

	if len(filtered) >= 3 {
		// Prioridade 4: Três palavras juntas (ex: "academiaatomarapongas")
		addHandle(strings.Join(filtered[:3], ""))
		// Prioridade 5: Três palavras com underscore
		addHandle(strings.Join(filtered[:3], "_"))
	}

	// Prioridade 6: Apenas a primeira palavra (ex: "academia")
	// Só é usada quando a empresa tem nome de uma única palavra significativa
	// para evitar falsos positivos em contas populares como @energy, @academia.
	if len(filtered) == 1 {
		addHandle(filtered[0])
	}

	return handles
}

func (i *InstagramProfileChecker) checkProfileExists(ctx context.Context, handle string) bool {
	// Usa User-Agent de bot de redes sociais (Facebot) para obter Open Graph tags.
	// Perfis válidos retornam: <meta property="og:type" content="profile" />
	// Perfis inválidos/inexistentes não retornam essa tag.
	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", handle)

	req, err := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	if err != nil {
		return false
	}

	// Instagram serve Open Graph completo para crawlers de redes sociais
	req.Header.Set("User-Agent", "Facebot Twitterbot/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: 6 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	// Lê até 16KB para garantir que os meta tags do <head> estejam incluídos
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))

	// Perfil válido tem og:type = "profile"; página 404 / not available não tem
	return strings.Contains(string(body), `og:type" content="profile"`)
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
