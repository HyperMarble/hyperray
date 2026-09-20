# Expressions

Expressions give exact values and rules to `in`, `mem`, and `req`.
They use prefix syntax so evaluation order is always clear.

## Grammar

```text
Expression        = Role | Variable | Literal | Application | Quantifier .
Application       = "(" Operation { Space Expression } ")" .
Quantifier        = "(" QuantifierWord Space "(" Binding { Space Binding } ")"
                    Space BooleanExpression ")" .
Binding           = "(" Variable Space Type ")" .
QuantifierWord    = "forall" | "exists" .
Variable          = "value" PositiveInteger .
BooleanExpression = Expression .
Role              = Argument | "ret" | "memory_before" | "memory_after" .
Argument          = "arg" PositiveInteger .
Literal           = BooleanLiteral | BinaryLiteral | HexLiteral | RegisteredLiteral .
BooleanLiteral    = "true" | "false" .
BinaryLiteral     = "#b" BinaryDigit { BinaryDigit } .
HexLiteral        = "#x" HexDigit { HexDigit } .
BinaryDigit       = "0" | "1" .
HexDigit          = DecimalDigit | "a" … "f" | "A" … "F" .
Type              = RegisteredType .
Operation         = RegisteredOperation .
```

A Boolean expression is an expression whose inferred type is `Bool`.
A `valueN` variable is legal only inside a quantifier that binds the same name.
Each binding in one quantifier must use a different name.
`PositiveInteger` and `DecimalDigit` use the lexical rules in [The `.hray` language](file.md).

## Types

The base type set comes from Isla:

- `Bool`.
- Fixed-size bit vectors.
- Enumerated values.
- Arrays.
- Floating-point values.
- Rounding modes.

Compiler metadata supplies the types of `argN` and `ret`.
The selected Isla model supplies machine-state types.

## Operations

Core Boolean operations and quantifiers follow SMT-LIB 2.7.
Machine operations come from the pinned Isla version and selected model.
The validator reads this registry instead of copying a handwritten list.
Z3-only extensions are not legal unless the pinned registry declares them.

An operation name outside the registry is an error.
An application with wrong argument types is an error.

## Literals

`RegisteredLiteral` is a named constant from the pinned Isla model registry.
Its registry entry supplies its exact type and value.
A name that is absent from that registry is not a literal.

Binary literals start with `#b`.
Hexadecimal literals start with `#x`.
Their digit count fixes their bit width.

```text
#b00000001
#x0000000000000001
```

Decimal machine literals are not legal because they do not state a width.

## Roles

`argN` means the value of argument `N` before execution.
`ret` means the return value after execution.
Memory names give the complete memory arrays before and after execution.

`memory_before` and `memory_after` are legal only when `mem` is not `none`.
No free alias is legal.
Names such as `A`, `Data`, and `Balance` are rejected.

## Example

```text
(= ret (bvadd arg1 #x0000000000000001))
```

Generated view fragment:

```text
advance(arg1) must return wrapping_add(arg1, 1)
```
