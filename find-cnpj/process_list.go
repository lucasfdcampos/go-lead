package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"go-lead/pkg/cnpj"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run process_list.go <arquivo.txt>")
		fmt.Println()
		fmt.Println("Formatos suportados:")
		fmt.Println("  - arquivo.txt (um nome por linha)")
		fmt.Println("  - arquivo.csv (formato: nome,cidade)")
		fmt.Println()
		fmt.Println("Exemplo de arquivo.txt:")
		fmt.Println("  dimazzo arapongas")
		fmt.Println("  magazine luiza")
		fmt.Println("  coca cola brasil")
		os.Exit(1)
	}

	filename := os.Args[1]
	
	// Configuração de delays para evitar rate limit (valores conservadores)
	delayBetweenQueries := 2 * time.Second   // Delay entre cada consulta (aumentado)
	delayBetweenBatches := 10 * time.Second  // Delay a cada lote de 25 (mais frequente)
	batchSize := 25                          // Lotes menores para evitar bloqueio

	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║   Processamento em Lote de Lista de CNPJs    ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("📁 Arquivo: %s\n", filename)
	fmt.Printf("⏱️  Delay entre consultas: %v\n", delayBetweenQueries)
	fmt.Printf("📦 Tamanho do lote: %d (pausa de %v a cada lote)\n", batchSize, delayBetweenBatches)
	fmt.Println()

	// Ler arquivo
	empresas, err := readFile(filename)
	if err != nil {
		fmt.Printf("❌ Erro ao ler arquivo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Total de empresas: %d\n\n", len(empresas))

	// Setup searchers (sem Google)
	searchers := []cnpj.Searcher{
		cnpj.NewDuckDuckGoSearcher(),
		cnpj.NewCNPJSearcher(),
	}

	// Criar arquivo de output
	outputFile, err := os.Create("resultados_cnpj.csv")
	if err != nil {
		fmt.Printf("❌ Erro ao criar arquivo de saída: %v\n", err)
		os.Exit(1)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	// Header do CSV
	writer.Write([]string{"Nome", "CNPJ", "CNPJ_Formatado", "Fonte", "Tempo_ms", "Status"})

	// Processar lista
	successCount := 0
	failureCount := 0
	startTime := time.Now()

	for i, empresa := range empresas {
		fmt.Printf("[%3d/%3d] %-50s ", i+1, len(empresas), empresa)

		query := empresa + " cnpj"
		
		// Timeout maior para evitar interrupções
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		
		queryStart := time.Now()
		// Usar versão quiet para não poluir output
		result := cnpj.SearchWithFallbackQuiet(ctx, query, searchers...)
		queryDuration := time.Since(queryStart)
		cancel()

		if result.Error == nil && result.CNPJ != nil {
			fmt.Printf("✅ %s (%s, %.2fs)\n", result.CNPJ.Formatted, result.Source, queryDuration.Seconds())
			writer.Write([]string{
				empresa,
				result.CNPJ.Number,
				result.CNPJ.Formatted,
				result.Source,
				fmt.Sprintf("%.0f", queryDuration.Milliseconds()),
				"sucesso",
			})
			successCount++
		} else {
			fmt.Printf("❌ Não encontrado (%.2fs)\n", queryDuration.Seconds())
			writer.Write([]string{
				empresa,
				"",
				"",
				"",
				fmt.Sprintf("%.0f", queryDuration.Milliseconds()),
				"falha",
			})
			failureCount++
		}

		// Flush CSV a cada 10 registros
		if (i+1)%10 == 0 {
			writer.Flush()
		}

		// Força flush imediato para não perder dados em caso de interrupção
		writer.Flush()
		
		// Delay entre consultas
		if i < len(empresas)-1 {
			time.Sleep(delayBetweenQueries)
		}

		// Pausa maior a cada lote
		if (i+1)%batchSize == 0 && i < len(empresas)-1 {
			remaining := len(empresas) - (i + 1)
			elapsed := time.Since(startTime)
			avgTime := elapsed / time.Duration(i+1)
			estimatedRemaining := avgTime * time.Duration(remaining)
			
			fmt.Printf("\n⏸️  Pausa de %v após %d consultas...\n", delayBetweenBatches, batchSize)
			fmt.Printf("   📊 Progresso: %d/%d (%.1f%%)\n", i+1, len(empresas), float64(i+1)/float64(len(empresas))*100)
			fmt.Printf("   ⏱️  Tempo decorrido: %v\n", elapsed.Round(time.Second))
			fmt.Printf("   ⏱️  Tempo estimado restante: %v\n", estimatedRemaining.Round(time.Second))
			fmt.Printf("   ✅ Sucessos até agora: %d/%d (%.1f%%)\n\n", successCount, i+1, float64(successCount)/float64(i+1)*100)
			
			// Força flush antes da pausa
			writer.Flush()
			time.Sleep(delayBetweenBatches)
		}
	}

	totalTime := time.Since(startTime)
	totalRequests := successCount + failureCount

	fmt.Println("\n" + "═══════════════════════════════════════════════════════════")
	fmt.Println("📊 RESUMO FINAL")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("✅ CNPJs encontrados:  %d/%d (%.1f%%)\n", successCount, totalRequests, float64(successCount)/float64(totalRequests)*100)
	fmt.Printf("❌ Não encontrados:    %d/%d (%.1f%%)\n", failureCount, totalRequests, float64(failureCount)/float64(totalRequests)*100)
	fmt.Printf("⏱️  Tempo total:        %v\n", totalTime)
	fmt.Printf("⏱️  Tempo médio:        %.2fs por consulta\n", totalTime.Seconds()/float64(totalRequests))
	fmt.Printf("🚀 Throughput:         %.2f consultas/minuto\n", float64(totalRequests)/totalTime.Minutes())
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("\n💾 Resultados salvos em: resultados_cnpj.csv\n")
}

func readFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var empresas []string
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			empresas = append(empresas, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return empresas, nil
}
