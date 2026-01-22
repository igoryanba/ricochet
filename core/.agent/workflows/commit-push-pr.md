---
description: Automates the git flow: Commit -> Push -> PR
command: /pr
---

## Context
These are the current git details I need you to know before proceeding:
- **Status**: !`git status`
- **Diff**: !`git diff HEAD`
- **Branch**: !`git branch --show-current`

## Objective
Your goal is to safely commit all changes, push them to the origin, and create a Pull Request using the GitHub CLI.

### Steps

1. **Review Changes**
   - Check the `git status` output above.
   - If the branch is `main` or `master`, you MUST stop and ask the user to create a feature branch first (unless it's a hotfix).
   - If there are no changes to commit, inform the user.

2. **Commit**
   - Stage all changes: `git add .`
   - Generate a semantic commit message based on the `git diff` output above.
   - Commit: `git commit -m "..."`

3. **Push**
   - Push to origin: `git push origin <current_branch>`
   - Handle upstream errors if needed (`--set-upstream`).

4. **Pull Request**
   - Create a PR: `gh pr create --fill` (or generate title/body if `gh` supports it, otherwise prompt user).
   - Display the PR URL.

5. **Verification**
   - Run `gh pr view` to confirm it exists.

## Constraints
- Do not force push.
- If `gh` CLI is not installed, stop after pushing and tell the user to open the PR textually.
