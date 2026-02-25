package leads

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// junkLabels são palavras-chave de UI que indicam que o texto não é nome de empresa
var junkLabels = []string{
	"ligar", "endereço", "whatsapp", "horário", "horario", "aberto",
	"avaliações", "avaliacao", "ver mais", "website", "rotas", "compartilhar",
	"maps", "google", "facebook", "instagram", "seg a", "seg–",
}

// extractLeadsFromText extrai leads de texto não estruturado (snippets de busca)
func extractLeadsFromText(text, city, state, source string) []*Lead {
	var result []*Lead
	phoneRe := regexp.MustCompile(`\(?\d{2}\)?[\s.]?\d{4,5}[-\s.]?\d{4}`)
	emailRe := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	digitStartRe := regexp.MustCompile(`^[\d./_\-]+`)
	urlRe := regexp.MustCompile(`https?://\S+`)
	junkRe := regexp.MustCompile(`\s{3,}`) // múltiplos espaços = UI junk

	lines := strings.Split(text, "\n")
	prevLine := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 5 {
			prevLine = line
			continue
		}

		phones := phoneRe.FindAllString(line, 2)
		emails := emailRe.FindAllString(line, 1)

		if len(phones) > 0 || len(emails) > 0 {
			lead := &Lead{
				City:   city,
				State:  state,
				Source: source,
			}

			if len(phones) > 0 {
				lead.Phone = normalizePhone(phones[0])
				if len(phones) > 1 {
					lead.Phone2 = normalizePhone(phones[1])
				}
			}
			if len(emails) > 0 {
				lead.Email = emails[0]
			}

			// Extrai nome: parte ANTES do primeiro telefone na linha
			phoneLoc := phoneRe.FindStringIndex(line)
			namePart := line
			if phoneLoc != nil && phoneLoc[0] > 0 {
				namePart = line[:phoneLoc[0]]
			}
			namePart = urlRe.ReplaceAllString(namePart, "")
			namePart = emailRe.ReplaceAllString(namePart, "")
			namePart = regexp.MustCompile(`[-|·•,·–]+$`).ReplaceAllString(namePart, "")
			namePart = strings.TrimSpace(namePart)

			// Descarta se parece lixo de UI
			isJunk := false
			lower := strings.ToLower(namePart)
			for _, lbl := range junkLabels {
				if strings.Contains(lower, lbl) {
					isJunk = true
					break
				}
			}
			if digitStartRe.MatchString(namePart) {
				isJunk = true
			}
			if junkRe.MatchString(namePart) {
				// múltiplos espaços = fragmento de UI (ex: "Ligar   Endereço   ...")
				isJunk = true
			}

			// Se o nome da linha atual é lixo, tenta usar a linha anterior
			if isJunk && prevLine != "" && len(prevLine) > 3 && len(prevLine) < 80 {
				prevLower := strings.ToLower(prevLine)
				prevIsJunk := false
				for _, lbl := range junkLabels {
					if strings.Contains(prevLower, lbl) {
						prevIsJunk = true
						break
					}
				}
				if !prevIsJunk && !digitStartRe.MatchString(prevLine) {
					namePart = prevLine
					isJunk = false
				}
			}

			if !isJunk && len(namePart) > 2 && len(namePart) < 100 {
				lead.Name = namePart
			}

			result = append(result, lead)
		}
		prevLine = line
	}

	return result
}

// ─── DuckDuckGo ──────────────────────────────────────────────────────────────

type DDGLeadScraper struct{}

func NewDDGLeadScraper() *DDGLeadScraper { return &DDGLeadScraper{} }
func (d *DDGLeadScraper) Name() string   { return "DuckDuckGo" }

func (d *DDGLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" "%s" telefone`, query, city, state)
	// Usa versão lite do DDG que tem menos bot-detection
	return searchEngineLeads(ctx, "https://lite.duckduckgo.com/lite/?q="+url.QueryEscape(q),
		"td.result-snippet, .result-snippet, td", city, state, "DuckDuckGo", 1500*time.Millisecond)
}

// ─── Bing ────────────────────────────────────────────────────────────────────

type BingLeadScraper struct{}

