package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"find-leads/pkg/leads"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	query := "loja de roupas"
	location := "Arapongas-PR"

	if len(os.Args) >= 3 {
		query = os.Args[1]
		location = os.Args[2]
	} else if len(os.Args) == 2 {
		query = os.Args[1]
	}

	city, state := leads.ParseLocation(location)

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    FIND LEADS - Buscador                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("  Busca : %s\n", query)
	fmt.Printf("  Local : %s - %s\n", city, state)
	fmt.Printf("  Início: %s\n\n", time.Now().Format("02/01/2006 15:04:05"))

	geoapifyKey := os.Getenv("GEOAPIFY_API_KEY")
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
	if groqKey != "" {
		searchers = append(searchers, leads.NewGroqScraper(groqKey))
	}
	if geminiKey != "" {
		searchers = append(searchers, leads.NewGeminiScraper(geminiKey))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	found, results := leads.SearchAll(ctx, query, location, searchers...)

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
