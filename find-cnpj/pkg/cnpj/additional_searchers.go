package cnpj

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ReceitaWSSearcher struct {
	CNPJ string
}

func NewReceitaWSSearcher(cnpj string) *ReceitaWSSearcher {
	return &ReceitaWSSearcher{CNPJ: cnpj}
}

func (r *ReceitaWSSearcher) Name() string {
	return "ReceitaWS API"
}

func (r *ReceitaWSSearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
	if r.CNPJ == "" {
		return nil, fmt.Errorf("CNPJ não fornecido")
	}

	url := fmt.Sprintf("https://www.receitaws.com.br/v1/cnpj/%s", r.CNPJ)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit excedido")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API retornou status %d", resp.StatusCode)
	}

	var result struct {
		CNPJ               string `json:"cnpj"`
		Nome               string `json:"nome"`
		Fantasia           string `json:"fantasia"`
		Telefone           string `json:"telefone"`
		AtividadePrincipal []struct {
			Code string `json:"code"`
			Text string `json:"text"`
		} `json:"atividade_principal"`
		QSA []struct {
			Nome string `json:"nome"`
		} `json:"qsa"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Fallback: tenta extrair CNPJ do texto bruto se JSON falhar
		body, _ := io.ReadAll(resp.Body)
		if cnpj := ExtractCNPJ(string(body)); cnpj != nil {
			return cnpj, nil
		}
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	cnpjObj := ExtractCNPJ(result.CNPJ)
	if cnpjObj == nil {
		return nil, fmt.Errorf("CNPJ inválido")
	}

	// Enriquece com dados da ReceitaWS
	if result.Nome != "" {
		cnpjObj.RazaoSocial = result.Nome
	}
	if result.Fantasia != "" {
		cnpjObj.NomeFantasia = result.Fantasia
	}
	if result.Telefone != "" {
		cnpjObj.Telefones = append(cnpjObj.Telefones, result.Telefone)
	}

	// Adiciona CNAE principal
	if len(result.AtividadePrincipal) > 0 {
		cnpjObj.CNAE = result.AtividadePrincipal[0].Code
		cnpjObj.CNAEDesc = result.AtividadePrincipal[0].Text
	}

	// Adiciona sócios
	for _, qsa := range result.QSA {
		if qsa.Nome != "" {
			cnpjObj.Socios = append(cnpjObj.Socios, qsa.Nome)
		}
	}

	return cnpjObj, nil
}

// SimpleHTTPSearcher realiza buscas genéricas via HTTP
type SimpleHTTPSearcher struct {
	BaseURL string
}

func NewSimpleHTTPSearcher(baseURL string) *SimpleHTTPSearcher {
	return &SimpleHTTPSearcher{BaseURL: baseURL}
}

func (s *SimpleHTTPSearcher) Name() string {
	return "Simple HTTP Scraper"
}

func (s *SimpleHTTPSearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
	searchURL := s.BaseURL + "?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer requisição: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	text := doc.Text()

	if cnpj := ExtractCNPJ(text); cnpj != nil {
		return cnpj, nil
	}

	return nil, fmt.Errorf("CNPJ não encontrado")
}

type CNPJSearcher struct{}

func NewCNPJSearcher() *CNPJSearcher {
	return &CNPJSearcher{}
}

func (c *CNPJSearcher) Name() string {
	return "Sites de Consulta CNPJ"
}

func (c *CNPJSearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
	sites := []string{
		"https://www.google.com/search?q=",
		"https://cnpj.biz/?q=",
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	for _, site := range sites {
		searchURL := site + url.QueryEscape(query)

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			continue
		}

		text := string(body)

		if cnpj := ExtractCNPJ(text); cnpj != nil {
			return cnpj, nil
		}
	}

	return nil, fmt.Errorf("CNPJ não encontrado em nenhum site")
}

type DuckDuckGoSearcher struct{}

func NewDuckDuckGoSearcher() *DuckDuckGoSearcher {
	return &DuckDuckGoSearcher{}
}

func (d *DuckDuckGoSearcher) Name() string {
	return "DuckDuckGo Search"
}

func (d *DuckDuckGoSearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer requisição: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	var results []string
	doc.Find(".result__snippet").Each(func(i int, s *goquery.Selection) {
		results = append(results, s.Text())
	})

	fullText := strings.Join(results, " ")
	if cnpj := ExtractCNPJ(fullText); cnpj != nil {
		return cnpj, nil
	}

	return nil, fmt.Errorf("CNPJ não encontrado no DuckDuckGo")
}

// EnrichFromReceitaWS tenta enriquecer dados usando ReceitaWS
func EnrichFromReceitaWS(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	searcher := NewReceitaWSSearcher(cnpj.Number)
	enriched, err := searcher.Search(ctx, "")
	if err != nil {
		return err
	}

	// Atualiza apenas campos vazios
	if cnpj.RazaoSocial == "" && enriched.RazaoSocial != "" {
		cnpj.RazaoSocial = enriched.RazaoSocial
	}
	if cnpj.NomeFantasia == "" && enriched.NomeFantasia != "" {
		cnpj.NomeFantasia = enriched.NomeFantasia
	}
	if len(cnpj.Telefones) == 0 && len(enriched.Telefones) > 0 {
		cnpj.Telefones = enriched.Telefones
	}
	if len(cnpj.Socios) == 0 && len(enriched.Socios) > 0 {
		cnpj.Socios = enriched.Socios
	}
	if cnpj.CNAE == "" && enriched.CNAE != "" {
		cnpj.CNAE = enriched.CNAE
		cnpj.CNAEDesc = enriched.CNAEDesc
	}
	return nil
}
