/* The fixture uses the Isla thread address and an explicit return point.
 * The empty return point is an Isla boundary, not a process exit syscall. */
ENTRY(_start)
PHDRS
{
  text PT_LOAD FLAGS(5);
  readonly PT_LOAD FLAGS(4);
  data PT_LOAD FLAGS(6);
}
SECTIONS
{
  . = 0x80100000;
  .text : ALIGN(2) { *(.text .text.*) } :text
  __hyperray_return = .;
  . = ALIGN(4096);
  .rodata : { *(.rodata .rodata.*) *(.srodata .srodata.*) } :readonly
  . = ALIGN(4096);
  .data : { *(.data .data.*) *(.sdata .sdata.*) } :data
  .bss : { *(.bss .bss.*) *(.sbss .sbss.*) } :data
  /DISCARD/ : { *(.comment) *(.eh_frame*) }
}
