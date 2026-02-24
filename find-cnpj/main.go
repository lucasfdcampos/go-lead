package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-lead/pkg/cnpj"

	"github.com/joho/godotenv"
)

func main() {
	// Carrega variáveis de ambiente
	godotenv.Load()

	// Query de exemplo
	query := "dimazzo arapongas cnpj"

	// Você pode mudar a query pela linha de comando
	if len(os.Args) > 1 {
		query = ""
		for i := 1; i < len(os.Args); i++ {
			query += os.Args[i] + " "
		}
	}

	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║     Busca de CNPJ com Múltiplas Estratégias  ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Printf("\n📝 Query: %s\n\n", query)

	// Configura todas as estratégias disponíveis (ordem de prioridade)
	searchers := setupSearchers()

	// Contexto com timeout geral
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Executa busca com fallback
	result := cnpj.SearchWithFallback(ctx, query, searchers...)

	// Exibe resultado
	fmt.Println("\n═══════════════════════════════════════════════")
	if result.Error != nil {
		fmt.Printf("❌ Erro: %v\n", result.Error)
		fmt.Printf("⏱️  Tempo total: %v\n", result.Duration)
		os.Exit(1)
	}

	fmt.Println("✅ CNPJ ENCONTRADO!")
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Printf("📊 Fonte: %s\n", result.Source)
	fmt.Printf("🔢 CNPJ: %s\n", result.CNPJ.Formatted)
	fmt.Printf("📝 Apenas números: %s\n", result.CNPJ.Number)

	// Busca dados adicionais (sócios e telefones)
	fmt.Printf("\n🔍 Buscando dados adicionais...\n")
	if err := cnpj.EnrichCNPJData(ctx, result.CNPJ); err != nil {
		fmt.Printf("⚠️  Aviso: não foi possível obter dados adicionais: %v\n", err)
	} else {
		// Exibe dados adicionais se disponíveis
		if result.CNPJ.RazaoSocial != "" {
			fmt.Printf("\n🏢 Razão Social: %s\n", result.CNPJ.RazaoSocial)
		}
		if result.CNPJ.NomeFantasia != "" {
			fmt.Printf("🏪 Nome Fantasia: %s\n", result.CNPJ.NomeFantasia)
		}

		if len(result.CNPJ.Telefones) > 0 {
			fmt.Printf("\n📞 Telefones:\n")
			for _, tel := range result.CNPJ.Telefones {
				fmt.Printf("   • %s\n", tel)
			}
		}

		if len(result.CNPJ.Socios) > 0 {
			fmt.Printf("\n👥 Sócios (%d):\n", len(result.CNPJ.Socios))
			for i, socio := range result.CNPJ.Socios {
				fmt.Printf("   %d. %s\n", i+1, socio)
			}
		}

		if result.CNPJ.CNAE != "" {
			fmt.Printf("\n🏭 CNAE: %s", result.CNPJ.CNAE)
			if result.CNPJ.CNAEDesc != "" {
				fmt.Printf(" - %s", result.CNPJ.CNAEDesc)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\n⏱️  Tempo total: %v\n", result.Duration)
	fmt.Println("═══════════════════════════════════════════════")
}

func setupSearchers() []cnpj.Searcher {
	var searchers []cnpj.Searcher

	// 1. DuckDuckGo (Gratuito, sem rate limit agressivo, rápido)
	searchers = append(searchers, cnpj.NewDuckDuckGoSearcher())

	// 2. Sites de consulta CNPJ (Gratuito, fallback confiável)
	searchers = append(searchers, cnpj.NewCNPJSearcher())

	// 3. Web Scraping com ChromeDP (Mais robusto, mais lento)
	// NOTA: Requer chromium-browser instalado! (veja Makefile)
	// Descomente para usar:
	// searchers = append(searchers, cnpj.NewChromeDPSearcher(true))

	return searchers
}

// Exemplos de uso alternativo
func exemploUsoEspecifico() {
	ctx := context.Background()

	// Exemplo 1: Buscar com DuckDuckGo apenas
	fmt.Println("=== Exemplo 1: DuckDuckGo ===")
	duckduckgo := cnpj.NewDuckDuckGoSearcher()
	result1, err1 := duckduckgo.Search(ctx, "dimazzo arapongas cnpj")
	if err1 == nil && result1 != nil {
		fmt.Printf("CNPJ encontrado: %s\n", result1.Formatted)
	}

	// Exemplo 2: Buscar com ChromeDP
	fmt.Println("\n=== Exemplo 2: ChromeDP ===")
	chromedp := cnpj.NewChromeDPSearcher(true)
	result2, err2 := chromedp.Search(ctx, "dimazzo arapongas cnpj")
	if err2 == nil && result2 != nil {
		fmt.Printf("CNPJ encontrado: %s\n", result2.Formatted)
	}

	// Exemplo 3: Validar CNPJ com BrasilAPI
	fmt.Println("\n=== Exemplo 3: Validar CNPJ ===")
	cnpjParaValidar := "04309163000101"
	valid, _ := cnpj.ValidateCNPJ(ctx, cnpjParaValidar)
	fmt.Printf("CNPJ %s é válido? %v\n", cnpjParaValidar, valid)

	// Exemplo 4: Extrair CNPJ de texto
	fmt.Println("\n=== Exemplo 4: Extrair de texto ===")
	texto := "A empresa Dimazzo tem CNPJ 04.309.163/0001-01 e atua em Arapongas"
	cnpjExtraido := cnpj.ExtractCNPJ(texto)
	if cnpjExtraido != nil {
		fmt.Printf("CNPJ extraído: %s\n", cnpjExtraido.Formatted)
	}

	// Exemplo 5: Extrair múltiplos CNPJs
	fmt.Println("\n=== Exemplo 5: Múltiplos CNPJs ===")
	textoMultiplo := `
		Empresa 1: 04.309.163/0001-01
		Empresa 2: 00.000.000/0001-91
		Empresa 3: 11.222.333/0001-81
	`
	cnpjs := cnpj.ExtractAllCNPJs(textoMultiplo)
	fmt.Printf("Foram encontrados %d CNPJs válidos\n", len(cnpjs))
	for i, c := range cnpjs {
		fmt.Printf("  %d. %s\n", i+1, c.Formatted)
	}
}
