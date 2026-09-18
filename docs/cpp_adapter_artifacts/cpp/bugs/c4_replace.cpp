extern "C" unsigned long nondet_ulong();
static const unsigned long CAP = 65536, STEP = 8192;
unsigned long scratch_len(unsigned long d) {
    __ESBMC_requires(d <= CAP);
    __ESBMC_ensures(__ESBMC_return_value <= STEP &&
                    __ESBMC_return_value + d <= CAP);
    if (d >= CAP) return 0;
    unsigned long r = CAP - d;
    return r < STEP ? r : STEP;
}
int main(){
    unsigned long d = nondet_ulong();
    __ESBMC_assume(d <= CAP);
    unsigned long r = scratch_len(d);
    __ESBMC_assert(r + d <= CAP, "caller sees cumulative bound");
    return 0;
}
