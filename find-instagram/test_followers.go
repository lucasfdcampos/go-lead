package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasfdcampos/find-instagram/pkg/instagram"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║       Teste de Busca de Seguidores           ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// Handles para testar
	handles := []string{
		"dimazzomenswear",
		"nike",
		"cocacola",
	}

	for _, handle := range handles {
		fmt.Println("═══════════════════════════════════════════════")
		fmt.Printf("📱 Testando: @%s\n", handle)
		fmt.Println("═══════════════════════════════════════════════")

		// Teste 1: InstaStoriesViewer
		fmt.Println("\n📍 Tentando InstaStoriesViewer...")
		ctx1, cancel1 := context.WithTimeout(context.Background(), 20*time.Second)
		scraper1 := instagram.NewInstaStoriesViewerScraper()
		result1, err1 := scraper1.Search(ctx1, handle)
		cancel1()

		if err1 != nil {
			fmt.Printf("   ❌ Erro: %v\n", err1)
		} else {
			fmt.Printf("   ✅ Sucesso! Seguidores: %s\n", result1.Followers)
		}

		// Teste 2: StoryNavigation
		fmt.Println("\n📍 Tentando StoryNavigation...")
		ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
		scraper2 := instagram.NewStoryNavigationScraper()
		result2, err2 := scraper2.Search(ctx2, handle)
		cancel2()

		if err2 != nil {
			fmt.Printf("   ❌ Erro: %v\n", err2)
		} else {
			fmt.Printf("   ✅ Sucesso! Seguidores: %s\n", result2.Followers)
		}

		// Teste 3: EnrichInstagramFollowers (automático com fallback)
		fmt.Println("\n📍 Tentando EnrichInstagramFollowers (automático)...")
		testInsta := instagram.NewInstagram(handle)
		ctx3, cancel3 := context.WithTimeout(context.Background(), 30*time.Second)
		err3 := instagram.EnrichInstagramFollowers(ctx3, testInsta)
		cancel3()

		if err3 != nil {
			fmt.Printf("   ❌ Erro: %v\n", err3)
		} else {
			fmt.Printf("   ✅ Sucesso! Seguidores: %s\n", testInsta.Followers)
		}

		fmt.Println()
		time.Sleep(2 * time.Second)
	}

	fmt.Println("═══════════════════════════════════════════════")
	fmt.Println("✅ Testes concluídos!")
	fmt.Println("═══════════════════════════════════════════════")
}
