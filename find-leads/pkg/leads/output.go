package leads

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// PrintResults exibe leads no terminal de forma organizada
func PrintResults(leads []*Lead) {
	if len(leads) == 0 {
		fmt.Println("  Nenhum lead encontrado.")
		return
	}

	fmt.Printf("\n%-4s %-45s %-18s %-35s %-20s\n", "#", "NOME", "TELEFONE", "ENDEREÇO", "FONTE")
	fmt.Println(strings.Repeat("─", 130))

	for i, l := range leads {
		name := truncate(l.Name, 44)
		phone := l.Phone
		if phone == "" {
			phone = l.Phone2
		}
		if phone == "" {
			phone = "-"
		}
		addr := truncate(l.Address, 34)
		if addr == "" {
			addr = "-"
		}
		source := truncate(firstSource(l.Source), 19)

		fmt.Printf("%-4d %-45s %-18s %-35s %-20s\n", i+1, name, phone, addr, source)
	}

	fmt.Println(strings.Repeat("─", 130))
	fmt.Printf("Total: %d leads únicos\n", len(leads))
}

// SaveCSV salva leads em arquivo CSV
func SaveCSV(leads []*Lead, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// BOM para Excel visualizar UTF-8 corretamente
	f.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"#", "Nome", "Telefone", "Telefone2", "Endereco", "Cidade", "Estado",
		"Categoria", "Website", "Email", "CNPJ", "Avaliacao", "Fontes"}
	if err := w.Write(header); err != nil {
		return err
	}

	for i, l := range leads {
		row := []string{
			fmt.Sprintf("%d", i+1),
			l.Name,
			l.Phone,
			l.Phone2,
			l.Address,
			l.City,
			l.State,
			l.Category,
			l.Website,
			l.Email,
			l.CNPJ,
			l.Rating,
			l.Source,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func firstSource(s string) string {
	parts := strings.SplitN(s, "+", 2)
	return parts[0]
}
