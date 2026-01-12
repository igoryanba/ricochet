package codegraph

import (
	"testing"
)

func TestAddFile_Go(t *testing.T) {
	s := NewService()
	code := `
		package main
		import "fmt"
		import "github.com/pkg/errors"

		func main() {
			fmt.Println("Hello")
		}
		
		type MyType struct {}
	`

	err := s.AddFile("main.go", []byte(code))
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	node := s.GetNode("main.go")
	if node == nil {
		t.Fatal("Node not found")
	}

	// Check Imports
	foundFmt := false
	foundPkg := false
	for _, imp := range node.Imports {
		if imp == "fmt" {
			foundFmt = true
		}
		if imp == "github.com/pkg/errors" {
			foundPkg = true
		}
	}
	if !foundFmt || !foundPkg {
		t.Errorf("Missing imports. Found: %v", node.Imports)
	}

	// Check Defs
	foundMain := false
	foundType := false
	for _, def := range node.Definitions {
		if def == "main" {
			foundMain = true
		}
		if def == "MyType" {
			foundType = true
		}
	}
	if !foundMain || !foundType {
		t.Errorf("Missing definitions. Found: %v", node.Definitions)
	}
}

func TestAddFile_TS(t *testing.T) {
	s := NewService()
	code := `
		import { foo } from "./utils";
		import * as path from "path";

		function bar() {
			foo();
		}

		class MyClass {}
	`

	err := s.AddFile("index.ts", []byte(code))
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	node := s.GetNode("index.ts")
	if node == nil {
		t.Fatal("Node not found")
	}

	// Check Imports
	foundUtils := false
	for _, imp := range node.Imports {
		if imp == "./utils" {
			foundUtils = true
		}
	}
	if !foundUtils {
		t.Errorf("Missing imports. Found: %v", node.Imports)
	}

	// Check Defs
	foundBar := false
	foundClass := false
	for _, def := range node.Definitions {
		if def == "bar" {
			foundBar = true
		}
		if def == "MyClass" {
			foundClass = true
		}
	}
	if !foundBar || !foundClass {
		t.Errorf("Missing definitions. Found: %v", node.Definitions)
	}
}
