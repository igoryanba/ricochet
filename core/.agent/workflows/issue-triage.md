---
description: Auto-triage GitHub issues using MCP
command: /triage
---

## Objective
Identify critical blocking issues in the repository and label them as `oncall`.

### Tools Required
- **GitHub MCP**: Ensure the GitHub MCP server is connected.

### Steps

1. **Fetch Issues**
   - Action: "Fetch the 5 most recently updated open issues using `mcp__github__list_issues` (state='open', orderBy='UPDATED_AT', direction='DESC')."

2. **Analyze Issues (Parallel)**
   - Type: parallel
   - Parallel:
     - Action: "Analyze Issue 1: Read details with `mcp__github__get_issue`. If it's a blocking bug with high engagement (reactions/comments), return 'CRITICAL'. Otherwise 'NORMAL'."
     - Action: "Analyze Issue 2: Read details with `mcp__github__get_issue`. If it's a blocking bug with high engagement (reactions/comments), return 'CRITICAL'. Otherwise 'NORMAL'."
     - Action: "Analyze Issue 3: Read details with `mcp__github__get_issue`. If it's a blocking bug with high engagement (reactions/comments), return 'CRITICAL'. Otherwise 'NORMAL'."
     - Action: "Analyze Issue 4: Read details with `mcp__github__get_issue`. If it's a blocking bug with high engagement (reactions/comments), return 'CRITICAL'. Otherwise 'NORMAL'."
     - Action: "Analyze Issue 5: Read details with `mcp__github__get_issue`. If it's a blocking bug with high engagement (reactions/comments), return 'CRITICAL'. Otherwise 'NORMAL'."

3. **Label Critical Issues**
   - Action: "Review the parallel analysis results. For any issue identified as 'CRITICAL', apply the 'oncall' label using `mcp__github__update_issue`. Provide a summary of actions taken."
