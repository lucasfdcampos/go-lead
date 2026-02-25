package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lucasfdcampos/find-leads/pkg/leads"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	enrichCNPJ := flag.Bool("enrich-cnpj", false, "Enriquecer leads com dados de CNPJ (razão social, sócios, CNAE, situação)")
	enrichInstagram := flag.Bool("enrich-instagram", false, "Enriquecer leads com perfil do Instagram (handle + seguidores)")
	flag.Parse()

	query := "loja de roupas"
	location := "Arapongas-PR"

	args := flag.Args()
	if len(args) >= 2 {
		query = args[0]
		location = args[1]
	} else if len(args) == 1 {
		query = args[0]
	}

	city, state := leads.ParseLocation(location)

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    FIND LEADS - Buscador                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("  Busca : %s\n", query)
	fmt.Printf("  Local : %s - %s\n", city, state)
	if *enrichCNPJ {
		fmt.Println("  CNPJ  : ativado")
	}
	if *enrichInstagram {
		fmt.Println("  IG    : ativado")
	}
	fmt.Printf("  Início: %s\n\n", time.Now().Format("02/01/2006 15:04:05"))

	geoapifyKey := os.Getenv("GEOAPIFY_API_KEY")
	tomtomKey := os.Getenv("TOMTOM_API_KEY")
	groqKey := os.Getenv("GROQ_API_KEY")
	geminiKey := os.Getenv("GEMINI_API_KEY")

	searchers := []leads.Searcher{
		leads.NewOverpassScraper(),
		leads.NewSolutudoScraper(),
		leads.NewGuiaMaisScraper(),
		leads.NewAppLocalScraper(),
		leads.NewApontadorScraper(),
		leads.NewTeleListasScraper(),
		leads.NewDDGLeadScraper(),
		leads.NewBingLeadScraper(),
		leads.NewBraveLeadScraper(),
		leads.NewYandexLeadScraper(),
	}

	if geoapifyKey != "" {
		searchers = append([]leads.Searcher{leads.NewGeoapifyScraper(geoapifyKey)}, searchers...)
	}
	if tomtomKey != "" {
		searchers = append([]leads.Searcher{leads.NewTomTomScraper(tomtomKey)}, searchers...)
	}
	if groqKey != "" {
		searchers = append(searchers, leads.NewGroqScraper(groqKey))
	}
	if geminiKey != "" {
		searchers = append(searchers, leads.NewGeminiScraper(geminiKey))
	}

	// Timeout total: 5 min de scraping + até 30 min de enriquecimento
	totalTimeout := 5 * time.Minute
	if *enrichCNPJ {
		totalTimeout += 20 * time.Minute
	}
	if *enrichInstagram {
		totalTimeout += 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	found, results := leads.SearchAll(ctx, query, location, searchers...)

	// ── Enriquecimento ────────────────────────────────────────────────────────
	if *enrichCNPJ || *enrichInstagram {
		fmt.Printf("\n  💡 Enriquecendo %d leads...", len(found))
		leads.EnrichAll(ctx, found, leads.EnrichOptions{
			CNPJ:      *enrichCNPJ,
			Instagram: *enrichInstagram,
		})
		fmt.Println(" OK")
	}

	fmt.Println()
	leads.PrintResults(found)

	slug := strings.ReplaceAll(strings.ToLower(query), " ", "_")
	citySlug := strings.ToLower(city)
	csvFile := fmt.Sprintf("leads_%s_%s.csv", slug, citySlug)
	if err := leads.SaveCSV(found, csvFile); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao salvar CSV: %v\n", err)
	} else {
		fmt.Printf("\n✓ Resultado salvo em: %s\n", csvFile)
	}

	fmt.Println("\n─── Resumo por fonte ───────────────────────────────────────────")
	for _, r := range results {
		status := "✓"
		detail := fmt.Sprintf("%d leads", len(r.Leads))
		if r.Err != nil {
			status = "✗"
			detail = r.Err.Error()
		}
		fmt.Printf("  %s %-25s %s\n", status, r.Source, detail)
	}
	fmt.Printf("\nTotal: %d leads únicos encontrados\n", len(found))
}
