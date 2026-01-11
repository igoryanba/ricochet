import * as fs from "fs/promises"
import * as path from "path"
import { LanguageParser, loadRequiredLanguageParsers } from "./languageParser.js"
import { parseMarkdown } from "./markdownParser.js"
import { QueryCapture } from "web-tree-sitter"

const METHOD_CAPTURE = ["definition.method", "definition.method.start"]

// Private constant
const DEFAULT_MIN_COMPONENT_LINES_VALUE = 4

// Getter function for MIN_COMPONENT_LINES (for easier testing)
let currentMinComponentLines = DEFAULT_MIN_COMPONENT_LINES_VALUE

export function getMinComponentLines(): number {
    return currentMinComponentLines
}

export function setMinComponentLines(value: number): void {
    currentMinComponentLines = value
}

function shouldSkipMinLines(lineCount: number, capture: QueryCapture, language: string) {
    if (METHOD_CAPTURE.includes(capture.name)) {
        return false
    }
    return lineCount < getMinComponentLines()
}

const extensions = [
    "tla", "js", "jsx", "ts", "vue", "tsx", "py", "rs", "go", "c", "h", "cpp", "hpp", "cs", "rb", "java", "php", "swift", "sol", "kt", "kts", "ex", "exs", "el", "html", "htm", "md", "markdown", "json", "css", "rdl", "ml", "mli", "lua", "scala", "toml", "zig", "elm", "ejs", "erb", "vb"
].map((e) => `.${e}`)

export { extensions }

export async function parseSourceCodeDefinitionsForFile(
    filePath: string
): Promise<string | undefined> {
    // check if the file exists
    try {
        await fs.access(filePath);
    } catch {
        return "This file does not exist or you do not have permission to access it.";
    }

    // Get file extension to determine parser
    const ext = path.extname(filePath).toLowerCase()
    // Check if the file extension is supported
    if (!extensions.includes(ext)) {
        return undefined
    }

    // Special case for markdown files
    if (ext === ".md" || ext === ".markdown") {
        // Read file content
        const fileContent = await fs.readFile(filePath, "utf8")

        // Split the file content into individual lines
        const lines = fileContent.split("\n")

        // Parse markdown content to get captures
        const markdownCaptures = parseMarkdown(fileContent)

        // Process the captures
        const markdownDefinitions = processCaptures(markdownCaptures, lines, "markdown")

        if (markdownDefinitions) {
            return `# ${path.basename(filePath)}\n${markdownDefinitions}`
        }
        return undefined
    }

    // For other file types, load parser and use tree-sitter
    const languageParsers = await loadRequiredLanguageParsers([filePath])

    // Parse the file if we have a parser for it
    const definitions = await parseFile(filePath, languageParsers)
    if (definitions) {
        return `# ${path.basename(filePath)}\n${definitions}`
    }

    return undefined
}

/**
 * Process captures from tree-sitter or markdown parser
 */
function processCaptures(captures: QueryCapture[], lines: string[], language: string): string | null {
    // Determine if HTML filtering is needed for this language
    const needsHtmlFiltering = ["jsx", "tsx"].includes(language)

    // Filter function to exclude HTML elements if needed
    const isNotHtmlElement = (line: string): boolean => {
        if (!needsHtmlFiltering) return true
        // Common HTML elements pattern
        const HTML_ELEMENTS = /^[^A-Z]*<\/?(?:div|span|button|input|h[1-6]|p|a|img|ul|li|form)\b/
        const trimmedLine = line.trim()
        return !HTML_ELEMENTS.test(trimmedLine)
    }

    // No definitions found
    if (captures.length === 0) {
        return null
    }

    let formattedOutput = ""

    // Sort captures by their start position
    captures.sort((a, b) => a.node.startPosition.row - b.node.startPosition.row)

    // Track already processed lines to avoid duplicates
    const processedLines = new Set<string>()

    // First pass - categorize captures by type
    captures.forEach((capture) => {
        const { node, name } = capture

        // Skip captures that don't represent definitions
        if (!name.includes("definition") && !name.includes("name")) {
            return
        }

        // Get the parent node that contains the full definition
        const definitionNode = name.includes("name") ? node.parent : node
        if (!definitionNode) return

        // Get the start and end lines of the full definition
        const startLine = definitionNode.startPosition.row
        const endLine = definitionNode.endPosition.row
        const lineCount = endLine - startLine + 1

        // Skip components that don't span enough lines
        if (shouldSkipMinLines(lineCount, capture, language)) {
            return
        }

        // Create unique key for this definition based on line range
        const lineKey = `${startLine}-${endLine}`

        // Skip already processed lines
        if (processedLines.has(lineKey)) {
            return
        }

        // Check if this is a valid component definition (not an HTML element)
        const startLineContent = lines[startLine].trim()

        // Special handling for component name definitions
        if (name.includes("name.definition")) {
            // Extract component name
            const componentName = node.text

            // Add component name to output regardless of HTML filtering
            if (!processedLines.has(lineKey) && componentName) {
                formattedOutput += `${startLine + 1}--${endLine + 1} | ${lines[startLine]}\n`
                processedLines.add(lineKey)
            }
        }
        // For other component definitions
        else if (isNotHtmlElement(startLineContent)) {
            formattedOutput += `${startLine + 1}--${endLine + 1} | ${lines[startLine]}\n`
            processedLines.add(lineKey)

            // If this is part of a larger definition, include its non-HTML context
            if (node.parent && node.parent.lastChild) {
                const contextEnd = node.parent.lastChild.endPosition.row
                const contextSpan = contextEnd - node.parent.startPosition.row + 1

                // Only include context if it spans multiple lines
                if (contextSpan >= getMinComponentLines()) {
                    // Add the full range first
                    const rangeKey = `${node.parent.startPosition.row}-${contextEnd}`
                    if (!processedLines.has(rangeKey)) {
                        formattedOutput += `${node.parent.startPosition.row + 1}--${contextEnd + 1} | ${lines[node.parent.startPosition.row]}\n`
                        processedLines.add(rangeKey)
                    }
                }
            }
        }
    })

    if (formattedOutput.length > 0) {
        return formattedOutput
    }

    return null
}

async function parseFile(
    filePath: string,
    languageParsers: LanguageParser
): Promise<string | null> {
    // Read file content
    const fileContent = await fs.readFile(filePath, "utf8")
    const extLang = path.extname(filePath).toLowerCase().slice(1)

    // Check if we have a parser for this file type
    const { parser, query } = languageParsers[extLang] || {}
    if (!parser || !query) {
        return `Unsupported file type: ${filePath}`
    }

    try {
        // Parse the file content into an Abstract Syntax Tree (AST)
        const tree = parser.parse(fileContent)

        // Apply the query to the AST and get the captures
        const captures = tree ? query.captures(tree.rootNode) : []

        // Split the file content into individual lines
        const lines = fileContent.split("\n")

        // Process the captures
        return processCaptures(captures, lines, extLang)
    } catch (error) {
        console.log(`Error parsing file: ${error}\n`)
        // Return null on parsing error to avoid showing error messages in the output
        return null
    }
}
