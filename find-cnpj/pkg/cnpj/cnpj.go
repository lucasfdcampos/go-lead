package cnpj

import (
	"regexp"
	"strings"
)

// CNPJ representa um CNPJ validado com informações adicionais
type CNPJ struct {
	Number       string   // Apenas números
	Formatted    string   // Com formatação XX.XXX.XXX/XXXX-XX
	RazaoSocial  string   // Razão Social da empresa
	NomeFantasia string   // Nome Fantasia
	Situacao     string   // Situação cadastral (ex: ATIVA, BAIXADA)
	Socios       []string // Lista de sócios
	Telefones    []string // Lista de telefones
	CNAE         string   // CNAE principal (código da atividade econômica)
	CNAEDesc     string   // Descrição do CNAE
	Municipio    string   // Município do estabelecimento
	UF           string   // Unidade Federativa (estado)
}

// ExtractCNPJ extrai o primeiro CNPJ válido de um texto
func ExtractCNPJ(text string) *CNPJ {
	// Regex para CNPJ com ou sem formata\u00e7\u00e3o
	patterns := []string{
		`\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}`, // 00.000.000/0000-00
		`\d{14}`,                          // 00000000000000
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)

		for _, match := range matches {
			// Remove formatação
			numbers := regexp.MustCompile(`\D`).ReplaceAllString(match, "")

			if len(numbers) == 14 && isValidCNPJ(numbers) {
				return &CNPJ{
					Number:    numbers,
					Formatted: formatCNPJ(numbers),
				}
			}
		}
	}

	return nil
}

// formatCNPJ formata o CNPJ no padrão XX.XXX.XXX/XXXX-XX
func formatCNPJ(cnpj string) string {
	if len(cnpj) != 14 {
		return cnpj
	}
	return cnpj[0:2] + "." + cnpj[2:5] + "." + cnpj[5:8] + "/" + cnpj[8:12] + "-" + cnpj[12:14]
}

// isValidCNPJ valida o CNPJ usando o algoritmo de dígitos verificadores
func isValidCNPJ(cnpj string) bool {
	if len(cnpj) != 14 {
		return false
	}

	// Verifica se todos os dígitos são iguais
	allSame := true
	for i := 1; i < len(cnpj); i++ {
		if cnpj[i] != cnpj[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}

	// Calcula primeiro dígito verificador
	sum := 0
	weight := 5
	for i := 0; i < 12; i++ {
		digit := int(cnpj[i] - '0')
		sum += digit * weight
		weight--
		if weight < 2 {
			weight = 9
		}
	}
	firstDigit := 11 - (sum % 11)
	if firstDigit >= 10 {
		firstDigit = 0
	}

	if int(cnpj[12]-'0') != firstDigit {
		return false
	}

	// Calcula segundo dígito verificador
	sum = 0
	weight = 6
	for i := 0; i < 13; i++ {
		digit := int(cnpj[i] - '0')
		sum += digit * weight
		weight--
		if weight < 2 {
			weight = 9
		}
	}
	secondDigit := 11 - (sum % 11)
	if secondDigit >= 10 {
		secondDigit = 0
	}

	return int(cnpj[13]-'0') == secondDigit
}

// ExtractAllCNPJs extrai todos os CNPJs válidos de um texto
func ExtractAllCNPJs(text string) []*CNPJ {
	var cnpjs []*CNPJ
	seen := make(map[string]bool)

	// Regex para CNPJ com ou sem formatação
	patterns := []string{
		`\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}`, // 00.000.000/0000-00
		`\d{14}`,                          // 00000000000000
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)

		for _, match := range matches {
			// Remove formatação
			numbers := regexp.MustCompile(`\D`).ReplaceAllString(match, "")

			if len(numbers) == 14 && isValidCNPJ(numbers) && !seen[numbers] {
				seen[numbers] = true
				cnpjs = append(cnpjs, &CNPJ{
					Number:    numbers,
					Formatted: formatCNPJ(numbers),
				})
			}
		}
	}

	return cnpjs
}

// NormalizarQuery normaliza a query de busca
func NormalizarQuery(query string) string {
	query = strings.ToLower(query)
	query = strings.TrimSpace(query)

	// Adiciona "cnpj" se não estiver presente
	if !strings.Contains(query, "cnpj") {
		query += " cnpj"
	}

	return query
}
