package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-lead/pkg/cnpj"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run process_list_safe.go <arquivo.txt>")
		fmt.Println()
		fmt.Println("🛡️  Versão SEGURA com:")
		fmt.Println("   ✅ Salvamento contínuo")
		fmt.Println("   ✅ Retry automático")
		fmt.Println("   ✅ Recuperação de progresso")
		fmt.Println("   ✅ Delays conservadores")
		fmt.Println()
		fmt.Println("Exemplo:")
		fmt.Println("  go run process_list_safe.go empresas.txt")
		os.Exit(1)
	}

	filename := os.Args[1]

	// Configuração conservadora para evitar rate limit
	delayBetweenQueries := 2 * time.Second
	delayBetweenBatches := 15 * time.Second
	delayAfterError := 5 * time.Second
	batchSize := 20
	maxRetries := 2

	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║  🛡️  Processamento SEGURO de Lista de CNPJs  ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("📁 Arquivo: %s\n", filename)
	fmt.Printf("⏱️  Delay entre consultas: %v\n", delayBetweenQueries)
	fmt.Printf("⏱️  Delay após erro: %v\n", delayAfterError)
	fmt.Printf("📦 Tamanho do lote: %d (pausa de %v)\n", batchSize, delayBetweenBatches)
	fmt.Printf("🔄 Tentativas por empresa: %d\n", maxRetries)
	fmt.Println()

	// Ler arquivo
	empresas, err := readFile(filename)
	if err != nil {
		fmt.Printf("❌ Erro ao ler arquivo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Total de empresas: %d\n", len(empresas))
	fmt.Printf("⏱️  Tempo estimado: ~%v\n\n", estimateTime(len(empresas), delayBetweenQueries, batchSize, delayBetweenBatches))

	// Setup searchers
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
	writer.Write([]string{"Nome", "CNPJ", "CNPJ_Formatado", "Razao_Social", "Nome_Fantasia", "Telefones", "Socios", "Fonte", "Tempo_ms", "Tentativas", "Status"})
	writer.Flush()

	// Capturar Ctrl+C para salvar antes de sair
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan bool)
	interrupted := false

	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Interrupção detectada! Salvando progresso...")
		interrupted = true
		writer.Flush()
		outputFile.Sync()
		fmt.Println("✅ Progresso salvo em resultados_cnpj.csv")
		fmt.Println("💡 Você pode continuar depois implementando retomada!")
		done <- true
	}()

	// Processar lista
	successCount := 0
	failureCount := 0
	startTime := time.Now()

	for i, empresa := range empresas {
		if interrupted {
			break
		}

		fmt.Printf("[%3d/%3d] %-45s ", i+1, len(empresas), truncate(empresa, 45))

		query := empresa + " cnpj"

		var result *cnpj.SearchResult
		var attempts int

		// Retry loop
		for attempts = 1; attempts <= maxRetries; attempts++ {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)

			queryStart := time.Now()
			result = cnpj.SearchWithFallbackQuiet(ctx, query, searchers...)
			queryDuration := time.Since(queryStart)
			cancel()

			// Se teve sucesso, sai do loop
			if result.Error == nil && result.CNPJ != nil {
				// Enriquecer com dados adicionais (sócios e telefones)
				enrichCtx, enrichCancel := context.WithTimeout(context.Background(), 15*time.Second)
				enrichErr := cnpj.EnrichCNPJData(enrichCtx, result.CNPJ)
				enrichCancel()

				fmt.Printf("✅ %s (%s, %.1fs", result.CNPJ.Formatted, result.Source, queryDuration.Seconds())
				if attempts > 1 {
					fmt.Printf(", %d tentativas", attempts)
				}
				if enrichErr != nil {
					fmt.Printf(", sem dados adicionais")
				}
				fmt.Printf(")\n")

				// Formatar telefones e sócios para CSV (separados por ;)
				telefones := ""
				if len(result.CNPJ.Telefones) > 0 {
					for i, tel := range result.CNPJ.Telefones {
						if i > 0 {
							telefones += "; "
						}
						telefones += tel
					}
				}

				socios := ""
				if len(result.CNPJ.Socios) > 0 {
					for i, socio := range result.CNPJ.Socios {
						if i > 0 {
							socios += "; "
						}
						socios += socio
					}
				}

				writer.Write([]string{
					empresa,
					result.CNPJ.Number,
					result.CNPJ.Formatted,
					result.CNPJ.RazaoSocial,
					result.CNPJ.NomeFantasia,
					telefones,
					socios,
					result.Source,
					fmt.Sprintf("%.0f", queryDuration.Milliseconds()),
					fmt.Sprintf("%d", attempts),
					"sucesso",
				})
				successCount++
				break
			}

			// Se falhou e ainda tem tentativas
			if attempts < maxRetries {
				fmt.Printf("⚠️  tentativa %d falhou, aguardando %v...\n", attempts, delayAfterError)
				time.Sleep(delayAfterError)
				fmt.Printf("[%3d/%3d] %-45s ", i+1, len(empresas), truncate(empresa, 45))
			}
		}

		// Se todas as tentativas falharam
		if result.Error != nil || result.CNPJ == nil {
			fmt.Printf("❌ Não encontrado após %d tentativas\n", attempts)
			writer.Write([]string{
				empresa,
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"0",
				fmt.Sprintf("%d", attempts),
				"falha",
			})
			failureCount++
		}

		// Salva imediatamente
		writer.Flush()
		outputFile.Sync()

		// Delay entre consultas
		if i < len(empresas)-1 && !interrupted {
			time.Sleep(delayBetweenQueries)
		}

		// Pausa maior a cada lote
		if (i+1)%batchSize == 0 && i < len(empresas)-1 && !interrupted {
			printProgress(i+1, len(empresas), successCount, failureCount, startTime, delayBetweenBatches)
			writer.Flush()
			outputFile.Sync()
			time.Sleep(delayBetweenBatches)
		}
	}

	totalTime := time.Since(startTime)
	totalRequests := successCount + failureCount

	if !interrupted {
		fmt.Println("\n" + "═══════════════════════════════════════════════════════════")
		fmt.Println("📊 RESUMO FINAL")
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("✅ CNPJs encontrados:  %d/%d (%.1f%%)\n", successCount, totalRequests, float64(successCount)/float64(totalRequests)*100)
		fmt.Printf("❌ Não encontrados:    %d/%d (%.1f%%)\n", failureCount, totalRequests, float64(failureCount)/float64(totalRequests)*100)
		fmt.Printf("⏱️  Tempo total:        %v\n", totalTime.Round(time.Second))
		fmt.Printf("⏱️  Tempo médio:        %.1fs por consulta\n", totalTime.Seconds()/float64(totalRequests))
		fmt.Printf("🚀 Throughput:         %.1f consultas/hora\n", float64(totalRequests)/totalTime.Hours())
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("\n💾 Resultados salvos em: resultados_cnpj.csv\n")
	}

	select {
	case <-done:
		os.Exit(0)
	default:
	}
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

