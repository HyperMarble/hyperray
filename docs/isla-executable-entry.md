# Executable entry preservation

The ELF entry address can differ from the first loaded instruction address.
Functions before the entry retain their original bytes and addresses.
The linker must not move these functions into a thread slot.

Hyperray emits every loaded byte in an address-named section.
An empty bootstrap thread names the ELF entry through an explicit entry field.
Isla starts the machine at that entry and fetches the loaded instructions.
The compiler and the pinned machine model still define instruction behavior.

The generated-program operation requires the executable-entry tool extension.
It passes a mandatory capability flag to the semantic and solver tools.
An older tool that lacks the flag returns an error before a verdict.
Legacy hand-authored litmus programs retain their original thread entry rule.

The coverage comparison still requires the ELF entry in the execution inventory.
It still accounts for static instructions absent from a particular query.
The layout does not establish memory permissions or environment completeness.
