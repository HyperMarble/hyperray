/* Connect the native subject, separate requirement, and input observations.
 * This ABI must not supply language behavior or a correctness verdict. */
#ifndef HYPERRAY_PREPARED_BRIDGE_H
#define HYPERRAY_PREPARED_BRIDGE_H
#include "bounds.h"
#include <stdint.h>
uint64_t hyperray_subject(uint64_t input);
int hyperray_requirement(uint64_t input, uint64_t output);
int begin(void);
int observe_value(uint64_t input);
int observe(const unsigned char *digits);
#endif
