# `mem`

`mem` defines caller-visible memory that the function can access.

## Syntax

```text
(mem none)
(mem AddressExpression read SizeExpression bytes)
(mem AddressExpression write SizeExpression bytes)
```

Use two sections when one region permits reads and writes.
A contract can contain any required number of memory sections.

## Meaning

The first expression gives the first byte address.
The second expression gives the region size in bytes.

A `read` section permits reads from that complete region.
A `write` section permits writes to that complete region.

The rule applies to every memory event in every allowed execution.
An event outside all matching regions breaks the contract.

`mem none` means that no caller-visible memory event is permitted.
Private execution memory remains part of the ISA proof.

## Example

```text
(mem arg1 read arg2 bytes)
```

Generated view fragment:

```text
read arg2 bytes starting at arg1
```

## Memory results

A `mem` section gives access, not the required contents.
A `req` section relates memory before execution to memory after execution.

## Rejection

The validator rejects these cases:

- An address expression is not an address-sized bit vector.
- A size expression has the wrong type.
- The access word is not `read` or `write`.
- `(mem none)` appears with another `mem` section.
- A memory event falls outside every declared region.
