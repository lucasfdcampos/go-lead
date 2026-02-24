package cnpj

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

body, _ := io.ReadAll(resp.Body)
if cnpj := ExtractCNPJ(string(body)); cnpj != nil {
return cnpj, nil
}

return nil, fmt.Errorf("CNPJ não encontrado")
}

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
