package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetInitialContext generates the initial context including the file tree
func GetInitialContext(cwd string) string {
	tree := generateFileTree(cwd)
	return fmt.Sprintf(`====
INITIAL CONTEXT

Current Working Directory: %s

File Structure:
%s
`, cwd, tree)
}

func generateFileTree(root string) string {
	var sb strings.Builder
	maxDepth := 3 // Limit depth to avoid massive context

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		// Skip hidden files/dirs (starting with .)
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate depth
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth >= maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s/\n", indent, info.Name()))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
		}

		return nil
	})

	if err != nil {
		return fmt.Sprintf("Error generating file tree: %v", err)
	}

	return sb.String()
}
