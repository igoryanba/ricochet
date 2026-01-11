package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/igoryan-dao/ricochet/internal/eval"
)

func main() {
	casesPath := flag.String("cases", "evals/cases", "Path to test cases directory")
	model := flag.String("model", "claude-3-5-sonnet-20241022", "Model to use for evaluation")
	flag.Parse()

	// 1. Load Test Cases
	files, err := os.ReadDir(*casesPath)
	if err != nil {
		log.Fatalf("Failed to read cases directory: %v", err)
	}

	var testCases []eval.TestCase
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(*casesPath, f.Name()))
			if err != nil {
				log.Printf("Warning: Failed to read %s: %v", f.Name(), err)
				continue
			}
			var tc eval.TestCase
			if err := json.Unmarshal(data, &tc); err != nil {
				log.Printf("Warning: Failed to parse %s: %v", f.Name(), err)
				continue
			}
			testCases = append(testCases, tc)
		}
	}

	if len(testCases) == 0 {
		fmt.Println("No test cases found.")
		return
	}

	fmt.Printf("🚀 Starting Evaluation Suite (%d cases) using model %s\n", len(testCases), *model)
	fmt.Println("------------------------------------------------------------")

	// 2. Run Evaluations
	runner := eval.NewRunner(&eval.Config{
		Model:     *model,
		MaxTurns:  10,
		MaxTokens: 8192,
	})

	summary := eval.Summary{
		Total:    len(testCases),
		Results:  []eval.Result{},
		Duration: 0,
	}

	suiteStartTime := time.Now()
	for _, tc := range testCases {
		fmt.Printf("Running [%s] %s... ", tc.ID, tc.Description)

		res, err := runner.Run(context.Background(), &tc)
		if err != nil {
			fmt.Printf("❌ CRITICAL ERROR: %v\n", err)
			summary.Failed++
			continue
		}

		if res.Success {
			fmt.Println("✅ PASSED")
			summary.Passed++
		} else {
			fmt.Println("❌ FAILED")
			summary.Failed++
			for _, e := range res.Errors {
				fmt.Printf("   - %s\n", e)
			}
		}
		summary.Results = append(summary.Results, *res)
	}
	summary.Duration = time.Since(suiteStartTime)

	// 3. Print Summary
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("🏁 Evaluation Complete in %v\n", summary.Duration)
	fmt.Printf("📊 Summary: %d Total, %d Passed, %d Failed\n", summary.Total, summary.Passed, summary.Failed)

	if summary.Failed > 0 {
		os.Exit(1)
	}
}