func printProgress(current, total, success, failure int, startTime time.Time, pauseDuration time.Duration) {
	remaining := total - current
	elapsed := time.Since(startTime)
	avgTime := elapsed / time.Duration(current)
	estimatedRemaining := avgTime * time.Duration(remaining)

	fmt.Printf("\n⏸️  Pausa de %v após %d consultas...\n", pauseDuration, current)
	fmt.Printf("   📊 Progresso: %d/%d (%.1f%%)\n", current, total, float64(current)/float64(total)*100)
	fmt.Printf("   ✅ Sucessos: %d (%.1f%%)\n", success, float64(success)/float64(current)*100)
	fmt.Printf("   ❌ Falhas: %d (%.1f%%)\n", failure, float64(failure)/float64(current)*100)
	fmt.Printf("   ⏱️  Decorrido: %v\n", elapsed.Round(time.Second))
	fmt.Printf("   ⏱️  Estimado restante: %v\n", estimatedRemaining.Round(time.Second))
	fmt.Printf("   🎯 Previsão de término: %v\n\n", time.Now().Add(estimatedRemaining).Format("15:04:05"))
}

func estimateTime(total int, delayQuery time.Duration, batchSize int, delayBatch time.Duration) time.Duration {
	batches := total / batchSize
	queryTime := time.Duration(total) * (delayQuery + 3*time.Second) // 3s média por query
	batchTime := time.Duration(batches) * delayBatch
	return queryTime + batchTime
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
