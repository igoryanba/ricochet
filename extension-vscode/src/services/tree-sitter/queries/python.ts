export default `
(class_definition
  name: (identifier) @name.definition.class) @definition.class

(decorated_definition
  definition: (class_definition
    name: (identifier) @name.definition.class)) @definition.class

(function_definition
  name: (identifier) @name.definition.function) @definition.function

(decorated_definition
  definition: (function_definition
    name: (identifier) @name.definition.function)) @definition.function

(expression_statement
  (assignment
    left: (identifier) @name.definition.lambda
    right: (parenthesized_expression
      (lambda)))) @definition.lambda

(function_definition
  name: (identifier) @name.definition.generator
  body: (block
    (expression_statement
      (yield)))) @definition.generator

(expression_statement
  (assignment
    left: (identifier) @name.definition.comprehension
    right: [
      (list_comprehension)
      (dictionary_comprehension)
      (set_comprehension)
    ])) @definition.comprehension

(with_statement) @definition.with_statement
(try_statement) @definition.try_statement
(import_from_statement) @definition.import
(import_statement) @definition.import

(function_definition
  body: (block
    [(global_statement) (nonlocal_statement)])) @definition.scope

(function_definition
  body: (block
    (match_statement))) @definition.match_case

(typed_parameter
  type: (type)) @definition.type_annotation

(expression_statement
  (assignment
    left: (identifier) @name.definition.type
    type: (type))) @definition.type_annotation
`
