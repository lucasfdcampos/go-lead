package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("❌ Uso: find-instagram <nome da empresa>")
		fmt.Println("\nExemplo:")
		fmt.Println("  find-instagram \"Magazine Luiza\"")
		os.Exit(1)
	}

	query := os.Args[1]
	
	fmt.Printf("🔍 Buscando Instagram para: %s\n\n", query)
	fmt.Println("🚧 Sistema em desenvolvimento...")
	
	// TODO: Implementar estratégias de busca
}
