# `out`

`out` names the result that exists after the function returns.

## Syntax

```text
out none
out ret
```

A contract contains exactly one `out` line.

## Meaning

Use `out none` when the compiler reports no return value.
Use `out ret` when the compiler reports a return value.

`out` does not state what the result must contain.
A `req` line states that rule.

The compiler artifact supplies the result type and machine location.
Missing location data gives `BLOCKED`.
This covers a register, multiple registers, or an indirect return area.

## Example

```text
out ret
req (= ret (bvadd arg1 #x0000000000000001))
```

Generated view fragment:

```text
advance(arg1) must return wrapping_add(arg1, 1)
```

## Rejection

The validator rejects these cases:

- `out none` conflicts with compiler metadata.
- `out ret` conflicts with compiler metadata.
- The file contains more than one `out` line.
- The line names a register or a return-area address.
