package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// Definition represents a code symbol definition
type Definition struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // function, struct, interface, const
	Signature string `json:"signature,omitempty"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
}

// ParseGo parses a Go file and extracts definitions
func ParseGo(content []byte) ([]Definition, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	var definitions []Definition

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Functions and Methods
			def := Definition{
				Name:      x.Name.Name,
				Type:      "function",
				LineStart: fset.Position(x.Pos()).Line,
				LineEnd:   fset.Position(x.End()).Line,
			}
			if x.Recv != nil {
				def.Type = "method"
			}
			definitions = append(definitions, def)

		case *ast.GenDecl:
			// Types (Structs, Interfaces)
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						typeStr := "type"
						if _, ok := typeSpec.Type.(*ast.StructType); ok {
							typeStr = "struct"
						} else if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
							typeStr = "interface"
						}

						definitions = append(definitions, Definition{
							Name:      typeSpec.Name.Name,
							Type:      typeStr,
							LineStart: fset.Position(typeSpec.Pos()).Line,
							LineEnd:   fset.Position(typeSpec.End()).Line,
						})
					}
				}
			}
		}
		return true
	})

	return definitions, nil
}
