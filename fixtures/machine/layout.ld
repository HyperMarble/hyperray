/* This layout makes separate executable and writable load segments.
 * It must not place file headers in the executable segment. */
ENTRY(_start)

PHDRS
{
  text PT_LOAD FLAGS(5);
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
