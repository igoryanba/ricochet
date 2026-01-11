import * as path from "path"
import { Parser as ParserT, Language as LanguageT, Query as QueryT } from "web-tree-sitter"
import {
    javascriptQuery,
    typescriptQuery,
    tsxQuery,
    pythonQuery,
    rustQuery,
    goQuery,
    cppQuery,
    cQuery,
    csharpQuery,
    rubyQuery,
    javaQuery,
    phpQuery,
    htmlQuery,
    swiftQuery,
    kotlinQuery,
    cssQuery,
    ocamlQuery,
    solidityQuery,
    tomlQuery,
    vueQuery,
    luaQuery,
    systemrdlQuery,
    tlaPlusQuery,
    zigQuery,
    embeddedTemplateQuery,
    elispQuery,
    elixirQuery,
} from "./queries"

export interface LanguageParser {
    [key: string]: {
        parser: ParserT
        query: QueryT
    }
}

async function loadLanguage(langName: string, sourceDirectory?: string) {
    // If sourceDirectory is provided, use it. Otherwise, try to resolve from tree-sitter-wasms package
    let wasmPath: string;
    if (sourceDirectory) {
        wasmPath = path.join(sourceDirectory, `tree-sitter-${langName}.wasm`)
    } else {
        // Fallback or development mode: try to find in node_modules/tree-sitter-wasms/out
        try {
            const wasmPkg = require.resolve('tree-sitter-wasms');
            const wasmDir = path.dirname(wasmPkg);
            wasmPath = path.join(wasmDir, `tree-sitter-${langName}.wasm`);
        } catch (e) {
            // If we can't find it via require (e.g. bundled), fallback to __dirname
            wasmPath = path.join(__dirname, `tree-sitter-${langName}.wasm`);
        }
    }

    try {
        const { Language } = require("web-tree-sitter")
        return await Language.load(wasmPath)
    } catch (error) {
        console.error(`Error loading language: ${wasmPath}: ${error instanceof Error ? error.message : error}`)
        throw error
    }
}

let isParserInitialized = false

export async function loadRequiredLanguageParsers(filesToParse: string[], sourceDirectory?: string) {
    const { Parser, Query } = require("web-tree-sitter")

    if (!isParserInitialized) {
        try {
            await Parser.init()
            isParserInitialized = true
        } catch (error) {
            console.error(`Error initializing parser: ${error instanceof Error ? error.message : error}`)
            throw error
        }
    }

    const extensionsToLoad = new Set(filesToParse.map((file) => path.extname(file).toLowerCase().slice(1)))
    const parsers: LanguageParser = {}

    for (const ext of extensionsToLoad) {
        let language: LanguageT
        let query: QueryT
        let parserKey = ext // Default to using extension as key

        switch (ext) {
            case "js":
            case "jsx":
            case "json":
                language = await loadLanguage("javascript", sourceDirectory)
                query = new Query(language, javascriptQuery)
                break
            case "ts":
                language = await loadLanguage("typescript", sourceDirectory)
                query = new Query(language, typescriptQuery)
                break
            case "tsx":
                language = await loadLanguage("tsx", sourceDirectory)
                query = new Query(language, tsxQuery)
                break
            case "py":
                language = await loadLanguage("python", sourceDirectory)
                query = new Query(language, pythonQuery)
                break
            case "rs":
                language = await loadLanguage("rust", sourceDirectory)
                query = new Query(language, rustQuery)
                break
            case "go":
                language = await loadLanguage("go", sourceDirectory)
                query = new Query(language, goQuery)
                break
            case "cpp":
            case "hpp":
                language = await loadLanguage("cpp", sourceDirectory)
                query = new Query(language, cppQuery)
                break
            case "c":
            case "h":
                language = await loadLanguage("c", sourceDirectory)
                query = new Query(language, cQuery)
                break
            case "cs":
                language = await loadLanguage("c_sharp", sourceDirectory)
                query = new Query(language, csharpQuery)
                break
            case "rb":
                language = await loadLanguage("ruby", sourceDirectory)
                query = new Query(language, rubyQuery)
                break
            case "java":
                language = await loadLanguage("java", sourceDirectory)
                query = new Query(language, javaQuery)
                break
            case "php":
                language = await loadLanguage("php", sourceDirectory)
                query = new Query(language, phpQuery)
                break
            case "swift":
                language = await loadLanguage("swift", sourceDirectory)
                query = new Query(language, swiftQuery)
                break
            case "kt":
            case "kts":
                language = await loadLanguage("kotlin", sourceDirectory)
                query = new Query(language, kotlinQuery)
                break
            case "css":
                language = await loadLanguage("css", sourceDirectory)
                query = new Query(language, cssQuery)
                break
            case "html":
                language = await loadLanguage("html", sourceDirectory)
                query = new Query(language, htmlQuery)
                break
            case "ml":
            case "mli":
                language = await loadLanguage("ocaml", sourceDirectory)
                query = new Query(language, ocamlQuery)
                break
            case "scala":
                language = await loadLanguage("scala", sourceDirectory)
                query = new Query(language, luaQuery) // Temporarily use Lua query until Scala is implemented
                break
            case "sol":
                language = await loadLanguage("solidity", sourceDirectory)
                query = new Query(language, solidityQuery)
                break
            case "toml":
                language = await loadLanguage("toml", sourceDirectory)
                query = new Query(language, tomlQuery)
                break
            case "vue":
                language = await loadLanguage("vue", sourceDirectory)
                query = new Query(language, vueQuery)
                break
            case "lua":
                language = await loadLanguage("lua", sourceDirectory)
                query = new Query(language, luaQuery)
                break
            case "rdl":
                language = await loadLanguage("systemrdl", sourceDirectory)
                query = new Query(language, systemrdlQuery)
                break
            case "tla":
                language = await loadLanguage("tlaplus", sourceDirectory)
                query = new Query(language, tlaPlusQuery)
                break
            case "zig":
                language = await loadLanguage("zig", sourceDirectory)
                query = new Query(language, zigQuery)
                break
            case "ejs":
            case "erb":
                parserKey = "embedded_template" // Use same key for both extensions.
                language = await loadLanguage("embedded_template", sourceDirectory)
                query = new Query(language, embeddedTemplateQuery)
                break
            case "el":
                language = await loadLanguage("elisp", sourceDirectory)
                query = new Query(language, elispQuery)
                break
            case "ex":
            case "exs":
                language = await loadLanguage("elixir", sourceDirectory)
                query = new Query(language, elixirQuery)
                break
            default:
                // Skip unsupported
                continue;
        }

        const parser = new Parser()
        parser.setLanguage(language)
        parsers[parserKey] = { parser, query }
    }

    return parsers
}
