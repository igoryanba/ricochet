package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/igoryan-dao/ricochet/internal/index"
)

// MockEmbedder implements index.Embedder
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	fmt.Printf("MockEmbedder: Embedding %d texts\n", len(texts))
	result := make([][]float32, len(texts))
	for i := range texts {
		// Return a dummy embedding of size 3
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func main() {
	cwd, _ := os.Getwd()
	tmpDir := filepath.Join(cwd, "tmp_verify_embeddings")
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	// cleanup later
	// defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "index.vdb")

	// 1. Initialize Store
	store, err := index.NewLocalStore(indexPath)
	if err != nil {
		fmt.Printf("Error creating store: %v\n", err)
		return
	}

	// 2. Initialize Indexer
	embedder := &MockEmbedder{}
	indexer := index.NewIndexer(store, embedder, tmpDir)

	// 3. Create a dummy file to index
	dummyFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(dummyFile, []byte("package main\n\nfunc main() {\n  // Hello world, this is a test file for vector search.\n}\n"), 0644)

	// 4. Index
	fmt.Println("Indexing...")
	err = indexer.IndexAll(context.Background())
	if err != nil {
		fmt.Printf("Indexing failed: %v\n", err)
		return
	}

	// 5. Verify index file existence
	if _, err := os.Stat(indexPath); err == nil {
		fmt.Println("SUCCESS: index.vdb created.")
	} else {
		fmt.Printf("FAILED: index.vdb not found: %v\n", err)
		return
	}

	// 6. Search
	fmt.Println("Searching...")
	results, err := indexer.Search(context.Background(), "world", 1)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	if len(results) > 0 {
		fmt.Printf("SUCCESS: Found %d results.\n", len(results))
		fmt.Printf("Top result: %s (Score: %.4f)\n", results[0].Document.FilePath, results[0].Score)
	} else {
		fmt.Println("FAILED: No results found.")
	}
}
