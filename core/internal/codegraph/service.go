package codegraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type Node struct {
	Path        string
	Language    string
	Imports     []string
	Definitions []string
	PageRank    float64
}

type Service struct {
	nodes map[string]*Node
	mu    sync.RWMutex
}

func NewService() *Service {
	return &Service{
		nodes: make(map[string]*Node),
	}
}

func (s *Service) AddFile(path string, content []byte) error {
	lang, queryStr := detectLanguage(path)
	if lang == nil {
		return nil // Unsupported language, ignore
	}

	// Parser instance (not thread safe, so new per file)
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return fmt.Errorf("parsing failed: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()

	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return fmt.Errorf("query creation failed: %w", err)
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, root)

	node := &Node{
		Path:     path,
		Language: extension(path),
	}

	uniqueImports := make(map[string]bool)
	uniqueDefs := make(map[string]bool)

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			name := q.CaptureNameForId(c.Index)
			text := c.Node.Content(content)

			// Clean up quotes for imports
			switch name {
			case "import_path":
				text = strings.Trim(text, "\"`")
				if !uniqueImports[text] {
					node.Imports = append(node.Imports, text)
					uniqueImports[text] = true
				}
			case "def_name":
				if !uniqueDefs[text] {
					node.Definitions = append(node.Definitions, text)
					uniqueDefs[text] = true
				}
			}
		}
	}

	s.mu.Lock()
	s.nodes[path] = node
	s.mu.Unlock()

	return nil
}

func (s *Service) GetNode(path string) *Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes[path]
}

func (s *Service) GetAllFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var files []string
	for k := range s.nodes {
		files = append(files, k)
	}
	return files
}

func (s *Service) Rebuild(root string) error {
	s.mu.Lock()
	// Clear existing? Or update? For now, simple clear.
	s.nodes = make(map[string]*Node)
	s.mu.Unlock()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" || info.Name() == "dist" || info.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".tsx" {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable
		}

		return s.AddFile(path, content)
	})
}

// FindReverseDependencies returns files that import the given path
func (s *Service) FindReverseDependencies(targetImport string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []string
	for path, node := range s.nodes {
		for _, imp := range node.Imports {
			// Naive check: string contains.
			// Ideally we resolve "github.com/foo/bar" to absolute paths.
			// But for now, if imported is "fmt" and we look for "fmt", it matches.
			// For local files, if target is "utils" and import is "./utils", we need fuzzy match or resolution.

			// Simple Exact Match or Suffix Match for MVP
			if imp == targetImport || strings.HasSuffix(imp, targetImport) {
				results = append(results, path)
			}
		}
	}
	return results
}

// GetContext returns the node and immediate dependencies
func (s *Service) GetContext(path string) (*Node, []*Node) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[path]
	if !ok {
		return nil, nil
	}

	var deps []*Node
	// Try to find nodes that match imports
	// This is O(N*M) where N=files, M=imports. Slow for huge repos but OK for MVP.
	// Optimization: Build an inverted index or lookup table by filename/package.

	for _, imp := range node.Imports {
		// Try to resolve import to a file path we know
		// 1. Direct match (rare for paths)
		if n, ok := s.nodes[imp]; ok {
			deps = append(deps, n)
			continue
		}

		// 2. Base name match (common for "./utils")
		// "utils" -> "/path/to/utils.ts" or "/path/to/utils.go"
		baseImp := filepath.Base(imp)
		for p, n := range s.nodes {
			// Check if file basename (minus ext) matches import basename
			fn := filepath.Base(p)
			ext := filepath.Ext(p)
			name := strings.TrimSuffix(fn, ext)

			if name == baseImp {
				deps = append(deps, n)
				// Break inner loop? Maybe multiple matches (ambiguous), but take first for now
				break
			}
		}
	}

	return node, deps
}

