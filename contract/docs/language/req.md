# `req`

`req` states one rule that must hold for every allowed execution.

## Syntax

```text
(req BooleanExpression)
```

A contract contains one or more `req` sections.
Every requirement must be true.

## Universal meaning

Let `I` mean all `in` rules.
Let `E` mean an execution permitted by the selected Isla model and installed OS contracts.
Let `R` mean all `req` rules joined with `and`.

The contract means:

```text
for every execution: if I and E are true, R must be true
```

Hyper-Ray searches for the opposite case:

```text
I and E and not R
```

If Z3 returns `sat`, Hyper-Ray gives `DISPROVED` and a counterexample.
If Z3 returns `unsat`, Hyper-Ray gives `PROVED`.
A timeout, `unknown`, or an unsupported operation is an engine error and
produces no verdict.

This search covers every allowed input and starting state in the selected execution scope.
The current engine scope is supported ARM64 code.
The compiler target, Isla model, and engine define that scope.
It does not claim support for another architecture or an unsupported code shape.
It does not run a fixed list of examples.

Every allowed execution must return normally.
A reachable trap or abnormal exit gives `DISPROVED`.
If Hyper-Ray cannot decide whether an execution returns, it produces an engine
error and no verdict.

## Example

```text
(req (= ret (bvadd arg1 #x0000000000000001)))
```

Generated one-line view:

```text
for every arg1 in u64: advance(arg1) must return wrapping_add(arg1, 1)
```

## Rejection

The validator rejects these cases:

- The expression does not have type `Bool`.
- The expression uses an unknown role.
- The expression uses an operation outside the selected registry.
- The file has no `req` section.
