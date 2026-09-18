// b2: buffer index out of range on a raw array
#include <cstddef>
extern "C" unsigned nondet_uint();
int main() {
    int buf[8];
    unsigned i = nondet_uint();
    __ESBMC_assume(i <= 8);          // <= 8 permits i==8 : one past the end
    buf[i] = 42;
    return buf[0];
}