func NewBingLeadScraper() *BingLeadScraper { return &BingLeadScraper{} }
func (b *BingLeadScraper) Name() string    { return "Bing" }

func (b *BingLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" "%s" telefone`, query, city, state)
	return searchEngineLeads(ctx, "https://www.bing.com/search?q="+url.QueryEscape(q),
		".b_caption p, .b_snippet, .b_dList", city, state, "Bing", 1*time.Second)
}

// ─── Brave ───────────────────────────────────────────────────────────────────

type BraveLeadScraper struct{}

func NewBraveLeadScraper() *BraveLeadScraper { return &BraveLeadScraper{} }
func (b *BraveLeadScraper) Name() string     { return "Brave Search" }

func (b *BraveLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" "%s" telefone`, query, city, state)
	return searchEngineLeads(ctx, "https://search.brave.com/search?q="+url.QueryEscape(q),
		".snippet-description, .snippet-content, .result-description, .fdb", city, state, "Brave", 2500*time.Millisecond)
}

// ─── Yandex ──────────────────────────────────────────────────────────────────

type YandexLeadScraper struct{}

func NewYandexLeadScraper() *YandexLeadScraper { return &YandexLeadScraper{} }
func (y *YandexLeadScraper) Name() string      { return "Yandex" }

func (y *YandexLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`%s %s-%s telefone Brasil`, query, city, state)
	return searchEngineLeads(ctx,
		fmt.Sprintf("https://yandex.com/search/?text=%s&lr=102", url.QueryEscape(q)),
		".Organic-Text, .OrganicText, .ExtendedText, .serp-item__text", city, state, "Yandex", 2000*time.Millisecond)
}

// ─── SearXNG ──────────────────────────────────────────────────────────────────

// searxngInstances lists public SearXNG instances to try in order.
var searxngInstances = []string{
	"https://searx.be",
	"https://search.bus-hit.me",
	"https://paulgo.io",
}

type SearXNGLeadScraper struct{}

func NewSearXNGLeadScraper() *SearXNGLeadScraper { return &SearXNGLeadScraper{} }
func (s *SearXNGLeadScraper) Name() string       { return "SearXNG" }

func (s *SearXNGLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" telefone`, query, city)
	for _, instance := range searxngInstances {
		searchURL := fmt.Sprintf("%s/search?q=%s&language=pt-BR&format=html", instance, url.QueryEscape(q))
		leads, err := searchEngineLeads(ctx, searchURL,
			".result-content, .result_header, .result-description", city, state, "SearXNG", 1500*time.Millisecond)
		if err == nil && len(leads) > 0 {
			return leads, nil
		}
	}
	return nil, fmt.Errorf("SearXNG: no results from any instance")
}

// ─── Mojeek ───────────────────────────────────────────────────────────────────

type MojeekLeadScraper struct{}

func NewMojeekLeadScraper() *MojeekLeadScraper { return &MojeekLeadScraper{} }
func (m *MojeekLeadScraper) Name() string      { return "Mojeek" }

func (m *MojeekLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" telefone`, query, city)
	return searchEngineLeads(ctx,
		"https://www.mojeek.com/search?q="+url.QueryEscape(q),
		".result-text, .result__body, .result-wrap p", city, state, "Mojeek", 1500*time.Millisecond)
}

// ─── Swisscows ────────────────────────────────────────────────────────────────

type SwisscowsLeadScraper struct{}

func NewSwisscowsLeadScraper() *SwisscowsLeadScraper { return &SwisscowsLeadScraper{} }
func (s *SwisscowsLeadScraper) Name() string         { return "Swisscows" }

func (s *SwisscowsLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`%s %s telefone Brasil`, query, city)
	return searchEngineLeads(ctx,
		"https://swisscows.com/web?query="+url.QueryEscape(q)+"&region=pt-BR",
		".web-results .item-body, .result-item .description", city, state, "Swisscows", 2000*time.Millisecond)
}

// ─── Helper genérico ─────────────────────────────────────────────────────────

