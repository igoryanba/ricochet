package index

import (
	"strings"
)

// GraphNode represents a file in the dependency graph
type GraphNode struct {
	FilePath     string
	Imports      []string
	ReferencedBy []string
	PageRank     float64
	OutDegree    int
}

// DependencyGraph manages file relationships
type DependencyGraph struct {
	Nodes map[string]*GraphNode
}

// NewDependencyGraph creates a new graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Nodes: make(map[string]*GraphNode),
	}
}

// BuildGraph constructs the graph from documents with "imports" metadata
func (g *DependencyGraph) BuildGraph(docs []Document) {
	// 1. Init nodes (unique by FilePath)
	for _, doc := range docs {
		if _, exists := g.Nodes[doc.FilePath]; !exists {
			g.Nodes[doc.FilePath] = &GraphNode{
				FilePath:     doc.FilePath,
				Imports:      make([]string, 0),
				ReferencedBy: make([]string, 0),
				PageRank:     1.0, // Initial value
			}
		}
	}

	// 2. Link nodes using imports
	// We need to map imports to file paths. This is heuristic-based without a real build system.
	for _, doc := range docs {
		node := g.Nodes[doc.FilePath]

		val, ok := doc.Metadata["imports"]
		if !ok {
			continue
		}

		// Handle []interface{} (from JSON unmarshal) or []string (fresh)
		var imports []string
		if strImports, ok := val.([]string); ok {
			imports = strImports
		} else if interfaceImports, ok := val.([]interface{}); ok {
			for _, v := range interfaceImports {
				if s, ok := v.(string); ok {
					imports = append(imports, s)
				}
			}
		}

		for _, imp := range imports {
			target := g.resolveImport(imp)
			if target != nil && target.FilePath != node.FilePath {
				// Avoid duplicate edges
				isUnique := true
				for _, existing := range node.Imports {
					if existing == target.FilePath {
						isUnique = false
						break
					}
				}
				if isUnique {
					node.Imports = append(node.Imports, target.FilePath)
					target.ReferencedBy = append(target.ReferencedBy, node.FilePath)
					node.OutDegree++
				}
			}
		}
	}
}

// resolveImport attempts to Find the target node for an import string
func (g *DependencyGraph) resolveImport(imp string) *GraphNode {
	// 1. Exact path match (rare)
	if node, ok := g.Nodes[imp]; ok {
		return node
	}

	// 2. Suffix match
	// e.g. import "agent" -> matches "core/internal/agent/agent.go" ? No, usually package name.
	// But in JS: import "./utils" -> matches "src/utils.js"

	var bestNode *GraphNode

	for path, node := range g.Nodes {
		// Normalize path separators
		normPath := strings.ReplaceAll(path, "\\", "/")

		// Check if the import path is a suffix of the file path
		// We strip extensions from file path for comparison
		pathNoExt := strings.TrimSuffix(normPath, ".go")
		pathNoExt = strings.TrimSuffix(pathNoExt, ".js")
		pathNoExt = strings.TrimSuffix(pathNoExt, ".ts")

		// Heuristic: Import "github.com/org/repo/internal/agent" matches ".../internal/agent/..."
		if strings.HasSuffix(pathNoExt, imp) {
			// If we have multiple matches, pick the shortest one (usually the most direct)
			// or the one that matches most specifically?
			// For now, just pick the first one found.
			bestNode = node
			break
		}
	}

	return bestNode
}

// ComputePageRank calculates importance scores
func (g *DependencyGraph) ComputePageRank(damping float64, iterations int) {
	N := float64(len(g.Nodes))
	if N == 0 {
		return
	}

	// Initialize PR to 1/N
	for _, node := range g.Nodes {
		node.PageRank = 1.0 / N
	}

	for i := 0; i < iterations; i++ {
		newPR := make(map[string]float64)

		for path, node := range g.Nodes {
			rank := (1.0 - damping) / N
			rankSum := 0.0

			for _, refPath := range node.ReferencedBy {
				refNode := g.Nodes[refPath]
				if refNode.OutDegree > 0 {
					rankSum += refNode.PageRank / float64(refNode.OutDegree)
				}
			}

			rank += damping * rankSum
			newPR[path] = rank
		}

		for path, pr := range newPR {
			g.Nodes[path].PageRank = pr
		}
	}
}
