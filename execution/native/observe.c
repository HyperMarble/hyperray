/* Observe requirement results for generated native inputs.
 * Diagnostic call counts must not become independent coverage certificates. */
#include "bridge.h"
#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>

static uint64_t observations;
static int overflow;

static void report(void) {
  printf("OBSERVATIONS count=%" PRIu64 " overflow=%d\n", observations,
         overflow);
}

int begin(void) { return atexit(report) == 0; }

static uint64_t offset_value(const unsigned char *digits) {
  uint64_t offset = 0;
  for (unsigned int index = 0; index < sizeof(uint64_t); index++) {
    offset |= (uint64_t)digits[index] << (index * 8);
  }
  return offset;
}

int observe_value(uint64_t input) {
  uint64_t output = hyperray_subject(input);
  int accepted = hyperray_requirement(input, output);
  if (observations == UINT64_MAX) {
    overflow = 1;
  }
  observations++;
  if (!accepted) {
    printf("COUNTEREXAMPLE input=%" PRIu64 " output=%" PRIu64 "\n", input,
           output);
  }
  return accepted;
}

int observe(const unsigned char *digits) {
  uint64_t offset = offset_value(digits);
  if (offset > INPUT_MAX - INPUT_MIN) {
    return 1;
  }
  return observe_value(INPUT_MIN + offset);
}
