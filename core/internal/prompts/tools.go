package prompts

func GetToolGuidelines() string {
	return `====
TOOL USE GUIDELINES

1.  **Assess needed information:** Before writing code, use tools like 'list_dir' and 'read_file' to understand the existing codebase.
2.  **Choose the right tool:** Use 'list_dir' to explore directories, 'grep_search' to find code patterns, and 'read_file' to examine file contents.
3.  **Use tools iteratively:** Use the output of one tool to inform the input of the next.
4.  **Execute commands:** Use 'execute_command' to run shell commands. Remember to properly handle background processes if needed.
5.  **Use Scripting for Complexity:** When needing to analyze many files, perform calculations, or process data, PREFER writing a Python script using 'execute_python' over making many individual tool calls. This is more efficient and reliable.
`
}
