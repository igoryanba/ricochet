---
description: Structured feature development workflow (Discovery -> Exploration -> Architecture -> Implementation -> Review)
---

# Feature Development Workflow

This workflow guides you through the 5 phases of building a robust feature.

## Phase 1: Discovery
[User Input] What feature do you want to build? Be specific about requirements and constraints.

## Phase 2: Exploration (Map the Terrain)
> **Explorer Agent** is analyzing the codebase...

Action: ` + "`" + `!explorer "Trace the execution paths relevant to: {{input}}. List key files and existing patterns to follow."` + "`" + `

## Phase 3: Architecture (Design the Solution)
> **Architect Agent** is designing the blueprint...

Action: ` + "`" + `!architect "Based on the explorer's findings, design a solution for: {{input}}. Create a strict implementation plan (` + "`" + `PLAN.md` + "`" + `) listing every file change."` + "`" + `

## Phase 4: Implementation (Build It)
> **Code Agent** is writing the code...

Action: ` + "`" + `!code "Implement the feature following ` + "`" + `PLAN.md` + "`" + ` exactly. Update ` + "`" + `PLAN.md` + "`" + ` as you complete items."` + "`" + `

## Phase 5: Review (Verify Quality)
> **Simplifier Agent** is reviewing the changes...

Action: ` + "`" + `!simplifier "Review the changes made in Phase 4. Refactor for clarity and project patterns. Ensure no new technical debt was introduced."` + "`" + `

## Completion
✅ Feature complete.
Recommended Next Steps:
- Run tests: ` + "`" + `!test "Run tests for the new feature"` + "`" + `
- Create PR: ` + "`" + `!workflow commit-push-pr` + "`" + `
