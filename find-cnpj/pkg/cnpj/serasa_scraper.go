package cnpj

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

// SerasaExperianScraper busca dados no Serasa Experian
type SerasaExperianScraper struct{}

func NewSerasaExperianScraper() *SerasaExperianScraper {
	return &SerasaExperianScraper{}
}

func (s *SerasaExperianScraper) Name() string {
	return "Serasa Experian"
}

func (s *SerasaExperianScraper) Search(ctx context.Context, cnpjNumber string) (*CNPJ, error) {
	if cnpjNumber == "" {
		return nil, fmt.Errorf("CNPJ não fornecido")
	}

	// Remove formatação
	cnpjClean := regexp.MustCompile(`\D`).ReplaceAllString(cnpjNumber, "")
	if len(cnpjClean) != 14 {
		return nil, fmt.Errorf("CNPJ inválido")
	}

	// Formato: cnpj-formatado-empresanome-cnpjlimpo
	// Ex: 63.940.409-julia-maria-constantino---me-63940409000108
	cnpjFormatted := formatCNPJ(cnpjClean)

	// Tenta formato direto (pode não funcionar sem nome)
	// Vamos tentar padrão genérico
	urlPath := fmt.Sprintf("%s-empresa-%s", cnpjFormatted, cnpjClean)
	serasaURL := fmt.Sprintf("https://empresas.serasaexperian.com.br/consulta-gratis/%s", urlPath)

	req, err := http.NewRequestWithContext(ctx, "GET", serasaURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	time.Sleep(1 * time.Second)

	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
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

	cnpjObj := &CNPJ{
		Number:    cnpjClean,
		Formatted: cnpjFormatted,
	}

	// Busca razão social
	doc.Find("h1, h2, h3, .company-name, .razao-social, [class*='name']").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text != "" && !strings.Contains(strings.ToLower(text), "cnpj") && len(text) > 5 {
			if cnpjObj.RazaoSocial == "" || len(text) > len(cnpjObj.RazaoSocial) {
				cnpjObj.RazaoSocial = text
			}
		}
	})

	// Busca nome fantasia
	doc.Find(".nome-fantasia, [class*='fantasia'], [class*='trade-name']").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text != "" && len(text) > 3 {
			cnpjObj.NomeFantasia = text
		}
	})

	// Busca telefones
	telefones := extractTelefones(doc.Text())
	cnpjObj.Telefones = telefones

	// Busca sócios
	doc.Find("*").Each(func(i int, sel *goquery.Selection) {
		text := strings.ToLower(sel.Text())
		if strings.Contains(text, "sócio") || strings.Contains(text, "administrador") ||
			strings.Contains(text, "proprietário") || strings.Contains(text, "qsa") {
			// Busca próximos elementos
			sel.NextAll().Each(func(j int, next *goquery.Selection) {
				nome := strings.TrimSpace(next.Text())
				if nome != "" && len(nome) > 5 && len(nome) < 100 {
					// Verifica se parece um nome
					if regexp.MustCompile(`^[A-ZÀ-Ú][a-zà-ú]+ [A-ZÀ-Ú]`).MatchString(nome) {
						// Evita duplicatas
						found := false
						for _, s := range cnpjObj.Socios {
							if s == nome {
								found = true
								break
							}
						}
						if !found {
							cnpjObj.Socios = append(cnpjObj.Socios, nome)
						}
					}
				}
			})
		}
	})

	// Busca CNAE em elementos de texto genérico
	doc.Find("*").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		ltext := strings.ToLower(text)
		if strings.Contains(ltext, "cnae") || strings.Contains(ltext, "atividade principal") {
			if cnpjObj.CNAE == "" {
				cnpjObj.CNAE, cnpjObj.CNAEDesc = parseCNAEField(text)
			}
		}
	})

	// Se não encontrou nada relevante
	if cnpjObj.RazaoSocial == "" && len(cnpjObj.Telefones) == 0 && len(cnpjObj.Socios) == 0 {
		return nil, fmt.Errorf("nenhum dado encontrado no Serasa Experian")
	}

	return cnpjObj, nil
}

// extractTelefones extrai telefones de um texto
func extractTelefones(text string) []string {
	var telefones []string
	seen := make(map[string]bool)

	// Regex para telefones brasileiros
	phoneRegex := regexp.MustCompile(`\(?(\d{2})\)?[\s\-]?(\d{4,5})[\s\-]?(\d{4})`)
	matches := phoneRegex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			telefone := fmt.Sprintf("(%s) %s-%s", match[1], match[2], match[3])
			if !seen[telefone] {
				telefones = append(telefones, telefone)
				seen[telefone] = true
			}
		}
	}

	return telefones
}

// EnrichFromSerasaExperian tenta enriquecer dados usando Serasa Experian
func EnrichFromSerasaExperian(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	scraper := NewSerasaExperianScraper()
	enriched, err := scraper.Search(ctx, cnpj.Number)
	if err != nil {
		return err
	}

	// Atualiza apenas se não tiver dados ou se os novos forem mais completos
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

// BuildSerasaURL constrói URL do Serasa a partir de CNPJ e nome
func BuildSerasaURL(cnpj, nomeEmpresa string) string {
	cnpjClean := regexp.MustCompile(`\D`).ReplaceAllString(cnpj, "")
	cnpjFormatted := formatCNPJ(cnpjClean)

	// Normaliza nome da empresa para URL
	nomeURL := strings.ToLower(nomeEmpresa)
	nomeURL = strings.ReplaceAll(nomeURL, " ", "-")
	nomeURL = strings.ReplaceAll(nomeURL, ".", "")
	nomeURL = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(nomeURL, "")

	// Remove traços múltiplos
	nomeURL = regexp.MustCompile(`-+`).ReplaceAllString(nomeURL, "-")
	nomeURL = strings.Trim(nomeURL, "-")

	// Formato: cnpj-formatado-nome-empresa-cnpjlimpo
	urlPath := fmt.Sprintf("%s-%s-%s", cnpjFormatted, nomeURL, cnpjClean)
	return fmt.Sprintf("https://empresas.serasaexperian.com.br/consulta-gratis/%s", url.PathEscape(urlPath))
}
