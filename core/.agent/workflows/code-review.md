---
description: Automated code review using multiple specialized agents (Silent Failure Hunter, PR Test Analyzer) with a final consolidated report.
---

# Unified Code Review Workflow

This workflow performs a comprehensive audit of recent changes or a specific target.

## Step 1: Silent Failure Analysis
> **Silent Failure Hunter** is checking for swallowed errors...

Action: ` + "`" + `!mode silent-failure-hunter
Check the changes in {{input}} (or recent changes if not specified) for swallowed errors, empty catch blocks, or unsafe error handling. Report only HIGH confidence issues.` + "`" + `

## Step 2: Test Coverage Analysis
> **PR Test Analyzer** is verifying test quality...

Action: ` + "`" + `!mode pr-test-analyzer
Analyze the test coverage for {{input}}. Are the added tests sufficient? Do they cover edge cases? Report only CRITICAL gaps.` + "`" + `

## Step 3: General Code Audit
> **Code Auditor** is checking for style and logic...

Action: ` + "`" + `!mode auditor
Review {{input}} for general code quality, variable naming, and logic simplifications.` + "`" + `

## Step 4: Scorekeeper & Consolidation
> **Scorekeeper** is filtering and formatting the report...

Action: ` + "`" + `!mode code
You are the Scorekeeper. Review the findings from the previous 3 steps.
1. Filter out any trivial or low-confidence issues (score < 80).
2. Consolidate the remaining issues into a single report.
3. Format as a GitHub PR comment (Markdown).
4. Ignore "nitpicks". Focus on bugs, security risks, and maintainability.

Output the final report.
` + "`" + `
