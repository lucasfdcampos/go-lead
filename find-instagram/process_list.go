package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lucasfdcampos/find-instagram/pkg/instagram"
)

type Resultado struct {
	Nome       string
	Handle     string
	URL        string
	Followers  string
	Fonte      string
	Tempo      int64
	Tentativas int
	Status     string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("❌ Uso: process-list <arquivo.txt>")
		fmt.Println("\nExemplo:")
		fmt.Println("  process-list empresas.txt")
		os.Exit(1)
	}

	filename := os.Args[1]

	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║  🛡️  Processamento SEGURO de Lista de Instagram ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// Configurações
	delayBetweenQueries := 2 * time.Second
	delayAfterError := 5 * time.Second
	batchSize := 20
	delayBetweenBatches := 15 * time.Second
	queryTimeout := 45 * time.Second
	maxRetries := 2

	fmt.Printf("📁 Arquivo: %s\n", filename)
	fmt.Printf("⏱️  Delay entre consultas: %v\n", delayBetweenQueries)
	fmt.Printf("⏱️  Delay após erro: %v\n", delayAfterError)
	fmt.Printf("📦 Tamanho do lote: %d (pausa de %v)\n", batchSize, delayBetweenBatches)
	fmt.Printf("🔄 Tentativas por empresa: %d\n\n", maxRetries)

	// Ler arquivo
	empresas, err := readFile(filename)
	if err != nil {
		fmt.Printf("❌ Erro ao ler arquivo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Total de empresas: %d\n", len(empresas))
	fmt.Printf("⏱️  Tempo estimado: ~%v\n\n", estimateTime(len(empresas), delayBetweenQueries, batchSize, delayBetweenBatches))

	// Setup searchers
	searchers := []instagram.Searcher{
		instagram.NewDuckDuckGoSearcher(),
		instagram.NewBingSearcher(),
	}

	// Criar arquivo de output
	outputFile, err := os.Create("resultados_instagram.csv")
	if err != nil {
		fmt.Printf("❌ Erro ao criar arquivo de saída: %v\n", err)
		os.Exit(1)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	// Escrever header
	writer.Write([]string{"Nome", "Handle", "URL", "Followers", "Fonte", "Tempo_ms", "Tentativas", "Status"})
	writer.Flush()

	// Captura Ctrl+C para salvar progresso
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\n\n⚠️  Interrompido pelo usuário. Salvando progresso...")
		writer.Flush()
		outputFile.Close()
		fmt.Println("💾 Progresso salvo em: resultados_instagram.csv")
		os.Exit(0)
	}()

	// Processar empresas
	var resultados []Resultado
	sucessos := 0
	falhas := 0
	startTime := time.Now()

	for i, empresa := range empresas {
		empresa = strings.TrimSpace(empresa)
		if empresa == "" {
			continue
		}

		fmt.Printf("[%3d/%3d] %-50s ", i+1, len(empresas), truncate(empresa, 50))

		var resultado Resultado
		resultado.Nome = empresa

		// Tentar com retry
		found := false
		for tentativa := 1; tentativa <= maxRetries && !found; tentativa++ {
			resultado.Tentativas = tentativa

			// Contexto com timeout
			ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)

			// Buscar
			queryStart := time.Now()
			searchResult := instagram.SearchWithFallbackQuiet(ctx, empresa, searchers...)
			queryDuration := time.Since(queryStart)

			cancel()

			if searchResult.Error == nil && searchResult.Instagram != nil {
				// Sucesso
				resultado.Handle = searchResult.Instagram.Handle
				resultado.URL = searchResult.Instagram.URL
				resultado.Fonte = searchResult.Source
				resultado.Tempo = queryDuration.Milliseconds()
				resultado.Status = "sucesso"
				found = true
				sucessos++

				// Buscar seguidores
				followersCtx, followersCancel := context.WithTimeout(context.Background(), 20*time.Second)
				if err := instagram.EnrichInstagramFollowers(followersCtx, searchResult.Instagram); err == nil {
					resultado.Followers = searchResult.Instagram.Followers
				}
				followersCancel()

				followersInfo := ""
				if resultado.Followers != "" {
					followersInfo = fmt.Sprintf(" [%s seguidores]", resultado.Followers)
				}

				fmt.Printf("✅ %s%s (%s, %.1fs)\n", 
					searchResult.Instagram.Formatted,
					followersInfo,
					searchResult.Source,
					queryDuration.Seconds())
			} else {
				// Falha nessa tentativa
				if tentativa < maxRetries {
					fmt.Printf("⚠️  Tentativa %d falhou, aguardando %v...\n", tentativa, delayAfterError)
					time.Sleep(delayAfterError)
				} else {
					// Falha definitiva
					resultado.Status = "não_encontrado"
					resultado.Tempo = queryDuration.Milliseconds()
					falhas++
					fmt.Printf("❌ Não encontrado\n")
				}
			}
		}

		// Salvar resultado
		resultados = append(resultados, resultado)
		writeResultado(writer, resultado)
		writer.Flush() // Flush imediato para não perder dados

		// Delay entre consultas
		if i < len(empresas)-1 {
			time.Sleep(delayBetweenQueries)

			// Pausa maior a cada lote
			if (i+1)%batchSize == 0 {
				elapsed := time.Since(startTime)
				remaining := len(empresas) - (i + 1)
				fmt.Println()
				printProgress(i+1, len(empresas), sucessos, falhas, elapsed, remaining, delayBetweenQueries, delayBetweenBatches)
				fmt.Printf("\n⏸️  Pausa de %v para respeitar rate limit...\n\n", delayBetweenBatches)
				time.Sleep(delayBetweenBatches)
			}
		}
	}

	// Resumo final
	totalTime := time.Since(startTime)
	printFinalSummary(len(empresas), sucessos, falhas, totalTime)
}

