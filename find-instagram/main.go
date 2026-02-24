package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lucasfdcampos/find-instagram/pkg/instagram"
)

func main() {
	// Query de exemplo
	query := "dimazzo arapongas instagram"

	// Você pode mudar a query pela linha de comando
	if len(os.Args) > 1 {
		query = ""
		for i := 1; i < len(os.Args); i++ {
			query += os.Args[i] + " "
		}
	}

	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║   Busca de Instagram com Múltiplas Estratégias║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Printf("\n📝 Query: %s\n\n", query)

	// Configura todas as estratégias disponíveis (ordem de prioridade)
	searchers := setupSearchers()

	// Contexto com timeout geral
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Executa busca com fallback
	result := instagram.SearchWithFallback(ctx, query, searchers...)

	// Exibe resultado
	fmt.Println("\n═══════════════════════════════════════════════")
	if result.Error != nil {
		fmt.Printf("❌ Erro: %v\n", result.Error)
		fmt.Printf("⏱️  Tempo total: %v\n", result.Duration)
		os.Exit(1)
	}

	fmt.Println("✅ INSTAGRAM ENCONTRADO!")
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Printf("📊 Fonte: %s\n", result.Source)
	fmt.Printf("📱 Handle: %s\n", result.Instagram.Formatted)
	fmt.Printf("🔗 URL: %s\n", result.Instagram.URL)
	fmt.Printf("⏱️  Tempo de busca: %v\n", result.Duration)
	fmt.Println("═══════════════════════════════════════════════")
}

func setupSearchers() []instagram.Searcher {
	var searchers []instagram.Searcher

	// 1. DuckDuckGo (Gratuito, sem rate limit agressivo)
	searchers = append(searchers, instagram.NewDuckDuckGoSearcher())

	// 2. Bing (Gratuito, fallback confiável)
	searchers = append(searchers, instagram.NewBingSearcher())

	// 3. Google (Gratuito, mas com rate limit mais agressivo)
	searchers = append(searchers, instagram.NewGoogleSearcher())

	// 4. Instagram Profile Checker (Tenta adivinhar handles)
	searchers = append(searchers, instagram.NewInstagramProfileChecker())

	return searchers
}
