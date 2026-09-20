# `os`

`os` names the operating-system operations that the function can use.

## Syntax

```text
os none
os InterfaceName OperationName
```

A contract can contain more than one `os` call.

## Meaning

`InterfaceName` selects a published interface.
`OperationName` selects one operation from that interface.

The installed interface supplies the parameter and result types.
A separate formal operation contract supplies behavior.
Hyper-Ray returns `BLOCKED` when that behavior contract is absent.
The `.hray` file does not repeat this information.

Examples of interfaces include WASI and native operating-system interfaces.
The selected target decides which installed interfaces are legal.

`os none` means that no operating-system call is permitted.
Internal function calls do not use `os`.

## Example

```text
os wasi_snapshot_preview1 fd_read
```

Generated view fragment:

```text
can call wasi_snapshot_preview1.fd_read
```

## Rejection

The validator rejects these cases:

- The interface is not installed.
- The operation is not part of the interface.
- The operation conflicts with the compiled target.
- `os none` appears with another `os` line.
- Execution uses an operation that the contract does not name.
