extern "C" unsigned nondet_uint();
int main() {
    unsigned n = nondet_uint();
    unsigned i = 0, s = 0;
    __ESBMC_loop_invariant(s == 2*i);
    while (i < n) { i++; s += 2; }
    __ESBMC_assert(s == 2*i, "s == 2i");
    return 0;
}
