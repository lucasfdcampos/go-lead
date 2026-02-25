package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasfdcampos/find-cnpj/pkg/cnpj"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║       Teste de Enriquecimento de CNPJ        ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// CNPJ da Di Mazzo para testar
	cnpjNumber := "04309163000101"

	fmt.Printf("🔍 Testando CNPJ: %s\n\n", cnpjNumber)

	// Teste 1: BrasilAPI
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Println("📍 Teste 1: BrasilAPI")
	fmt.Println("═══════════════════════════════════════════════")

	ctx := context.Background()
	searcher := cnpj.NewBrasilAPISearcher(cnpjNumber)
	result, err := searcher.Search(ctx, "")

	if err != nil {
		fmt.Printf("❌ Erro: %v\n", err)
	} else {
		fmt.Printf("✅ Sucesso!\n")
		fmt.Printf("   Razão Social: %s\n", result.RazaoSocial)
		fmt.Printf("   Nome Fantasia: %s\n", result.NomeFantasia)
		fmt.Printf("   Telefones: %d encontrados\n", len(result.Telefones))
		for _, tel := range result.Telefones {
			fmt.Printf("      • %s\n", tel)
		}
		fmt.Printf("   Sócios: %d encontrados\n", len(result.Socios))
		for i, socio := range result.Socios {
			fmt.Printf("      %d. %s\n", i+1, socio)
		}
	}

	// Teste 2: cnpj.biz scraper
	fmt.Println("\n═══════════════════════════════════════════════")
	fmt.Println("📍 Teste 2: cnpj.biz scraper (fallback)")
	fmt.Println("═══════════════════════════════════════════════")

	ctx2, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scraper := cnpj.NewCNPJBizScraper()
	result2, err2 := scraper.Search(ctx2, cnpjNumber)

	if err2 != nil {
		fmt.Printf("❌ Erro: %v\n", err2)
	} else {
		fmt.Printf("✅ Sucesso!\n")
		fmt.Printf("   Razão Social: %s\n", result2.RazaoSocial)
		fmt.Printf("   Nome Fantasia: %s\n", result2.NomeFantasia)
		fmt.Printf("   Telefones: %d encontrados\n", len(result2.Telefones))
		for _, tel := range result2.Telefones {
			fmt.Printf("      • %s\n", tel)
		}
		fmt.Printf("   Sócios: %d encontrados\n", len(result2.Socios))
		for i, socio := range result2.Socios {
			fmt.Printf("      %d. %s\n", i+1, socio)
		}
	}

	// Teste 3: EnrichCNPJData (com fallback automático)
	fmt.Println("\n═══════════════════════════════════════════════")
	fmt.Println("📍 Teste 3: EnrichCNPJData (automático)")
	fmt.Println("═══════════════════════════════════════════════")

	testCNPJ := &cnpj.CNPJ{
		Number:    cnpjNumber,
		Formatted: cnpj.ExtractCNPJ(cnpjNumber).Formatted,
	}

	ctx3 := context.Background()
	if err := cnpj.EnrichCNPJData(ctx3, testCNPJ); err != nil {
		fmt.Printf("❌ Erro: %v\n", err)
	} else {
		fmt.Printf("✅ Sucesso!\n")
		fmt.Printf("   Razão Social: %s\n", testCNPJ.RazaoSocial)
		fmt.Printf("   Nome Fantasia: %s\n", testCNPJ.NomeFantasia)
		fmt.Printf("   Telefones: %d encontrados\n", len(testCNPJ.Telefones))
		for _, tel := range testCNPJ.Telefones {
			fmt.Printf("      • %s\n", tel)
		}
		fmt.Printf("   Sócios: %d encontrados\n", len(testCNPJ.Socios))
		for i, socio := range testCNPJ.Socios {
			fmt.Printf("      %d. %s\n", i+1, socio)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════")
	fmt.Println("✅ Testes concluídos!")
	fmt.Println("═══════════════════════════════════════════════")
}