func readFile(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var empresas []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			empresas = append(empresas, line)
		}
	}

	return empresas, nil
}

func writeResultado(writer *csv.Writer, r Resultado) {
	writer.Write([]string{
		r.Nome,
		r.Handle,
		r.URL,
		r.Followers,
		r.Fonte,
		fmt.Sprintf("%d", r.Tempo),
		fmt.Sprintf("%d", r.Tentativas),
		r.Status,
	})
}

func printProgress(current, total, sucessos, falhas int, elapsed time.Duration, remaining int, delayQuery, delayBatch time.Duration) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("📊 Progresso: %d/%d (%.1f%%)\n", current, total, float64(current)/float64(total)*100)
	fmt.Printf("✅ Encontrados: %d\n", sucessos)
	fmt.Printf("❌ Não encontrados: %d\n", falhas)

	avgTime := elapsed / time.Duration(current)
	estimatedRemaining := time.Duration(remaining) * avgTime

	fmt.Printf("   ⏱️  Decorrido: %v\n", elapsed.Round(time.Second))
	fmt.Printf("   ⏱️  Estimado restante: %v\n", estimatedRemaining.Round(time.Second))
	fmt.Printf("   🎯 Previsão de término: %v\n\n", time.Now().Add(estimatedRemaining).Format("15:04:05"))
}

func printFinalSummary(total, sucessos, falhas int, duration time.Duration) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("📊 RESUMO FINAL")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("✅ Handles encontrados:  %d/%d (%.1f%%)\n", sucessos, total, float64(sucessos)/float64(total)*100)
	fmt.Printf("❌ Não encontrados:    %d/%d (%.1f%%)\n", falhas, total, float64(falhas)/float64(total)*100)
	fmt.Printf("⏱️  Tempo total:        %v\n", duration.Round(time.Second))
	if total > 0 {
		avgTime := duration / time.Duration(total)
		fmt.Printf("⏱️  Tempo médio:        %v por consulta\n", avgTime.Round(100*time.Millisecond))
		throughput := float64(total) / duration.Hours()
		fmt.Printf("🚀 Throughput:         %.1f consultas/hora\n", throughput)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("💾 Resultados salvos em: resultados_instagram.csv")
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
