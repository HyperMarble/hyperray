# `in`

`in` defines the allowed input set before the function starts.

## Syntax

```text
in none
in argN all
in when BooleanExpression
```

Use `in none` only when the function has no inputs.
Otherwise, each compiler-defined argument appears once as `in argN all`.

An `in when` line narrows the allowed set.
All `in when` lines must be true together.
An input rule can use `argN` and `memory_before` only.
It cannot use `ret` or `memory_after`.

## Universal meaning

`all` means every value of the compiler-defined type.
It never means one selected example.

For a condition `P`, Hyper-Ray uses every input for which `P` is true.
It does not choose a sample from that set.

```text
in arg1 all
in arg2 all
in when (bvule arg2 arg1)
```

This input set contains every pair where `arg2` is not greater than `arg1`.

Generated view fragment:

```text
for every arg1 and arg2 where arg2 <= arg1
```

## Types and locations

The compiler artifact supplies each argument type and machine location.
The artifact can use DWARF or another compiler-produced record.
Missing location data gives `BLOCKED`.
The contract never names a register, stack offset, or source-language alias.

## Rejection

The validator rejects these cases:

- An argument is missing.
- An argument occurs more than once.
- The argument number does not exist.
- An `in when` expression does not have type `Bool`.
- An `in when` expression uses `ret` or `memory_after`.
- `in none` appears with another `in` line.
