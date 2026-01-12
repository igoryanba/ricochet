package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/igoryan-dao/ricochet/internal/codegraph"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("🚀 Initializing CodeGraph in %s...\n", cwd)
	service := codegraph.NewService()

	// 1. Rebuild
	start := time.Now()
	if err := service.Rebuild(cwd); err != nil {
		log.Fatalf("❌ Rebuild failed: %v", err)
	}
	fmt.Printf("✅ Graph built in %v. Files: %d\n", time.Since(start), len(service.GetAllFiles()))

	// 2. PageRank
	fmt.Println("📊 Calculating PageRank...")
	prStart := time.Now()
	service.CalculatePageRank()
	fmt.Printf("✅ PageRank calculated in %v\n", time.Since(prStart))

	// 3. Generate Repo Map
	fmt.Println("\n🗺️  Generating Repo Map (Top 20 files):")
	repoMap := service.GenerateRepoMap(20)

	fmt.Println("---------------------------------------------------")
	fmt.Println(repoMap)
	fmt.Println("---------------------------------------------------")
}
