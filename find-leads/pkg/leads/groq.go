package leads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type GroqScraper struct {
	APIKey  string
	client  *http.Client
	baseURL string
}

func NewGroqScraper(apiKey string) *GroqScraper {
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	return &GroqScraper{
		APIKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.groq.com/openai/v1/chat/completions",
	}
}

func (g *GroqScraper) Name() string { return "Groq AI" }

func (g *GroqScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY nao configurada")
	}
	city, state := ParseLocation(location)

	// Tenta coletar texto de contexto da web
	rawText, _ := collectRawSearchText(ctx, query, city, state)

	var prompt string
	if len(rawText) >= 100 {
		maxLen := len(rawText)
		if maxLen > 5000 {
			maxLen = 5000
		}
		prompt = fmt.Sprintf(`Você é um especialista em dados comerciais do Brasil. Analise o texto abaixo e extraia TODOS os estabelecimentos comerciais LOCAIS do tipo "%s" localizados em "%s-%s".

REGRAS:
- Extraia apenas negócios LOCAIS com endereço físico em %s-%s
- NÃO inclua Mercado Livre, Amazon, OLX, Shopee ou outros marketplaces online
- NÃO inclua lojas nacionais sem endereço local
- Priorize entries com telefone e/ou endereço

Retorne APENAS um JSON array (sem explicações, sem markdown) com objetos contendo: name, phone, address, website, email.
Extraia o MÁXIMO de estabelecimentos locais possível.

Texto:
%s`, query, city, state, city, state, rawText[:maxLen])
	} else {
		// Usa conhecimento interno sem contexto web
		prompt = fmt.Sprintf(`Você é um extrator de dados de negócios REAIS. Liste estabelecimentos comerciais FÍSICOS do tipo "%s" em %s, %s, Brasil.

REGRAS ESTRITAS:
- Inclua SOMENTE negócios locais com loja física em %s
- NÃO inclua varejistas puramente online (Zattini, Dafiti, Shoptime, etc.)
- Para phone: inclua SOMENTE se você tem certeza do número real; caso contrário use ""
- Para address: inclua SOMENTE endereços que você conhece com certeza; caso contrário use ""
- NÃO invente ou extrapole números de telefone
- Inclua redes nacionais que tenham CONFIRMADAMENTE unidade em %s (ex: Renner, C&A, Riachuelo, Havan)

Retorne APENAS um JSON array com campos: name, phone, address, website, email.`, query, city, state, city, city)
	}
	body := map[string]interface{}{
		"model": "llama-3.3-70b-versatile",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  2048,
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, err
	}
	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("groq retornou resposta vazia")
	}
	return parseAILeads(groqResp.Choices[0].Message.Content, city, state, "Groq AI")
}

func collectRawSearchText(ctx context.Context, query, city, state string) (string, error) {
	q := fmt.Sprintf(`"%s" "%s" "%s" telefone`, query, city, state)
	// Tenta DDG primeiro, depois Yandex como fallback
	engines := []struct{ url, selector string }{
		// Bing — mais estável, sempre retorna 200
		{
			"https://www.bing.com/search?q=" + url.QueryEscape(q) + "&setlang=pt-BR",
			"p, .b_caption p, .b_snippet, li",
		},
		// DDG Lite — fallback
		{
			"https://lite.duckduckgo.com/lite/?q=" + url.QueryEscape(q),
			"td.result-snippet, td, a",
		},
		// Yandex — segundo fallback
		{
			fmt.Sprintf("https://yandex.com/search/?text=%s&lr=102", url.QueryEscape(q)),
			".Organic-Text, .OrganicText, p, .serp-item__text",
		},
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for _, eng := range engines {
		time.Sleep(1 * time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", eng.url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			continue
		}
		var sb strings.Builder
		doc.Find(eng.selector).Each(func(_ int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if len(t) > 10 {
				sb.WriteString(t)
				sb.WriteString("\n")
			}
		})
		if sb.Len() > 100 {
			return sb.String(), nil
		}
	}
	return "", fmt.Errorf("todos os motores de busca falharam ou estão bloqueados")
}

func parseAILeads(content, city, state, source string) ([]*Lead, error) {
	content = strings.TrimSpace(content)
	for _, pfx := range []string{"```json", "```"} {
		content = strings.TrimPrefix(content, pfx)
	}
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("resposta AI nao contem JSON array valido")
	}
	var extracted []struct {
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
		Website string `json:"website"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &extracted); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON da AI: %w", err)
	}
	// Lojas exclusivamente online — não têm endereço físico local
	onlineOnly := []string{"zattini", "dafiti", "shoptime", "netshoes", "kanui", "centauro.com",
		"lojas americanas", "submarino", "extra.com", "shopify", "amazon"}
	var found []*Lead
	for _, e := range extracted {
		if e.Name == "" {
			continue
		}
		// Filtra lojas online
		nameLower := strings.ToLower(e.Name)
		isOnline := false
		for _, ol := range onlineOnly {
			if strings.Contains(nameLower, ol) {
				isOnline = true
				break
			}
		}
		if isOnline {
			continue
		}
		phone := normalizePhone(e.Phone)
		found = append(found, &Lead{Name: e.Name, Phone: phone,
			Address: e.Address, Website: e.Website, Email: e.Email,
			City: city, State: state, Source: source})
	}
	return found, nil
}
