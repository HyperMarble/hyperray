# `fn`

`fn` selects the function that the contract describes.

## Syntax

```text
fn FunctionName
```

A contract contains exactly one `fn` line.
It follows the `hray` header.

## Meaning

`FunctionName` must select exactly one function from compiler metadata.
The name is not a code identity.
Code identity is outside the `.hray` language.

The compiler supplies the legal names.
The contract writer cannot create an alias.

## Example

```text
fn advance
```

Generated view fragment:

```text
advance(...)
```

## Rejection

The validator rejects these cases:

- The name does not exist.
- The name selects more than one function.
- The line contains a wildcard.
- The file contains more than one `fn` line.
