import typescriptQuery from "./typescript"

export default `\${typescriptQuery}

(function_declaration
  name: (identifier) @name) @definition.component

(variable_declaration
  (variable_declarator
    name: (identifier) @name
    value: (arrow_function))) @definition.component

(export_statement
  (variable_declaration
    (variable_declarator
      name: (identifier) @name
      value: (arrow_function)))) @definition.component

(class_declaration
  name: (type_identifier) @name) @definition.class_component

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(type_alias_declaration
  name: (type_identifier) @name) @definition.type

(variable_declaration
  (variable_declarator
    name: (identifier) @name
    value: (call_expression
      function: (identifier)))) @definition.component

(jsx_element
  open_tag: (jsx_opening_element
    name: [(identifier) @component (member_expression) @component])) @definition.jsx_element

(jsx_self_closing_element
  name: [(identifier) @component (member_expression) @component]) @definition.jsx_self_closing_element

(jsx_expression
  (identifier) @jsx_component) @definition.jsx_component

(member_expression
  object: (identifier) @object
  property: (property_identifier) @property) @definition.member_component

(ternary_expression
  consequence: (parenthesized_expression
    (jsx_element
      open_tag: (jsx_opening_element
        name: (identifier) @component)))) @definition.conditional_component

(ternary_expression
  alternative: (jsx_self_closing_element
    name: (identifier) @component)) @definition.conditional_component

(function_declaration
  name: (identifier) @name
  type_parameters: (type_parameters)) @definition.generic_component
`