// CalculatePageRank computes the importance of each file.
// Algorithm: PR(A) = (1-d) + d * Sum(PR(T)/C(T)) where T links to A
// Here, "Link" means T imports A. So we need reverse dependencies.
func (s *Service) CalculatePageRank() {
	s.mu.Lock()
	defer s.mu.Unlock()

	damping := 0.85
	iterations := 20
	// Base score implicit in formula (1-d)

	// 1. Build Adjacency Graph (Nodes importing X)
	// Map: ImportedFile -> []ImporterFiles
	graph := make(map[string][]string)

	for importerPath, node := range s.nodes {
		node.PageRank = 1.0 // Reset

		for _, imp := range node.Imports {
			// Resolve import to file path
			// Simplified resolution logic similar to GetContext
			baseImp := filepath.Base(imp)

			for candidatePath := range s.nodes {
				// Exact match logic + fuzzy logic
				// If import is "github.com/foo/bar/baz", candidate "/.../baz.go" matches base.
				// This is heuristic.

				// 1. Check if candidate *is* the import (local relative)
				// 2. Check if candidate *package* matches import

				if strings.HasSuffix(candidatePath, imp) || strings.HasSuffix(candidatePath, imp+".go") || strings.HasSuffix(candidatePath, imp+".ts") {
					graph[candidatePath] = append(graph[candidatePath], importerPath)
				} else {
					// Fallback: match by filename base if import looks like local file
					candBase := filepath.Base(candidatePath)
					candName := strings.TrimSuffix(candBase, filepath.Ext(candBase))
					if candName == baseImp {
						graph[candidatePath] = append(graph[candidatePath], importerPath)
					}
				}
			}
		}
	}

	// 2. Iterate
	for i := 0; i < iterations; i++ {
		newScores := make(map[string]float64)

		for path := range s.nodes {
			inboundSum := 0.0

			// Who imports 'path'?
			importers := graph[path]
			for _, importerPath := range importers {
				importerNode := s.nodes[importerPath]
				outDegree := float64(len(importerNode.Imports))
				if outDegree == 0 {
					outDegree = 1
				}
				inboundSum += importerNode.PageRank / outDegree
			}

			newScores[path] = (1 - damping) + damping*inboundSum
		}

		// Update scores
		for path, score := range newScores {
			s.nodes[path].PageRank = score
		}
	}
}

// GenerateRepoMap returns a formatted string of the most important files.
// It formats them as an XML tree for the LLM.
func (s *Service) GenerateRepoMap(maxFiles int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type RankedNode struct {
		Path  string
		Score float64
		Defs  []string
	}

	var ranked []RankedNode
	for path, node := range s.nodes {
		ranked = append(ranked, RankedNode{
			Path:  path,
			Score: node.PageRank,
			Defs:  node.Definitions,
		})
	}

	// Sort by Score descending
	// Simple bubble sort for now, or could use sort package
	for i := 0; i < len(ranked)-1; i++ {
		for j := 0; j < len(ranked)-i-1; j++ {
			if ranked[j].Score < ranked[j+1].Score {
				ranked[j], ranked[j+1] = ranked[j+1], ranked[j]
			}
		}
	}

	if maxFiles > len(ranked) {
		maxFiles = len(ranked)
	}

	var sb strings.Builder
	sb.WriteString("<repository_map>\n")

	for i := 0; i < maxFiles; i++ {
		node := ranked[i]
		// Shorten path relative to CWD if possible?
		// We stored absolute paths usually.
		rel := node.Path
		// Assuming we don't have CWD here effectively, just output the stored path.
		// Or assume stored path is already useful.

		sb.WriteString(fmt.Sprintf("  <file path=\"%s\" score=\"%.2f\">\n", filepath.Base(rel), node.Score))
		for _, def := range node.Defs {
			sb.WriteString(fmt.Sprintf("    <def>%s</def>\n", def))
		}
		sb.WriteString("  </file>\n")
	}
	sb.WriteString("</repository_map>")

	return sb.String()
}

func detectLanguage(path string) (*sitter.Language, string) {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return golang.GetLanguage(), GoQueries
	case ".ts", ".tsx":
		return typescript.GetLanguage(), TypescriptQueries
	default:
		return nil, ""
	}
}

func extension(path string) string {
	return filepath.Ext(path)
}
