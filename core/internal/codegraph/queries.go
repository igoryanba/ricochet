package codegraph

// Tree-sitter queries for extracting context

const GoQueries = `
(import_spec path: (interpreted_string_literal) @import_path)
(function_declaration name: (identifier) @def_name)
(method_declaration name: (field_identifier) @def_name)
(type_declaration (type_spec name: (type_identifier) @def_name))
`

const TypescriptQueries = `
(import_statement source: (string) @import_path)
(function_declaration name: (identifier) @def_name)
(class_declaration name: (type_identifier) @def_name)
(interface_declaration name: (type_identifier) @def_name)
(variable_declarator name: (identifier) @def_name)
`
