# The `.hray` language

This reference defines the syntax and meaning of one `.hray` contract.
The complete file is one SMT-LIB term.

## Source text

A `.hray` file is UTF-8 text. Its root operation is `hray`.
Each direct child is one contract section.
SMT-LIB handles symbols, literals, expressions, nesting, and whitespace.

## Section order

Every contract uses this order:

```text
fn, in, out, mem, os, req
```

These six names are the complete fixed section set.
Registry updates cannot add a seventh section.
`fn` and `out` occur once. `in`, `mem`, `os`, and `req` can repeat.
A file can therefore contain more than six section entries.
A section with no items uses `none`.

## Grammar

```text
Contract       = "(" "hray" Function Inputs Output Memory OS Requirements ")" .
Function       = "(" "fn" FunctionName ")" .
Inputs         = "(" "in" "none" ")"
               | Input { Input } { InputRule } .
Input          = "(" "in" Argument "all" ")" .
InputRule      = "(" "in" "when" BooleanExpression ")" .
Output         = "(" "out" ( "none" | "ret" ) ")" .
Memory         = "(" "mem" "none" ")"
               | MemoryAccess { MemoryAccess } .
MemoryAccess   = "(" "mem" AddressExpression Access SizeExpression "bytes" ")" .
Access         = "read" | "write" .
OS             = "(" "os" "none" ")"
               | OSCall { OSCall } .
OSCall         = "(" "os" InterfaceName OperationName ")" .
Requirements   = Requirement { Requirement } .
Requirement    = "(" "req" BooleanExpression ")" .
Argument       = "arg" PositiveInteger .
PositiveInteger = NonzeroDigit { DecimalDigit } .
DecimalDigit   = "0" … "9" .
NonzeroDigit   = "1" … "9" .
```

Adjacent grammar items use SMT-LIB token boundaries.
Whitespace and indentation do not change the parsed contract.
Names use SMT-LIB symbol syntax.
Expressions use the rules in [Expressions](expressions.md).

## Complete example

```text
(hray
  (fn advance)
  (in arg1 all)
  (out ret)
  (mem none)
  (os none)
  (req (= ret (bvadd arg1 #x0000000000000001))))
```

Generated view:

```text
for every arg1 in u64: advance(arg1) must return wrapping_add(arg1, 1)
```

## Parsing and validation

The parser calls the pinned SMT-LIB parser once for the complete file.
It preserves each section value as an SMT syntax tree.
Syntax errors name an exact source position.
Structure errors name the root or direct section index.

The validator checks section order, section contents, names, types, and roles.
It reads compiler and model metadata instead of guessing missing facts.
