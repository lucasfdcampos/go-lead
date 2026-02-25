package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lucasfdcampos/find-cnpj/pkg/cnpj"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║   Teste de Rate Limit - DuckDuckGo Search    ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// Lista de empresas para testar
	empresas := []string{
		"dimazzo arapongas cnpj",
		"magazine luiza cnpj",
		"coca cola brasil cnpj",
		"petrobras cnpj",
		"google brasil cnpj",
		"amazon brasil cnpj",
		"natura cnpj",
		"athrium arapongas cnpj",
		"itau cnpj",
		"bradesco cnpj",
		"nubank cnpj",
		"mercado livre brasil cnpj",
		"americanas cnpj",
		"casas bahia cnpj",
		"carrefour brasil cnpj",
		"pao de acucar cnpj",
		"extra cnpj",
		"globo cnpj",
		"record cnpj",
		"sbt cnpj",
	}

	searcher := cnpj.NewDuckDuckGoSearcher()

	successCount := 0
	failureCount := 0
	totalDuration := time.Duration(0)

	fmt.Printf("🧪 Testando %d consultas sequenciais...\n\n", len(empresas))

	startTime := time.Now()

	for i, empresa := range empresas {
		fmt.Printf("[%2d/%2d] Buscando: %-40s ", i+1, len(empresas), empresa)

		queryStartTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := searcher.Search(ctx, empresa)
		cancel()

		queryDuration := time.Since(queryStartTime)
		totalDuration += queryDuration

		if err != nil {
			fmt.Printf("❌ Falhou (%.2fs) - %v\n", queryDuration.Seconds(), err)
			failureCount++

			// Se falhar, pode ser rate limit - vamos verificar
			if i < 5 { // Se falhar nas primeiras 5, não é rate limit
				continue
			} else {
				fmt.Println("\n⚠️  POSSÍVEL RATE LIMIT DETECTADO!")
				fmt.Printf("   Falhou na consulta #%d\n", i+1)
				fmt.Printf("   Aguardando 10 segundos para confirmar...\n\n")
				time.Sleep(10 * time.Second)

				// Tenta novamente
				fmt.Printf("   Tentando novamente: %s ", empresa)
				ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
				result2, err2 := searcher.Search(ctx2, empresa)
				cancel2()

				if err2 == nil && result2 != nil {
					fmt.Printf("✅ Recuperou! CNPJ: %s\n\n", result2.Formatted)
					fmt.Println("📊 CONCLUSÃO: DuckDuckGo tem rate limit temporário.")
					fmt.Println("   Solução: Adicionar delay entre requisições.")
					break
				} else {
					fmt.Printf("❌ Ainda falhou\n\n")
				}
				break
			}
		} else if result != nil {
			fmt.Printf("✅ %s (%.2fs)\n", result.Formatted, queryDuration.Seconds())
			successCount++
		} else {
			fmt.Printf("⚠️  Não encontrado (%.2fs)\n", queryDuration.Seconds())
			failureCount++
		}

		// Pequeno delay entre requisições (educado)
		if i < len(empresas)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	endTime := time.Now()
	totalTime := endTime.Sub(startTime)
	avgTime := totalDuration / time.Duration(successCount+failureCount)

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("📊 RESULTADOS DO TESTE")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Printf("✅ Sucessos:        %d/%d (%.1f%%)\n", successCount, len(empresas), float64(successCount)/float64(len(empresas))*100)
	fmt.Printf("❌ Falhas:          %d/%d (%.1f%%)\n", failureCount, len(empresas), float64(failureCount)/float64(len(empresas))*100)
	fmt.Printf("⏱️  Tempo total:     %v\n", totalTime)
	fmt.Printf("⏱️  Tempo médio:     %v por consulta\n", avgTime)
	fmt.Printf("🚀 Throughput:      %.2f consultas/minuto\n", float64(successCount+failureCount)/totalTime.Minutes())
	fmt.Println(strings.Repeat("═", 80))

	// Análise de rate limit
	fmt.Println("\n💡 ANÁLISE DE RATE LIMIT:")
	fmt.Println(strings.Repeat("─", 80))

	if failureCount == 0 {
		fmt.Println("✅ EXCELENTE! Nenhuma falha detectada.")
		fmt.Println("   DuckDuckGo NÃO aplicou rate limit neste teste.")
		fmt.Printf("   Você pode fazer até ~%.0f consultas por hora tranquilamente.\n", 60/avgTime.Minutes()*60)
	} else if float64(failureCount)/float64(len(empresas)) < 0.2 {
		fmt.Println("⚡ BOM! Poucas falhas detectadas.")
		fmt.Println("   Rate limit é leve ou inexistente.")
		fmt.Println("   Recomendação: Delay de 500ms-1s entre requisições.")
	} else {
		fmt.Println("⚠️  ATENÇÃO! Muitas falhas detectadas.")
		fmt.Println("   Possível rate limit aplicado.")
		fmt.Println("   Recomendação: Delay de 2-5s entre requisições.")
	}

	fmt.Println(strings.Repeat("─", 80))
	fmt.Println("\n📝 RECOMENDAÇÕES PARA LISTA DE EMPRESAS:")
	fmt.Println("   1. Adicione delay de 500-1000ms entre consultas")
	fmt.Println("   2. Use ChromeDP como fallback (mais lento mas sem rate limit)")
	fmt.Println("   3. Considere cache/banco de dados para CNPJs já encontrados")
	fmt.Println("   4. Processe em lotes com pausa maior a cada 50 consultas")
	fmt.Println()
}
