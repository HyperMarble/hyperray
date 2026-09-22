# The `.hray` language

The canonical language reference is in [`language/file.md`](language/file.md).
It follows the structure of official Go, Python, Rust, and SMT-LIB references.

The reference has one document for each language part:

- [`fn`](language/fn.md).
- [`in`](language/in.md).
- [`out`](language/out.md).
- [`mem`](language/mem.md).
- [`os`](language/os.md).
- [`req`](language/req.md).
- [Expressions](language/expressions.md).
- [English view](language/english-view.md).

The English view is one generated line of Python-level pseudocode.
The `.hray` file remains the only source for a proof.

The complete `.hray` file is one SMT-LIB-shaped term.
The pinned SMT parser reads the whole file in one operation.

Hashes, patches, batches, and caches are not language features.
The reliability layer documents those items separately.
