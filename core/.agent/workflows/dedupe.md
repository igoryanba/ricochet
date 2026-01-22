---
description: Find semantic duplicates for a GitHub issue
command: /dedupe
---

## Objective
Find up to 3 likely duplicate issues for **Issue #{{input}}**.

### Tools Required
- **GitHub MCP**: Ensure the GitHub MCP server is connected.

### Steps

1. **Analyze Original Issue**
   - Action: "Read Issue #{{input}} using `mcp__github__get_issue`. Summarize its core bug report or feature request keyphrases."

2. **Search Parallels (Parallel)**
   - Type: parallel
   - Parallel:
     - Action: "Search GitHub issues using `mcp__github__search_issues` with keywords from the summary. Focus on error messages."
     - Action: "Search GitHub issues using `mcp__github__search_issues` with different synonyms describing the same user intent."
     - Action: "Search **Closed** issues (`state:closed`) that might have been resolved recently."
     - Action: "Search issues created by the same author to see if they reported it twice."
     - Action: "Search using specific file names or component names mentioned in the issue."

3. **Filter & Report**
   - Action: "Review the search results. Identify the top 3 most likely semantic duplicates. If duplicates are found, output them in a list. If none, say 'No duplicates found'."