func searchEngineLeads(ctx context.Context, searchURL, selector, city, state, source string, delay time.Duration) ([]*Lead, error) {
	time.Sleep(delay)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := DoWithRetry(ctx, client, req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	phoneRe := regexp.MustCompile(`\(?\d{2}\)?[\s.]?\d{4,5}[-\s.]?\d{4}`)
	emailRe := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	digitStartRe := regexp.MustCompile(`^[\d./_\-]+`)
	junkMultiSpaceRe := regexp.MustCompile(`\s{3,}`)

	isJunkName := func(s string) bool {
		if s == "" || digitStartRe.MatchString(s) || junkMultiSpaceRe.MatchString(s) {
			return true
		}
		lower := strings.ToLower(s)
		for _, lbl := range junkLabels {
			if strings.Contains(lower, lbl) {
				return true
			}
		}
		return false
	}

	extractName := func(titleText, snippetText string) string {
		// Tenta título primeiro
		titleText = strings.TrimSpace(titleText)
		if !isJunkName(titleText) && len(titleText) > 2 && len(titleText) < 80 {
			// Remove sufixos comuns de título de resultados de busca
			for _, sep := range []string{" - ", " | ", " – ", " · "} {
				if idx := strings.Index(titleText, sep); idx > 2 {
					titleText = titleText[:idx]
					break
				}
			}
			return strings.TrimSpace(titleText)
		}
		// Fallback: parte antes do primeiro telefone no snippet
		if phoneLoc := phoneRe.FindStringIndex(snippetText); phoneLoc != nil && phoneLoc[0] > 2 {
			namePart := strings.TrimSpace(snippetText[:phoneLoc[0]])
			namePart = regexp.MustCompile(`[-|·•,–]+$`).ReplaceAllString(namePart, "")
			namePart = strings.TrimSpace(namePart)
			if !isJunkName(namePart) && len(namePart) > 2 && len(namePart) < 80 {
				return namePart
			}
		}
		return ""
	}

	var leads []*Lead

	// Tenta extrair resultados estruturados (título + snippet por resultado)
	// Seletores para diferentes motores
	resultSelectors := []struct{ container, title, snippet string }{
		// DuckDuckGo
		{".result", ".result__title", ".result__snippet"},
		// Bing
		{"#b_results .b_algo", "h2", ".b_caption p"},
		// Brave
		{"[data-type='web'] .snippet", ".heading-results a, .result-title a, h3", ".snippet-description, .result-description"},
		// Brave alternativo
		{".card", "h3, .title", ".description, .snippet-description"},
	}

	for _, rs := range resultSelectors {
		count := 0
		doc.Find(rs.container).Each(func(_ int, item *goquery.Selection) {
			titleText := strings.TrimSpace(item.Find(rs.title).First().Text())
			snippetText := strings.TrimSpace(item.Find(rs.snippet).First().Text())
			combined := titleText + "\n" + snippetText

			phones := phoneRe.FindAllString(combined, 2)
			emails := emailRe.FindAllString(combined, 1)
			if len(phones) == 0 && len(emails) == 0 {
				return
			}

			lead := &Lead{City: city, State: state, Source: source}
			if len(phones) > 0 {
				lead.Phone = normalizePhone(phones[0])
				if len(phones) > 1 {
					lead.Phone2 = normalizePhone(phones[1])
				}
			}
			if len(emails) > 0 {
				lead.Email = emails[0]
			}
			lead.Name = extractName(titleText, snippetText)
			leads = append(leads, lead)
			count++
		})
		if count > 0 {
			break
		}
	}

	// Fallback: extração por texto corrido do seletor original
	if len(leads) == 0 {
		var allText strings.Builder
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			allText.WriteString(s.Text())
			allText.WriteString("\n")
		})
		// Segundo fallback: divs de conteúdo
		if allText.Len() < 50 {
			doc.Find(".content, .description, p, li, [class*='snippet'], [class*='result'], [class*='caption']").Each(func(i int, s *goquery.Selection) {
				t := strings.TrimSpace(s.Text())
				if len(t) > 10 && len(t) < 500 {
					allText.WriteString(t)
					allText.WriteString("\n")
				}
			})
		}
		// Terceiro fallback: texto completo do body
		if allText.Len() < 50 {
			bodyText := strings.TrimSpace(doc.Find("body").Text())
			allText.WriteString(bodyText)
		}
		leads = extractLeadsFromText(allText.String(), city, state, source)
	}

	return leads, nil
}
