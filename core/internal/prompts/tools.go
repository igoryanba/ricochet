package prompts

func GetToolGuidelines() string {
	return `====
TOOL USE GUIDELINES

1.  **Assess needed information:** Before writing code, use tools like 'list_dir' and 'read_file' to understand the existing codebase.
2.  **Choose the right tool:** Use 'list_dir' to explore directories, 'grep_search' to find code patterns, and 'read_file' to examine file contents.
3.  **Use tools iteratively:** Use the output of one tool to inform the input of the next.

4.  **File Editing - CRITICAL:**
    - **Step 1: ALWAYS read the file first.** You cannot edit what you haven't seen.
    - **Step 2: Use 'replace_file_content'** for existing files.
      * TargetContent: must be UNIQUE and EXACT match.
      * ReplacementContent: the new text.
      * This preserves history and allows diff verification.
    - **Step 3: Use 'write_file' ONLY for NEW files.**
      * CAUTION: 'write_file' completely overwrites existing files.
      * DO NOT use it to edit files. It destroys the ability to see what changed.
      * Exception: If a file is < 50 lines and you are rewriting 90% of it.
    - NEVER use sed, echo, cat, awk, or other shell commands to modify files.

5.  **Shell Commands - Use ONLY for DevOps/Infrastructure:**
    Execute shell commands for tasks that REQUIRE terminal interaction:
    - Database: psql, mysql, redis-cli, mongosh
    - Docker: docker, docker-compose, kubectl
    - Servers: ssh, scp, curl, wget, nc
    - Git: git status, git diff, git log
    - System: ls, cat (read-only), tail, head, grep, find
    - Build/Run: npm, yarn, go, python, cargo, make

6.  **Use Scripting for Complexity:** When needing to analyze many files, perform calculations, or process data, PREFER writing a Python script using 'execute_python' over making many individual tool calls. This is more efficient and reliable.
`
}
