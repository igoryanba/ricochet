package prompts

func GetCapabilities() string {
	return `====
CAPABILITIES

- You can access the user's filesystem to list, read, write, and delete files.
- You can execute terminal commands on the user's system (e.g., git, npm, go, grep).
- You can analyze project structure and dependencies.
- You can run builds and tests to verify your work.
`
}
