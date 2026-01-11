import { QueryCapture } from "web-tree-sitter"

/**
 * Parses markdown content and extracts headers as captures
 * @param content The markdown content
 * @returns Array of QueryCapture objects
 */
export function parseMarkdown(content: string): QueryCapture[] {
    const captures: QueryCapture[] = []
    const lines = content.split("\n")

    lines.forEach((line, rowIndex) => {
        const match = line.match(/^(#{1,6})\s+(.+)$/)
        if (match) {
            const level = match[1].length
            const text = match[2]

            captures.push({
                name: "definition.header",
                node: {
                    type: "header",
                    text: line,
                    startPosition: { row: rowIndex, column: 0 },
                    endPosition: { row: rowIndex, column: line.length },
                    parent: null,
                    children: [],
                    child: () => null,
                    namedChildren: [],
                    namedChild: () => null,
                    childCount: 0,
                    namedChildCount: 0,
                    firstChild: null,
                    lastChild: null,
                    firstNamedChild: null,
                    lastNamedChild: null,
                    nextSibling: null,
                    previousSibling: null,
                    nextNamedSibling: null,
                    previousNamedSibling: null,
                    id: 0,
                    tree: null as any,
                    walk: () => null as any,
                    toString: () => "",
                    descendantForIndex: () => null as any,
                    descendantForPosition: () => null as any,
                    namedDescendantForIndex: () => null as any,
                    namedDescendantForPosition: () => null as any,
                    hasChanges: () => false,
                    hasError: () => false,
                    isMissing: () => false,
                    isNamed: () => true,
                },
            })
        }
    })

    return captures
}
