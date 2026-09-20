# `mem`

`mem` defines caller-visible memory that the function can access.

## Syntax

```text
mem none
mem AddressExpression read SizeExpression bytes
mem AddressExpression write SizeExpression bytes
```

Use two lines when one region permits reads and writes.
A contract can contain any required number of memory lines.

## Meaning

The first expression gives the first byte address.
The second expression gives the region size in bytes.

A `read` line permits reads from that complete region.
A `write` line permits writes to that complete region.

The rule applies to every memory event in every allowed execution.
An event outside all matching regions breaks the contract.

`mem none` means that no caller-visible memory event is permitted.
Private execution memory remains part of the ISA proof.

## Example

```text
mem arg1 read arg2 bytes
```

Generated view fragment:

```text
read arg2 bytes starting at arg1
```

## Memory results

A `mem` line gives access, not the required contents.
A `req` line relates memory before execution to memory after execution.

## Rejection

The validator rejects these cases:

- An address expression is not an address-sized bit vector.
- A size expression has the wrong type.
- The access word is not `read` or `write`.
- `mem none` appears with another `mem` line.
- A memory event falls outside every declared region.
