# The `.hray` language

This reference defines the syntax and meaning of one `.hray` contract.
It follows the structure of the Go, Python, Rust, and SMT-LIB references.

## Reference structure

This language reference follows these official references:

- [The Go Programming Language Specification](https://go.dev/ref/spec).
- [The Python Language Reference](https://docs.python.org/3/reference/).
- [The Rust Reference](https://doc.rust-lang.org/reference/).
- [The SMT-LIB Standard](https://smt-lib.org/language.shtml).

## Notation

The grammar uses Extended Backus-Naur Form.

```text
Rule       = required item .
[ Rule ]   = optional item .
{ Rule }   = zero or more items .
"word"     = exact text .
```

A space in a grammar rule means one ASCII space.
A newline means LF.

## Source text

A `.hray` file is UTF-8 text.
It contains no comments and no free English text.
Each statement occupies one line.
Names use SMT-LIB symbol syntax.

## File order

Every file uses this order:

```text
hray
fn
in
out
mem
os
req
```

A missing section is an error.
A section that has no items uses `none`.

## Grammar

```text
Contract       = "hray" Newline Function Inputs Output Memory OS Requirements EOF .
Function       = "fn" Space FunctionName Newline .
Inputs         = "in" Space "none" Newline
               | Input { Input } { InputRule } .
Input          = "in" Space Argument Space "all" Newline .
InputRule      = "in" Space "when" Space BooleanExpression Newline .
Output         = "out" Space ( "none" | "ret" ) Newline .
Memory         = "mem" Space "none" Newline
               | MemoryAccess { MemoryAccess } .
MemoryAccess   = "mem" Space AddressExpression Space Access Space SizeExpression Space "bytes" Newline .
Access         = "read" | "write" .
OS             = "os" Space "none" Newline
               | OSCall { OSCall } .
OSCall         = "os" Space InterfaceName Space OperationName Newline .
Requirements   = Requirement { Requirement } .
Requirement    = "req" Space BooleanExpression Newline .
Argument       = "arg" PositiveInteger .
AddressExpression = Expression .
SizeExpression = Expression .
PositiveInteger = NonzeroDigit { DecimalDigit } .
DecimalDigit   = "0" … "9" .
NonzeroDigit   = "1" … "9" .
```

`Space` is U+0020. `Newline` is U+000A. `EOF` is the end of the file.
`FunctionName`, `InterfaceName`, and `OperationName` use SMT-LIB symbols.
Their values must come from installed metadata.
The language does not create these names.

## Complete example

```text
hray
fn advance
in arg1 all
out ret
mem none
os none
req (= ret (bvadd arg1 #x0000000000000001))
```

Generated view:

```text
for every arg1 in u64: advance(arg1) must return wrapping_add(arg1, 1)
```

## Rejection

The validator rejects unknown words, missing sections, repeated `none`, and statements in the wrong order.
It also rejects names and types that the compiler or selected model does not define.
