package context

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	// Uncomment when grammars are available:
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

// ParseDefinitions extracts code definitions from source code
func (lp *LanguageParser) ParseDefinitions(ctx context.Context, path string, source []byte) ([]Definition, error) {
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
		return lp.extractJavaScriptDefinitions(root, source), nil
	case ".ts", ".tsx":
		return lp.extractTypeScriptDefinitions(root, source), nil
	case ".py":
		return lp.extractPythonDefinitions(root, source), nil
	case ".rs":
		return lp.extractRustDefinitions(root, source), nil
	default:
		return nil, fmt.Errorf("no extractor for %s", ext)
	}
}

// extractJavaScriptDefinitions walks the JS AST and extracts definitions
func (lp *LanguageParser) extractJavaScriptDefinitions(root *sitter.Node, source []byte) []Definition {
	var defs []Definition

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		nodeType := node.Type()

		switch nodeType {
		case "function_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
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
			defs = append(defs, Definition{
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
			defs = append(defs, Definition{
				Type:      "method",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "arrow_function":
			// Arrow functions assigned to variables
			parent := node.Parent()
			if parent != nil && parent.Type() == "variable_declarator" {
				if nameNode := parent.ChildByFieldName("name"); nameNode != nil {
					defs = append(defs, Definition{
						Type:      "function",
						Name:      nameNode.Content(source),
						LineStart: int(node.StartPoint().Row) + 1,
						LineEnd:   int(node.EndPoint().Row) + 1,
					})
				}
			}
		}

		// Recurse into children
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return defs
}

// extractTypeScriptDefinitions - similar to JS with interface/type support
func (lp *LanguageParser) extractTypeScriptDefinitions(root *sitter.Node, source []byte) []Definition {
	// TypeScript shares most patterns with JavaScript
	defs := lp.extractJavaScriptDefinitions(root, source)

	// Add TypeScript-specific types
	var walkTS func(node *sitter.Node)
	walkTS = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "interface_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "interface",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "type_alias_declaration":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "type",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walkTS(node.Child(i))
		}
	}

	walkTS(root)
	return defs
}

// extractPythonDefinitions walks Python AST
func (lp *LanguageParser) extractPythonDefinitions(root *sitter.Node, source []byte) []Definition {
	var defs []Definition

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "function_definition":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "function",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "class_definition":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "class",
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
	return defs
}

// extractRustDefinitions walks Rust AST
func (lp *LanguageParser) extractRustDefinitions(root *sitter.Node, source []byte) []Definition {
	var defs []Definition

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "function_item":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "function",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "struct_item":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "struct",
				Name:      name,
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "impl_item":
			defs = append(defs, Definition{
				Type:      "impl",
				Name:      "impl",
				LineStart: int(node.StartPoint().Row) + 1,
				LineEnd:   int(node.EndPoint().Row) + 1,
			})

		case "trait_item":
			name := ""
			if nameNode := node.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}
			defs = append(defs, Definition{
				Type:      "trait",
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
	return defs
}
