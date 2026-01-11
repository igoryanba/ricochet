package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ricochetContext "github.com/igoryan-dao/ricochet/internal/context"
)

// Embedder interface for generating embeddings
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Indexer handles the codebase indexing process
type Indexer struct {
	mu            sync.RWMutex
	store         VectorStore
	provider      Embedder
	parser        *ricochetContext.LanguageParser
	workspaceRoot string
	isIndexing    bool
}

func NewIndexer(store VectorStore, provider Embedder, workspaceRoot string) *Indexer {
	return &Indexer{
		store:         store,
		provider:      provider,
		parser:        ricochetContext.NewLanguageParser(),
		workspaceRoot: workspaceRoot,
	}
}

// IndexAll performs a full scan of the workspace
func (idx *Indexer) IndexAll(ctx context.Context) error {
	idx.mu.Lock()
	if idx.isIndexing {
		idx.mu.Unlock()
		return fmt.Errorf("indexing already in progress")
	}
	idx.isIndexing = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.isIndexing = false
		idx.mu.Unlock()
	}()

	var allDocs []Document

	err := filepath.Walk(idx.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden and dependency directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "dist" || name == "out" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if extension is supported by parser
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".py", ".rs", ".go":
			// Go ahead
		default:
			return nil
		}

		docs, err := idx.indexFile(ctx, path)
		if err != nil {
			fmt.Printf("Warning: failed to index file %s: %v\n", path, err)
			return nil // Continue with other files
		}

		allDocs = append(allDocs, docs...)
		return nil
	})

	if err != nil {
		return err
	}

	if len(allDocs) > 0 {
		// Generate embeddings in batches
		batchSize := 20
		for i := 0; i < len(allDocs); i += batchSize {
			end := i + batchSize
			if end > len(allDocs) {
				end = len(allDocs)
			}

			var batchTexts []string
			for _, d := range allDocs[i:end] {
				batchTexts = append(batchTexts, d.Content)
			}

			embeddings, err := idx.provider.Embed(ctx, batchTexts)
			if err != nil {
				return fmt.Errorf("failed to generate embeddings: %w", err)
			}

			for j, emb := range embeddings {
				allDocs[i+j].Embedding = emb
			}
		}

		if err := idx.store.Clear(); err != nil {
			return err
		}
		if err := idx.store.Add(allDocs); err != nil {
			return err
		}
		return idx.store.Save()
	}

	return nil
}

func (idx *Indexer) indexFile(ctx context.Context, path string) ([]Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(idx.workspaceRoot, path)

	// Try to get definitions (functions, classes) for logical chunking
	defs, err := idx.parser.ParseDefinitions(ctx, path, content)
	if err != nil {
		// Fallback to simple line-based chunking if treesitter fails or unsupported
		return idx.chunkSimple(relPath, string(content)), nil
	}

	var docs []Document
	lines := strings.Split(string(content), "\n")

	for _, def := range defs {
		// Ensure line ranges are valid
		start := def.LineStart - 1
		end := def.LineEnd
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start >= end {
			continue
		}

		chunk := strings.Join(lines[start:end], "\n")
		// Limit chunk size to avoid context issues (embeddings usually have tokens limit)
		if len(chunk) > 8000 {
			chunk = chunk[:8000]
		}

		docs = append(docs, Document{
			ID:        fmt.Sprintf("%s:%d-%d", relPath, def.LineStart, def.LineEnd),
			FilePath:  relPath,
			Content:   fmt.Sprintf("// File: %s\n// %s: %s\n%s", relPath, def.Type, def.Name, chunk),
			LineStart: def.LineStart,
			LineEnd:   def.LineEnd,
			Metadata:  map[string]interface{}{"type": def.Type, "name": def.Name},
		})
	}

	// If no definitions found but file is not empty, add one big chunk or simple chunks
	if len(docs) == 0 && len(content) > 0 {
		return idx.chunkSimple(relPath, string(content)), nil
	}

	return docs, nil
}

func (idx *Indexer) chunkSimple(relPath, content string) []Document {
	// Split by ~50 lines if no AST structure found
	lines := strings.Split(content, "\n")
	chunkSize := 50
	var docs []Document

	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}

		chunk := strings.Join(lines[i:end], "\n")
		docs = append(docs, Document{
			ID:        fmt.Sprintf("%s:%d-%d", relPath, i+1, end),
			FilePath:  relPath,
			Content:   fmt.Sprintf("// File: %s (Lines %d-%d)\n%s", relPath, i+1, end, chunk),
			LineStart: i + 1,
			LineEnd:   end,
		})
	}
	return docs
}

func (idx *Indexer) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	emb, err := idx.provider.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(emb) == 0 {
		return nil, nil
	}

	return idx.store.Search(emb[0], limit)
}
