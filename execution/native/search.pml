/* Explore the declared native input interval through bit choices.
 * No subject instruction or scheduler semantics belong in this template. */
#include "bounds.h"
c_decl {
    int begin(void);
    int observe(const unsigned char *digits);
}
byte digits[8];
byte position;
bool accepted;
init {
    c_code { now.accepted = begin(); };
    assert(accepted);
    do
    :: position < INPUT_BITS ->
        if
        :: skip
        :: digits[position / 8] = digits[position / 8] | (1 << (position % 8))
        fi;
        position++
    :: else -> break
    od;
    c_code { now.accepted = observe(now.digits); };
    assert(accepted)
}
