// unbounded while: guard depends on nondet input, no syntactic bound
#include <cstddef>
extern "C" unsigned nondet_uint();
int main() {
    unsigned n = nondet_uint();
    unsigned i = 0, s = 0;
    while (i < n) { i++; s += 2; }
    __ESBMC_assert(s == 2*i, "s == 2i");   // true invariant, needs induction
    return 0;
}
