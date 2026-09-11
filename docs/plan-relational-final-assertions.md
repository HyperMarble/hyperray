# Relational final assertions

Status: design, not implemented.

## Problem

The final-assertion grammar can compare a memory location to a literal only:

```
*lane0 = 0x00000001
```

A location cannot appear inside an expression, so a rule that relates the final
value to the initial value cannot be stated. Every assertion therefore needs
concrete expected values, and concrete expected values need concrete inputs.

This blocks symbolic inputs. With unknown inputs there is no literal to compare
against, so the whole point of symbolic execution is lost.

## Goal

State the rule once, over unknown inputs:

> For each lane, the final word equals the initial word when that word is not
> negative, and equals the arithmetic right shift by three otherwise.

## What already exists

- `Exp::App(name, args, keywords)` renders as `(name arg ...)` directly into
  SMT. Any SMT operator is therefore already reachable by name.
- `Loc::LastWriteTo { address, bytes }` resolves a final memory value.
- `initial_memory::value(address, bytes)` renders the initial value at an
  address from the loaded byte image.
- `parse_memory_observations` binds a name to an address and a byte width.

The only missing piece is a way to name a memory value inside an expression.

## Proposed contract

One new expression form. The existing `*name` keeps its meaning on the left of
an equality. A new prefix names the initial value of the same observation:

```
*lane0 = ite(bvsge(^lane0, 0x00000000), ^lane0, bvashr(^lane0, 0x00000003))
```

- `*name` is the final value at the observation, unchanged.
- `^name` is the initial value at the same observation, new.

Both use the observation's declared width. No new width rule is introduced.

`ite`, `bvsge`, and `bvashr` need no special support. They are ordinary `App`
names that SMT already defines.

## Rejected alternative

Allowing `*name` inside an expression was rejected. The same token would then
mean the final value in one position and a readable value in another, and a
reader could not tell which without knowing the position. A separate prefix
keeps one token, one meaning.

## Changes

1. `exp.rs`: add `Exp::InitialValue(A)` carrying the observation name.
2. `exp_lexer.rs`: add `^` as a token.
3. `exp_parser.lalrpop`: add `"^" <name:Id>` to `AtomicExp`.
4. `final_assertion.rs`: render `InitialValue` through
   `initial_memory::value` at the observation's address and width.
5. Resolution: reject a name that is not a declared observation, with the
   name in the message. An unresolved name must never become a true assertion.

## Width contract

`initial_memory::value` and `LastWriteTo` both yield the observation width.
The `last_write_to_N` predicates take a 64-bit value, so the relational form
reuses the existing widening step rather than adding a second rule.

## Tests

Each test states its expected result before it runs.

1. `^lane0` on an undeclared name returns an error naming the name.
2. `*lane0 = ^lane0` on a program that copies its input proves.
3. The Leaky ReLU rule above proves on the real fixture with unknown inputs.
4. The same rule with the shift changed to four is disproved, and the
   counterexample names a negative lane.
5. A program that writes a constant is disproved against the copy rule.

Test 4 is the negative control. Without it, a vacuous assertion would pass.

## Out of scope

Symbolic input declaration is a separate change. It splits the `backing` field,
which currently both declares a readable region and supplies its bytes. This
design assumes that split has landed, and states the rule only.
