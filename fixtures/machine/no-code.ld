/* This layout produces an ELF without an executable load segment.
 * It must exist only as an adversarial loader fixture. */
ENTRY(_start)

PHDRS
{
  text PT_LOAD FLAGS(4);
  data PT_LOAD FLAGS(6);
}

SECTIONS
{
  . = 0x10000;
  .text : ALIGN(2) { *(.text .text.*) } :text

  . = 0x20000;
  .data : { *(.sdata .sdata.*) *(.data .data.*) } :data
  .bss : { *(.sbss .sbss.*) *(.bss .bss.*) } :data

  /DISCARD/ : { *(.comment) *(.eh_frame*) }
}
