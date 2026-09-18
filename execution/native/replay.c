/* Replay one declared input through the generated native connection.
 * Invalid input and a requirement violation have separate exit codes. */
#include "bridge.h"
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(int count, char **arguments) {
  if (count != 2 || arguments[1][0] == '\0' ||
      strspn(arguments[1], "0123456789") != strlen(arguments[1])) {
    fputs("usage: replay DECIMAL_INPUT\n", stderr);
    return 2;
  }
  char *end;
  errno = 0;
  uint64_t input = strtoull(arguments[1], &end, 10);
  if (errno != 0 || *end != '\0' || input < INPUT_MIN || input > INPUT_MAX) {
    fputs("input outside the declared interval\n", stderr);
    return 2;
  }
  if (!begin()) {
    fputs("cannot register observation report\n", stderr);
    return 2;
  }
  return observe_value(input) ? 0 : 1;
}
