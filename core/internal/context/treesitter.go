package context

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	// Uncomment when grammars are available/needed:
	// "github.com/smacker/go-tree-sitter/python"
	// "github.com/smacker/go-tree-sitter/typescript/typescript"
	// "github.com/smacker/go-tree-sitter/rust"
)

// Definition represents a code symbol (function, class, etc.)
type Definition struct {
	Type      string // "function", "class", "method", "struct", "interface"
	Name      string
	Signature string
	LineStart int
	LineEnd   int
}

// FileAnalysis contains definitions and imports found in a file
type FileAnalysis struct {
	Definitions []Definition
	Imports     []string
}

// LanguageParser wraps Tree-sitter for multi-language AST parsing
type LanguageParser struct {
	parser *sitter.Parser
}

// NewLanguageParser creates a new parser instance
func NewLanguageParser() *LanguageParser {
	return &LanguageParser{
		parser: sitter.NewParser(),
	}
}

// Close releases parser resources
func (lp *LanguageParser) Close() {
	if lp.parser != nil {
		lp.parser.Close()
	}
}

// GetLanguageForFile returns the appropriate tree-sitter language based on file extension
func (lp *LanguageParser) GetLanguageForFile(path string) (*sitter.Language, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".mjs":
		return javascript.GetLanguage(), nil
	case ".go":
		return golang.GetLanguage(), nil
	// Uncomment when grammars are installed:
	// case ".ts", ".tsx":
	// 	return typescript.GetLanguage(), nil
	// case ".py":
	// 	return python.GetLanguage(), nil
	// case ".rs":
	// 	return rust.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported language for extension: %s", ext)
	}
}

// ParseDefinitions extracts code definitions and imports from source code
func (lp *LanguageParser) ParseDefinitions(ctx context.Context, path string, source []byte) (*FileAnalysis, error) {
	lang, err := lp.GetLanguageForFile(path)
	if err != nil {
		return nil, err
	}

	lp.parser.SetLanguage(lang)
	tree, err := lp.parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("empty parse tree")
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".mjs":
		return lp.extractJavaScript(root, source), nil
	case ".go":
		return lp.extractGo(root, source), nil
	// case ".ts", ".tsx":
	// 	return lp.extractTypeScript(root, source), nil
	// case ".py":
	// 	return lp.extractPython(root, source), nil
	// case ".rs":
	// 	return lp.extractRust(root, source), nil
	default:
		return nil, fmt.Errorf("no extractor for %s", ext)
	}
}

func (lp *LanguageParser) extractJavaScript(root *sitter.Node, source []byte) *FileAnalysis {
	var analysis FileAnalysis

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		nodeType := node.Type()

		switch nodeType {
		case "import_statement":
			// import X from 'source'
			if sourceNode := node.ChildByFieldName("source"); sourceNode != nil {
				// remove quotes
				path := strings.Trim(sourceNode.Content(source), "\"'`")
				analysis.Imports = append(analysis.Imports, path)
			}

		case "call_expression":
			// require('source')
			if funcNode := node.ChildByFieldName("function"); funcNode != nil {
				if funcNode.Content(source) == "require" {
					if args := node.ChildByFieldName("arguments"); args != nil {
						if args.NamedChildCount() > 0 {
							arg := args.NamedChild(0)
							// simplistic: only string literals
							if arg.Type() == "string" {
								path := strings.Trim(arg.Content(source), "\"'`")
								analysis.Imports = append(analysis.Imports, path)
							}
						}
					}
				}
			}

		case "function_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "function",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "class_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "class",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "method_definition":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "method",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return &analysis
}

func (lp *LanguageParser) extractGo(root *sitter.Node, source []byte) *FileAnalysis {
	var analysis FileAnalysis

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		nodeType := node.Type()

		switch nodeType {
		case "import_spec":
			if pathNode := node.ChildByFieldName("path"); pathNode != nil {
				path := strings.Trim(pathNode.Content(source), "\"")
				analysis.Imports = append(analysis.Imports, path)
			}

		case "function_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "function",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "method_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			// receiver logic could be added here
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "method",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "type_spec":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			analysis.Definitions = append(analysis.Definitions, Definition{
				Type:      "type",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return &analysis
}
