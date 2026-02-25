package cnpj

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// CNPJBizScraper busca dados no cnpj.biz
type CNPJBizScraper struct{}

func NewCNPJBizScraper() *CNPJBizScraper {
	return &CNPJBizScraper{}
}

func (c *CNPJBizScraper) Name() string {
	return "CNPJ.biz Scraper"
}

func (c *CNPJBizScraper) Search(ctx context.Context, cnpjNumber string) (*CNPJ, error) {
	if cnpjNumber == "" {
		return nil, fmt.Errorf("CNPJ não fornecido")
	}

	// Remove formatação
	cnpjClean := regexp.MustCompile(`\D`).ReplaceAllString(cnpjNumber, "")
	if len(cnpjClean) != 14 {
		return nil, fmt.Errorf("CNPJ inválido")
	}

	url := fmt.Sprintf("https://cnpj.biz/%s", cnpjClean)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// Delay para respeitar rate limit
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

	cnpjObj := &CNPJ{
		Number:    cnpjClean,
		Formatted: formatCNPJ(cnpjClean),
	}

	// Busca telefones
	telefones := c.extractTelefones(doc)
	cnpjObj.Telefones = telefones

	// Busca sócios no quadro societário
	socios := c.extractSocios(doc)
	cnpjObj.Socios = socios

	// Busca razão social, nome fantasia e CNAE
	doc.Find("table tr").Each(func(i int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("td:first-child").Text())
		value := strings.TrimSpace(s.Find("td:last-child").Text())

		if strings.Contains(label, "Razão Social") || strings.Contains(label, "Nome Empresarial") {
			cnpjObj.RazaoSocial = value
		}
		if strings.Contains(label, "Nome Fantasia") {
			cnpjObj.NomeFantasia = value
		}
		if strings.Contains(label, "CNAE") || strings.Contains(label, "Atividade Principal") {
			// Valor pode ser no formato "4781-4/00 - Comércio varejista"
			cnpjObj.CNAE, cnpjObj.CNAEDesc = parseCNAEField(value)
		}
	})

	if len(cnpjObj.Telefones) == 0 && len(cnpjObj.Socios) == 0 {
		return nil, fmt.Errorf("nenhum dado adicional encontrado")
	}

	return cnpjObj, nil
}

func (c *CNPJBizScraper) extractTelefones(doc *goquery.Document) []string {
	var telefones []string
	seen := make(map[string]bool)

	// Busca por telefones no HTML
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

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
	})

	// Busca específica em tabelas
	doc.Find("table tr").Each(func(i int, s *goquery.Selection) {
		label := strings.ToLower(strings.TrimSpace(s.Find("td:first-child").Text()))
		value := strings.TrimSpace(s.Find("td:last-child").Text())

		if strings.Contains(label, "telefone") || strings.Contains(label, "fone") {
			if value != "" && !seen[value] {
				// Adiciona se ainda não estiver na lista
				phoneRegex := regexp.MustCompile(`\d`)
				if phoneRegex.MatchString(value) {
					telefones = append(telefones, value)
					seen[value] = true
				}
			}
		}
	})

	return telefones
}

func (c *CNPJBizScraper) extractSocios(doc *goquery.Document) []string {
	var socios []string
	seen := make(map[string]bool)

	// Busca por quadro societário
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// Verifica se é a tabela de sócios
		headerText := strings.ToLower(table.Find("th").Text())
		if strings.Contains(headerText, "sóci") || strings.Contains(headerText, "quadro") {
			table.Find("tr").Each(func(j int, row *goquery.Selection) {
				// Pula header
				if j == 0 {
					return
				}

				nome := strings.TrimSpace(row.Find("td").First().Text())
				if nome != "" && !seen[nome] {
					// Limpa nome
					nome = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(nome, " "))
					if len(nome) > 3 { // Nome deve ter pelo menos 3 caracteres
						socios = append(socios, nome)
						seen[nome] = true
					}
				}
			})
		}
	})

	return socios
}

// EnrichCNPJFromCNPJBiz tenta enriquecer dados de um CNPJ usando cnpj.biz
func EnrichCNPJFromCNPJBiz(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	scraper := NewCNPJBizScraper()
	enriched, err := scraper.Search(ctx, cnpj.Number)
	if err != nil {
		return err
	}

	// Atualiza apenas se não tiver dados
	if len(cnpj.Telefones) == 0 && len(enriched.Telefones) > 0 {
		cnpj.Telefones = enriched.Telefones
	}
	if len(cnpj.Socios) == 0 && len(enriched.Socios) > 0 {
		cnpj.Socios = enriched.Socios
	}
	if cnpj.RazaoSocial == "" && enriched.RazaoSocial != "" {
		cnpj.RazaoSocial = enriched.RazaoSocial
	}
	if cnpj.NomeFantasia == "" && enriched.NomeFantasia != "" {
		cnpj.NomeFantasia = enriched.NomeFantasia
	}
	if cnpj.CNAE == "" && enriched.CNAE != "" {
		cnpj.CNAE = enriched.CNAE
		cnpj.CNAEDesc = enriched.CNAEDesc
	}

	return nil
}

// parseCNAEField extrai código e descrição de um campo CNAE
// Formatos suportados:
// "4781-4/00 - Comércio varejista..."
// "47.81-4-00 Comércio varejista..."
// "4781400"
func parseCNAEField(value string) (code, desc string) {
	if value == "" {
		return
	}
	// Tenta extrair código CNAE com hífen/barra: 4781-4/00 ou 47.81-4/00
	re := regexp.MustCompile(`(\d[\d\.\/\-]+\d+)`)
	if m := re.FindStringSubmatch(value); len(m) >= 2 {
		code = m[1]
		// Descrição vem após o código
		for _, sep := range []string{" - ", " – ", " "} {
			if idx := strings.Index(value, sep); idx != -1 && idx > len(code)-2 {
				desc = strings.TrimSpace(value[idx+len(sep):])
				break
			}
		}
	} else {
		desc = value
	}
	return
}
