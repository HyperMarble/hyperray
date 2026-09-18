# BuildARM64Program implementation contract

Status: active production implementation, 2026-09-09.

## Delivery

Implement the existing approved BuildARM64Program API, not a stub.
One worker owns machine/arm64 loading. Another owns machine/isla construction.
The first acceptance document remains the public boundary contract.
Each worker preserves RISC-V and unrelated staged work. No push yet.

## Loader API and ownership

Add arm64.FunctionBoundary {StartAddress uint64; EndAddress uint64}.
Add arm64.LoadFunction(content []byte, maximumLoadedBytes uint64,
boundary FunctionBoundary) (machine.Image, error).
Use a distinct arm64.ProfileName for static little-endian ARM64 Mach-O.
ELFFlags remains zero because Mach-O has no ELF flags. ArtifactSHA256 is
computed from original content. EntryAddress is the explicit function start.
The returned image retains ALL admitted bytes and code, not just the function.
The boundary declares an analysis region, not an inferred symbol extent.

Compose existing header, command, segment, section, permission and flag APIs.
Before allocation, validate total materialized Memsz against caller capacity,
segment VM overlaps including PAGEZERO, and ambiguous file-backed overlaps.
Require one header-bearing __TEXT with fileoff zero and header/table inside its
file-backed range. Slide is zero. Keep __PAGEZERO inaccessible, empty of file
bytes and sections, and with zero protections. Do not materialize PAGEZERO.
Map Filesz original bytes at Addr and zero-fill through Memsz. Preserve
permissions. Sort by virtual address after rejecting overlaps.

For the initial closed loader, command admission is explicit and conservative.
Admit LC_SEGMENT_64, shape-valid LC_UUID, LC_SOURCE_VERSION, LC_SYMTAB, and
LC_UNIXTHREAD ARM_THREAD_STATE64 only. Every other command returns a named
unsupported-command error. This is a scoped initial profile, not a claim that
other Mach-O commands are invalid. Do not silently skip dynamic obligations.
Use SDK command IDs and exact structure sizes, not invented values.
SYMTAB requires bounded nlist_64 and string ranges, terminated names, and no
undefined or indirect runtime symbols. Validate N_SECT section ordinals.
UNIXTHREAD requires exact flavor/count/state framing. It is retained file
metadata, never the requested process state or function boundary.
No production debug/macho.NewFile until nested preflight is actually complete.

Validate section ownership, address/file correspondence, alignment with safe
shift bounds, range containment and overlap. Reject relocations and unsupported
section types. Initial supported types are regular and zero-fill forms with
source-backed semantics. Reject mixed instruction/data and runtime section
obligations. Pure-instruction regular sections in executable segments form
4-byte encodings. Headers, padding, data and LINKEDIT are not instructions.
Require a nonempty aligned half-open function boundary entirely in admitted
contiguous code. Every failure returns a typed rejection and zero image.

Actual fixture from otool on 2026-09-09: __TEXT Memsz=16384, Filesz=16384;
__LINKEDIT Memsz=16384, Filesz=72; __PAGEZERO Memsz=4294967296.
Thus loaded bytes total 32768, not the 16456-byte artifact size. The test
capacity must permit 32768 bytes and must reject 32767. No fake omission of
LINKEDIT zero-fill to fit artifact length is permitted.

## Builder API and ownership

BuildARM64Program(content []byte, maximumLoadedBytes uint64,
boundary ARM64ProgramBoundary) (Program,error), with the approved fields from
plan-arm64-first-execution-acceptance.md. Call arm64.LoadFunction.
Validate finite text/size limits, boundary alignment, return address outside
all loaded ranges, no entry/return collision, and canonical post-reset register
names and values. Require R30 to equal the declared return address. Reject
post-reset PC overrides that conflict with the chosen entry.

Render AArch64, explicit thread entry, native thread reset syntax from the
actual parser, return_address, and data-only sections preserving every loaded
byte at its virtual address. Reuse existing section output and size limits.
Do not rename RISCV output as ARM or rewrite instruction bytes.
Return existing opaque Program with original image identity, ordered thread
identity, complete code inventory, and explicit immutable architecture/profile
and return-boundary metadata. Content and evidence access remain public.
RISC-V BuildProgram output and behavior must remain unchanged.

The builder does not claim native execution support from generated text alone.
Until the measured native release supports ARM memory, return interception,
and terminal transport, VerifyProgram must reject the ARM profile explicitly
before invoking a RISC-V footprint/execution route. Do not add dummy terminal
rows or success fallbacks. This temporary visible rejection is a remaining
execution prerequisite, not completion of the end-to-end acceptance.

## Tests and review

Loader: real checked-in Mach-O positive, exact digest, all 32768 loaded bytes,
zero-fill and permissions, exactly two code encodings, capacity boundary,
wrong function regions, malformed command/section/overlap/runtime negatives.
Builder: external public construction, AArch64 and reset/return rendering,
identity and copy independence, invalid register/boundary/size cases, explicit
unsupported native profile before invocation, unchanged RISC-V regressions.
No fixture addresses or values in production logic.

Run red-first focused tests then go test -count=1 ./machine/...
./coverage/compilercatalog and go vet for the same packages. Record commands,
exits, logs and exact frozen hashes. Independent review is mandatory. Keep the
full ARM acceptance red until real native behavior and evidence are connected.
